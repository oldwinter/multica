# 本地开发

推荐入口是 [`Makefile`](../../Makefile) 的环境命令。它们把 checkout 当作可登记、可列出、可证明归属、可完整销毁的对象，适合主 checkout、多个 Git worktree 和自动化任务并行运行。

## 前置条件

- Node.js 26（[`.nvmrc`](../../.nvmrc)）
- pnpm 10.28.2（[`package.json`](../../package.json)）
- Go 1.27；CI 使用最新 1.27.x（[`server/go.mod`](../../server/go.mod)）
- `curl`
- 本地 PostgreSQL 17，或 Docker + Compose CLI 插件
- 推荐安装 `psql`、`lsof`；Linux 可用 `ss` 作为端口检查后备

只启动 API 时不要求已有 pnpm install，但 `make up` 的环境分配脚本仍使用 Node 处理 URL/时间。Web 或 Desktop 会在缺少 `node_modules` 时运行 `pnpm install`。

## 推荐生命周期

```bash
make up                         # 默认 api + web
make up C=api,web,daemon       # 加本地 Agent daemon
make up C=desktop              # 实际会自动补 api
make status                    # PID、端口、commit、启动时间、DB 和日志目录
make list                      # 这台机器上的所有登记环境
make down                      # 停进程，保留 DB/profile/槽位
make up                        # 快速恢复
make destroy                   # 永久删除当前开发环境
```

`web`、`daemon`、`desktop` 都依赖 API；即使只选择其中之一，[`scripts/dev-env.sh`](../../scripts/dev-env.sh) 也会自动加入 `api`。后台日志保存在 `make status` 输出的 registry 日志目录中。

> **危险操作警告：** 日常停止只用 `make down`。`make destroy` 会删除数据库、开发 CLI profile、daemon workspace、Desktop `userData` 和环境登记；它不是“更彻底的 stop”。

需要在前台同时看 API 和 Web 输出时：

```bash
make down
make dev
```

[`scripts/dev.sh`](../../scripts/dev.sh) 会准备 env、依赖、数据库和迁移，然后以前台进程组启动两项服务，Ctrl-C 同时停止它们。

## Registry、端口和身份隔离

registry 默认位于：

```text
~/.multica/dev/
└── envs/<environment-name>/
    ├── manifest.env
    ├── api.pid / web.pid / desktop.pid
    └── logs/
```

可用 `MULTICA_DEV_HOME` 改根目录。分配过程使用原子 `mkdir` 锁 `~/.multica/dev/lock.d`，从 checkout 路径 hash 起探测最多 1000 个槽位；只有 registry 未占用且三个端口都空闲才采用：

```text
offset:           0..999
API:              18080 + offset
Web:              13000 + offset
Desktop renderer: 5174 + offset（6000 特判为 6174）
DB:               multica_<checkout_slug>_<offset>
CLI profile:      dev-<checkout_slug>-<offset>
```

分配后，manifest 而不是再次计算的 hash 成为事实来源。`make up` 导出 manifest 中的端口、`DATABASE_URL` 和数据库名给子进程；`make list` 可以发现目录已消失但仍遗留的环境。

API 复用不只检查 `/health` 为 200。启动脚本比较：

- `/health` 返回的进程 PID 是否就是该端口 listener；
- listener 是否属于本环境启动的进程树/会话；
- 响应中的 commit 是否等于当前 checkout；
- 新启动时 `started_at` 是否晚于 launcher。

任何 mismatch 都拒绝复用或杀死未知进程，避免在错误 checkout 的 API 上运行测试。

## 主 checkout 与 worktree

主 checkout 使用 `.env`；worktree 使用 `.env.worktree`：

```bash
cp .env.example .env           # 主 checkout
make worktree-env              # worktree；拒绝覆盖已有文件
```

[`scripts/init-worktree-env.sh`](../../scripts/init-worktree-env.sh) 先按路径生成独立数据库和端口；第一次 `make up` 若发现冲突，会在锁内重新分配并重写该文件。主 checkout 和所有 worktree 共用 `docker-compose.yml` 的 PostgreSQL 17 容器和 `pgdata` volume，但数据库名不同。

> 不要把主 checkout 的 `.env` 复制进 worktree。环境检测在 `.env` 和 `.env.worktree` 同时存在时优先 `.env`，这样会让 worktree 指向主数据库。

保留的兼容命令：

```bash
make setup-main
make start-main
make stop-main
make setup-worktree
make start-worktree
make stop-worktree
```

它们直接使用指定 env 文件，不提供 registry 的完整进程所有权与 list/destroy 体验；新流程优先使用 `make up`。

## 临时环境与垃圾回收

自动化可登记带 TTL 的环境：

