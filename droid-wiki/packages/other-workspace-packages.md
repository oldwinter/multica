# 其他工作区工具包

活跃贡献者：Naiyuan Qing、Jiayuan Zhang、Multica Eve

[`pnpm-workspace.yaml`](../../pnpm-workspace.yaml) 只声明两个 glob：`apps/*` 与 `packages/*`。因此“是不是 workspace package”由这些目录下是否存在 `package.json` 决定，而不是由目录名或 `node_modules` 中的依赖决定。

## 当前工作区清单

`pnpm list -r` 会列出根私有项目 [`multica`](../../package.json)，以及下面 10 个由 glob 匹配到的子 workspace：

| 范围 | 包 | 作用 |
| --- | --- | --- |
| 应用 | [`@multica/web`](../../apps/web/package.json) | Next.js 产品 Web |
| 应用 | [`@multica/desktop`](../../apps/desktop/package.json) | Electron 主进程、preload 与 renderer |
| 应用 | [`@multica/mobile`](../../apps/mobile/package.json) | Expo / React Native，独立 UI 与数据层 |
| 应用 | [`@multica/docs`](../../apps/docs/package.json) | Fumadocs 文档站 |
| 共享层 | [`@multica/core`](../../packages/core/package.json) | 无头业务逻辑、API、Query、store |
| 共享层 | [`@multica/ui`](../../packages/ui/package.json) | 原子组件与样式 |
| 共享层 | [`@multica/views`](../../packages/views/package.json) | Web/Desktop 业务视图 |
| 工具 | [`@multica/eslint-config`](../../packages/eslint-config/package.json) | 共享 ESLint flat config |
| 工具 | [`@multica/tsconfig`](../../packages/tsconfig/package.json) | 共享严格 TypeScript 配置 |
| 扩展 SDK | [`@multica/plugin-sdk`](../../packages/plugin-sdk/package.json) | 第三方插件 surface 客户端 SDK |

下文聚焦 `eslint-config`、`tsconfig` 和 `plugin-sdk` 这三个不承载普通产品页面的包。

## `@multica/eslint-config`

[`packages/eslint-config/package.json`](../../packages/eslint-config/package.json) 暴露三个层叠配置：

| 导出 | 文件 | 内容 |
| --- | --- | --- |
| `@multica/eslint-config/base` | [`packages/eslint-config/base.js`](../../packages/eslint-config/base.js) | ESLint recommended、typescript-eslint、`import-x/no-extraneous-dependencies`、生成目录 ignore |
| `@multica/eslint-config/react` | [`packages/eslint-config/react.js`](../../packages/eslint-config/react.js) | base + React JSX + React Hooks `recommended-latest` |
| `@multica/eslint-config/next` | [`packages/eslint-config/next.js`](../../packages/eslint-config/next.js) | react + Next recommended / Core Web Vitals |

`base` 故意关闭 TypeScript 已负责的 unused 检查，并允许需要边界处理时显式使用 `any`。最重要的仓库级约束是 phantom dependency 防线：每个 `apps/` 或 `packages/` workspace 必须在自己的 `package.json` 声明直接导入的外部依赖。测试、配置、脚本、Electron main/preload 等允许从 `devDependencies` 解析，运行时代码不能依靠根包“碰巧装过”。

修改规则时先判断它属于所有 TypeScript、所有 React，还是 Next 专属层；不要在每个 app 复制同一规则。包本身没有独立脚本，验证通过实际消费者运行 `pnpm lint`。

## `@multica/tsconfig`

[`packages/tsconfig/base.json`](../../packages/tsconfig/base.json) 是共享编译契约：

- `ESNext` target/module 与 `bundler` resolution；
- `strict`、`isolatedModules`、`noImplicitReturns`；
- `noUnusedLocals`、`noUnusedParameters`、`noUncheckedIndexedAccess`；
- JSON module、declaration、declaration map 与 source map。

[`packages/tsconfig/react-library.json`](../../packages/tsconfig/react-library.json) 在 base 上增加 `react-jsx` 和 DOM lib。`packages/ui`、`packages/views`、`packages/plugin-sdk` 均通过各自 [`tsconfig.json`](../../packages/ui/tsconfig.json) 继承它；应用可按 Next、Electron 或 Expo 构建需求再扩展。

这个包只提供 JSON 配置，没有构建或测试脚本。修改后应运行受影响 workspace 的 `typecheck`，最好再运行根 `pnpm typecheck`，因为收紧一个选项可能同时影响多个 app/package。

## `@multica/plugin-sdk`

[`packages/plugin-sdk/README.md`](../../packages/plugin-sdk/README.md) 描述插件 surface 的运行模型：一个 bundle 后的脚本运行在无 `allow-same-origin` 的 sandboxed iframe 中。SDK 是第三方作者打包进该脚本的零运行时依赖客户端。

[`index.ts`](../../packages/plugin-sdk/index.ts) 暴露 `multica`：

- `context.get()`：当前工作区、用户、可选任务和非机密配置；
- `issue.get/update/comments/comment()`：经宿主代理的任务能力；
- `storage.workspace` / `storage.user`：替代 iframe 中不可用的浏览器存储；
- `hooks.invoke()`：调用 manifest 允许的 `ui` trigger hook；
- `ui.resize()` / `ui.onThemeChange()`：尺寸和宿主主题令牌。

