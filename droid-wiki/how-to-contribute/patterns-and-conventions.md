# 模式与约定

本页汇总修改 Multica 时最容易违反、又很难只靠局部代码推断出的规则。权威来源是 `CLAUDE.md`、`AGENTS.md`、`apps/mobile/CLAUDE.md` 与 `apps/docs/content/docs/developers/conventions.zh.mdx`。

## 包边界

| 位置 | 可以做什么 | 禁止 |
| --- | --- | --- |
| `packages/core/` | 类型、API、Query、mutation、Zustand、纯逻辑 | `react-dom`、`localStorage`、`process.env`、UI 库 |
| `packages/ui/` | 无业务语义的原子组件、样式和 Markdown 基础 | 导入 `@multica/core`、业务逻辑 |
| `packages/views/` | Web/Desktop 共用业务页面与组件 | `next/*`、`react-router-dom`、本地 store |
| `apps/web/platform/` | Next.js 路由与平台 API | 把平台 API 泄漏进共享包 |
| `apps/desktop/src/renderer/src/platform/` | React Router 与 Electron 导航适配 | 复制共享业务页面 |
| `apps/mobile/` | 原生 UI、自有 Query/store/realtime | 导入 Web/Desktop 页面或 store |

相同业务逻辑如果同时出现在 Web 和 Desktop，应提取到共享包。Mobile 只共享 `@multica/core` 类型和纯函数，产品语义一致但实现独立。

## 状态所有权

TanStack Query 拥有所有从 API 获取的服务端状态。Zustand 只保存筛选、草稿、弹窗、标签页布局和偏好等客户端状态。工作区身份由路由驱动，平台 store 只为 header、持久化命名空间和重连镜像 slug/id。

标准查询和 mutation 分别放在 `packages/core/<domain>/queries.ts` 与 `packages/core/<domain>/mutations.ts`。工作区 key 必须包含 `wsId`。Zustand selector 必须返回稳定引用，不能在 selector 内每次创建新对象或数组。

乐观更新只用于结果可预测、用户留在同一页面、失败少且回滚简单的字段修改。创建、删除、离开工作区等会导航或确认的流程必须等待服务端。聊天使用 `packages/core/chat/pending.ts` 的 pending-message 模式。

## API 兼容

已安装的 Desktop 可能连接较新的后端，所以网络 JSON 不能直接断言为 TypeScript 类型。使用 `packages/core/api/schema.ts` 的 `parseWithFallback` 与 `packages/core/api/schemas.ts` 的 Zod schema：

```ts
const body: unknown = await response.json();
return parseWithFallback(body, issueSchema, fallbackIssue, {
  endpoint: "GET /api/issues/:id",
});
```

新增或修改端点时同步更新 schema，并增加畸形响应测试。下游可选链并提供默认值；布尔字段优先显式比较 `=== true`；服务端枚举 switch 必须有 `default`。

## Go Handler 与 UUID

`server/internal/handler/handler.go` 提供三类 UUID 路径：

- 可为 UUID 或人类可读 ID 的 path 参数，先通过 `loadIssueForUser` 等 loader 解析，写操作使用加载后实体的 `ID`。
- 纯 UUID 请求输入使用 `parseUUIDOrBadRequest`，失败立即返回 400。
- sqlc 返回值和测试夹具等可信回环使用 panic 版 `parseUUID`。

Handler 处理协议边界，复杂事务进入 `server/internal/service/` 或领域包。错误响应使用 `writeError` 或带稳定机器码的 `writeErrorCode`；revision 冲突使用 409 与 `revision_conflict`。

## 数据库与迁移

仓库不创建数据库 foreign key，也不使用级联删除或更新。关系验证与依赖清理在应用层显式完成，需要原子性时使用事务。

所有索引迁移必须：

1. 使用 `CREATE INDEX CONCURRENTLY` 或 `CREATE UNIQUE INDEX CONCURRENTLY`。
2. 每个索引独占一个单语句迁移文件。
3. 提供配对的 `.up.sql` 与 `.down.sql`。

