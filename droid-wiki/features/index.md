# 功能域导航

活跃贡献者：Bohan Jiang、Naiyuan Qing、Jiayuan Zhang

本页按产品能力组织代码入口，适合回答“一个功能从哪里开始追踪”。Multica 的多数功能会同时经过 PostgreSQL、Go handler/service、共享数据层、共享视图和一个或多个客户端；不要只在当前界面目录中寻找完整语义。

## 核心功能域

| 功能域 | 产品职责 | 后端入口 | Web / Desktop 共享入口 | 其他入口 |
| --- | --- | --- | --- | --- |
| 任务、项目与规划 | 任务创建、状态、负责人、依赖、评论、属性、项目和时间线 | [任务 handler](../../server/internal/handler/issue.go)、[任务 service](../../server/internal/service/issue.go)、[SQL 查询](../../server/pkg/db/queries/issue.sql) | [任务视图](../../packages/views/issues/)、[任务数据层](../../packages/core/issues/) | [Mobile 任务界面](../../apps/mobile/components/issue/) |
| 智能体与执行 task | 智能体配置、分派、执行记录、模型、环境变量和 MCP | [智能体 handler](../../server/internal/handler/agent.go)、[task 编排](../../server/internal/service/task.go)、[智能体适配器](../../server/pkg/agent/) | [智能体视图](../../packages/views/agents/)、[智能体数据层](../../packages/core/agents/) | [守护进程](../../server/internal/daemon/) |
| 运行时与守护进程 | 检测本机 CLI、注册运行时、领取 task、准备目录和回传执行事件 | [守护进程实现](../../server/internal/daemon/)、[运行时 handler](../../server/internal/handler/runtime.go) | [运行时视图](../../packages/views/runtimes/) | [CLI 守护进程命令](../../server/cmd/multica/cmd_daemon.go) |
| 工作区、成员与权限 | 租户边界、成员身份、角色、邀请、工作区设置和作用域选择 | [工作区 handler](../../server/internal/handler/workspace.go)、[工作区中间件](../../server/internal/middleware/workspace.go) | [工作区数据层](../../packages/core/workspace/)、[设置视图](../../packages/views/settings/) | [认证中间件](../../server/internal/middleware/auth.go) |
| Skill、Squad 与协作 | 可复用知识包、智能体小组、成员角色和协作活动 | [Skill handler](../../server/internal/handler/skill.go)、[Squad handler](../../server/internal/handler/squad.go) | [Skill 视图](../../packages/views/skills/)、[Squad 视图](../../packages/views/squads/) | [CLI Skill 命令](../../server/cmd/multica/cmd_skill.go) |
| 自动化与 Autopilot | 规则、触发器、计划任务、运行记录和外部 webhook | [Autopilot handler](../../server/internal/handler/autopilot.go)、[公开 webhook 入口](../../server/internal/handler/autopilot_webhook.go) | [Autopilot 视图](../../packages/views/autopilots/) | [调度器](../../server/internal/scheduler/) |
| 聊天、收件箱与通知 | 对话线程、跨来源消息、待处理事项、通知偏好和实时更新 | [聊天 handler](../../server/internal/handler/chat.go)、[收件箱 handler](../../server/internal/handler/inbox.go) | [聊天视图](../../packages/views/chat/)、[实时同步](../../packages/core/realtime/use-realtime-sync.ts) | [Mobile 聊天](../../apps/mobile/components/chat/) |
| 文件与附件 | 上传、关联、下载、文本/媒体预览和存储后端 | [文件 handler](../../server/internal/handler/file.go)、[存储实现](../../server/internal/storage/) | [编辑器与预览](../../packages/views/editor/) | [CLI 附件命令](../../server/cmd/multica/cmd_attachment.go) |
| 集成与代码托管 | GitHub 等 VCS、Slack、Lark、钉钉、企业微信、Telegram 和外部事件 | [集成 handler](../../server/internal/handler/)、[VCS webhook](../../server/internal/handler/vcs_webhook.go) | [设置中的集成界面](../../packages/views/settings/components/) | [CLI 仓库命令](../../server/cmd/multica/cmd_repo.go) |
| Room | 多智能体讨论、参与者、轮次、预算、产物、复用和结果闭环 | [Room handler](../../server/internal/handler/room.go)、[Room service 层](../../server/internal/service/) | [Room 视图](../../packages/views/rooms/) | [Room 设计规格](../../docs/downstream/specs/rooms-outcome-loop.md) |
| Wiki、LM Wiki 与 Twin | 知识页面、来源策略、修订、提案、知识激活和 Twin 执行简报 | [Wiki handler](../../server/internal/handler/wiki.go)、[LM Wiki service](../../server/internal/service/lm_wiki.go)、[Twin service](../../server/internal/service/twin.go) | [Wiki 视图](../../packages/views/wiki/)、[Twin 视图](../../packages/views/twins/) | [知识执行闭环规格](../../docs/downstream/specs/twin-wiki-knowledge-execution-loop.md) |
| Skill Evolution | 从证据、评估、提案、修订到发布的 Skill 改进闭环 | [Skill Evolution 服务](../../server/internal/skillevolution/)、[相关 SQL 查询](../../server/pkg/db/queries/workspace_skill_evolution.sql) | [Skill Evolution 视图](../../packages/views/skill-evolution/) | [自改进 Skill 规格](../../docs/downstream/specs/loosely-coupled-self-improving-skills.md) |

