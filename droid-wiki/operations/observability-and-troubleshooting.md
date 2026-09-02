# 可观测性与故障排查

运维入口按“存活 → 就绪 → 依赖/子系统 → 日志/指标 → profile”逐层缩小范围。API 的健康实现位于 [`server/cmd/server/health.go`](../../server/cmd/server/health.go)，指标装配位于 [`server/internal/metrics/`](../../server/internal/metrics/)。

## 健康端点

| 端点 | 成功条件 | 用途 |
| --- | --- | --- |
| `GET /health` | Go 进程能处理请求 | liveness；不依赖 PostgreSQL |
| `GET /readyz` | DB Ping 成功且当前二进制要求的每条迁移均在 ledger | readiness、流量摘除 |
| `GET /healthz` | 与 `/readyz` 相同 | Kubernetes/operator 习惯别名 |
| `GET /health/realtime` | token 正确，或无 token 时为无转发头的直接 loopback | 实时/daemon WS JSON snapshot |

实际 `/health` 还返回 PID、编译 commit 和 `started_at`：

```json
{
  "status": "ok",
  "pid": 12345,
  "commit": "abc1234",
  "started_at": "2026-09-01T01:02:03Z"
}
```

这让 `make status` 能证明 listener 来自当前 checkout，而不只是“端口上有个返回 200 的旧进程”。readiness 使用 2 秒数据库上下文和 3 秒结果 cache；失败返回 503，`checks.db`/`checks.migrations` 会是 `error`、`unknown` 或 `out_of_date`。

```bash
curl -fsS http://127.0.0.1:8080/health
curl -i http://127.0.0.1:8080/readyz
```

Helm 的 startup/readiness probe 使用 `/healthz`，liveness 使用 `/health`。数据库故障应让 Pod NotReady，而不是不断重启仍然活着的 API。

### Realtime snapshot 的访问控制

设置 `REALTIME_METRICS_TOKEN` 后：

```bash
curl -H "Authorization: Bearer $REALTIME_METRICS_TOKEN" \
  https://api.example.com/health/realtime
```

未设置 token 时，只允许直接 `127.0.0.1`/`::1` 且请求没有任何 forwarding header；其他请求返回 404。反向代理后的请求即使 Go 层看到 loopback，也因 forwarding header 被拒绝。实现见 [`server/cmd/server/health_realtime.go`](../../server/cmd/server/health_realtime.go)。

## Prometheus

默认不启动 Prometheus listener。设置独立管理地址：

```bash
METRICS_ADDR=127.0.0.1:9090 ./server/bin/server
curl http://127.0.0.1:9090/metrics
```

公共 API 端口不提供 `/metrics`。若容器内绑定 `0.0.0.0:9090`，只发布到私网/host loopback，并用 NetworkPolicy、allowlist 或 proxy auth 保护。指标会暴露内部 route、流量、依赖状态和运行时健康。

主要指标族：

| 前缀/指标 | 观察内容 |
| --- | --- |
| `multica_build_info` | version/commit |
| `multica_http_*` | 请求数、延迟、in-flight、daemon workspace response size |
| `multica_db_pool_*` | acquired/idle/max、获取等待时长 |
| `multica_realtime_*` | 连接、发送/丢弃、Redis XADD/XREAD、Stream 大小/TTL/eviction |
| `multica_daemonws_*` | daemon 连接与 wakeup 结果 |
| `multica_channel_lease_*` | lease 操作、owner、renew error、接管延迟 |
| `multica_channel_media_*` | 对象删除、引用、失败与 tombstone invariant |
| `multica_wecom_*` | 连接、认证、出站 delivered/dropped/unconfirmed |
| `multica_agent_task_*` / `multica_runtime_*` | task 队列/执行和运行时状态 |

registry 同时注册 Go runtime 与 process collector。只有 `METRICS_ADDR` 启用时，HTTP metrics middleware 才装配并开始累计请求指标。

建议告警：

- `/readyz` 连续 503，但 `/health` 200；
- `multica_db_pool_*` 显示 acquired 接近 max、等待持续增长；
- `multica_realtime_redis_connected=0`、XADD/XREAD error 或 Stream missing/evicted；
- realtime `messages_dropped_total`、slow eviction 增长；
- channel lease renewal error 或租约 Redis eviction 非零；
- task queued/running backlog 和 stuck 增长；
- WeCom `outbound_dropped_total`，特别是 `no_live_connection`；
- `sys_cron_executions` 最近 plan 无 SUCCESS。

## 日志

