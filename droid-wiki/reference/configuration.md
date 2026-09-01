# 配置参考

根配置模板是 [.env.example](../../.env.example)。本地 main checkout 默认读 `.env`，worktree 默认读 `.env.worktree`；选择逻辑在 [Makefile](../../Makefile)。自托管 Compose、Helm 与各 app 还会派生少量变量。

> 不要把 secret、token、私钥或真实连接串提交到仓库。下表只描述变量名和行为。

## 加载与优先级

| 场景 | 配置源 |
| --- | --- |
| `make` / 本地服务 | `.env`，若不存在则 `.env.worktree`；Makefile 派生 URL/端口 |
| Go API | 进程环境；[server/cmd/server/main.go](../../server/cmd/server/main.go) 和 [router.go](../../server/cmd/server/router.go) |
| Web | 构建/运行时环境；公开值用 `NEXT_PUBLIC_*`，SSR rewrite 可用 `REMOTE_API_URL` |
| Desktop | `apps/desktop/.env*` 的 `VITE_*`，外加 renderer/app 实例隔离变量 |
| Mobile | `apps/mobile/.env.development.local`、`.env.staging` 或 `apps/mobile/.env.production`；`EXPO_PUBLIC_*` 会烘焙进 JS bundle |
| Feature flag | `MULTICA_FEATURE_FLAGS_FILE` YAML，再由 `FF_<FLAG_KEY>` 覆盖 |

## 数据库

| 变量 | 默认/是否必需 | 用途 |
| --- | --- | --- |
| `DATABASE_URL` | 本地默认指向 `multica`；生产必配 | pgx/sqlc 主连接串；也可带 `pool_max_conns`、`pool_min_conns` |
| `POSTGRES_DB`、`POSTGRES_USER`、`POSTGRES_PASSWORD`、`POSTGRES_PORT` | 本地/Compose 默认 | 创建和连接本地 PostgreSQL |
| `DATABASE_MAX_CONNS`、`DATABASE_MIN_CONNS` | 25 / 5 | 每 API pod 的 pgxpool 上下限；高于 URL 参数优先级 |
| `MULTICA_DATABASE_STARTUP_TIMEOUT` | 3m（模板建议） | entrypoint/API 启动阶段重试总时限；`0` 表示单次 fail-fast |
| `MULTICA_DATABASE_CONNECT_TIMEOUT` | 5s（模板建议） | 单次 native connect 上限 |
| `PGCONNECT_TIMEOUT` | libpq/pgx 兼容变量 | 若设置则优先于 Multica connect timeout |

池实现见 [dbstats.go](../../server/cmd/server/dbstats.go)，启动重试见 [server/internal/dbstartup/](../../server/internal/dbstartup/)。

## HTTP、URL、代理与进程

| 变量 | 默认 | 用途 |
| --- | --- | --- |
| `PORT` | `8080` | Go API listen 端口 |
| `FRONTEND_PORT` | `3000` | 本地 Next 端口 |
| `FRONTEND_ORIGIN` | `http://localhost:${FRONTEND_PORT}` | 前端 origin；也是多个 URL 的 fallback |
| `MULTICA_APP_URL` | `FRONTEND_ORIGIN` | 用户可见 Web 地址；登录/绑定跳转 |
| `MULTICA_PUBLIC_URL` | 空 | API 对公网绝对地址；webhook、daemon setup、Plugin callback |
| `MULTICA_DAEMON_SERVER_URL` | Public URL → App URL fallback | 仅生成 daemon setup 命令时覆盖 API URL |
| `MULTICA_SERVER_URL` | 本地派生为 WS URL | CLI/daemon 连接 server |
| `CORS_ALLOWED_ORIGINS` | 本地 3000/5173/5174 | 逗号分隔的 CORS 与客户端 WS origin |
| `FRONTEND_ORIGIN` | 同上 | `CORS_ALLOWED_ORIGINS` 未设置时的单 origin fallback |
| `MULTICA_TRUSTED_PROXIES` | 空 | 可信反向代理 CIDR；影响 forwarded host/IP 的安全判断 |
| `MULTICA_HTTP_TIMEOUT` | 客户端内建值 | CLI/daemon HTTP timeout |
| `MULTICA_SHUTDOWN_HOLD_DURATION` | `0` | 收到 SIGTERM 后进入 graceful shutdown 前的延迟 |
| `LOG_LEVEL` | 实现默认 | Go 日志级别 |
| `APP_ENV` | 本地空/开发 | `production` 会启用 JWT/dev-code 安全约束 |

