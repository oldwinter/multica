# CI、测试与发布

Multica 把 TypeScript/Vitest、Go、数据库迁移、生成代码、平台回归与发布验证分开运行。根命令定义于 [`package.json`](../../package.json)、[`Makefile`](../../Makefile) 和 [`turbo.json`](../../turbo.json)。

## 本地测试层级

### TypeScript

```bash
pnpm typecheck
pnpm test
pnpm lint
pnpm build
```

- `pnpm typecheck`、`lint`、`build` 通过 Turborepo 运行除 Mobile 外的工作区。
- `pnpm test` 先校验 Office assets，再运行除 Mobile 外的 workspace test。
- `packages/core`、`packages/views`、Web、Desktop 和 Docs 主要使用 Vitest；`packages/ui` 还运行 Node token contract 测试。
- Mobile 独立执行 `pnpm exec turbo typecheck lint test --filter=@multica/mobile`。

可按 package 缩小范围：

```bash
pnpm -C packages/core test
pnpm -C packages/views test -- --run path/to/file.test.tsx
pnpm -C apps/web test
```

Turborepo 的 `test` 不执行依赖包测试，而用 [`turbo.json`](../../turbo.json) 的 `^cache-inputs` 把依赖源码 hash 纳入消费者 cache key；Views 测试在 CI 额外分成两个 Vitest shard。

### Go

```bash
make test
# 或窄范围
(cd server && go test ./internal/service -run TestName -count=1)
```

`make test` 确保目标数据库存在、运行迁移，再调用 [`scripts/test-go.sh`](../../scripts/test-go.sh) 的 `--race` 模式。wrapper：

1. 普通 package 一组运行；
2. `pkg/agent/...` 用 `-p 2 -parallel 2` 限制 subprocess/race 资源争用；
3. [`scripts/go-test-with-agent-cli-guard.sh`](../../scripts/go-test-with-agent-cli-guard.sh) 在 PATH 前放 sentinel，任何默认测试误调用户已登录的真实 Agent CLI 都会失败。

需要真实 Agent 账号/额度的 smoke test 必须使用 `agentintegration` build tag 和显式 `MULTICA_RUN_REAL_AGENT_SMOKE=1`，不属于普通 CI。

### E2E

[`playwright.config.ts`](../../playwright.config.ts) 只运行 headless Chromium、1 worker、0 retry，且不会自动启动服务：

```bash
make up
make env-exec ARGS="-- pnpm exec playwright test"
```

或使用统一流程：

```bash
make check
```

[`scripts/check.sh`](../../scripts/check.sh) 顺序为 typecheck、TS unit、Go、启动/复用 API 与 Web、Playwright。它只停止自己启动的服务。

> 当前 GitHub Actions 主 CI **没有运行 Playwright E2E**；E2E 是 `make check`/显式本地命令负责的门。不要因为 PR 的 `frontend`/`backend` 绿色就声称 E2E 已通过。

## 主 CI

