# Multica 代码库

Multica 是一个面向小型技术团队的 AI 原生任务管理平台。人和智能体在同一工作区中领取任务、发表评论、推进状态并留下可审查的执行记录；智能体真正执行代码的地方则是用户控制的本地或云端运行时。产品定位见 `README.md`、`PRODUCT.md` 与 `VISION.zh.md`。

## 从哪里开始

| 你想了解什么 | 推荐入口 |
| --- | --- |
| 系统如何连接 | [系统架构](architecture.md) |
| 怎样启动开发环境 | [本地开发](getting-started.md) |
| 产品与代码术语 | [术语表](glossary.md) |
| 当前规模与近期热点 | [代码库数据](../by-the-numbers.md) |
| 项目如何演变至今 | [项目历史](../project-history/index.md)与[项目掌故](../lore.md) |
| 四个前端应用 | [应用](../apps/index.md) |
| 一项用户功能如何跨层实现 | [功能](../features/index.md) |
| Go 服务与后台系统 | [系统](../systems/index.md) |
| REST、WebSocket 与守护进程协议 | [后端与 API](../backend/index.md) |
| 包边界与共享规则 | [共享包](../packages/index.md) |
| 准备修改代码 | [参与贡献](../how-to-contribute/index.md) |

## 产品模型

Multica 把工作组织成工作区。工作区中有成员、智能体、任务、项目、收件箱、运行时、skill、自动化、Room、Wiki 与 Twin 等对象。产品语义的核心区分写在 `apps/docs/content/docs/developers/conventions.zh.mdx`：

- **任务（Issue）** 是团队要完成的一件事，对应数据库与 API 中的 `issue`。
- **task** 是智能体的一次执行，同一个任务可以产生多次 task。
- **智能体（Agent）** 是可被分配工作的主体，配置中引用运行时、模型、skill 和 MCP。
- **运行时（Runtime）** 是守护进程检测并注册的某个智能体 CLI 能力。
- **守护进程（Daemon）** 在用户机器上领取 task、准备隔离目录、启动智能体 CLI，并回传消息、进度、用量和终态。

主路径从任务分派开始：Go 服务把待执行工作写入 `agent_task_queue`，通过守护进程 WebSocket 发出唤醒提示；守护进程领取 task 后调用 `server/pkg/agent/` 中的适配器；执行事件又通过 REST、事件总线和实时广播回到 Web、Desktop 与 Mobile。核心服务编排位于 `server/internal/service/task.go`，守护进程入口位于 `server/internal/daemon/daemon.go`。

## 仓库结构

```text
multica/
├── apps/
│   ├── web/          # Next.js App Router
│   ├── desktop/      # Electron 主进程、预加载与 React 渲染器
│   ├── mobile/       # Expo Router / React Native
│   └── docs/         # Fumadocs 文档站
├── packages/
│   ├── core/         # API、类型、TanStack Query、Zustand、实时同步
│   ├── ui/           # 无业务语义的 UI 原子组件与设计令牌
│   ├── views/        # Web 与 Desktop 共用的业务页面
│   ├── plugin-sdk/   # 插件表面 SDK
│   ├── eslint-config/
│   └── tsconfig/
├── server/
│   ├── cmd/          # API、CLI、迁移与回填命令
│   ├── internal/     # Handler、service、daemon、集成与后台系统
│   ├── pkg/          # agent 适配器、协议、数据库生成代码等
│   └── migrations/   # PostgreSQL 迁移
├── e2e/              # Playwright 端到端测试
├── scripts/          # 开发环境、安装、检查与生成脚本
├── deploy/           # Helm 等部署资源
└── docs/downstream/  # 下游功能设计、ADR 与上游同步账本
```

Web 和 Desktop 共享 `packages/core/`、`packages/ui/` 与 `packages/views/`。Mobile 只共享类型和纯函数，独立拥有 UI、状态、数据层与发布节奏；这一边界由 `CLAUDE.md` 和 `apps/mobile/CLAUDE.md` 共同规定。

## 主要运行单元

| 运行单元 | 入口 | 职责 |
| --- | --- | --- |
| API 服务 | `server/cmd/server/main.go` | HTTP、WebSocket、事件监听、调度器和后台 worker |
| CLI | `server/cmd/multica/main.go` | 登录、工作区、任务、智能体、skill、Wiki 与守护进程命令 |
| 守护进程 | `server/internal/daemon/daemon.go` | 注册运行时、领取并执行 task、回传结果 |
| Web | `apps/web/app/` | 浏览器产品、登录/上手引导与公开页面 |
| Desktop | `apps/desktop/src/main/index.ts` | Electron 生命周期、本地守护进程与桌面能力 |
| Mobile | `apps/mobile/app/_layout.tsx` | iOS 优先的原生客户端 |
| Docs | `apps/docs/app/` | 多语言产品与运维文档 |

## 代码库特点

- 后端使用 Go、Chi、pgx/sqlc、gorilla/websocket，并以 PostgreSQL 17 为事实来源，见 `server/go.mod` 与 `server/sqlc.yaml`。
- 前端是 pnpm workspace 与 Turborepo，依赖和脚本定义于 `pnpm-workspace.yaml`、`package.json` 与 `turbo.json`。
- 服务端状态由 TanStack Query 管理，界面状态由 Zustand 管理。规则见 `CLAUDE.md`，实现集中在 `packages/core/*/queries.ts`、`packages/core/*/mutations.ts` 与各功能 store。
- API 响应必须通过 `packages/core/api/schema.ts` 的 `parseWithFallback` 和 Zod schema 防御版本漂移。
- 下游 fork 在上游产品旁维护 Room、Twin / LM Wiki、命名皮肤与 Skill Evolution；所有权和迁移兼容历史记录在 `docs/downstream/upstream-sync.md`。

## 关键源文件

| 文件 | 用途 |
| --- | --- |
| `server/cmd/server/main.go` | 服务启动、Redis、worker、调度器与优雅关闭 |
| `server/cmd/server/router.go` | 中间件、集成装配与 HTTP 路由 |
| `server/internal/service/task.go` | task 入队、领取、归因、执行生命周期 |
| `server/internal/daemon/daemon.go` | 本地守护进程主循环 |
| `packages/core/api/client.ts` | Web/Desktop API 客户端 |
| `packages/core/realtime/use-realtime-sync.ts` | Web/Desktop 实时事件到 Query cache 的协调 |
| `packages/views/` | Web/Desktop 共享业务页面 |
| `apps/mobile/data/api.ts` | Mobile 自有的 API 边界 |
| `Makefile` | 本地环境、构建、测试与部署入口 |

下一步可阅读[系统架构](architecture.md)，再按要修改的运行单元进入[应用](../apps/index.md)、[功能](../features/index.md)或[系统](../systems/index.md)。
