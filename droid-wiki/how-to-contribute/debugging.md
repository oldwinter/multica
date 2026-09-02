# 调试

调试 Multica 时先证明当前进程、端口和数据库属于这个 checkout，再追业务日志。工作树可同时运行多个环境，端口可通不代表监听者就是当前提交。

## 环境归属

```bash
make status
make list
```

`make status` 会检查健康端点返回的 listener PID、进程组和 commit。若环境记录损坏，先看 [`scripts/dev-env.sh`](../../scripts/dev-env.sh) 和当前 `.env.worktree`，不要直接删除其他 checkout 的进程、数据库或 registry 项。

常见情况：

| 现象 | 首先检查 |
| --- | --- |
| API 端口可访问但代码没更新 | `make status` 的 PID/commit proof |
| Web 请求去了错误 API | Web rewrite、`REMOTE_API_URL`、当前 workspace slug |
| worktree 读到主库 | `.env` 是否被误复制；`DATABASE_URL` 是否匹配 registry |
| Desktop 显示旧页面 | renderer 端口、app name、userData 隔离和 updater 状态 |
| Daemon 没有领取 task | runtime 在线、workspace assignment、Daemon WS、claim lease |

## API 与数据库

服务入口在 [`server/cmd/server/main.go`](../../server/cmd/server/main.go)，路由在 [`server/cmd/server/router.go`](../../server/cmd/server/router.go)。先检查 `/health`，再按 `request_id` 串联 handler、service 和事件日志。

数据库故障按顺序排查：

1. `DATABASE_URL` 是否连接到当前 checkout 的数据库。
2. `/health` 是否报告 migration 缺失。
3. pool 指标是否显示 acquire wait、empty acquire 或取消增长。
4. SQL 是否按 `workspace_id` 过滤。
5. 条件迁移是否被账本记录但实际对象缺失。

迁移 runner 和恢复限制见[数据库迁移](../backend/migrations.md)，连接池与 SQL 层见[数据库与 sqlc](../backend/database-and-sqlc.md)。

## 实时与缓存

Web/Desktop 的事件入口集中在 `packages/core/realtime/`，Mobile 在 `apps/mobile/data/realtime/` 独立实现。出现“刷新后才正确”时，检查：

- 服务端是否发布了正确 workspace/user scope 的事件；
- payload 是否足以 patch 当前 Query cache；
- Query key 是否含正确 `wsId`；
- 列表原始 cache shape 是否被误当成组件 `select` 后的形状；
- reconnect 是否只 invalidate 该订阅拥有的 key。

不要用 Zustand 镜像服务端实体来遮盖缺失事件。实时结构见[实时事件总线](../systems/realtime-event-bus.md)与[Core 实时同步](../packages/core-realtime.md)。

## Daemon 与智能体 CLI

Daemon 调试从 CLI 开始：

```bash
multica daemon status
multica daemon logs
multica daemon probe
```

重点区分“未唤醒”“claim 失败”“执行环境准备失败”“CLI 启动失败”和“终态回传失败”。任务队列事实位于 `agent_task_queue`；WebSocket wakeup 只是低延迟提示，轮询仍是兜底。不要让默认测试解析或运行用户机器上的真实智能体 CLI。

执行链见[task 生命周期](../systems/agent-task-lifecycle.md)，本机边界见[守护进程与运行时](../systems/daemon-and-runtime.md)。

## 前端

前端问题先确定行为属于共享层还是平台 wiring：

- Web/Desktop 都错：先看 `packages/core` 或 `packages/views`。
- 只有 Web 错：看 `apps/web/app/`、`apps/web/platform/` 和 proxy/rewrite。
- 只有 Desktop 错：看 main/preload IPC、renderer route、tab coordinator。
- 只有 Mobile 错：看 Expo route、Mobile-owned Query/store/realtime。

安装版 Desktop 可能连接不同版本后端。网络响应必须经过 schema fallback；调试时保留原始 endpoint、解析警告和稳定 error code，不直接 `as T` 绕过边界。

## 观测入口

日志、Prometheus 指标、健康检查和 pprof 的配置与安全限制见[可观测性与故障排查](../operations/observability-and-troubleshooting.md)。生产环境不要临时暴露 profiling 端口或记录 token、headers、消息正文等敏感内容。

## 相关页面

- [本地开发](../operations/local-development.md)
- [配置参考](../reference/configuration.md)
- [认证与授权](../backend/authentication-and-authorization.md)
- [安全模型](../security/index.md)
