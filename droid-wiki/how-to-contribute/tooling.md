# 工具链

Multica 使用 Make、pnpm/Turborepo、Go 工具链、sqlc、Vitest、Playwright 和平台构建器。稳定入口在仓库根，workspace 脚本负责各自的框架细节。

## 根命令

| 工具 | 配置或入口 | 用途 |
| --- | --- | --- |
| Make | [`Makefile`](../../Makefile) | 环境生命周期、Go 服务、迁移、sqlc、测试 |
| pnpm | [`package.json`](../../package.json)、[`pnpm-workspace.yaml`](../../pnpm-workspace.yaml) | workspace 依赖与统一脚本 |
| Turborepo | [`turbo.json`](../../turbo.json) | build、typecheck、lint、test 任务图 |
| Go | [`server/go.mod`](../../server/go.mod) | API、CLI、Daemon 与后端包 |
| sqlc | [`server/sqlc.yaml`](../../server/sqlc.yaml) | SQL query 生成类型安全 Go |
| Playwright | [`playwright.config.ts`](../../playwright.config.ts) | 浏览器端到端测试 |

根 `pnpm build/typecheck/lint/test` 不包含 Mobile；Expo/RN 版本和测试必须从 `apps/mobile/package.json` 单独运行。

## TypeScript 质量边界

共享配置位于：

- [`packages/tsconfig/`](../../packages/tsconfig/)：strict TypeScript 基线；
- [`packages/eslint-config/`](../../packages/eslint-config/)：base、React、Next flat config；
- [`CLAUDE.md`](../../CLAUDE.md)：workspace 直接依赖与包边界规则；
- [`packages/ui/styles/check-token-contract.mjs`](../../packages/ui/styles/check-token-contract.mjs)：设计 token contract。

每个 workspace 必须声明自己直接 import 的外部包。共享版本使用 `pnpm-workspace.yaml` 的 `catalog:`；Expo/React Native 依赖由 Mobile 直接 pin。

## 数据库生成

修改 `server/pkg/db/queries/*.sql` 或迁移后运行：

```bash
make sqlc
git diff -- server/pkg/db/generated
```

`server/pkg/db/generated/` 是派生产物。冲突时合并 SQL 与 migration，再生成；不要手工编辑 Go 输出。

## UI 组件

从根目录添加组件：

```bash
pnpm ui:add badge
pnpm ui:add @reui/<name>
```

组件进入 `packages/ui/components/ui/`，业务组合进入 `packages/views/<domain>/`。ReUI 是源码分发，提示覆盖现有文件时选择 `n`，并在落盘后改写为仓库 token、字号和包边界。许可证密钥只能来自环境变量，不能写入仓库。

## 应用构建

| 应用 | 主要工具 |
| --- | --- |
| Web | Next.js App Router，`apps/web/next.config.ts` |
| Desktop | electron-vite + electron-builder，`apps/desktop/electron.vite.config.ts`、`apps/desktop/electron-builder.yml` |
| Mobile | Expo CLI/EAS，`apps/mobile/app.config.ts`、`apps/mobile/metro.config.js` |
| Docs | Next.js + Fumadocs MDX，`apps/docs/source.config.ts` |

完整命令和 CI 运行环境见[CI、测试与发布](../operations/ci-testing-and-release.md)。

## 常见陷阱

- 只运行根 pnpm 命令，遗漏 Mobile。
- 手工编辑 sqlc 或 reserved slug 生成文件。
- 在根 `package.json` 增加依赖，却不在实际消费 workspace 声明。
- 直接运行开发服务而绕过 worktree 端口/数据库分配。
- 把 app 平台 API引入 `packages/core`、`packages/ui` 或 `packages/views`。

## 相关页面

- [开发工作流](development-workflow.md)
- [测试](testing.md)
- [共享包](../packages/index.md)
- [仓库地图](../reference/repository-map.md)
