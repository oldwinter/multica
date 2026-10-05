# 配置参考

Multica 主要从环境变量读取配置，但 `.env`、Make、Docker Compose、Helm 和浏览器构建变量的优先级并不相同。变量的注释与默认值以 [`.env.example`](../../.env.example) 为总入口，最终行为以 [`server/cmd/server/main.go`](../../server/cmd/server/main.go)、[`server/cmd/server/router.go`](../../server/cmd/server/router.go) 和各部署模板为准。

## 配置来源与优先级

| 入口 | 配置来源 | 关键规则 |
| --- | --- | --- |
| `make up` | checkout 的 `.env` 或 `.env.worktree`，随后由 registry manifest 固化 | 如果两者都存在，`.env` 优先；registry 可能为避开冲突重写端口和数据库名 |
| 普通 Make target | [`Makefile`](../../Makefile) `include` 的 `ENV_FILE` | Make 命令行赋值最高；env 文件通常高于同名 shell 环境变量 |
| `docker compose` | shell 环境、`.env`、Compose `${VAR}` 插值 | shell 环境高于 `.env`；`.env` 只是插值来源，不会自动把所有键注入容器 |
| Helm | [`deploy/helm/multica/values.yaml`](../../deploy/helm/multica/values.yaml) 生成 ConfigMap，敏感值来自 `existingSecret` | Chart 不创建 Secret；backend 对 ConfigMap 和整个 Secret 使用 `envFrom` |
| 直接运行二进制 | 当前进程环境 | 未配置时使用代码默认值，生产安全检查可能直接拒绝启动 |

Compose 的“插值”和“容器环境”必须区分。只有 [`docker-compose.selfhost.yml`](../../docker-compose.selfhost.yml) 中 `backend.environment` 明确列出的键才会进入后端容器。当前默认文件未声明 Redis、`LOG_LEVEL` 等所有高级变量；只在 `.env` 写入这些键不会生效，应通过 Compose override 显式透传：

```yaml
# compose.ops.yml
services:
  backend:
    environment:
      LOG_LEVEL: ${LOG_LEVEL:-info}
      REDIS_URL: ${REDIS_URL:-}
      REALTIME_RELAY_REDIS_URL: ${REALTIME_RELAY_REDIS_URL:-}
      CHANNEL_WS_LEASE_BACKEND: ${CHANNEL_WS_LEASE_BACKEND:-}
      CHANNEL_WS_LEASE_REDIS_URL: ${CHANNEL_WS_LEASE_REDIS_URL:-}
      CHANNEL_WS_LEASE_NAMESPACE: ${CHANNEL_WS_LEASE_NAMESPACE:-}
```

```bash
docker compose \
  -f docker-compose.selfhost.yml \
  -f compose.ops.yml \
  up -d
```

Helm 对未建模变量可利用 backend 的 `envFrom.secretRef`：把键加入 `existingSecret`，或在自有 chart overlay 中增加显式 `env`。不要把真实 Secret 写入 `deploy/helm/multica/values.yaml` 或提交到仓库。

## 生产启动必需项

| 变量 | 默认/要求 | 作用 |
| --- | --- | --- |
| `APP_ENV` | Compose/Helm 默认为 `production` | 开启生产 JWT 和开发验证码安全检查 |
| `DATABASE_URL` | 必需 | PostgreSQL 连接串；生产应启用 TLS |
| `JWT_SECRET` | 生产必需，强随机值 | 签发登录 JWT、daemon JWT 和下载 URL capability HMAC |
| `FRONTEND_ORIGIN` | 生产应显式设置 | Cookie 安全属性、CORS 与 WebSocket Origin 基准 |
| `CORS_ALLOWED_ORIGINS` | 回退到 `FRONTEND_ORIGIN` | 逗号分隔的 HTTP CORS 与 WebSocket Origin allowlist |
| `MULTICA_APP_URL` | 回退到前端 origin | CLI 登录和面向用户的应用 URL |
| `MULTICA_PUBLIC_URL` | 可空；公网 webhook 部署应设置 | 生成绝对 webhook URL；不从 Host header 推导，避免 spoofing |

生成密钥：

```bash
openssl rand -hex 32       # JWT_SECRET
openssl rand -base64 32    # VCS、插件或渠道的 32 字节部署密钥
```

> **安全警告：** 不要复用示例值，不要提交 `.env`，不要在日志或命令历史中粘贴生产密钥。生产进程会拒绝空的或已知占位 `JWT_SECRET`。`MULTICA_DEV_VERIFICATION_CODE` 在 `APP_ENV=production` 下被忽略；任何公网环境都不应切到 development 来启用固定验证码。

## 网络、URL 与 Cookie

后端端口优先级在 Make、local script 和 Compose 中保持一致：

```text
BACKEND_PORT → API_PORT → SERVER_PORT → PORT → 8080
```

