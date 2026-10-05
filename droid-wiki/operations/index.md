# 工程与运维

Multica 的运行面由 Go API、Next.js Web、PostgreSQL、可选 Redis、附件存储以及用户机器上的 CLI/守护进程组成。工程侧同时维护两种环境：

- **开发环境**：一个 checkout 对应一条登记记录、独立端口、独立数据库、独立 CLI profile 和 Desktop `userData`。入口是 [`Makefile`](../../Makefile) 的 `make up/status/down/destroy`，生命周期实现位于 [`scripts/dev-env.sh`](../../scripts/dev-env.sh)。
- **部署环境**：Docker Compose 或 Helm 先执行数据库迁移，再启动 API；Web 通过运行时上游地址访问 API。入口分别是 [`docker-compose.selfhost.yml`](../../docker-compose.selfhost.yml)、[`docker/entrypoint.sh`](../../docker/entrypoint.sh) 和 [`deploy/helm/multica/`](../../deploy/helm/multica/)。

## 页面导航

| 目标 | 页面 |
| --- | --- |
| 找变量、确认配置优先级与重启要求 | [配置参考](configuration.md) |
| 启动 checkout、管理 worktree 和数据库 | [本地开发](local-development.md) |
| Compose、Helm、对象存储和多副本 | [部署与自托管](deployment-and-self-hosting.md) |
| 本地验证、GitHub Actions 和发布产物 | [CI、测试与发布](ci-testing-and-release.md) |
| 健康检查、指标、日志和故障定位 | [可观测性与故障排查](observability-and-troubleshooting.md) |

## 运行依赖

| 依赖 | 版本/要求 | 是否必需 | 说明 |
| --- | --- | --- | --- |
| Node.js | 22；见 [`.nvmrc`](../../.nvmrc) 与 [`package.json`](../../package.json) | Web/Desktop/Docs/Mobile 构建必需 | 根脚本使用 pnpm workspace 与 Turborepo |
| pnpm | `10.28.2`；由 `packageManager` 固定 | 前端工作区必需 | 容器通过 Corepack 激活同一版本 |
| Go | 模块声明 `1.26.6`；CI/发布取最新 `1.26.x` | API、CLI、迁移必需 | 来源见 [`server/go.mod`](../../server/go.mod) |
| PostgreSQL | 17 | 必需 | `pgcrypto`、`pg_trgm` 必需；`pg_bigm`、`pg_cron` 可选 |
| Redis/Valkey | Redis 6.2+ 才能满足 Stream `XTRIM MINID` 路径 | 单副本可不配，多副本强烈需要 | 实时跨副本、缓存、限流和渠道租约可使用不同实例 |
| Docker Compose | Compose CLI 插件，拒绝旧 `docker-compose` v1 | 默认本地 DB 与 Compose 自托管必需 | `make selfhost` 会先检查 |

默认自托管镜像虽然使用 `pgvector/pgvector:pg17`，当前迁移并不创建 `vector` 扩展或向量列；标准 PostgreSQL 17 加 contrib 扩展即可。默认 Compose 也**没有 Redis 服务**，所以它是可选依赖，不会因未配置而阻止单节点启动。

## 启动与停止边界

后端启动顺序是：

1. [`docker/entrypoint.sh`](../../docker/entrypoint.sh) 或本地命令执行 `migrate up`。
2. 迁移器用 PostgreSQL session advisory lock 串行化并发迁移，见 [`server/cmd/migrate/main.go`](../../server/cmd/migrate/main.go)。
3. [`server/cmd/server/main.go`](../../server/cmd/server/main.go) 验证生产 JWT、feature flag 和 LLM 重试配置，连接数据库，装配 Redis、实时 Hub、渠道 supervisor、worker、DB-backed scheduler、指标与 pprof。
4. API 开始监听；`/readyz` 只有在数据库可达且当前二进制要求的所有迁移均已登记时才返回 200。

收到 SIGINT/SIGTERM 后，服务按固定顺序停止自动化、排空 HTTP、排空跨副本出站、取消 worker、刷新 heartbeat、等待渠道 worker，再关闭指标和 pprof。`MULTICA_SHUTDOWN_HOLD_DURATION` 会在这一流程前额外等待；平台的 termination grace period 必须覆盖“hold + 实际排空时间”。

## 数据保留与危险操作

| 操作 | 数据结果 |
| --- | --- |
| `make down` | 只停当前开发环境的进程；数据库、profile 和槽位保留 |
| `make destroy` | 停进程，并删除当前开发数据库、CLI profile、守护进程工作区、Desktop `userData` 和 registry 记录 |
| `make selfhost-stop` | Compose 容器停止；named volume 保留 |
| `helm uninstall multica` | workload 删除，PVC 与预先创建的 Secret 通常保留 |
| `kubectl delete namespace multica` | namespace 内 PostgreSQL、上传 PVC、Secret 等全部删除 |

> **危险操作警告：** `make destroy`、`make db-drop`、`make db-reset`、`make remove-worktree` 和 `kubectl delete namespace` 都可能永久删除数据。生产升级前应分别备份 PostgreSQL 与附件存储；数据库回滚不能由镜像回滚自动完成。

## 关键源码

| 源码 | 运维意义 |
| --- | --- |
| [`Makefile`](../../Makefile) | 开发、测试、数据库、自托管的命令入口 |
| [`.env.example`](../../.env.example) | 最完整的变量契约及安全说明 |
| [`scripts/dev-env.sh`](../../scripts/dev-env.sh) | 开发环境 registry、锁、端口、身份和销毁 |
| [`server/cmd/server/main.go`](../../server/cmd/server/main.go) | 进程装配、Redis、worker、指标和关闭顺序 |
| [`server/cmd/server/health.go`](../../server/cmd/server/health.go) | liveness/readiness 判定 |
| [`docker-compose.selfhost.yml`](../../docker-compose.selfhost.yml) | 默认单节点 Compose 拓扑 |
| [`deploy/helm/multica/values.yaml`](../../deploy/helm/multica/values.yaml) | Kubernetes 默认值和持久卷约束 |
| [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) | 主 CI 的路径门控与作业矩阵 |
| [`.github/workflows/release.yml`](../../.github/workflows/release.yml) | CLI、镜像、Helm 与 Desktop 发布 |

产品运行单元和请求路径见[系统架构](../overview/architecture.md)；准备修改代码时同时阅读[模式与约定](../how-to-contribute/patterns-and-conventions.md)。