[`server/internal/logger/logger.go`](../../server/internal/logger/logger.go) 使用 `slog` + tint 写 stderr；TTY 才带颜色，重定向文件不带 ANSI。`LOG_LEVEL` 接受 `debug`、`info`、`warn`、`error`，代码默认是 debug。

HTTP access log 由 [`server/internal/middleware/request_logger.go`](../../server/internal/middleware/request_logger.go) 生成：

- 记录 method、path、status、duration、request ID 和客户端元数据；
- 5xx 为 Error，普通 4xx 为 Warn，预期的 stale runtime/task 404 为 Info；
- 跳过高频 `/health`；
- 自动把 autopilot webhook path 中的 bearer token 替换为 `[redacted]`。

后端每 15 秒记录数据库 pool stats；出现空池等待或取消时升级为 `db pool pressure` Warn。启动日志会打印有效 max/min、Redis relay mode、Stream retention、metrics/pprof 地址和被禁用的可选集成。

### 查看日志

开发 registry：

```bash
make status
tail -f <status输出的日志目录>/api.log
tail -f <status输出的日志目录>/web.log
```

Compose：

```bash
docker compose -f docker-compose.selfhost.yml ps
docker compose -f docker-compose.selfhost.yml logs --tail=200 backend
docker compose -f docker-compose.selfhost.yml logs -f backend frontend postgres
```

Kubernetes：

```bash
kubectl -n multica get pods
kubectl -n multica describe pod <backend-pod>
kubectl -n multica logs deploy/multica-backend --all-pods --tail=200
kubectl -n multica logs -f deploy/multica-backend
```

Daemon：

```bash
multica daemon status
multica daemon logs
```

后端不管理生产日志留存，Compose logging driver、journald 或 log shipper 决定保留期和访问权限。

> **敏感日志警告：** 未配置邮件服务时验证码写日志。`MULTICA_WECOM_TRACE=1` 还会记录每帧前 120 个 rune 的用户消息和原样附件文件名；只在受控调试窗口启用，重启 backend 生效，完成后立即清除并再次重启。日志读者范围和保留期必须在启用前评估。

## Go pprof

后端总是在固定 loopback `127.0.0.1:6060` 启动标准 pprof，公共 API 不提供该路径：

```bash
go tool pprof 'http://127.0.0.1:6060/debug/pprof/profile?seconds=30'
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

容器 loopback 只在其 network namespace 内可达：

```bash
docker compose -f docker-compose.selfhost.yml exec backend \
  wget -qO /tmp/heap.pprof http://127.0.0.1:6060/debug/pprof/heap
docker compose -f docker-compose.selfhost.yml cp \
  backend:/tmp/heap.pprof ./heap.pprof