| 变量 | 默认 | 注意事项 |
| --- | --- | --- |
| `PORT` | `8080` | bare server 的监听端口；Compose 中是宿主机发布端口，容器内仍为 8080 |
| `FRONTEND_PORT` | `3000` | Compose 宿主机发布端口，容器内仍为 3000 |
| `NEXT_PUBLIC_API_URL` | 空时使用页面同源 | 只填 origin，不要填 `/api` |
| `NEXT_PUBLIC_WS_URL` | 空时使用页面同源 `/ws` | 跨主机时使用 `ws://`/`wss://` 完整地址 |
| `REMOTE_API_URL` | Compose 中 `http://backend:8080` | Next.js 服务端 rewrite 的后端上游 |
| `COOKIE_DOMAIN` | 空，host-only | 只有前后端位于同一注册域的不同子域时才设置；不能填 IP |
| `MULTICA_TRUSTED_PROXIES` | 空，不信任转发头 | webhook 限流可接受 `X-Forwarded-*` 的代理 CIDR |
| `RATE_LIMIT_TRUSTED_PROXIES` | 空 | 登录限流独立的可信代理 CIDR |

同源部署最简单也更安全：让反向代理把 `/ws` 直接转到 Go API，其余请求进入 Web，保持 `NEXT_PUBLIC_*` 为空和 `COOKIE_DOMAIN` 为空。分域部署必须把 `COOKIE_DOMAIN` 设为覆盖前后端的**最窄**父域，否则 GET 可能正常而所有写操作因缺少 CSRF header 返回 403；调整后应删除两个域上的旧认证 Cookie 并重新登录。

## 数据库

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `DATABASE_MAX_CONNS` | 每副本 25 | 高于 URL 的 `pool_max_conns` |
| `DATABASE_MIN_CONNS` | 每副本 5 | 高于 URL 的 `pool_min_conns`，且不会超过 max |
| `MULTICA_DATABASE_STARTUP_TIMEOUT` | `3m` | migration + API 两阶段共享的瞬时故障重试预算 |
| `MULTICA_DATABASE_CONNECT_TIMEOUT` | `5s` | 单次连接上限；原生 `connect_timeout`/`PGCONNECT_TIMEOUT` 优先 |

池配置优先级为环境变量、`DATABASE_URL` 的 `pool_*` 参数、代码默认值。容量规划按 `副本数 × DATABASE_MAX_CONNS` 计算，并为迁移、运维连接和其他应用留余量。实现见 [`server/cmd/server/dbstats.go`](../../server/cmd/server/dbstats.go)。

PostgreSQL 17 必须可创建 `pgcrypto` 和 `pg_trgm`；`pg_bigm` 只改善 CJK 搜索，`pg_cron` 已被 DB-backed 进程内 scheduler 取代。条件迁移被记入 ledger 不代表可选对象真实存在，应使用 `SELECT extname FROM pg_extension` 核查。

## 认证、邮件与注册

| 变量 | 行为 |
| --- | --- |
| `SMTP_HOST` | 非空时启用 SMTP，并优先于 Resend |
| `SMTP_PORT`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`SMTP_TLS` | 配置 relay；465 在未指定模式时自动使用 implicit TLS |
| `RESEND_API_KEY`、`RESEND_FROM_EMAIL` | SMTP 未启用时的邮件后端 |
| `GOOGLE_CLIENT_ID`、`GOOGLE_CLIENT_SECRET`、`GOOGLE_REDIRECT_URI` | Google OAuth |
| `ALLOW_SIGNUP` | `false` 禁止创建新账号 |
| `ALLOWED_EMAIL_DOMAINS`、`ALLOWED_EMAILS` | 注册 allowlist |
| `DISABLE_WORKSPACE_CREATION` | `true` 禁止所有用户新建工作区 |
| `AUTH_TOKEN_TTL` | 登录 token 生命周期，接受 Go duration 或秒数 |

SMTP 和 Resend 都未配置时，验证码会写到后端日志，只适合受控测试。私有实例通常先创建共享工作区，再启用 `DISABLE_WORKSPACE_CREATION=true`；如果邀请用户仍需首次注册，不要同时关闭 `ALLOW_SIGNUP`。

## 附件与对象存储

未设置 `S3_BUCKET` 时，router 回退到本地存储，选择逻辑见 [`server/cmd/server/router.go`](../../server/cmd/server/router.go)。

| 变量 | 说明 |
| --- | --- |
| `LOCAL_UPLOAD_DIR` | 默认 `./data/uploads`；手动运行建议使用绝对路径 |
| `LOCAL_UPLOAD_BASE_URL` | 生成本地附件 URL 的 API base |
| `S3_BUCKET` | 只填 bucket 名，不填 S3 hostname |
| `S3_REGION` | 必须与真实 bucket region 一致 |
| `AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY` | 可选静态凭据；都为空时走 AWS SDK 默认凭据链 |
| `AWS_ENDPOINT_URL` | MinIO、R2、B2 等 S3-compatible endpoint |
| `S3_USE_PATH_STYLE` | custom endpoint 默认 true，AWS 默认 false |
| `ATTACHMENT_DOWNLOAD_MODE` | `auto`、`cloudfront`、`presign` 或 `proxy` |
| `ATTACHMENT_DOWNLOAD_URL_TTL` | 默认 `30m` |
| `CLOUDFRONT_DOMAIN`、`CLOUDFRONT_KEY_PAIR_ID`、`CLOUDFRONT_PRIVATE_KEY` | CloudFront 公网/签名下载 |

