# 部署与自托管

自托管的服务端最小拓扑是 PostgreSQL、Go backend 和 Next.js frontend。真正执行 Agent CLI 的 daemon 不在服务端容器或 Kubernetes Pod 中运行，而在每个用户控制的机器上通过 `multica setup self-host` 连接部署。

## Docker Compose 快速部署

前置条件是 Docker 和 Compose CLI 插件；旧 `docker-compose` v1 不受支持。

```bash
git clone https://github.com/multica-ai/multica.git
cd multica
make selfhost
```

[`Makefile`](../../Makefile) 会在缺少 `.env` 时从 [`.env.example`](../../.env.example) 创建它，并随机生成 `JWT_SECRET`、PostgreSQL 密码和 `MULTICA_VCS_SECRET_KEY`。随后拉取 `.env` 中 `MULTICA_IMAGE_TAG` 指定的后端/Web 镜像，运行 [`docker-compose.selfhost.yml`](../../docker-compose.selfhost.yml)，等待 `/health`：

```text
postgres  pgvector/pgvector:pg17 + pgdata volume
backend   GHCR image + backend_uploads volume，容器内 :8080
frontend  GHCR image，容器内 :3000
```

默认只发布到宿主机 loopback：

```text
127.0.0.1:${PORT:-8080}          -> backend:8080
127.0.0.1:${FRONTEND_PORT:-3000} -> frontend:3000
```

不要为了公网访问直接改成 `0.0.0.0`；Docker 的端口规则可能绕过主机防火墙。保留 loopback，在前面放 TLS 反向代理。

常用命令：

```bash
make selfhost                     # 拉取正式镜像
make selfhost-build               # 从当前 checkout 构建 dev 镜像
make selfhost-stop                # 停容器，保留 named volumes

docker compose -f docker-compose.selfhost.yml ps
docker compose -f docker-compose.selfhost.yml logs -f backend frontend
```

若 tag 尚未发布，`make selfhost` 会提示使用 `make selfhost-build`。本地构建 override 位于 [`docker-compose.selfhost.build.yml`](../../docker-compose.selfhost.build.yml)，生成 `multica-backend:dev` 和 `multica-web:dev`，不会覆盖拉取的 `latest`。

## 镜像启动行为

后端 [`Dockerfile`](../../Dockerfile) 使用 Go 1.26 Alpine builder 构建 server、CLI、migrate 和回填工具，runtime 是 Alpine 3.21。[`docker/entrypoint.sh`](../../docker/entrypoint.sh) 在同一个数据库启动预算内：

1. 执行 `./migrate up`；
2. 迁移失败则容器退出；
3. 成功后 `exec ./server`，让 signal 直接到达 Go 进程。

Web [`Dockerfile.web`](../../Dockerfile.web) 使用 Node 22、仓库固定的 pnpm 版本和 Next.js standalone 输出，以非 root `nextjs` 用户运行。

生产不要依赖 mutable `latest`；固定：

```dotenv
MULTICA_IMAGE_TAG=v0.4.37
MULTICA_BACKEND_IMAGE=ghcr.io/multica-ai/multica-backend
MULTICA_WEB_IMAGE=ghcr.io/multica-ai/multica-web
```

## 登录和本地 daemon

Compose 默认 `APP_ENV=production`。正式部署配置 SMTP 或 Resend；两者都没有时，生成的验证码只写到 backend 日志。不要在公网实例通过 `APP_ENV=development` + 固定验证码绕过邮件。

每个需要执行智能体任务的用户机器：

```bash
brew install multica-ai/tap/multica
multica setup self-host \
  --server-url https://api.example.com \
  --app-url https://app.example.com
multica daemon status
```

daemon 机器还必须安装至少一种受支持的 Agent CLI。Web/API 没有 daemon 时仍能进行协作，但运行时保持离线，Agent task 不会执行。

## 反向代理

推荐同源布局：

```caddy
multica.example.com {
    @multica_ws path /ws /ws/*
    handle @multica_ws {
        reverse_proxy localhost:8080 {
            flush_interval -1
        }
    }
    reverse_proxy localhost:3000
}
```

后端配置：

```dotenv
FRONTEND_ORIGIN=https://multica.example.com
CORS_ALLOWED_ORIGINS=https://multica.example.com
COOKIE_DOMAIN=
NEXT_PUBLIC_API_URL=
NEXT_PUBLIC_WS_URL=
```