`BACKEND_PORT`、`API_PORT`、`SERVER_PORT` 是 Make/Compose 对 `PORT` 的覆盖别名，优先级见 [Makefile](../../Makefile)。

## 认证、注册与邮件

| 变量 | 用途 |
| --- | --- |
| `JWT_SECRET` | JWT、daemon JWT、头像/附件 capability HMAC；`APP_ENV=production` 时必须为强随机值 |
| `COOKIE_DOMAIN` | 跨同一注册域的前后端子域共享 session/CSRF cookie；单 host 留空 |
| `AUTH_TOKEN_TTL` | auth token 生命周期，Go duration 或秒数；默认 30 天 |
| `ALLOW_SIGNUP` | `false` 禁止新账号注册；默认允许 |
| `ALLOWED_EMAILS` | 允许注册的精确邮箱，逗号分隔 |
| `ALLOWED_EMAIL_DOMAINS` | 允许注册的域名，逗号分隔 |
| `DISABLE_WORKSPACE_CREATION` | `true` 禁止创建新 workspace |
| `MULTICA_DEV_VERIFICATION_CODE` | 非 production 的固定 6 位本地验证码 |
| `GOOGLE_CLIENT_ID`、`GOOGLE_CLIENT_SECRET`、`GOOGLE_REDIRECT_URI` | Google 登录 |
| `RESEND_API_KEY`、`RESEND_FROM_EMAIL` | Resend 邮件；无邮件 backend 时验证码写日志 |
| `SMTP_HOST`、`SMTP_PORT`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`SMTP_FROM_EMAIL` | SMTP relay；设置 `SMTP_HOST` 后优先于 Resend |
| `SMTP_TLS`、`SMTP_TLS_INSECURE`、`SMTP_EHLO_NAME` | STARTTLS/implicit TLS、私有 CA 和 EHLO 名 |

## 功能开关、Cloud 与服务端 LLM

| 变量 | 用途 |
| --- | --- |
| `MULTICA_FEATURE_FLAGS_FILE` | flag YAML 路径；文件缺失/语法错误会启动失败 |
| `FF_<FLAG_KEY>` | 环境级 kill switch/variant/百分比覆盖；优先于 YAML |
| `MULTICA_CLOUD_URL` | 托管 Fleet、Billing、entitlement、seat capacity 的统一内部服务地址；自托管留空 |
| `MULTICA_LLM_API_KEY` | 服务端 assist layer 上游 key |
| `MULTICA_LLM_BASE_URL` | OpenAI-compatible base URL；留空用默认 OpenAI URL |
| `MULTICA_LLM_DEFAULT_MODEL` | 服务端自动标题、follow-up、Twin/Skill replay 默认模型 |
| `MULTICA_LLM_MAX_RETRIES` | `0..5`；空为内建 2，非法值会阻止启动 |

`MULTICA_LLM_*` 只供服务端固定业务调用，不是通用 LLM proxy，也不会替代 daemon 中各 agent CLI 自己的凭据。实现与调用者约束在 [server/pkg/llm/](../../server/pkg/llm/)。

## Redis、Realtime 与指标

| 变量 | 默认/作用 |
| --- | --- |
| `REDIS_URL` | 可选共享 Redis：request store、PAT/daemon/membership cache、限流，以及 relay/lease fallback |
| `REDIS_DISABLE_CLIENT_NAME` | `true` 跳过 `CLIENT SETNAME`，兼容受限托管 Redis |
| `REALTIME_RELAY_REDIS_URL` | relay 专用 Redis；空则回退 `REDIS_URL` |
| `REALTIME_RELAY_MODE` | `sharded`（默认）、`dual`、`legacy` |
| `REALTIME_RELAY_SHARDS` | stream 分片数；默认由 realtime package 提供 |
| `REALTIME_RELAY_STREAM_MAXLEN` | 每 stream MAXLEN 压力阀；模板建议 2000 |
| `REALTIME_RELAY_XREAD_COUNT`、`REALTIME_RELAY_XREAD_BLOCK` | reader batch 与阻塞时间 |
| `REALTIME_RELAY_REPLAY_GRACE` | 重连 replay 窗口；模板建议 5m |
| `REALTIME_RELAY_TRIM_HORIZON` | 时间裁剪窗口；模板建议 10m |
| `REALTIME_RELAY_STREAM_TTL_ENABLED`、`REALTIME_RELAY_STREAM_TTL` | 整条 stream TTL，需分阶段启用 |
| `REALTIME_RELAY_TTL_REFRESH_INTERVAL`、`REALTIME_RELAY_MAINTENANCE_INTERVAL` | TTL refresh 与维护周期 |
| `METRICS_ADDR` | Prometheus listener；默认关闭，建议只绑定私网/loopback |
| `REALTIME_METRICS_TOKEN` | `/health/realtime` bearer token；反向代理部署应设置 |

