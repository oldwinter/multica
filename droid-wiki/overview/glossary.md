# 术语表

本页解释代码、数据库、API 和产品文案中反复出现的 Multica 术语。中文用词以 `apps/docs/content/docs/developers/conventions.zh.mdx` 为准；代码标识符保留英文。

| 术语 | 含义 | 主要代码 |
| --- | --- | --- |
| 工作区（Workspace） | 租户和协作边界。查询必须按 `workspace_id` 过滤，`X-Workspace-ID` 或 slug 选择当前工作区。 | `server/internal/middleware/workspace.go`、`packages/core/workspace/` |
| 任务（Issue） | 团队要完成的一件事。可有负责人、状态、项目、子任务、评论和多次执行。代码/API/DB 仍使用 `issue`。 | `server/internal/service/issue.go`、`packages/core/issues/` |
| task | 智能体的一次执行。一个任务、聊天会话、自动化或 Room 可以产生多个 task。中文文案保留 `task` 以区别于任务。 | `server/internal/service/task.go` |
| 智能体（Agent） | 可被分配工作的第一类主体，关联运行时、skill、MCP、环境变量和权限范围。 | `server/internal/handler/agent.go`、`packages/core/agents/` |
| 运行时（Runtime） | 守护进程在某台机器上注册的智能体 CLI 能力。运行时和智能体是多对多配置关系中的执行端。 | `server/internal/handler/runtime.go` |
| 守护进程（Daemon） | 运行在用户机器上的执行进程，检测 CLI、领取 task、准备目录并回传执行结果。 | `server/internal/daemon/` |
| skill | 可复用的智能体工作说明和文件包。中文产品文案保留小写 `skill`。 | `server/internal/skill/`、`packages/core/skills/` |
| Skill Evolution | 从 task 复盘、精确反馈与证据中生成、评估并调度 skill 改进的下游系统。 | `server/internal/skillevolution/` |
| 项目（Project） | 对任务和资源进行分组的业务容器。 | `server/internal/handler/project.go` |
| 自动化（Autopilot） | 按 cron、Webhook 或事件规则触发 task 或创建任务的自动化配置。 | `server/internal/service/autopilot.go` |
| Squad | 由人和智能体组成的小队，包含 leader 与成员角色，可参与任务路由和 Room。 | `server/internal/handler/squad.go` |
| Room | 持续协作空间。成员向 facilitator 或参与智能体发消息，系统按 cycle/turn 组织运行、记忆、审查和产物提升。 | `server/internal/room/` |
| Wiki | 工作区或个人知识页，具有 revision、编辑冲突、提案与搜索。 | `server/internal/handler/wiki.go` |
| LM Wiki | 从工作区证据汇总出的受控知识层，保留来源策略、版本与证据状态。 | `server/internal/service/lm_wiki.go` |
| Twin | 基于已接受 LM Wiki 证据构建并演进的工作画像，可生成执行 briefing、binding 和 deposition。 | `server/internal/service/twin.go` |
| 收件箱（Inbox） | 面向成员的注意力队列，聚合任务、评论、Room 阻塞等通知。 | `server/internal/handler/inbox.go` |
| 来源上下文（Source Context） | 从外部消息或快速创建捕获的证据快照，随 task 传递并在期限后清理。 | `server/internal/service/source_context.go` |
| 归因（Attribution） | 记录谁发起、谁负责、证据来自哪里以及智能体委派链。 | `server/internal/attribution/` |
| Pending message | 聊天发送时先显示带 pending 状态的本地消息，失败可重试，不使用无声乐观更新。 | `packages/core/chat/pending.ts` |
| Query cache | TanStack Query 管理的服务端状态副本，由 REST 与 WebSocket patch/invalidate。 | `packages/core/query-client.ts` |
| Store | Zustand 管理的客户端偏好、草稿、筛选和布局，不保存权威服务端实体。 | `packages/core/*/stores/` |
| 运行清单（Execution Manifest） | task 领取时固定的 skill、插件和运行配置版本，避免执行中配置漂移。 | `server/pkg/skillbundle/execution_manifest.go` |
| Channel engine | Slack、Lark、钉钉、企业微信和 Telegram 共用的安装租约、消息解析和会话路由。 | `server/internal/integrations/channel/engine/` |
| Realtime relay | 多 API 节点之间转发工作区事件和守护进程唤醒的 Redis Stream 层。 | `server/internal/realtime/` |
| MCP | Model Context Protocol。工作区、智能体、插件与 Composio 可以向一次 task 提供工具。 | `server/pkg/remotemcp/`、`server/internal/daemon/remote_mcp_broker.go` |
| Plugin | 带 manifest 的可安装扩展，可声明 surface、hook、Action API、存储、MCP 与定时触发。 | `server/internal/service/plugin.go` |
| 命名皮肤（Skin） | `tension`、`relay`、`field` 等独立于明暗模式的视觉材料选择。 | `packages/core/appearance/`、`packages/ui/styles/tokens.css` |
| Tension Map | Multica 的设计系统，强调浅色结构面、深色工作面、信号线和语义色。 | `DESIGN.md` |
| 上游（upstream） | `multica-ai/multica`。本 fork 定期合并其 `main`，同时保留本地下游功能。 | `docs/downstream/upstream-sync.md` |
| 迁移身份 | `schema_migrations` 记录的完整迁移文件 stem。已发布身份不可改名或改写。 | `server/cmd/migrate/main.go` |

## 状态词

任务状态的 schema 标识符保留英文：`backlog`、`todo`、`in_progress`、`in_review`、`done`、`blocked`、`cancelled`。工作区可配置自定义状态，因此服务端枚举 switch 和客户端渲染必须保留未知值兜底。状态目录位于 `server/internal/issuestatus/` 与 `packages/core/issue-statuses/`。

task 生命周期常见值包括 `queued`、`dispatched`、`running`、`waiting_local_directory`、`completed`、`failed` 和 `cancelled`。领取与重试语义集中在 `server/internal/service/task.go`、`server/pkg/taskfailure/` 与 `server/internal/daemon/`。

## 身份与权限词

- `owner`、`admin`、`member` 是工作区角色，中文 UI 保留小写英文。
- `originator_user_id` 表示最初授权执行的顶层人类。
- `accountable_user_id` 表示对该次执行负责的人。
- `actor` 是当前直接执行操作的身份，可能来自成员、智能体、task token 或系统。
- `assignee_type + assignee_id` 表示多态负责人，可以是成员或智能体。

这些字段的分类和 fail-closed 路径见 `server/internal/attribution/attribution.go` 与 `server/internal/service/task.go`。

## 导航词

- **工作区前置路由**：用户进入工作区前的 `/login`、`/workspaces/new` 等路由。
- **工作区路由**：固定为 `/{slug}/{section}`。
- **Session route**：Desktop 中工作区范围的标签页目的地。
- **Transition flow**：Desktop 中创建工作区、接受邀请等一次性流程，以 `WindowOverlay` 表达而不是路由。

路由规范见 `apps/docs/content/docs/developers/conventions.zh.mdx`，Desktop 细节见 `apps/desktop/src/renderer/src/routes.tsx`。

更多实体关系见[领域模型](../reference/data-models.md)，跨层用户能力见[功能](../features/index.md)。