### CMEK 约束

当前 [`server/internal/storage/s3.go`](../../server/internal/storage/s3.go) 的 `PutObjectInput` **没有**设置 `ServerSideEncryption` 或 `SSEKMSKeyId`，仓库也没有专用的 `AWS_KMS_KEY_ID`/`SSE_KMS_KEY_ID` 变量。因此不要为 Multica 发明一个不会被读取的 CMEK 环境变量。

如果合规要求客户管理密钥（CMEK）：

1. 在 S3/兼容对象存储侧配置 bucket 默认加密和拒绝未加密写入的 bucket policy。
2. 给 workload identity/IAM role 授予目标 KMS key 的 encrypt/decrypt/data-key 权限。
3. 验证普通 `PutObject` 在该 policy 下会自动使用指定 key，并验证 presign/proxy 下载。
4. PostgreSQL 磁盘、Kubernetes PVC 和备份的 CMEK 同样由云数据库、StorageClass/CSI 与备份平台配置，不由 Multica 应用层配置。

## Redis、实时与多副本

| 变量 | 作用 |
| --- | --- |
| `REDIS_URL` | 默认共享实例：认证限流、PAT/daemon token/cache、运行时 liveness、实时 relay fallback |
| `REALTIME_RELAY_REDIS_URL` | 推荐给实时 Stream 的独立实例；未设时回退到 `REDIS_URL` |
| `REALTIME_RELAY_MODE` | `sharded`（默认）、`dual` 或 `legacy` |
| `REALTIME_RELAY_STREAM_MAXLEN` | 默认 2000；即时内存压力阀 |
| `REALTIME_RELAY_REPLAY_GRACE` / `TRIM_HORIZON` | 默认 5m/10m |
| `REDIS_DISABLE_CLIENT_NAME` | 托管 Redis 禁止 `CLIENT SETNAME` 时设为 true |
| `CHANNEL_WS_LEASE_BACKEND` | 多副本渠道长连接推荐 `redis`；默认 PostgreSQL rollback 路径 |
| `CHANNEL_WS_LEASE_REDIS_URL` | 推荐租约专用 Redis；未设时回退 `REDIS_URL` |
| `CHANNEL_WS_LEASE_NAMESPACE` | Redis lease 模式必需，且不同环境必须不同 |

多副本 API 若没有 Redis relay，只能在本进程广播，连接到另一副本的客户端和 daemon 会漏掉唤醒或实时事件。租约专用 Redis 要使用 `noeviction`，启用适合业务的 HA/持久化，并告警可用性、延迟、内存和 eviction（必须为零）。Stream TTL 是分阶段 opt-in：先让所有副本运行支持版本且 TTL 关闭，再整体打开；回滚前先整体关闭并确认 PTTL 恢复为 `-1`。

## Feature flag、LLM 与集成密钥

- `MULTICA_FEATURE_FLAGS_FILE` 指向启动时一次性加载的 YAML；读/解析失败会阻止启动。`FF_<FLAG_KEY>` 高于 YAML，适合作为重启生效的 kill switch。
- `MULTICA_LLM_API_KEY` 与 `MULTICA_LLM_BASE_URL` 都为空时，服务端 LLM assist 层完全禁用；这不影响 daemon 调用本地 Agent CLI。
- `MULTICA_LLM_MAX_RETRIES`：空为内建默认 2，`0` 禁用，`1..5` 为精确上限；其他值阻止启动。
- `MULTICA_VCS_SECRET_KEY`、`MULTICA_PLUGIN_SECRET_KEY` 和各 `MULTICA_<CHANNEL>_SECRET_KEY` 是数据库内凭据的部署级加密 key。轮换旧 key 会让既有密文无法解密，不能当作普通无状态 Secret 随意替换。

## 何时需要重启或重建

后端变量均在进程启动时读取，修改后重启 backend。`ALLOW_SIGNUP`、`DISABLE_WORKSPACE_CREATION`、`GOOGLE_CLIENT_ID` 由 Web 运行时读取 `/api/config`，只重启后端，不需重建 Web。`NEXT_PUBLIC_*` 影响浏览器 bundle 的配置时应按当前镜像路径重建 Web；`REMOTE_API_URL` 和 `DOCS_URL` 在当前 standalone Web 容器运行时读取。

完整部署流程见[部署与自托管](deployment-and-self-hosting.md)，配置验证和故障信号见[可观测性与故障排查](observability-and-troubleshooting.md)。