所有调用通过私有 `MessagePort` 进入宿主。frame 中没有 API credential；宿主以当前登录用户会话执行请求，同时受插件已授权 scope 和该用户原有权限限制。调用失败抛出带 HTTP-like `status` 的 `MulticaPluginError`。

### Bridge 协议

[`packages/plugin-sdk/protocol.ts`](../../packages/plugin-sdk/protocol.ts) 定义：

- `BRIDGE_PROTOCOL_VERSION = 2`；
- 注入 guest port 的全局名；
- action / `ui.resize` 请求；
- success/error response 与 host 推送的 theme event；
- 窄化未知 message 的 type guard。

宿主侧实现位于 [`packages/views/plugins/`](../../packages/views/plugins/)。协议形状有意重复而不直接导入 SDK：SDK 会发布给第三方，不能把 `@multica/core` / `@multica/ui` 拉入 bundle；宿主也不能被第三方安装的 SDK 版本控制。修改 wire format 必须同步宿主握手、surface bridge 和协议测试，而不是只改一边。

## 关于 `plugins/typescript-sdk` 与 `plugins/mcp-bridge`

当前 checkout 没有仓库根目录 `plugins/`，`pnpm-workspace.yaml` 也不匹配 `plugins/*`。因此：

- `plugins/typescript-sdk` 不是当前 workspace；名字相近的 `@modelcontextprotocol/sdk` 若出现在安装树中，也是第三方依赖，不是 Multica 源码包。
- `plugins/mcp-bridge` 也不是当前 workspace。Multica 当前插件 surface bridge 在 [`packages/plugin-sdk`](../../packages/plugin-sdk/) 与 [`packages/views/plugins`](../../packages/views/plugins/) 之间；MCP 的服务端/守护进程能力位于 [`server/pkg/remotemcp`](../../server/pkg/remotemcp/) 和 [`server/internal/daemon/remote_mcp_broker.go`](../../server/internal/daemon/remote_mcp_broker.go) 等领域代码中，不能把它们虚构为一个 workspace package。

若未来新增这两个目录，需要同时把它们纳入 [`pnpm-workspace.yaml`](../../pnpm-workspace.yaml) 的匹配范围并提供 `package.json`，否则 pnpm、Turbo 和 `workspace:*` 都不会把它们当工作区。

## 依赖版本与新增包

共享版本目录位于 [`pnpm-workspace.yaml`](../../pnpm-workspace.yaml) 的 `catalog:`。普通 Web/Desktop workspace 对 React、TanStack、i18next、zod 等共享依赖使用 `catalog:`；内部包使用 `workspace:*`。Expo/React Native 相关版本由 Mobile 自己锁定，不能为了统一 catalog 破坏 Expo 兼容矩阵。

新增 workspace package 时：

1. 放到 `apps/<name>` 或 `packages/<name>`，或显式更新 workspace glob。
2. 创建唯一 `name` 的 `package.json`，声明所有直接外部依赖。
3. 配置 `typecheck` / `lint` / `test` 等 Turbo 可发现脚本；纯配置包可保持无脚本，但应由消费者验证。
4. 使用 `@multica/tsconfig` 和适合的 `@multica/eslint-config` 层。
5. 只导出稳定入口，避免消费者深导入内部实现。

## 测试与验证

```bash
# SDK 本身没有 test 脚本，至少运行：
pnpm --filter @multica/plugin-sdk typecheck
pnpm --filter @multica/plugin-sdk lint

# 配置变更通过消费者验证：
pnpm typecheck
pnpm lint

# Bridge 协议/握手的宿主回归：
pnpm --filter @multica/views exec vitest run plugins
```

宿主侧已有 [`packages/views/plugins/plugin-sdk-protocol.test.ts`](../../packages/views/plugins/plugin-sdk-protocol.test.ts)、[`packages/views/plugins/plugin-sdk-handshake.test.ts`](../../packages/views/plugins/plugin-sdk-handshake.test.ts) 和 [`packages/views/plugins/surface-bridge.test.ts`](../../packages/views/plugins/surface-bridge.test.ts)。涉及安全边界时还应跑完整 `@multica/views` 测试，而不是只依赖 SDK typecheck。

## 常见陷阱

- 把安装在 `node_modules` 的第三方包误列为 workspace。
- 新 package 直接 import 外部库却只依赖根 `package.json` 的 hoist。
- 收紧共享 tsconfig/eslint 后只验证配置包自身；真正的破坏发生在消费者。
- 在插件 iframe 使用 `localStorage`、cookie 或假设有 `Origin`；opaque origin 下这些假设不成立。
- 把 token、session 或 credential 发进插件 frame。安全模型要求宿主代请求。
- 只升级 `BRIDGE_PROTOCOL_VERSION` 一端，或让宿主直接依赖可由第三方替换的 SDK 包。
- 发布插件 surface 时保留 bare import/module graph；产物必须是单脚本 bundle。

## 相关页面

- [共享业务视图](views-business-ui.md)
- [UI 原语与设计令牌](ui-primitives.md)
- [本地化与中文内容](localization-and-content.md)
- [系统架构](../overview/architecture.md)
- [模式与约定](../how-to-contribute/patterns-and-conventions.md)