已发布迁移的完整文件 stem 和 SQL 内容不可修改。`docs/downstream/upstream-sync.md` 记录 fork 曾出现的编号碰撞、别名与前向修复。涉及迁移时要对新数据库和携带旧下游 ledger 的数据库都运行升级，并再跑受影响功能测试。

## 实时事件

服务端事件由 `server/internal/events/` 与 `server/pkg/protocol/events.go` 定义。Web/Desktop 的消费集中在 `packages/core/realtime/use-realtime-sync.ts`：

- payload 足够完整且目标 cache 确定时直接 patch。
- 无法预测筛选投影或权限结果时 invalidate。
- 事件不能把服务端实体镜像到 Zustand。

Mobile 在 `apps/mobile/data/realtime/` 独立实现。列表级订阅挂在工作区 layout，单记录订阅挂在对应 screen；在蜂窝网络下优先 patch，重连只 invalidate 自己拥有的 key。

## 路由与导航

共享代码使用 `useNavigation().push()` 或 `<AppLink>`。工作区路由使用 `/{slug}/{section}`；新增根路由必须是单词或 `/{noun}/{verb}`。新增保留 slug 时修改 `server/internal/handler/reserved_slugs.json` 并运行 `pnpm generate:reserved-slugs`。

Desktop 的工作区页面属于 session route。创建工作区、接受邀请等前置流程属于 `WindowOverlay`，定义于 `apps/desktop/src/renderer/src/stores/window-overlay-store.ts`，不能塞进 `apps/desktop/src/renderer/src/routes.tsx`。

## UI 与中文

优先复用 `packages/ui/components/ui/` 中的 Base UI / shadcn 组件。产品组件只使用 `packages/ui/styles/tokens.css` 定义的语义 token 与角色字号，不使用 Tailwind 默认字号或硬编码颜色。选中态在 hover 时仍必须可识别。

中文术语遵守 `apps/docs/content/docs/developers/conventions.zh.mdx`：

- Issue 译为“任务”，task 保留英文以避免混淆。
- Agent、Workspace、Project、Runtime 分别译为“智能体”“工作区”“项目”“运行时”。
- skill 保留小写英文，Autopilot 译为“自动化”。
- 代码注释一律英文。

## 测试放置

| 行为 | 测试位置 |
| --- | --- |
| 共享逻辑、Query、store | `packages/core/*.test.ts` |
| 共享页面、表单、弹窗 | `packages/views/*.test.tsx` |
| Web/Desktop 平台 wiring | 对应 `apps/web/` 或 `apps/desktop/` |
| Mobile | `apps/mobile/` |
| 端到端行为 | `e2e/*.spec.ts` |
| 后端 | 对应 `server/` Go package |

不需要 DOM 的 Vitest 文件以 `// @vitest-environment node` 开头。共享组件行为不能在 app 层重复测试。DB-backed Go 测试使用 `server/internal/testutil/` 的 fixture 与 `Call(...).Want(...).JSON(...)`。

## 修改入口

先确认行为属于哪个边界：

1. 协议或持久化规则，从 `server/cmd/server/router.go`、对应 handler/service 与 `server/pkg/db/queries/` 开始。
2. Web/Desktop 共享行为，从 `packages/core/<domain>/` 和 `packages/views/<domain>/` 开始。
3. 平台 wiring，从 `apps/web/platform/` 或 `apps/desktop/src/renderer/src/platform/` 开始。
4. Mobile 行为，先按 `apps/mobile/CLAUDE.md` 完成语义对照和交互预检。

## 关键源文件

| 文件 | 用途 |
| --- | --- |
| `CLAUDE.md` | 架构、状态、数据库、平台和测试硬规则 |
| `apps/docs/content/docs/developers/conventions.zh.mdx` | 命名、术语与中文风格 |
| `apps/mobile/CLAUDE.md` | Mobile 预检、数据、实时与 UI 规则 |
| `packages/core/api/schema.ts` | API 漂移防线 |
| `server/internal/handler/handler.go` | Handler 依赖和边界 helper |
| `docs/downstream/upstream-sync.md` | fork 所有权和迁移兼容账本 |

实际开发步骤见[添加功能](adding-a-feature.md)，验证策略见[测试](testing.md)。