没有 Redis 时 realtime 使用进程内 hub，适合单节点；多 API replica 应独立规划 relay Redis、request/cache Redis 与 channel lease Redis。

## Channel WebSocket lease

| 变量 | 默认 | 规则 |
| --- | --- | --- |
| `CHANNEL_WS_LEASE_BACKEND` | `postgres` | 多副本生产建议 `redis` |
| `CHANNEL_WS_LEASE_REDIS_URL` | 回退 `REDIS_URL` | lease 专用 Redis；Redis 模式不可用时 supervisor fail closed |
| `CHANNEL_WS_LEASE_NAMESPACE` | 空 | Redis 模式稳定部署命名空间 |
| `CHANNEL_WS_LEASE_TTL` | `180s` | lease TTL |
| `CHANNEL_WS_LEASE_RENEW_INTERVAL` | `60s` | renew |
| `CHANNEL_WS_LEASE_POLL_INTERVAL` | `30s` | 安装扫描/迁移延迟 |
| `CHANNEL_WS_LEASE_ERROR_RETRY_INTERVAL` | `5s` | Redis 错误重试 |
| `CHANNEL_WS_LEASE_EXPIRY_SAFETY_MARGIN` | `5s` | 过期前主动断开余量 |

必须满足 `0 < poll <= renew < TTL`；解析和校验见 [router.go](../../server/cmd/server/router.go)。

## 限流

| 变量 | 默认 | 端点/维度 |
| --- | --- | --- |
| `RATE_LIMIT_AUTH` | 5/min | send-code、Google，每 IP |
| `RATE_LIMIT_AUTH_VERIFY` | 20/min | verify-code，每 IP |
| `RATE_LIMIT_CONTACT_SALES` | 5/hour | contact-sales，每 IP |
| `RATE_LIMIT_PLUGIN_API` | 120/min | Plugin Public API |
| `RATE_LIMIT_INVITATION_ACTOR_10M` | 内建默认 | 邀请人 10 分钟预算，`0` 可禁用 |
| `RATE_LIMIT_INVITATION_WORKSPACE_24H` | 内建默认 | workspace 24 小时预算 |
| `RATE_LIMIT_INVITATION_RECIPIENT_24H` | 内建默认 | 收件邮箱摘要 24 小时预算 |
| `RATE_LIMIT_TRUSTED_PROXIES` | 空 | auth limiter 可信任 X-Forwarded-For 的 CIDR |

Redis 缺失时 auth 公共限流为 no-op；邀请 gate 会使用进程内内存，不能跨 replica 共享。

## 存储、附件与 CloudFront

| 变量 | 用途 |
| --- | --- |
| `S3_BUCKET`、`S3_REGION` | S3 bucket 与 region |
| `AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY` | 对象存储凭据 |
| `AWS_ENDPOINT_URL` | MinIO/R2/其他 S3-compatible endpoint |
| `S3_USE_PATH_STYLE` | path-style 开关；兼容本地对象存储 |
| `LOCAL_UPLOAD_DIR` | 未启用 S3 时的本地存储目录；默认 `./data/uploads` |
| `LOCAL_UPLOAD_BASE_URL` | 浏览器访问本地上传的 base URL |
| `ATTACHMENT_DOWNLOAD_MODE` | `auto`/代理或直连策略 |
| `ATTACHMENT_DOWNLOAD_URL_TTL` | signed URL TTL；默认 30m |
| `CLOUDFRONT_KEY_PAIR_ID`、`CLOUDFRONT_PRIVATE_KEY`、`CLOUDFRONT_PRIVATE_KEY_SECRET`、`CLOUDFRONT_DOMAIN` | CloudFront signed cookie/URL |

