# 测试指南

测试跟着行为的所有者走。先用最窄测试建立反馈，再用更宽的检查证明包边界、生成物和端到端流程没有被破坏。

## 测试放在哪里

| 被测行为 | canonical 位置 |
| --- | --- |
| 共享纯逻辑、store、Query、mutation、hook | `packages/core/**/*.test.ts(x)` |
| 共享页面、组件、表单、弹窗 | `packages/views/**/*.test.tsx` |
| Next.js cookie、redirect、search params 等 wiring | `apps/web/**/*.test.ts(x)` |
| Electron 路由、preload、平台 wiring | `apps/desktop/src/**/*.test.ts(x)` |
| Mobile 纯 helper 与 headless data logic | `apps/mobile/lib/**/*.test.ts`、`apps/mobile/data/**/*.test.ts` |
| 完整用户流程 | `e2e/*.spec.ts` |
| Go 领域、handler、迁移 | 对应 `server/` package 的 `*_test.go` |

每个产品行为只保留一个 canonical 矩阵。纯解析、状态转换和边界组合放在 helper 旁；组件测试保留 happy path、wiring、可访问性和具名回归，不要再经 DOM 重跑 helper 的整套矩阵。共享组件行为不能移到 app 测试。

## TypeScript 与组件测试

- 测试与源文件同目录，命名 `<file>.test.ts` 或 `<file>.test.tsx`。
- 无需 DOM 的 Vitest 文件以 `// @vitest-environment node` 开头；如果被测代码按 `window` 或 `document` 分支，则不能用该声明掩盖浏览器路径。
- `packages/views/` 测试不得 mock `next/*` 或 `react-router-dom`。
- mock `@multica/core` store 时保留 Zustand callable-store 形状，即 selector 函数与 `getState`。
- API 调用 mock `@multica/core/api`，不要绕过共享边界。
- API 端点新增或变化时，为 Zod schema 增加字段缺失、类型错误等畸形响应测试。

各 workspace 的 Vitest 配置见 [core](../../packages/core/vitest.config.ts)、[views](../../packages/views/vitest.config.ts)、[web](../../apps/web/vitest.config.ts)、[desktop](../../apps/desktop/vitest.config.ts) 和 [mobile](../../apps/mobile/vitest.config.ts)。Web、Desktop、Views 默认使用 jsdom；Mobile 的 Vitest 只在 Node 环境运行纯函数和 headless data 测试，不伪装 RN 组件环境。

## Go 与数据库测试

新 DB-backed 测试使用 [`server/internal/testutil`](../../server/internal/testutil/)：

- 用 `dbfx.Issue`、`dbfx.Task`、`dbfx.Insert` 创建行，不手写 `INSERT ... RETURNING id` 加 cleanup。
- 用 `testutil.Call(h, req).Want(status).JSON(&out)` 驱动 handler，不重复 recorder、状态检查和 JSON decode。
- helper 负责夹具和协议机械步骤，不替测试断言产品规则。
- 如果测试自己的错误消息包含 case 上下文，共享 `.Want()` 无法表达时保留原断言。

默认测试不得查找或执行用户安装的真实智能体 CLI。为 subprocess 测试传入测试创建的 fake executable 或 missing path；新增默认 agent command 时同步 [`scripts/agent-cli-command-names.txt`](../../scripts/agent-cli-command-names.txt)。真实账号 smoke test 只能放在 `agentintegration` build tag 后，并在得到明确授权时设置 `MULTICA_RUN_REAL_AGENT_SMOKE=1`。

## 运行策略

### 1. 先跑受影响范围

```bash
pnpm -C packages/core test
pnpm -C packages/views test
pnpm -C apps/web test
pnpm -C apps/desktop test
pnpm -C apps/mobile test

(cd server && go test ./internal/service -run TestName -count=1)
```

Mobile 不在根 `pnpm test` 中，必须单独运行。具体 script 以各 workspace 的 `package.json` 为准。

### 2. 再跑跨包检查

```bash
pnpm typecheck
pnpm test
pnpm lint
make test
```

`make test` 确保目标数据库存在、应用迁移，再通过仓库 wrapper 运行 Go race 测试。

### 3. 需要完整栈时

```bash
make up
make env-exec ARGS="-- pnpm exec playwright test"
```

Playwright 配置在 [`playwright.config.ts`](../../playwright.config.ts)：只跑 Chromium、单 worker，且不会自动启动服务。也可让全量脚本按需启动 API/Web：

```bash
make check
```

[`scripts/check.sh`](../../scripts/check.sh) 的顺序是 TypeScript typecheck、TypeScript tests、Go tests、启动 E2E 所需服务、Playwright。主 checkout 可用 `make check-main`，worktree 可用 `make check-worktree`。

## 提交前 checklist

- [ ] 新行为先由失败测试或明确回归用例定义。
- [ ] 测试位于行为所有者所在层，没有跨层重复矩阵。
- [ ] pure test 使用 Node 环境；组件测试使用真实 jsdom lane。
- [ ] API 漂移、权限、错误和未知 enum 路径按风险覆盖。
- [ ] DB-backed Go 测试使用共享 fixture 与 handler driver。
- [ ] 默认测试不会触碰已登录的真实 agent CLI。
- [ ] 先跑窄测试，再运行本次修改需要的 typecheck、lint、Go、E2E 或 `make check`。

## 相关页面与源码

- [参与贡献](index.md)
- [添加功能](adding-a-feature.md)
- [模式与约定](patterns-and-conventions.md)
- [root package scripts](../../package.json)
- [Turborepo 测试图](../../turbo.json)
- [Go 测试 wrapper](../../scripts/test-go.sh)
- [CI workflow](../../.github/workflows/ci.yml)
- [Mobile verify workflow](../../.github/workflows/mobile-verify.yml)
