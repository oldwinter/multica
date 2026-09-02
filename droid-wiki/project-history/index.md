# 项目历史

Multica 的 Git 历史很短，却不是一条直线。仓库在 2026 年 1 月 28 日以 13 个 TypeScript 文件起步；51 天后，一次明确写着“pivot to AI-native task management platform”的提交把原有智能体框架替换成 Go 服务、Next.js 客户端和工作区协作模型。此后 Desktop、Mobile、共享包、实时系统、渠道集成、插件以及下游的 Twin、Wiki、Rooms、命名皮肤和 Skill Evolution 逐步进入同一棵树。

本章只陈述默认分支在固定快照上的可验证事实：

- 快照：[685cf788777e24724b0d82ffe88c56459341fb2a](https://github.com/oldwinter/multica/commit/685cf788777e24724b0d82ffe88c56459341fb2a)
- 快照作者日期：2026-09-01；收集日期：2026-09-01
- 根提交：[6b34ddc3dc7a2cf5b7054492f0bf0f46cc286e4d](https://github.com/oldwinter/multica/commit/6b34ddc3dc7a2cf5b7054492f0bf0f46cc286e4d)，2026-01-28
- 默认分支可达提交：5,315
- 默认分支可达 tag：145
- 跟踪文件：6,408
- 去重作者身份：306；统计只使用已提交的 author name，不展示邮箱

## 阅读路径

| 页面 | 回答的问题 |
| --- | --- |
| [时间线](timeline.md) | 哪些日期真正改变了产品或发布节奏？ |
| [架构演进](architecture-evolution.md) | 各子系统何时首次出现，又如何变成今天的边界？ |
| [项目掌故](../lore.md) | 哪些命名、重启和版本号细节能由源码或 Git 证明？ |
| [代码库数据](../by-the-numbers.md) | 文件、语言、测试、应用、包和服务端分别有多大？ |

如需理解当前系统而非形成过程，请转到[代码库概览](../overview/index.md)和[系统架构](../overview/architecture.md)。

## 四个阶段

### 1. 原始智能体框架，2026-01-28 至 2026-03-19

根提交只有 `src/agent/`、`src/client/`、`src/gateway/` 与 `src/shared/` 等 13 个文件。次日加入 Turborepo 和 Web，随后出现 UI 包、Electron Desktop、Expo Mobile。2026-02-10 的[重组提交](https://github.com/oldwinter/multica/commit/6ef58a0cab97e9579a7c9d5a9702248c70768a1c)已经把项目整理成 monorepo，但其业务中心仍是早期智能体框架。

### 2. AI 原生任务管理平台，2026-03-20 至 2026-04

[3 月 20 日的转向提交](https://github.com/oldwinter/multica/commit/d4f5c5b16fdb75b223b5e77a9948843bd8436554)不是渐进重构：提交说明明确写出替换原代码库，并删除当时的 Desktop、Mobile、Gateway 与旧 `packages/core`。新树建立 Go/Chi/sqlc/PostgreSQL 服务、工作区隔离、任务与智能体模型、REST 和 WebSocket。接下来一周形成 task service、统一 CLI/守护进程、每 task 隔离环境和事件总线。

4 月 9 日，`packages/core`、`packages/ui`、`packages/views` 被重新提炼为今天的共享层；Desktop 以 electron-vite 形态回归。4 月 10 日加入 Fumadocs 文档站。

### 3. 产品面与运行基础设施扩张，2026-05 至 2026-07

Mobile 在 5 月 22 日以“Multica for iOS — first version”重新进入仓库。守护进程从 REST 轮询扩展出 WebSocket 唤醒，实时广播增加 Redis 分片 relay；6 月的 Lark MVP 又演进为平台无关的渠道引擎，随后承载 Slack、钉钉、企业微信和 Telegram。7 月开始的 `v0.4.x` 系列对应一个已同时拥有 Web、Desktop、Mobile、Docs 与多运行时的产品树。

### 4. 上游产品与下游功能并行，2026-08 至快照

当前仓库是 `multica-ai/multica` 的 fork。8 月先后落地 Twin、LM Wiki、Rooms 与命名皮肤；同期上游重建插件 Action API、Surface 与 SDK。Skill Evolution 在 8 月 28 日开始冻结领域契约并作为下游 sidecar 组合。下游通过频繁合并 `upstream/main` 保留这些叶模块；[同步账本](https://github.com/oldwinter/multica/blob/685cf788777e24724b0d82ffe88c56459341fb2a/docs/downstream/upstream-sync.md)记录了从 40 个冲突文件下降到多次零文本冲突、同时保持已发布迁移身份的过程。

## 统计口径

所有数字都以同一个提交为对象，而不是易变的工作区：

```bash
SNAPSHOT=685cf788777e24724b0d82ffe88c56459341fb2a
git rev-parse "$SNAPSHOT"
git rev-list --count "$SNAPSHOT"
git tag --merged "$SNAPSHOT" | wc -l
git ls-tree -r --name-only "$SNAPSHOT" | wc -l
git log --use-mailmap --format='%aN' "$SNAPSHOT" | sort -u | wc -l
```

`--use-mailmap` 明确启用 Git 的身份映射规则；该快照本身没有跟踪 `.mailmap`，因此页面不会擅自把相似显示名合并。行数由 `git grep -I -c '^'` 对快照中的非二进制文本计数，详见[代码库数据](../by-the-numbers.md)。个人提交排行不作为代码库健康指标；当前领域维护线索见[维护上下文](../maintainers.md)。
