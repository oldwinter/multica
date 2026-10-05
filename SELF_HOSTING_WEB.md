# 仅使用 Web 和后端的自托管指南

Multica 可以只运行 Web、后端和 PostgreSQL，不需要安装 Desktop 客户端或 CLI。

这种模式适合浏览器内的协作、任务、项目、Wiki、Rooms、Twin、设置和集成功能。需要注意：如果没有任何机器运行 Multica CLI 和守护进程，本地 AI 智能体不会领取或执行任务。Web 和后端仍然可用，但运行时相关功能会显示为离线或不可执行。

## 选择启动方式

| 目标 | 推荐命令 | 特点 |
| --- | --- | --- |
| 日常开发与快速调试 | `make up` | 只启动 API 和 Web，自动准备数据库、迁移和隔离端口 |
| 在前台查看完整日志 | `make dev` | API 和 Web 日志直接输出到当前终端 |
| 验证当前源码的自托管形态 | `make selfhost-build` | 从当前 checkout 构建 Docker 镜像，最接近实际部署 |
| 运行已经发布的版本 | `make selfhost` | 从 GHCR 拉取指定版本的后端和 Web 镜像 |

日常修改代码时优先使用 `make up`。只有在验证容器、镜像、生产环境变量或部署流程时，才使用 `make selfhost-build`。

## 日常开发与调试

### 前置依赖

- Node.js 26
- pnpm 10.28.2
- Go 1.27
- Docker 和 Docker Compose
- `curl`

### 启动

```bash
cd /path/to/multica
make up
```

`make up` 默认启动 `api,web`，不会启动 Desktop 或守护进程。首次运行会：

1. 创建 `.env` 或工作树专用的 `.env.worktree`。
2. 安装尚未安装的 pnpm 依赖。
3. 启动或复用 PostgreSQL。
4. 创建隔离数据库并执行迁移。
5. 分配不会与其他 checkout 冲突的后端和 Web 端口。
6. 启动 Go API 和 Next.js 开发服务器。

端口可能不是 `8080/3000`，应以启动结果或以下命令为准：

```bash
make status
```

状态输出包含 Web 地址、后端地址、进程信息、Git commit 和日志目录。

### 登录

本地开发环境会在环境文件中配置开发验证码，并在启动结果中显示登录邮箱和验证码。该验证码仅用于本机开发，不能用于公网实例。

### 修改 Web

Next.js 开发服务器支持热更新。修改 `apps/web/`、`packages/core/`、`packages/ui/` 或 `packages/views/` 后，通常无需重启。

### 修改后端

仓库没有默认启用 Go 自动重载。修改 `server/` 后，只重启 API：

```bash
make down ARGS="--components api"
make up C=api,web
```

仍在运行的 Web 会被复用。

### 前台调试

需要直接观察 API 和 Web 日志时：

```bash
make down
make dev
```

`make dev` 会在前台运行两个服务。按 `Ctrl+C` 会同时停止它们，PostgreSQL 数据仍然保留。

后台模式的日志目录可通过 `make status` 查询：

```bash
make status
tail -f <日志目录>/api.log <日志目录>/web.log
```

### 停止或删除

```bash
make down
```

该命令停止进程但保留数据库，下一次启动通常只需要几秒。

如果需要删除数据库、开发账号、日志和端口登记：

```bash
make destroy
```

`make destroy` 会删除本地开发数据，不要把它当作普通停止命令。

## 从当前源码构建自托管环境

要验证当前 checkout 的完整 Docker 部署形态：

```bash
make selfhost-build
```

该命令会生成自托管所需的随机密钥，从当前源码构建后端和 Web 镜像，然后启动 PostgreSQL、后端和 Web。

默认地址：

- Web：<http://localhost:3000>
- 后端：<http://localhost:8080>
- 健康检查：<http://localhost:8080/health>

查看日志：

```bash
docker compose -f docker-compose.selfhost.yml logs -f backend frontend
```

停止服务但保留 Docker volume：

```bash
make selfhost-stop
```

源码发生变化后需要重新构建镜像。这个循环比 `make up` 慢，因此更适合发布前验证，而不是日常开发。

## 运行个人发布版本

以下命令使用 `oldwinter` 的后端和 Web 镜像。将 tag 替换为需要部署的版本：

```bash
make selfhost \
  MULTICA_IMAGE_TAG=v0.4.32-oldwinter.2 \
  MULTICA_BACKEND_IMAGE=ghcr.io/oldwinter/multica-backend \
  MULTICA_WEB_IMAGE=ghcr.io/oldwinter/multica-web
```

首次启动会创建 `.env` 并生成 `JWT_SECRET`、PostgreSQL 密码和 VCS 加密密钥。启动完成后，把以下三个值持久化到 `.env`，否则以后不带参数执行 `make selfhost` 时会回到官方镜像配置：

```dotenv
MULTICA_IMAGE_TAG=v0.4.32-oldwinter.2
MULTICA_BACKEND_IMAGE=ghcr.io/oldwinter/multica-backend
MULTICA_WEB_IMAGE=ghcr.io/oldwinter/multica-web
```

如果 GHCR 镜像是私有的，需要先登录：

```bash
docker login ghcr.io
```

## 自托管登录

Docker 自托管环境默认使用 `APP_ENV=production`，不会接受固定开发验证码。

- 正式部署：在 `.env` 配置 `RESEND_API_KEY` 或 SMTP。
- 本机一次性验证：不配置邮件服务，从后端日志读取生成的验证码。

```bash
docker compose -f docker-compose.selfhost.yml logs -f backend
```

不要在公网实例设置 `APP_ENV=development` 或固定的 `MULTICA_DEV_VERIFICATION_CODE`。知道用户邮箱的人可能借此直接登录。

## 从其他机器访问

Docker Compose 默认只把 Web 和后端绑定到 `127.0.0.1`，不会直接暴露到局域网或公网。

临时远程调试可以使用 SSH tunnel：

```bash
ssh \
  -L 3000:127.0.0.1:3000 \
  -L 8080:127.0.0.1:8080 \
  user@server
```

然后在本机浏览器打开 <http://localhost:3000>。

正式部署时保持容器端口绑定在 loopback，在前面部署 Caddy、Nginx 或等价的反向代理，并配置 HTTPS、正确的 `FRONTEND_ORIGIN` 和公开 URL。不要直接把 PostgreSQL 或后端原始端口暴露到公网。

## 验证

开发环境先查看实际端口：

```bash
make status
```

自托管默认端口可以直接检查：

```bash
curl -fsS http://localhost:8080/health
curl -fsSI http://localhost:3000
```

如果 Web 可以打开，但接口请求失败，优先检查：

1. `make status` 或后端容器健康状态。
2. `NEXT_PUBLIC_API_URL` 是否误带 `/api` 路径；它只能填写 origin。
3. `FRONTEND_ORIGIN` 是否与浏览器访问地址一致。
4. 后端和 Web 是否使用了同一组端口或域名配置。

完整生产部署、邮件、对象存储、OAuth 和高级安全配置参见 [SELF_HOSTING.md](SELF_HOSTING.md) 与 [SELF_HOSTING_ADVANCED.md](SELF_HOSTING_ADVANCED.md)。