S3 可用时优先，否则尝试 LocalStorage；组合逻辑见 [router.go](../../server/cmd/server/router.go) 和 [server/internal/storage/](../../server/internal/storage/)。

## 渠道集成

| 集成 | 环境变量 | 行为 |
| --- | --- | --- |
| Lark/飞书 | `MULTICA_LARK_SECRET_KEY` | 32-byte base64 主密钥；未设置则整个集成返回 503/不启动 |
| Lark override | `MULTICA_LARK_HTTP_BASE_URL`、`MULTICA_LARK_CALLBACK_BASE_URL`、`MULTICA_LARK_WS_PROXY_URL`、`MULTICA_LARK_REGISTRATION_DOMAIN`、`MULTICA_LARK_REGISTRATION_LARK_DOMAIN` | 测试、代理或单云 host override；正常多区域部署通常留空 |
| Slack | `MULTICA_SLACK_SECRET_KEY` | 加密每安装 BYO bot/app token；未设置则禁用 |
| DingTalk | `MULTICA_DINGTALK_SECRET_KEY` | 加密 AppSecret；未设置则禁用 |
| WeCom | `MULTICA_WECOM_SECRET_KEY` | 加密智能机器人 secret；未设置则禁用 |
| WeCom 媒体/调试 | `MULTICA_WECOM_MEDIA_ALLOW_CIDRS`、`MULTICA_WECOM_TRACE` | fake-IP 代理 allowlist；`TRACE=1` 会记录有界消息正文，只在短时诊断启用 |
| Telegram | `MULTICA_TELEGRAM_SECRET_KEY` | 加密每安装 bot token；未设置则禁用 |

安装级 bot/app/token 不应放进 `.env`，而是由设置页写入 `channel_installation` 的加密 config。

## GitHub、VCS、Composio 与 Plugin

| 领域 | 变量 |
| --- | --- |
| GitHub App | `GITHUB_APP_SLUG`、`GITHUB_WEBHOOK_SECRET`；PR enrichment/repository picker 还需 `GITHUB_APP_ID`、`GITHUB_APP_PRIVATE_KEY` |
| 自托管 VCS | `MULTICA_VCS_INTEGRATION_ENABLED=true` + `MULTICA_VCS_SECRET_KEY`；支持 Forgejo/Gitea/GitLab token 加密 |
| Composio | `COMPOSIO_API_KEY`、`COMPOSIO_CALLBACK_BASE_URL`、`COMPOSIO_STATE_SECRET`；state secret 可回退派生自 `JWT_SECRET` |
| Plugin secret/surface | `MULTICA_PLUGIN_SECRET_KEY`、`MULTICA_PLUGIN_SURFACE_ORIGIN` |
| Plugin callback/dev publish | `MULTICA_PLUGIN_API_URL`、`MULTICA_PLUGIN_DIR` |
| Plugin 本地开发 | `MULTICA_PLUGIN_DEV_ORIGINS`、`MULTICA_PLUGIN_DEV_CA` |

Plugin surface origin 必须与 app/API origin 分离且不携带 app session cookie。

## Analytics

| 变量 | 默认/作用 |
| --- | --- |
| `POSTHOG_API_KEY` | 空时启用 no-op client，不发送事件 |
| `POSTHOG_HOST` | `https://us.i.posthog.com` |
| `ANALYTICS_ENVIRONMENT` | 覆盖 production/staging/dev 事件属性 |
| `ANALYTICS_DISABLED` | 强制 no-op，适合 CI/opt-out |

## Daemon 与 agent CLI

常用 operator 配置：

| 变量 | 用途 |
| --- | --- |
| `MULTICA_DAEMON_CONFIG`、`MULTICA_DAEMON_ID`、`MULTICA_DAEMON_DEVICE_NAME` | daemon 配置路径/身份 |
| `MULTICA_DAEMON_POLL_INTERVAL`、`MULTICA_DAEMON_HEARTBEAT_INTERVAL` | polling/heartbeat；模板为 30s/15s |
| `MULTICA_WORKSPACE_ID` | 单 workspace 启动/测试上下文 |
| `MULTICA_CODEX_PATH`、`MULTICA_CODEX_MODEL`、`MULTICA_CODEX_WORKDIR` | Codex executable/model/workdir |
| `MULTICA_RUNTIME_RECONNECT_GRACE` | runtime 离线后的任务保留窗口；默认 3h |
| `MULTICA_WORKSPACES_ROOT`、`MULTICA_REPO_CHECKOUT_MODE` | daemon workspace 根与 checkout 策略 |
| `MULTICA_GC_ENABLED`、`MULTICA_KEEP_ENV_AFTER_TASK` | 本地任务 GC 与调试保留 |