[`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) 在 `main` push 和以 `main` 为目标的 PR 触发。并发组按 workflow + PR/ref，后续提交取消旧 run。

同仓 PR 和可信 push 使用带 `multica` label 的持久 self-hosted Linux X64 runner；外部 fork PR 使用临时 `ubuntu-latest`，避免不受信代码在持久主机执行。

### 路径门控

`changes` job 用 `dorny/paths-filter` 计算 frontend、backend、sqlc、images：

- PR 只运行受影响区域。
- push 到 main 强制 frontend/backend/sqlc 全量。
- 聚合 job 本身不 skip；被路径过滤的步骤仍让必需的 `frontend`/`backend` check 返回真正 success。
- `apps/mobile` 有单独 workflow。

### Frontend 作业

| 作业 | 内容 |
| --- | --- |
| `frontend-build` | Node 22、pnpm install、Turborepo cache、自托管/env/worktree registry shell 测试、reserved slugs drift、build/typecheck/lint、UI export contract |
| `frontend-test` | Web、Core、Desktop 等非 Views Vitest |
| `frontend-views-test` | `@multica/views` 的 2 shard 矩阵 |
| `frontend` | 聚合以上结果，保持稳定 required-check 名 |

Docs 和 Mobile 被主 Turbo build/test 过滤；Docs 变化仍会因 knip/path 规则触发 frontend job。`pnpm knip` 当前是 `continue-on-error` 警告，不是阻塞门。

### Backend 作业

`backend-tests` 使用 PostgreSQL `pgvector/pgvector:pg17`（宿主端口 55432）和 Redis 7 Alpine（56379）service，Go 取最新 1.26.x。它执行：

1. Helm 模板/config shell 测试；
2. backend entrypoint signal 测试；
3. Windows build 输出命名 shell 测试；
4. `go build ./...`；
5. `migrate up`；
6. Go wrapper 自测；
7. `scripts/test-go.sh --race`。

并行的 `sqlc-check` 运行 `make sqlc` 并拒绝 generated diff/新未跟踪文件；`go-vulnerability-scan` 执行 `go tool govulncheck ./...`。`backend` 聚合三者。

`windows-execenv` 在真实 `windows-latest` 上针对 Job Object、PowerShell shim、超长 stdin、junction、process tree、task 临时目录和可执行文件解析运行精确测试。它不是完整 Windows Go suite。

### 其他 workflow

| Workflow | 触发/矩阵 | 用途 |
| --- | --- | --- |
| [`.github/workflows/mobile-verify.yml`](../../.github/workflows/mobile-verify.yml) | Mobile/Core 等路径；Node 22 | Mobile typecheck、lint、Vitest 与 iOS runner script test |
| [`.github/workflows/downstream-features.yml`](../../.github/workflows/downstream-features.yml) | main、PR、手工；PG+Redis | Rooms、Wiki、Twin、命名皮肤和下游 contract |
| [`.github/workflows/desktop-smoke.yml`](../../.github/workflows/desktop-smoke.yml) | 手工；Linux/Windows | x64 + arm64 Desktop 打包，不发布 |
| `installer`（主 CI 内） | Ubuntu/macOS/Windows | shell 与 PowerShell installer stub 测试 |
| `image-budget`（主 CI 内） | PR 修改 bitmap | 新增/变大的 300KB+ bitmap 要求 PR 描述说明 |

## 发布触发

[`.github/workflows/release.yml`](../../.github/workflows/release.yml) 接受 tag push：

```text
vX.Y.Z
vX.Y.Z-suffix
```

严格校验由 job 内 regex 完成；`-dirty` 被拒绝。无 suffix 是 stable，suffix 自动成为 prerelease。普通发布应从已评审、主 CI 绿色的 `main` commit 创建原子 tag：

```bash
git tag v0.4.38
git push origin v0.4.38
```

> **发布警告：** 推送 tag 会触发 GitHub Release、GHCR 镜像和可能的 Homebrew/Helm 发布，是远端且不易撤销的操作。创建前核对 commit、版本、迁移升级路径和 CI；不要用移动/覆盖 tag 修正错误，应发布新版本。

`workflow_dispatch` 的存在主要是让下游 auto-tagger 对 GITHUB_TOKEN 创建的 tag 显式启动 Release；从 branch 手工 dispatch 会因 tag-name 校验失败。

## 发布验证与产物

`verify` 在任何发布前：

- 记录最新 Go 1.26.x toolchain；
- 自测 Go wrapper；
- `scripts/test-go.sh --race`；
- `govulncheck ./...`。

发布 workflow 本身不重复 TypeScript、Mobile 或 E2E，因此发布 commit 必须先通过正常 CI 和需要的本地验收。

### CLI / GitHub Release

[`.goreleaser.yml`](../../.goreleaser.yml) 构建：

| OS | 架构 |
| --- | --- |
| darwin | amd64、arm64 |
| linux | amd64、arm64 |
| windows | amd64、arm64 |

产物同时保留旧 `multica_<os>_<arch>` 和新 `multica-cli-<version>-<os>-<arch>` 命名，附 `checksums.txt`。只有 `multica-ai` canonical owner 上传 Homebrew tap；fork 仍向自己的 Release 发布 CLI，但跳过 upstream tap。

### 容器与 Helm

backend/Web 分别在原生 amd64 `ubuntu-latest` 和 arm64 `ubuntu-24.04-arm` 上按 digest 构建，再合并 multi-arch manifest，避免 QEMU。tag 包含：

- 精确 release tag；
- `sha-...`；
- 仅 stable release 更新 `latest`。

Helm chart 只在 repository owner 为 `multica-ai` 时发布到 `oci://ghcr.io/<owner>/charts/multica`；chart version 去掉 `v`，`appVersion` 保留 release tag。

### Desktop

Release Desktop 矩阵：

| runner | 目标 | 产物 |
| --- | --- | --- |
| Ubuntu | Linux x64/arm64 | AppImage、deb、rpm |
| Windows | Windows x64/arm64 | NSIS |
| macOS | macOS x64/arm64 | DMG、ZIP |

当前 CI 禁用 certificate auto-discovery；macOS 产物未签名、未 notarize，自动更新也不能依赖该临时路径。详情见 [`.github/RELEASING.md`](../../.github/RELEASING.md)。

## 下游发布

`oldwinter/multica` 使用带 `-oldwinter.N` 后缀的 tag 区分下游构建，例如
`v0.4.36-oldwinter.1`。当前快照没有跟踪自动 tag workflow；发布仍以
[`.github/workflows/release.yml`](../../.github/workflows/release.yml)
接受的 tag push 为入口。新增自动 tag 流程时，应同时更新发布说明、测试和
workflow 触发语义，避免 `GITHUB_TOKEN` 创建的 tag 因 GitHub 事件递归保护而漏掉发布。

脚本在 HEAD 已有匹配 tag 时幂等复用。下游 suffix release 被归类为 prerelease，不更新 `latest`。

## 紧急漏洞扫描旁路

发布默认 fail closed。只有 `govulncheck` 服务/数据库不可用，或有经维护者确认并记录的误报阻塞紧急发布时，才可把 repository variable `ALLOW_VULN_BYPASS_FOR_TAG` 设置为**精确 tag**。发布后必须立即删除，并在事故记录中保留 workflow URL。不能用它发布尚未解决的 reachable vulnerability。

提交代码的测试放置和工程规则见[模式与约定](../how-to-contribute/patterns-and-conventions.md)；部署发布产物见[部署与自托管](deployment-and-self-hosting.md)。