## 一项功能通常如何跨层

按下面顺序定位，可以减少只修改一层造成的行为漂移：

1. **数据与约束**：检查 [迁移](../../server/migrations/) 和 [sqlc 查询源](../../server/pkg/db/queries/)；生成的 Go 位于 `server/pkg/db/generated/`，不应手工修改。
2. **协议与业务语义**：HTTP 边界通常在 `server/internal/handler/`，跨 handler 的业务编排通常在 `server/internal/service/`。
3. **共享客户端协议**：Web/Desktop 的 API schema、Query、mutation 和 store 位于 [packages/core](../../packages/core/)。
4. **共享业务界面**：相同的 Web/Desktop 页面和组件位于 [packages/views](../../packages/views/)；平台路由只留在各 app。
5. **平台 wiring**：Next.js wiring 在 [apps/web](../../apps/web/)，Electron wiring 在 [apps/desktop](../../apps/desktop/)。Mobile 独立实现，先阅读 [Mobile 规则](../../apps/mobile/CLAUDE.md)。
6. **实时与缓存**：确认事件是否发布、Query key 是否含 `wsId`，以及 [实时同步协调器](../../packages/core/realtime/use-realtime-sync.ts) 是否需要失效或补丁。
7. **权限与兼容**：确认工作区成员、角色、机器身份和人类操作边界；网络 JSON 要经过 Zod schema 与 `parseWithFallback`。
8. **测试与文档**：把行为矩阵放在唯一的规范测试层，并同步产品文档、内置 Skill 及 source map。

## 选择客户端实现位置

- Web 与 Desktop 可共享的无界面业务逻辑放在 `packages/core/`。
- Web 与 Desktop 可共享的业务页面放在 `packages/views/`。
- Next.js、Electron 或路由器 API 留在对应 app 的平台层。
- Mobile 只复用 `@multica/core` 的类型和纯函数，拥有独立 UI、状态、数据层与发布节奏。
- 原子 UI 放在 `packages/ui/`，不得依赖业务包。

完整硬约束以 [CLAUDE.md](../../CLAUDE.md) 为准。

## 相关页面

- [代码库概览](../overview/index.md)
- [系统架构](../overview/architecture.md)
- [安全边界](../security/index.md)
- [CLI 命令参考](../reference/cli-commands.md)
- [参与贡献](../how-to-contribute/index.md)
- [下游扩展](../background/downstream-extensions.md)
