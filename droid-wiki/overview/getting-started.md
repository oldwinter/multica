# 本地开发

仓库把一个 checkout 的数据库、端口、CLI profile 和进程视为一个开发环境。主 checkout 与 Git worktree 共用 PostgreSQL 容器，但使用独立数据库和端口；环境登记与所有权证明由 `scripts/dev-env.sh` 管理。

## 前置条件

版本来源是 `README.md`、`package.json` 与 `server/go.mod`：

- Node.js 26
- pnpm 10.28.2
- Go 1.27 或当前 CI 使用的兼容补丁版
- Docker 与 Docker Compose CLI 插件
- Git

Mobile 还需要 Xcode、Expo 工具链与 iOS 签名环境，详见 `apps/mobile/README.md`。

## 最快启动

```bash
make dev
```

`make dev` 调用 `scripts/dev.sh`，自动选择 `.env` 或 `.env.worktree`，安装依赖，检查 PostgreSQL，运行迁移，并以前台模式启动 Go API 和 Web。适合需要在终端用 Ctrl-C 统一停止的日常开发。

更适合并行 checkout 与自动化的环境命令是：

```bash
make up                         # 默认启动 api + web
make up C=api,web,daemon       # 同时运行本地守护进程
make up C=desktop              # Electron 连接当前环境
make status                    # 显示 PID、端口、commit 与所有权证明
make list                      # 列出这台机器上的所有 Multica 环境
make down                      # 停进程，保留数据库和环境槽位
make destroy                   # 删除当前环境的数据库、profile 和本地状态
```

`make destroy` 会删除数据，只在明确不再需要当前开发环境时使用。普通重启使用 `make down` 和 `make up`。

## 环境文件

主 checkout 使用 `.env`：

```bash
cp .env.example .env
```

worktree 使用 `.env.worktree`：

```bash
make worktree-env
```

不要把主 checkout 的 `.env` 复制到 worktree。`CONTRIBUTING.md` 说明当前命令优先读取 `.env`，误复制会让 worktree 指向主数据库。`scripts/init-worktree-env.sh` 为 worktree 分配独立数据库名、API 端口、Web 端口和 Desktop renderer 端口。

## 只运行一个组件

```bash
make server              # Go API
pnpm dev:web             # Next.js Web
pnpm dev:desktop         # Electron
pnpm dev:docs            # Fumadocs
pnpm dev:mobile          # Expo
make daemon              # 使用 local profile 重启守护进程
make cli ARGS="issue list"
```

源码 CLI 入口是 `server/cmd/multica/main.go`。`make cli` 最终调用 `go run ./cmd/multica`，不会覆盖系统安装的 `multica`。

## 构建与生成

```bash
pnpm build               # 除 Mobile 外的 workspace 构建
make build               # server、multica CLI 和 migrate 二进制
make sqlc                # 从 server/pkg/db/queries 生成 Go 数据层
pnpm generate:reserved-slugs
```

修改 `server/pkg/db/queries/*.sql` 后运行 `make sqlc`。修改 `server/internal/handler/reserved_slugs.json` 后运行 `pnpm generate:reserved-slugs`，并提交生成的 `packages/core/paths/reserved-slugs.ts`。

## 数据库

本地开发默认使用 `docker-compose.yml` 中的 PostgreSQL。常用命令：

```bash
make db-up
make migrate-up
make migrate-down
make db-reset             # 只重建当前环境数据库
make db-drop              # 确认后永久删除当前环境数据库
```

迁移文件位于 `server/migrations/`，由 `server/cmd/migrate/main.go` 运行。迁移规则与历史兼容要求很严格：不使用数据库外键；每个索引放在单独迁移中并用 `CREATE INDEX CONCURRENTLY`；已发布的完整迁移文件名和内容不可改写。详情见[数据模型](../reference/data-models.md)和[迁移与上游同步](../background/migrations-and-upstream-sync.md)。

## 验证

最小检查应贴近修改位置：

```bash
pnpm -C packages/core test
pnpm -C packages/views test
pnpm -C apps/web test
pnpm -C apps/desktop test
(cd server && go test ./internal/service -run TestName)
```

跨仓库检查：

```bash
pnpm typecheck
pnpm test
pnpm lint
make test
pnpm exec playwright test
make check
```

`make check` 调用 `scripts/check.sh`，依次运行 TypeScript 类型检查、前端测试、Go 测试和 Playwright。Go 默认测试通过 `scripts/test-go.sh` 和 `scripts/go-test-with-agent-cli-guard.sh` 防止意外执行用户机器上已安装并登录的真实智能体 CLI。

## 常见调试入口

| 现象 | 先检查 |
| --- | --- |
| 端口上响应的不是当前 checkout | `make status`，比较 `/health` 返回的 PID 与 commit |
| worktree 读到主环境数据 | 是否误放了 `.env`；查看 `.env.worktree` 的 `DATABASE_URL` |
| 守护进程在线但不领取任务 | `multica daemon status` 与 `multica daemon logs`；再看 API 的 daemon WS 日志 |
| API 启动后 503 | `/readyz`、数据库迁移、可选集成所需密钥 |
| UI 与 API 字段不一致 | schema warning；`packages/core/api/schema.ts` 与 `packages/core/api/schemas.ts` |
| 多节点事件缺失 | `REALTIME_RELAY_MODE`、Redis 连接与 `/health/realtime` |

更完整的运行手册见[本地开发](../operations/local-development.md)和[可观测性与故障排查](../operations/observability-and-troubleshooting.md)。

## Mobile 特别规则

任何 Mobile 功能修改前先读 `apps/mobile/CLAUDE.md`。它要求先核对 Web/Desktop 实现和产品语义，再说明原生交互方案并取得明确执行指令。Mobile 使用独立 Query key、实时 updater 与 UI 组件，不能直接导入 Web/Desktop store 或页面。

## 关键源文件

| 文件 | 用途 |
| --- | --- |
| `Makefile` | 所有主开发命令 |
| `scripts/dev-env.sh` | 环境登记、端口、进程与清理 |
| `scripts/dev.sh` | 前台快速启动 |
| `scripts/check.sh` | 全仓验证 |
| `scripts/ensure-postgres.sh` | 创建和检查目标数据库 |
| `CONTRIBUTING.md` | worktree、数据库与排错说明 |
| `package.json` | workspace 脚本 |
| `turbo.json` | Turborepo 任务图 |

准备提交修改前，请继续阅读[添加功能](../how-to-contribute/adding-a-feature.md)与[测试](../how-to-contribute/testing.md)。
