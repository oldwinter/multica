# 添加功能

新增功能时，先确定行为属于哪一层，再让测试、实现、平台 wiring、文档和验证沿同一边界展开。不要从“在哪个页面看到它”反推所有代码都应放在 app 里。

## 1. 写出变更契约

动代码前列出：

- 用户动作、成功结果、失败结果和权限；
- API 请求/响应、数据库对象和工作区隔离；
- Query cache、Zustand、实时事件和断线重连；
- Web、Desktop、Mobile 是否必须保持相同语义；
- 路由、文案、CLI、built-in skill 或公开文档是否受影响。

先搜索同类功能，复用现有 handler/service、Query key factory、mutation、组件和测试 fixture。命名与状态规则见[模式与约定](patterns-and-conventions.md)。

## 2. 选择实现层

```text
数据库/协议变化
  └─ server/migrations + queries + handler/service + event
       └─ packages/core schema/query/mutation/realtime
            └─ packages/views shared business UI
                 ├─ apps/web platform wiring
                 └─ apps/desktop platform wiring

Mobile：读取上述产品语义，但在 apps/mobile 独立实现
```

- **服务端状态**由 TanStack Query 管理；工作区 query key 必须含 `wsId`。
- **客户端/视图状态**才放 Zustand，且共享 store 只在 `packages/core/`。
- **React Context**只用于平台 plumbing。
- 创建、删除、离开等会导航或确认的流程等待服务端；只有结果可预测、停留原屏、失败少且易回滚的字段 patch 才做 optimistic update。
- 网络 JSON 用 [`parseWithFallback`](../../packages/core/api/schema.ts) 与 Zod schema 解析，不用 `as T`。

若相同功能同时服务 Web 和 Desktop，按[前端共享](frontend-sharing.md)放置；若涉及 Mobile，先读 [`apps/mobile/CLAUDE.md`](../../apps/mobile/CLAUDE.md) 并完成其 pre-flight。

## 3. 先放 canonical 测试

按[测试指南](testing.md)选择一个行为所有者：

- Go 业务和事务在对应 package 测试；
- API 漂移在 `packages/core` schema/helper 测试；
- 共享 UI 在 `packages/views` 测试；
- 平台 adapter 在 app 测试；
- 关键跨层用户流程才进入 Playwright。

优先写最小失败用例，明确权限、状态转换、未知字段或回归条件；不要把 helper 的组合矩阵再次装进组件。

## 4. 实现最窄闭环

### 后端闭环

1. 若需要，增加双向迁移和 sqlc 查询。
2. 在 handler 解析边界输入、校验 membership/permission；复杂事务放 service。
3. UUID path 若支持 UUID 或人类可读 ID，先通过 loader 解析，写入时使用实体 `ID`；纯 UUID 输入用 `parseUUIDOrBadRequest`。
4. 事务提交后发布领域/实时事件，确保所有查询都按 `workspace_id` 过滤。
5. 修改查询后运行 `make sqlc`，不要手改 `server/pkg/db/generated/`。

详见[数据库变更](database-changes.md)。

### Web/Desktop 闭环

1. 类型、schema、API、Query、mutation、cache updater 放 `packages/core/<domain>/`。
2. 共享业务页面/组件放 `packages/views/<domain>/`，导航只用 `useNavigation()` 或 `<AppLink>`。
3. 分别接入 `apps/web/` 与 Desktop router/platform；不要把 Next.js 或 React Router import 带入共享包。
4. 事件 payload 完整且目标 cache 确定时 patch；投影不确定时 invalidate；不把服务端 payload 写入 Zustand。
5. 已安装 Desktop 可能连接较新 backend，因此 optional-chain/default、显式 boolean 和 enum `default` 都是兼容要求。

### Mobile 闭环

Mobile 只可从 `@multica/core` 导入类型和纯函数，拥有自己的 UI、Query/store、API helper、实时 updater 和发布节奏。开始编码前：

1. 阅读真实 Web/Desktop 实现，列出 counts、permissions、enums/transitions、data identity 与跨 cache side effect。
2. 向用户展示原生交互方案、语义对齐点和必须不同之处。
3. 等待明确的执行指令。

Mobile UI 可以不同，产品语义不能不同；不要直接导入 Web/Desktop 页面、store 或实时 updater。

## 5. 同步契约与生成物

- 改 `server/pkg/db/queries/*.sql`：运行 `make sqlc` 并提交生成代码。
- 改保留 slug：编辑 [`server/internal/handler/reserved_slugs.json`](../../server/internal/handler/reserved_slugs.json)，运行 `pnpm generate:reserved-slugs`，提交生成的 [`packages/core/paths/reserved-slugs.ts`](../../packages/core/paths/reserved-slugs.ts)。
- 改 CLI 命令/flag、API 字段或 built-in skill 描述的产品行为：同一 PR 更新对应 `SKILL.md` 与 `references/*-source-map.md`。
- 改用户可见文案或文档：按[文档与本地化](documentation-and-localization.md)同步语言和术语。
- 新增外部依赖：每个直接 import 它的 workspace 都在自己的 `package.json` 声明；共享版本用 `catalog:`，Expo/RN 相关依赖由 Mobile 直接 pin。

## 6. 验证与提交

先运行受影响 package/Go package 的测试，再按修改范围运行：

```bash
pnpm typecheck
pnpm test
pnpm lint
make test
pnpm exec playwright test
make check
```

提交前：

- [ ] 功能形成从持久化/API 到缓存/UI 的最小闭环。
- [ ] 测试在 canonical 层，错误与版本漂移路径有覆盖。
- [ ] Web/Desktop 两端 wiring 完整；Mobile 边界和语义对齐清楚。
- [ ] 迁移、sqlc、保留 slug、docs、locale、skill/source map 已同步。
- [ ] `git diff` 没有生成物漂移、调试代码或无关重构。
- [ ] commit 原子且使用 Conventional 前缀。

## 关键源码

- [server router](../../server/cmd/server/router.go)
- [API schema compatibility helper](../../packages/core/api/schema.ts)
- [Web navigation adapter](../../apps/web/platform/navigation.tsx)
- [Desktop navigation adapter](../../apps/desktop/src/renderer/src/platform/navigation.tsx)
- [Web/Desktop realtime sync](../../packages/core/realtime/use-realtime-sync.ts)
- [Mobile rules](../../apps/mobile/CLAUDE.md)