```bash
make up ARGS=--ephemeral                  # agent owner，默认 24 小时
make up ARGS="--ttl 6 --name review-123"
make gc ARGS=--dry-run
make gc
```

每次 `make up` 都会 best-effort 清理 checkout 目录已消失或 TTL 已过的记录。清理失败会保留 manifest，便于以后重试，不会伪装成已释放。

## 数据库操作

```bash
make db-up                 # 启动共享 PostgreSQL 容器
make migrate-up
make migrate-down          # 有破坏风险，先确认 down migration
make db-reset              # 删除并重建当前 env 的本地数据库
make db-drop               # 交互确认后永久删除当前 env 的数据库
make db-down               # 停容器，保留 volume
```

环境启动优先通过应用实际使用的 `DATABASE_URL` 检查/创建数据库，并以 `migrate up` 成功作为最终可达证明。这样能识别“原生 PostgreSQL 占了 5432、Docker 容器中的建库却发生在另一个服务”这一类隐蔽错误。远程 `DATABASE_URL` 不会触发本地 Docker；删除脚本拒绝远程 host、系统数据库和默认主数据库（除非显式设置 `ALLOW_MAIN_DB_DROP=1`）。

> **危险操作警告：** `db-reset`、`db-drop` 与 `migrate-down` 会改变或删除数据。先核对 `make status` 中的数据库名和 URL。不要靠 `docker exec` 看到数据库存在就断言应用连接的是同一个 PostgreSQL。

移除 worktree 时，用：

```bash
make remove-worktree WORKTREE=/absolute/path/to/worktree
```

[`scripts/remove-worktree.sh`](../../scripts/remove-worktree.sh) 拒绝主 checkout、当前 worktree、locked worktree 和 dirty worktree；如存在 `.env.worktree`，先确认并删除其数据库，再调用 `git worktree remove`。

## 单组件、CLI 与环境内命令

```bash
make server                         # 当前 env 的 Go API
pnpm dev:web                       # Web
pnpm dev:desktop                   # Electron
pnpm dev:docs                      # 文档站
pnpm dev:mobile                    # Expo
make daemon                         # local profile daemon restart
make cli ARGS="issue list"         # 从源码运行 CLI
make env-exec ARGS="-- pnpm dev:desktop"
```

`make env-exec` 会注入 registry 变量、持久化的开发 TMPDIR 和正确 daemon workspace，并清除 Agent 任务上下文里指向生产环境的 `MULTICA_*` 变量。daemon 组件会构建持久的 `server/bin/multica`，而不是用 `go run` 启动；后者的临时二进制可能在任务结束后消失。

## 构建与生成

```bash
pnpm install
pnpm build                          # 除 Mobile 外
make build                          # server、multica、migrate 到 server/bin
make sqlc                           # 生成 server/pkg/db/generated
pnpm generate:reserved-slugs
```

前端任务图见 [`turbo.json`](../../turbo.json)。`mdx` 被抽成共享依赖，避免并发生成 `.source/`；`test` 通过 hash-only `cache-inputs` 让共享包源码变化进入消费者的 cache key。

## 本地验证

窄范围优先：

```bash
pnpm -C packages/core test
pnpm -C packages/views test
(cd server && go test ./internal/service -run TestName)
```

全量入口：

```bash
pnpm typecheck
pnpm test
pnpm lint
make test
pnpm exec playwright test
make check
```

`make check` 依次执行 TS typecheck、Vitest、Go、启动/复用 API 和 Web、Playwright Chromium E2E。Go wrapper 会把所有已登记 Agent CLI 命令替换为 sentinel，默认测试若意外调用用户机器上真实且已登录的 Agent CLI 会失败。完整测试和 CI 差异见[CI、测试与发布](ci-testing-and-release.md)。

## 快速排查

| 现象 | 操作 |
| --- | --- |
| 端口返回旧代码 | `make status`；查看 API 是否 `mismatch`，不要手工 kill 未知 listener |
| worktree 看到主环境数据 | 检查 worktree 是否误有 `.env`，核对 manifest 和 `.env.worktree` |
| 5432 有服务但迁移报数据库不存在 | 查看启动脚本打印的端口 owner；停止冲突的原生 PG 或修改 `DATABASE_URL` |
| 后端代码修改未生效 | 当前开发流没有 Go hot reload；只停/重启 API 组件 |
| daemon 启动后 stopped | 查看 profile 的 `daemon.log`；确认没有从 daemon-managed task 内嵌套启动 |
| E2E 打到错误端口 | 用 `make check` 或通过 `make env-exec` 运行，核对 `PLAYWRIGHT_BASE_URL` |

更完整的服务级信号见[可观测性与故障排查](observability-and-troubleshooting.md)，架构背景见[系统架构](../overview/architecture.md)。
