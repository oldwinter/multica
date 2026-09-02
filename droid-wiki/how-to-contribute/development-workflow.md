# 开发工作流

本页说明从领取修改到交付可审查变更的标准路径。命令以根 [`Makefile`](../../Makefile)、[`package.json`](../../package.json) 和仓库规则为准；具体分层约束见[模式与约定](patterns-and-conventions.md)。

## 1. 确认范围与所有权

开始前先定位行为的权威层：

| 修改类型 | 首选位置 |
| --- | --- |
| HTTP、权限、跨表事务 | `server/internal/handler/`、`server/internal/service/` |
| SQL 与 schema | `server/pkg/db/queries/`、`server/migrations/` |
| Web/Desktop API、Query、store | `packages/core/` |
| Web/Desktop 业务页面 | `packages/views/` |
| 无业务语义的组件与 token | `packages/ui/` |
| Next/Electron 平台 wiring | `apps/web/`、`apps/desktop/` |
| Mobile | `apps/mobile/`，先读 [`apps/mobile/CLAUDE.md`](../../apps/mobile/CLAUDE.md) |

若改动会进入 fork 与上游共同维护的区域，先读 [`docs/downstream/upstream-sync.md`](../../docs/downstream/upstream-sync.md)。下游功能应拥有深叶模块，只在组合根注册，避免把大量下游分支散进上游高冲突文件。

## 2. 建立隔离环境

不要把主 checkout 的 `.env` 复制进 worktree。推荐流程：

```bash
pnpm install
make up
make status
```

`make up` 会为当前 checkout 分配 API/Web/Desktop 端口和数据库名，并登记在 `~/.multica/dev/`。`make down` 只停止进程、保留数据；`make destroy` 会删除当前环境数据库和运行状态。完整生命周期见[本地开发](../operations/local-development.md)。

## 3. 先固定行为合同

行为变更优先在正确层写失败测试：

- 后端使用邻近 Go 测试；DB-backed 测试使用 `server/internal/testutil/` 的 fixture 与 handler caller。
- Query、schema、store 和纯转换在 `packages/core` 测。
- 共享页面和组件在 `packages/views` 测，不在 Web/Desktop app 层重复。
- 平台路由、cookie、search params、Electron IPC 在对应 app 测。
- Mobile 使用自己的 Node/RN 测试边界。

测试不应替实现断言产品规则。更完整的放置矩阵见[测试](testing.md)和[测试地图](../reference/test-map.md)。

## 4. 实现最小完整切片

跨层功能通常按以下顺序推进：

```mermaid
flowchart LR
    Contract["领域合同与权限"] --> Data["迁移 / SQL"]
    Data --> API["Handler / Service"]
    API --> Schema["客户端 schema / Query"]
    Schema --> View["共享或平台 UI"]
    View --> Realtime["事件与 cache 协调"]
    Realtime --> Verify["定向与跨层验证"]
```

每一步都要保持可编译。新增端点时同步 Zod schema 和畸形响应测试；新增事件时明确 patch 或 invalidate；新增 Web/Desktop 页面时两端都接线；Mobile 只镜像产品语义，不复制共享 React 页面。

## 5. 生成派生产物

只在源文件变化后运行对应生成器：

```bash
make sqlc                         # SQL → Go
pnpm generate:reserved-slugs      # reserved_slugs.json → TS
pnpm fumadocs-mdx                 # Docs 内容索引，按 workspace script 为准
```

生成文件不手工编辑或解决冲突。先合并源，再重新生成。数据库规则和迁移账本见[数据库变更](database-changes.md)。

## 6. 分层验证

先运行最窄测试，再按影响面扩大：

```bash
pnpm --filter @multica/core test
pnpm --filter @multica/views test
(cd server && go test ./internal/handler -run 'TestRelevantBehavior' -count=1)

pnpm typecheck
pnpm test
make test
```

高风险跨端流程再运行 `pnpm exec playwright test` 或对应 E2E。不要声称未运行的检查通过；失败若与现有工作区状态无关，也要记录命令和失败原因。

## 7. 自审与交付

提交前检查：

- diff 只包含当前任务文件，没有覆盖用户未提交修改；
- 代码、测试、文档和 built-in Skill source map 同步；
- API 兼容、workspace 隔离、UUID 来源和数据库规则未被绕过；
- 新 UI 在窄屏、长文本、键盘和选中态 hover 下可用；
- commit 使用仓库约定的 conventional prefix。

本流程不自动 push、merge、发布或部署。发布入口、tag 和 CI 见[CI、测试与发布](../operations/ci-testing-and-release.md)。

## 相关页面

- [添加功能](adding-a-feature.md)
- [前端共享](frontend-sharing.md)
- [文档与本地化](documentation-and-localization.md)
- [调试](debugging.md)