go tool pprof ./heap.pprof
```

CPU profile/trace 会增加负载并可能暴露进程内部，只允许运维人员访问，避免在高峰期长时间采集。

## 故障定位手册

### API 不启动

1. 看 entrypoint 是否卡在 `Running database migrations...`。
2. 核对 `APP_ENV=production` 时 `JWT_SECRET` 是否为空/占位。
3. 查 `DATABASE_URL`、TLS、DNS和网络 ACL。
4. 查 `MULTICA_DATABASE_STARTUP_TIMEOUT` 是否耗尽；单次连接 timeout 是否过长。
5. 查 malformed `MULTICA_FEATURE_FLAGS_FILE` 或非法 `MULTICA_LLM_MAX_RETRIES`，两者会 fail fast。
6. 并发 Pod 正在迁移时，其他 Pod 等 advisory lock 是正常现象；长期不动再查锁 holder 和迁移日志。

```bash
docker compose -f docker-compose.selfhost.yml logs backend postgres
psql "$DATABASE_URL" -c 'select now()'
```

### `/health` 正常但 `/readyz` 503

- `db=error`：数据库不可达或 pool 无连接。
- `migrations=out_of_date`：当前二进制内嵌的某个 migration version 未登记；执行匹配版本的 `migrate up`。
- `migrations=error`：ledger 查询失败或二进制无法枚举迁移；检查 `schema_migrations` 和镜像内容。

不要把 liveness 改成 `/readyz` 来“修复”告警，否则数据库故障会触发 Pod 重启风暴。

### 数据库延迟或 task claim 慢

查日志中的 `db pool pressure` 和 Prometheus pool 指标。容量规划：

```text
总上限 ≈ backend 副本数 × DATABASE_MAX_CONNS + migration/运维/其他客户端
```

如果 acquired 长期等于 max 且 `empty_acquire_delta` 增长，可在数据库 `max_connections` 留有余量时调大 pool，或引入 PgBouncer/RDS Proxy；如果 DB CPU/IO 已饱和，盲目加连接会更差。

### Web 能开但 API 全 404

检查 `NEXT_PUBLIC_API_URL`/`REMOTE_API_URL` 是否错误包含 `/api`。客户端会再拼接 `/api`，错误配置表现为 `/api/api/...`。重新构建镜像或重建容器，以确保 Web 获得新值。

### GET 正常但所有写操作 403

分域部署通常是 CSRF Cookie 问题：

1. `COOKIE_DOMAIN` 是否覆盖 app/API 两个 host；
2. 值是否为域名而不是 IP；
3. 调整后是否删除两个 host 的旧 `multica_auth`/`multica_csrf`；
4. `FRONTEND_ORIGIN` 是否为实际浏览器 origin。

优先切到同源代理，保持 Cookie host-only。

### 实时更新停止或 WebSocket 403

1. 浏览器 origin 是否在 `CORS_ALLOWED_ORIGINS`；
2. 反向代理 `/ws` 是否携带 Upgrade/Connection；
3. Caddy 是否错误使用 exact `/ws` 或过宽 `/ws*`；
4. 是否有 response buffering；
5. 多副本是否配置 Redis relay；
6. `REALTIME_RELAY_MODE=legacy` 是否导致 daemon wakeup 不跨副本。

日志典型信息是 `websocket: request origin not allowed by Upgrader.CheckOrigin`；前端控制台会反复重连。

### Redis/多副本事件缺失

启动日志应出现 `realtime: Redis relay enabled`，并显示 mode/node/shard。若出现 fallback 到 in-memory：

- 确认 Redis env 真正进入容器；Compose `.env` 只插值，默认文件未透传这些高级键；
- 检查 URL、ACL、TLS、`REDIS_DISABLE_CLIENT_NAME`；
- 监控 `redis_connected`、XADD/XREAD error、evicted keys；
- Stream 使用 Redis 6.2+；
- lease Redis 必须 `noeviction` 且 namespace 跨环境唯一。

TTL rollout/rollback 不可在部分副本随意切换，按[配置参考](configuration.md#redis实时与多副本)的两阶段顺序操作。

### 附件上传成功但不可下载

1. `S3_BUCKET` 是否误填完整 hostname；
2. region 是否真实匹配；
3. custom endpoint 是否需要 path style/virtual host style；
4. 浏览器能否访问 endpoint；VPC-only endpoint 应用 `proxy`；
5. 私有 bucket 的 presign/CloudFront key 和 TTL；
6. 多副本是否仍用每 Pod 本地/RWO 存储；
7. bucket CMEK policy 与 workload IAM/KMS 权限。

应用没有专用 CMEK header 变量；bucket policy 若要求每次请求显式携带 SSE-KMS header，会拒绝当前上传。应采用 bucket 默认加密，或在修改应用并测试前不要启用这种显式-header policy。

### Daemon 在线但不领取 task

```bash
multica daemon status
multica daemon logs
curl -fsS https://api.example.com/health
```

核对 daemon profile 的 server/app URL、PAT、workspace、Agent CLI PATH；多副本下核对 sharded relay 的 daemon wakeup。daemon 仍有默认 30 秒 catch-up poll，完全没有领取通常不只是 wakeup 延迟。

### 周期任务没有运行

```sql
SELECT job_name, plan_time, status, attempt, runner_id,
       error_code, error_msg, started_at, finished_at
  FROM sys_cron_executions
 ORDER BY plan_time DESC
 LIMIT 50;
```

临时 DB 错误不会杀掉 server，scheduler 会记录并在后续 tick 重试。Usage 历史缺口使用 `backfill_task_usage_hourly`，不要先安装额外 `pg_cron` 掩盖 scheduler 故障。

## 优雅关闭

平台 rollout 时观察：

- `termination signal received; holding before shutdown`；
- `shutting down server`；
- worker/渠道 drain timeout Warn；
- `server stopped`。

`MULTICA_SHUTDOWN_HOLD_DURATION` 适合先让外部负载均衡摘流，但它位于 Go 的实际 graceful shutdown **之前**。Kubernetes `terminationGracePeriodSeconds` 或平台 stop timeout 必须大于 hold，再为 HTTP 10 秒、worker/channel drain 和余量留时间；否则 SIGKILL 会绕过租约释放与排空。

部署形态和副本约束见[部署与自托管](deployment-and-self-hosting.md)，开发环境的 PID/commit 证明见[本地开发](local-development.md)。