WebSocket 路由必须支持 Upgrade 且不能缓冲；Caddy 的匹配应写 `/ws /ws/*`，不要写会误匹配工作区路径的 `/ws*`。如果 Origin 不在 allowlist，HTTP 页面可能可用，但 WebSocket 返回 403，界面只能刷新后更新。

分域布局需要：

```dotenv
FRONTEND_ORIGIN=https://app.example.com
CORS_ALLOWED_ORIGINS=https://app.example.com
COOKIE_DOMAIN=.example.com
REMOTE_API_URL=https://api.example.com
NEXT_PUBLIC_API_URL=https://api.example.com
NEXT_PUBLIC_WS_URL=wss://api.example.com/ws
```

`NEXT_PUBLIC_API_URL`/`REMOTE_API_URL` 只能是 origin，不要附加 `/api`。`COOKIE_DOMAIN` 必须是覆盖两个 host 的最窄可信父域；它也会把登录 session 发给该域下所有 host。无法建立这一信任边界时必须改用同源。

## PostgreSQL 与迁移

生产要求 PostgreSQL 17。标准发行版加 contrib 即可：

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```

默认镜像的 `pgvector` 名称是历史选择，Multica 当前不依赖 vector extension。`pg_bigm` 可提高 CJK 搜索质量；`pg_cron` 可不安装，周期任务由 `sys_cron_executions` 支持的进程内 scheduler 执行。

每个 backend 容器启动都会迁移。[`server/cmd/migrate/main.go`](../../server/cmd/migrate/main.go) 使用 session-level advisory lock 串行化并发 runner，因此多个副本同时冷启动不会竞跑；等待者拿到锁后会重新检查 ledger 并 no-op 已完成版本。迁移文件可包含 `CREATE INDEX CONCURRENTLY`，因此 runner 不把整个迁移循环包在事务内。

手动运行：

```bash
./server/bin/migrate up
# 或
(cd server && go run ./cmd/migrate up)
```

生产升级前同时备份数据库和附件。镜像/Helm rollback 只回退应用，不自动回退 schema；不要在不了解 down migration 数据影响时执行 `migrate down`。

## Kubernetes / Helm

OCI chart：

```bash
kubectl create namespace multica

kubectl -n multica create secret generic multica-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -hex 32)" \
  --from-literal=POSTGRES_PASSWORD="<strong-random-password>" \
  --from-literal=RESEND_API_KEY="" \
  --from-literal=GOOGLE_CLIENT_SECRET="" \
  --from-literal=CLOUDFRONT_PRIVATE_KEY="" \
  --from-literal=MULTICA_DEV_VERIFICATION_CODE=""

helm install multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica
```

Chart 定义在 [`deploy/helm/multica/`](../../deploy/helm/multica/)。主要默认值：

- PostgreSQL Deployment + 10Gi PVC；可设 `postgres.external.enabled=true`，从 `existingSecret` 提供 `DATABASE_URL`。
- backend 1 副本 + 5Gi uploads PVC，Deployment strategy 为 `Recreate`。
- frontend 1 副本。
- frontend/backend 两个 Ingress。
- 非敏感配置由 ConfigMap 管理；Chart 只引用、从不创建 `existingSecret`。
- backend 的 startup/readiness probe 使用 `/healthz`，liveness 使用不依赖数据库的 `/health`。

冷启动时 backend 可能已是 Running，但 entrypoint 正在等待 PostgreSQL/迁移；10 分钟 startup probe 窗口覆盖默认 3 分钟数据库重试预算和迁移时间。

升级与回滚：

```bash
helm upgrade multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica \
  -f my-values.yaml