运行中的 task 还会由 daemon 注入 `MULTICA_TASK_ID`、`MULTICA_AGENT_ID`、`MULTICA_TOKEN`、`MULTICA_SERVER_URL` 等 task-scoped 变量；这些是内部执行契约，不是长期部署 secret。

## Web、Desktop 与 Mobile

### Web

| 变量 | 用途 |
| --- | --- |
| `NEXT_PUBLIC_API_URL` | 浏览器 API origin；不要附加 `/api` 路径 |
| `NEXT_PUBLIC_WS_URL` | 显式 WS URL；空时从 API/page origin 推导 |
| `REMOTE_API_URL` | Next SSR/dev proxy 的远程 backend |
| `DOCS_URL` | Docs rewrite/链接目标 |
| `NEXT_PUBLIC_APP_VERSION` | UI 版本；构建时通常由 tag 注入 |
| `NEXT_PUBLIC_ENABLE_CLOUD_RUNTIME` | 显示 Cloud Runtime 页面 |
| `NEXT_PUBLIC_REACT_SCAN` | 本地 React Scan 开关 |

解析与防止 `/api/api` 的规范化逻辑在 [apps/web/config/runtime-urls.ts](../../apps/web/config/runtime-urls.ts)。

### Desktop

| 变量 | 用途 |
| --- | --- |
| `VITE_API_URL`、`VITE_WS_URL`、`VITE_APP_URL` | renderer/API/WS/Web origin；解析见 [runtime-config.ts](../../apps/desktop/src/shared/runtime-config.ts) |
| `DESKTOP_RENDERER_PORT` | Electron Vite renderer 端口 |
| `DESKTOP_APP_SUFFIX` | 多 worktree 并行的 app 名/userData 隔离 |
| `VITE_REACT_GRAB` | 开发期 renderer 调试开关 |

### Mobile

| 变量 | 用途 |
| --- | --- |
| `EXPO_PUBLIC_API_URL` | 必需；手机可达的 API origin，编译进 bundle |
| `EXPO_PUBLIC_WEB_URL` | 从 native 页面打开 Web issue/project/wiki 的 origin |
| `APP_ENV` | development/staging/production 变体 |
| `EXPO_BUNDLE_IDENTIFIER_DEV`、`EXPO_BUNDLE_IDENTIFIER_PROD` | bundle id override |
| `EXPO_APPLE_TEAM_ID` | iOS signing team |

模板与加载说明见 [apps/mobile/.env.example](../../apps/mobile/.env.example) 和 [app.config.ts](../../apps/mobile/app.config.ts)。

## 测试专用变量

`REDIS_TEST_URL`、`PLAYWRIGHT_BASE_URL`、`PLAYWRIGHT_RECORD_VIDEO`、`MULTICA_RUN_REAL_AGENT_SMOKE`、`MULTICA_RUN_SKILLS_SH_INTEGRATION`、`CLAUDE_FAKE_MODE` 等只用于测试/显式 smoke。默认测试不得解析或执行用户已安装的 agent CLI；真实账号 smoke 必须按 [CLAUDE.md](../../CLAUDE.md) 的显式授权命令运行。

## 去哪里改

| 目标 | 位置 |
| --- | --- |
| 新自托管变量与说明 | [.env.example](../../.env.example)，并同步 Compose/Helm/SELF_HOSTING 文档 |
| API 启动变量 | [server/cmd/server/main.go](../../server/cmd/server/main.go) |
| Router/集成变量 | [server/cmd/server/router.go](../../server/cmd/server/router.go) |
| Feature flag | [server/pkg/featureflag/](../../server/pkg/featureflag/) 与调用方默认值 |
| Web URL | [apps/web/config/runtime-urls.ts](../../apps/web/config/runtime-urls.ts) |
| Desktop URL | [apps/desktop/src/shared/runtime-config.ts](../../apps/desktop/src/shared/runtime-config.ts) |
| Mobile env | [apps/mobile/.env.example](../../apps/mobile/.env.example)、[app.config.ts](../../apps/mobile/app.config.ts) |

## 相关页面

- [参考资料](index.md)
- [配置](../operations/configuration.md)
