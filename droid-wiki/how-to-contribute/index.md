# 参与贡献

本章把一次修改从定位代码、补测试、实现、验证到提交串成一条可复查的路径。硬约束以 [CLAUDE.md](../../CLAUDE.md) 为准；命名、状态所有权、API 兼容、迁移等细则可先查[模式与约定](patterns-and-conventions.md)。

提交补丁即表示同意 [CONTRIBUTING.md](../../CONTRIBUTING.md#contribution-terms) 所述贡献条款和 [Multica License](../../LICENSE)。

## 标准流程

1. **确认行为和影响面。** 找到现有同类实现，列出 API、权限、缓存、实时事件、导航、文案和跨客户端语义是否会变化。
2. **选择事实所在的层。** 协议与持久化放 Go；无界面业务逻辑放 `packages/core/`；Web/Desktop 共用界面放 `packages/views/`；平台 wiring 留在 app；Mobile 独立实现。
3. **在规范层写测试。** 行为规则只选一个 canonical 测试层，不把同一矩阵在 helper、组件和 app 中重复一遍。
4. **做最小实现。** 复用现有组件、Query key、mutation、handler helper 和 fixture，避免任务之外的重构或兼容层。
5. **先窄后宽验证。** 先运行受影响 package 或 Go package 的测试，再按风险运行类型检查、lint、构建、E2E 或 `make check`。
6. **检查生成物和文档。** SQL 改动运行 `make sqlc`；保留 slug 改动运行 `pnpm generate:reserved-slugs`；命令、API 字段或产品行为变更同步 built-in skill 及其 source map。
7. **审阅并提交。** 用 `git diff` 和 `git status` 确认范围，只提交相关文件；commit 使用 `feat(scope)`、`fix(scope)`、`refactor(scope)`、`docs`、`test(scope)` 或 `chore(scope)`，并按意图保持原子化。

## 选择修改层

| 变化 | 首选位置 | 配套检查 |
| --- | --- | --- |
| HTTP、权限、事务、事件发布 | `server/internal/handler/`、`server/internal/service/` | Go 单元/DB-backed 测试 |
| SQL 查询或数据结构 | `server/pkg/db/queries/`、`server/migrations/` | `make sqlc`、迁移和 Go 测试 |
| Web/Desktop API、Query、mutation、store | `packages/core/<domain>/` | core 测试、畸形响应测试 |
| Web/Desktop 业务页面或组件 | `packages/views/<domain>/` | views 测试，并接入两个 app |
| Next.js 或 Electron 路由/平台能力 | 对应 app 的 platform 层 | app wiring 测试 |
| Mobile 功能 | `apps/mobile/` | 先完成 Mobile pre-flight，再跑独立验证 |
| 原子 UI | `packages/ui/` | 无业务依赖、消费端验证 |
| 产品文档或翻译 | `apps/docs/content/docs/`、`packages/views/locales/` | 文档构建、locale parity 与术语检查 |

更具体的判断流程见[添加功能](adding-a-feature.md)、[前端共享](frontend-sharing.md)和[数据库变更](database-changes.md)。

## 常用命令

```bash
make dev                       # 初始化并以前台模式运行 API + Web
make up                        # 后台启动当前 checkout 的 api + web 环境
make status                    # 核对 PID、端口和 commit 所有权

pnpm typecheck
pnpm test                      # TS/Vitest；根脚本排除 Mobile
pnpm lint
make test                      # 迁移后运行 Go race 测试
pnpm exec playwright test
make check                     # typecheck → TS tests → Go tests → E2E
```

主 checkout 使用 `.env`，worktree 使用由 `make worktree-env` 生成的 `.env.worktree`；不要把 `.env` 复制到 worktree。完整环境流程见[本地开发](../overview/getting-started.md)和 [CONTRIBUTING.md](../../CONTRIBUTING.md)。

## 提交前 checklist

- [ ] 修改位于正确的包、平台或服务边界，没有重复已有实现。
- [ ] 行为测试先落在 canonical 层，测试位置符合[测试指南](testing.md)。
- [ ] 工作区作用域查询包含 `wsId`，服务端状态没有镜像进 Zustand。
- [ ] 网络响应经过 Zod schema 与 `parseWithFallback`，并覆盖畸形响应。
- [ ] 数据库改动遵守 no FK、concurrent index 和双向迁移规则。
- [ ] Web/Desktop 共同行为在共享包实现并完成两端 wiring；Mobile 保持独立。
- [ ] 文案符合术语表；相关 docs、locale、built-in skill/source map 已同步。
- [ ] 已运行并记录与风险相称的验证；没有声称未运行的检查通过。
- [ ] `git diff`、`git status` 和 commit 内容只包含本次意图。

## 相关页面

- [开发工作流](development-workflow.md)
- [添加功能](adding-a-feature.md)
- [测试指南](testing.md)
- [调试](debugging.md)
- [工具链](tooling.md)
- [数据库变更](database-changes.md)
- [前端共享](frontend-sharing.md)
- [文档与本地化](documentation-and-localization.md)
- [模式与约定](patterns-and-conventions.md)
- [系统架构](../overview/architecture.md)

## 事实来源

- [CLAUDE.md](../../CLAUDE.md)
- [CONTRIBUTING.md](../../CONTRIBUTING.md)
- [Makefile](../../Makefile)
- [package.json](../../package.json)
- [scripts/check.sh](../../scripts/check.sh)
