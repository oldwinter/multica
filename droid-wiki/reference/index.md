# 参考手册

本节面向需要定位代码、核对平台差异、修改接口或排查配置的维护者。内容以当前仓库源码为准；产品语义与强制工程规则仍以仓库根目录的 [CLAUDE.md](../../CLAUDE.md) 为最高优先级。

## 页面索引

| 页面 | 回答的问题 | 主要源码入口 |
| --- | --- | --- |
| [仓库地图](repository-map.md) | 顶层目录、Go 后端、应用与共享包分别负责什么？ | [server/](../../server/)、[apps/](../../apps/)、[packages/](../../packages/) |
| [路由地图](route-map.md) | Web、Desktop、Mobile 的页面路径如何对应？ | [apps/web/app/](../../apps/web/app/)、[Desktop routes.tsx](../../apps/desktop/src/renderer/src/routes.tsx)、[apps/mobile/app/](../../apps/mobile/app/) |
| [API 地图](api-map.md) | HTTP/WS API 有哪些鉴权边界和业务域？ | [server/cmd/server/router.go](../../server/cmd/server/router.go) |
| [数据模型](data-models.md) | 核心实体如何关联，删除与一致性由谁负责？ | [server/migrations/](../../server/migrations/)、[server/pkg/db/queries/](../../server/pkg/db/queries/) |
| [配置参考](configuration.md) | 环境变量按数据库、认证、存储、集成等领域如何配置？ | [.env.example](../../.env.example)、[server/cmd/server/main.go](../../server/cmd/server/main.go) |
| [依赖参考](dependencies.md) | 主要运行时、workspace 与外部服务依赖在哪里声明？ | [server/go.mod](../../server/go.mod)、[pnpm-workspace.yaml](../../pnpm-workspace.yaml) |
| [测试地图](test-map.md) | 行为应该在哪一层测试，常用命令和 CI 门禁是什么？ | [scripts/check.sh](../../scripts/check.sh)、[.github/workflows/ci.yml](../../.github/workflows/ci.yml) |

## 去哪里改

| 想改什么 | 首选位置 | 同步检查 |
| --- | --- | --- |
| Go API 路由或鉴权边界 | [server/cmd/server/router.go](../../server/cmd/server/router.go) | 对应 [server/internal/handler/](../../server/internal/handler/)、schema 解析、畸形响应测试 |
| 后端业务规则 | [server/internal/service/](../../server/internal/service/)；Room 独立领域见 [server/internal/room/](../../server/internal/room/) | handler 只保留 HTTP 边界；数据库写入是否需要事务 |
| SQL 查询 | [server/pkg/db/queries/](../../server/pkg/db/queries/) | 运行 `make sqlc`，提交 [server/pkg/db/generated/](../../server/pkg/db/generated/) |
| 表结构或索引 | [server/migrations/](../../server/migrations/) | 禁止新增 FK；索引必须在单独迁移中 `CREATE INDEX CONCURRENTLY` |
| Web 页面入口 | [apps/web/app/](../../apps/web/app/) | 若为共享业务页，同步 Desktop；平台导航只放 [apps/web/platform/](../../apps/web/platform/) |
| Desktop 路由或窗口壳 | [apps/desktop/src/renderer/src/routes.tsx](../../apps/desktop/src/renderer/src/routes.tsx) | 预工作区流程应使用 Window Overlay，而不是新增路由 |
| Mobile 页面 | [apps/mobile/app/](../../apps/mobile/app/) | 先读 [apps/mobile/CLAUDE.md](../../apps/mobile/CLAUDE.md)；Mobile UI/状态独立 |
| 共享 API、Query、Mutation、Store | [packages/core/](../../packages/core/) | Query 管服务端状态，Zustand 管客户端状态；网络 JSON 必须经 zod schema |
| Web/Desktop 共用业务界面 | [packages/views/](../../packages/views/) | 不得导入 `next/*` 或 `react-router-dom` |
| 原子 UI 与设计 token | [packages/ui/](../../packages/ui/) | 不得导入 `@multica/core`；优先复用 Base UI/shadcn |
| 中英文产品文案 | [packages/views/locales/](../../packages/views/locales/) | 先读 [conventions.mdx](../../apps/docs/content/docs/developers/conventions.mdx) 与 [conventions.zh.mdx](../../apps/docs/content/docs/developers/conventions.zh.mdx) |
| 自托管配置 | [.env.example](../../.env.example)、[docker-compose.selfhost.yml](../../docker-compose.selfhost.yml) | [SELF_HOSTING.md](../../SELF_HOSTING.md) 与 Helm values |
| 测试 | 与被测代码同层的 `*.test.*` / `*_test.go` | 先看 [测试地图](test-map.md)，不要跨层重复同一行为矩阵 |

## 三条定位原则

1. **先判断边界。** 平台路由与原生能力留在 app；可共享的无头逻辑进入 `packages/core`；共享业务视图进入 `packages/views`；纯原子组件进入 `packages/ui`。
2. **先找权威入口。** 页面路由看三端路由文件，API 看 `server/cmd/server/router.go`，数据结构看迁移，SQL 行为看 query 文件，构建与测试命令看根 `package.json`、`Makefile` 和工作流。
3. **关系由应用负责。** 新数据库设计不使用外键或级联；跨表验证、依赖清理和需要原子性的多步写入由服务/handler 事务显式完成。