helm -n multica rollback multica
```

发布 chart 的版本去掉 Git tag 的前导 `v`，但默认应用镜像 tag 保留 `v`。例如 Git tag `v0.4.37` 对应 chart `0.4.37` 和应用镜像 `v0.4.37`。

## 附件、对象存储与多副本

### 单副本

Compose 的 `backend_uploads` named volume 和 Helm 默认 RWO PVC 适合单副本。手动运行时应把 `LOCAL_UPLOAD_DIR` 配成绝对持久路径；相对路径随 backend 工作目录变化。

### 多副本

Kubernetes 默认 PVC 是 `ReadWriteOnce`，`backend.replicas > 1` 往往让第二个 Pod 出现 Multi-Attach。三种选择：

1. 配置 S3/兼容对象存储，并设置 `backend.uploads.persistence.enabled=false`；
2. 使用支持 `ReadWriteMany` 的 StorageClass，把 `accessModes` 改为 `ReadWriteMany`；
3. 保持单副本。

即使 RWO 卷在同一节点能被多个 Pod 挂载，也不应把它当作稳定的跨节点共享语义。S3 是水平扩展的推荐路径。

对象存储设置详见[配置参考](configuration.md#附件与对象存储)。`ATTACHMENT_DOWNLOAD_MODE=proxy` 适合浏览器无法访问的 VPC/容器内 endpoint；私有 AWS bucket 可用 presign 或 CloudFront 签名 URL。

### CMEK

应用的 S3 `PutObject` 当前不发送 SSE/KMS 参数，也没有 CMEK 变量，见 [`server/internal/storage/s3.go`](../../server/internal/storage/s3.go)。需要客户管理密钥时，在 bucket 默认加密、bucket policy、IAM/KMS 权限和存储平台层强制执行；PostgreSQL/PVC/备份的 CMEK 也由其托管服务或 CSI 配置。上线前用实际 workload identity 验证上传、代理下载、presign 和删除。

## 多副本实时与渠道约束

数据库 scheduler 和 startup migration 已具备跨副本互斥，但 WebSocket 是进程本地连接。横向扩容 API 时还必须处理：

1. **实时 fan-out**：配置 `REDIS_URL` 或专用 `REALTIME_RELAY_REDIS_URL`。未配置时服务明确运行 in-memory single-node mode。
2. **daemon wakeup**：使用默认 `sharded` 或迁移期 `dual` relay；`legacy` 不转发 daemon wakeup。
3. **渠道长连接租约**：生产设置 `CHANNEL_WS_LEASE_BACKEND=redis`、专用 `CHANNEL_WS_LEASE_REDIS_URL` 和稳定且环境唯一的 namespace。租约 Redis 使用 `noeviction`。
4. **WeCom**：sharded/dual relay 能把另一个副本产生的回复转发给持有 socket 的副本；没有 relay 或使用 legacy 时，应保持 WeCom backend 单副本。即使有 relay，在所有副本都重连、暂无 live connection 的窗口仍可能丢回复，应监控 `multica_wecom_outbound_dropped_total{reason="no_live_connection"}`。
5. **本地附件**：必须换 S3 或共享 RWX。
6. **数据库连接数**：按副本数乘以每 Pod pool max。

默认 Compose 不包含 Redis，也未透传 Redis env，不能直接用它声称支持多 backend 副本；应提供自有 Redis/Valkey 与 Compose override，或使用生产平台的 Secret/env 注入。

## 周期任务与历史回填

Usage/Runtime dashboard 的 `task_usage_hourly` 由每副本都运行的 DB-backed scheduler 更新。唯一键让每个 5 分钟 plan 只有一个 winner，SQL 函数再用 advisory lock `4246` 防止和旧 `pg_cron`/手工调用重写。

检查：

```sql
SELECT plan_time, status, attempt, runner_id,
       error_code, error_msg, started_at, finished_at
  FROM sys_cron_executions
 WHERE job_name = 'rollup_task_usage_hourly'
 ORDER BY plan_time DESC
 LIMIT 20;
```

历史回填：

```bash
docker compose -f docker-compose.selfhost.yml exec backend \
  ./backfill_task_usage_hourly --sleep-between-slices=2s
```

回填可中断和重跑。`--months-back` 会跳过更老 bucket 且推进 watermark，必须配 `--force-partial`，属于有意永久放弃旧区间的危险选项。

## 停止与删除

```bash
make selfhost-stop
multica daemon stop

helm -n multica uninstall multica       # workload 删除，PVC/Secret 通常保留
kubectl delete namespace multica        # 删除 namespace 内全部数据
```

> **危险操作警告：** 删除 namespace 会连同 PostgreSQL 和上传 PVC 一起删除。Compose `down -v` 也会删除 named volume；仓库提供的 `make selfhost-stop` 不带 `-v`，日常停止应使用它。

更完整的上游部署说明见 [`SELF_HOSTING.md`](../../SELF_HOSTING.md) 与 [`SELF_HOSTING_ADVANCED.md`](../../SELF_HOSTING_ADVANCED.md)。健康探针和扩容故障定位见[可观测性与故障排查](observability-and-troubleshooting.md)。
