# 前端共享

Web 与 Desktop 共享业务逻辑和业务界面；Mobile 保持独立。依赖方向固定为 `packages/views -> packages/core + packages/ui`，而 `core` 与 `ui` 彼此独立。

## 放置矩阵

| 位置 | 所有内容 | 不得出现 |
| --- | --- | --- |
| `packages/core/` | 类型、纯函数、API、schema、Query、mutation、Zustand、Web/Desktop realtime | `react-dom`、`localStorage`、`process.env`、UI 库、app API |
| `packages/ui/` | 无业务语义的原子组件与设计 token | `@multica/core`、领域逻辑 |
| `packages/views/` | Web/Desktop 共用业务页面与组件 | `next/*`、`react-router-dom`、本地 store |
| `apps/web/platform/` | Next.js navigation 与平台 API | 可共享业务逻辑 |
| `apps/desktop/src/renderer/src/platform/` | React Router/Electron adapter | 共享业务页面的副本 |
| `apps/mobile/` | 原生 UI、自有状态、数据层、实时和发布流程 | Web/Desktop 页面、store、adapter |

相同逻辑出现在 Web 与 Desktop 时应提取，即使重复很小。每个 workspace 必须在自己的 `package.json` 声明直接 import 的外部依赖。

## Web/Desktop 新功能流程

1. **Headless 行为放 core。** 在 `packages/core/<domain>/` 定义类型、Zod schema、API 方法、Query key factory、query/mutation、cache updater 或 client store。
2. **业务 UI 放 views。** 在 `packages/views/<domain>/` 组合 `@multica/core` 与 `@multica/ui`，不直接调用框架路由。
3. **注入导航。** 共享代码使用 `useNavigation().push()` 或 `<AppLink>`；Web 与 Desktop 分别提供 adapter。
4. **同时接入两端。** 共享页面应增加 Web App Router wiring 和 Desktop router wiring，除非 Desktop 需求属于一次性 transition overlay。
5. **按所有者测试。** core 测逻辑和边界，views 测共享组件，app 只测平台 wiring。

关键实现：

- [Web navigation adapter](../../apps/web/platform/navigation.tsx)
- [Desktop navigation adapter](../../apps/desktop/src/renderer/src/platform/navigation.tsx)
- [共享 navigation API](../../packages/core/navigation/)
- [共享 views](../../packages/views/)

## 状态和实时同步

- TanStack Query 拥有 API 服务端状态；Zustand 只拥有 filter、draft、modal、tab layout、navigation history 等客户端状态。
- 共享 Zustand store 只放 `packages/core/`，selector 必须返回稳定引用。
- 工作区身份由路由驱动；平台 store 只能为 header、持久化 namespace 和重连镜像 slug/id。
- workspace-scoped query key 必须包含 `wsId`；需要 workspace 的 hook 优先显式接收 `wsId`。
- WebSocket payload 足够完整、目标 cache 确定时 patch；无法可靠预测筛选投影或权限时 invalidate。
- WebSocket 不得把服务端实体镜像到 Zustand。

Web/Desktop 的集中协调器是 [`packages/core/realtime/use-realtime-sync.ts`](../../packages/core/realtime/use-realtime-sync.ts)。创建、删除和离开等导航流程等待服务器；聊天使用 pending-message 模式；简单字段 patch 仅在可预测、原屏、低失败率、易回滚时 optimistic。

## 平台边界

### Web

只有 `apps/web/platform/` 使用 Next.js navigation/platform API。工作区页面仍消费共享 views，不在 app 层复制业务组件。

### Desktop

- 工作区页面是 session route。
- 创建工作区、接受邀请等前置一次性动作使用 `WindowOverlay`，不新增 Desktop route。
- 跨工作区导航必须走 adapter，以便调用 `switchWorkspace`。
- 离开工作区上下文时显式 `setCurrentWorkspace(null, null)`。
- 全窗口非 dashboard 视图把 `<DragStrip />` 作为第一个 flex child。

Desktop 规则来源见 [CLAUDE.md](../../CLAUDE.md#desktop-rules)。

## Mobile 独立性

Mobile 可以 `import type` `@multica/core/types/*`，也可导入纯函数；其他内容由 Mobile 自己实现。它不能复用 Web/Desktop 的 React 页面、Zustand store、Query key runtime、平台 adapter 或 realtime hook。

产品语义必须一致：

- counts 与 visibility；
- permissions 与 access；
- enums、状态转换和未知值 fallback；
- `id`、`slug` 和 canonical fields；
- 跨 cache side effect。

UI 与交互可以为手机原生体验不同。任何 Mobile 功能编码前必须执行 [`apps/mobile/CLAUDE.md`](../../apps/mobile/CLAUDE.md#pre-flight--before-you-write-any-code) 的 pre-flight：阅读真实 Web/Desktop 实现、展示交互与 parity 方案、等待明确执行指令。Mobile 使用独立的 `apps/mobile/data/`、`apps/mobile/data/realtime/` 和 QueryClient；根 `pnpm test` 不覆盖它，验证用：

```bash
pnpm -C apps/mobile typecheck
pnpm -C apps/mobile lint
pnpm -C apps/mobile test
```

## checklist

- [ ] 逻辑、原子 UI、业务 UI 和平台 wiring 位于正确层。
- [ ] Web/Desktop 没有小段重复；共享 view 已在两端接入。
- [ ] `packages/views` 没有框架路由 import 或 store。
- [ ] server state 属于 Query，client state 才属于 Zustand。
- [ ] API 响应经过 schema，Desktop 版本漂移有 fallback。
- [ ] Query key 含 `wsId`，实时事件不会镜像服务端状态到 store。
- [ ] Mobile 只共享类型/纯函数，并完成语义 parity 与独立验证。
- [ ] 新外部依赖在每个直接使用它的 workspace 声明。

## 相关页面

- [参与贡献](index.md)
- [添加功能](adding-a-feature.md)
- [测试指南](testing.md)
- [模式与约定](patterns-and-conventions.md)
- [前端共享架构图](../overview/architecture.md#前端共享边界)
- [Mobile verify workflow](../../.github/workflows/mobile-verify.yml)
