# 依赖参考

本页说明主要依赖在哪里声明，以及哪些依赖构成运行时边界。精确版本以当前提交的 manifest 和 lockfile 为准，不在 Wiki 中复制一份易漂移的完整清单。

## 语言与构建

| 依赖组 | 权威来源 | 作用 |
| --- | --- | --- |
| Go 模块 | [`server/go.mod`](../../server/go.mod)、[`server/go.sum`](../../server/go.sum) | API、CLI、Daemon、数据库与集成 |
| JS workspace | [`pnpm-workspace.yaml`](../../pnpm-workspace.yaml)、[`pnpm-lock.yaml`](../../pnpm-lock.yaml) | 应用和共享包 |
| 根任务 | [`package.json`](../../package.json)、[`turbo.json`](../../turbo.json) | build、typecheck、lint、test |
| 容器 | [`Dockerfile`](../../Dockerfile)、[`Dockerfile.web`](../../Dockerfile.web) | API/Daemon 与 Web 镜像 |

## 后端核心

`server/go.mod` 中最重要的运行依赖按职责分组如下：

| 类别 | 主要依赖 | 使用位置 |
| --- | --- | --- |
| HTTP | Chi、CORS middleware | `server/cmd/server/`、`server/internal/handler/` |
| 数据库 | pgx v5、sqlc 生成代码 | `server/pkg/db/`、`server/internal/service/` |
| 实时 | gorilla/websocket、go-redis | `server/internal/realtime/`、`server/internal/daemonws/` |
| 认证 | JWT、OAuth2、bcrypt | `server/internal/auth/`、`server/internal/middleware/` |
| 对象存储 | AWS SDK v2 | `server/internal/storage/` |
| 可观测性 | Prometheus client、structured logging | `server/internal/metrics/`、`server/internal/logger/` |
| 外部平台 | Slack/Lark 等 SDK 或协议客户端 | `server/internal/integrations/` |

`server/pkg/agent/` 不把模型 SDK 作为统一执行面。它启动用户已安装并登录的 26 种智能体 CLI，命令 allowlist 见 [`scripts/agent-cli-command-names.txt`](../../scripts/agent-cli-command-names.txt)。

## Web 与 Desktop 共享层

共享前端依赖由各包自己的 `package.json` 声明：

- [`packages/core/package.json`](../../packages/core/package.json)：TanStack Query、Zod、Zustand、i18next 和无界面业务依赖；
- [`packages/ui/package.json`](../../packages/ui/package.json)：Base UI、Radix/样式辅助和原子组件依赖；
- [`packages/views/package.json`](../../packages/views/package.json)：业务视图、编辑器、拖拽、图表等组合依赖；
- [`apps/web/package.json`](../../apps/web/package.json)：Next.js 平台依赖；
- [`apps/desktop/package.json`](../../apps/desktop/package.json)：Electron main/preload/renderer 和打包依赖。

依赖方向固定为 `views -> core + ui`。`core` 与 `ui` 彼此独立；`views` 不导入 Next.js 或 React Router。

## Mobile

[`apps/mobile/package.json`](../../apps/mobile/package.json) 直接 pin Expo、React 和 React Native 相关版本，不使用共享 catalog 代替 Expo 兼容矩阵。Mobile 自有 TanStack Query、Zustand、Zod、Realtime、NativeWind 和原生组件依赖；它只从 `@multica/core` 导入类型或纯函数。

修改 Mobile 依赖前先读 [`apps/mobile/CLAUDE.md`](../../apps/mobile/CLAUDE.md)，并以 `package.json` 的实际版本为构建事实。

## 外部服务

| 服务 | 是否必需 | 用途 |
| --- | --- | --- |
| PostgreSQL 17 + pgvector | 必需 | 权威业务数据、队列、调度与迁移账本 |
| Redis | 单节点可选，多副本按功能需要 | 实时 relay、缓存、渠道租约与请求状态 |
| S3 兼容存储 | 可由本地存储替代 | 附件、渠道媒体和来源上下文 |
| Agent CLI | 执行智能体 task 时需要 | 本地或云运行时上的真实执行器 |
| VCS / 消息渠道 / Composio / Cloud billing | 按功能启用 | 外部集成 |

生产拓扑和缺失依赖时的降级行为见[部署与自托管](../operations/deployment-and-self-hosting.md)与[配置参考](configuration.md)。

## 升级原则

1. 在实际消费 workspace 修改依赖声明，不依靠根 hoist。
2. 保持 API schema 对旧 Desktop 客户端兼容。
3. Expo/RN 升级按 Expo 兼容矩阵成组进行。
4. Go、Node、PostgreSQL 的 CI 基线同步文档和容器。
5. 依赖升级后运行受影响 workspace 测试，再运行根 typecheck/test。

## 相关页面

- [共享包](../packages/index.md)
- [应用层总览](../apps/index.md)
- [配置参考](configuration.md)
- [工具链](../how-to-contribute/tooling.md)
