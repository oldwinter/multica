# 时间线

这条时间线以提交作者日期和可达 tag 的创建日期为准。它不是 changelog 的改写，而是从默认分支历史中挑出改变产品方向、运行边界、共享方式或发布方式的节点。

## 2026 年 1—2 月：从 13 个文件到多客户端 monorepo

| 日期 | 事件 | 证据 |
| --- | --- | --- |
| 01-28 | 仓库建立：`src/agent`、`client`、`gateway`、`shared`，共 13 个文件 | [6b34ddc3](https://github.com/oldwinter/multica/commit/6b34ddc3dc7a2cf5b7054492f0bf0f46cc286e4d) |
| 01-29 | Turborepo 与 Web 客户端首次进入 `apps/web` | [84fe9d99](https://github.com/oldwinter/multica/commit/84fe9d99f564e74df9fc49fb227a909245ba3e48) |
| 01-30 | `@multica/ui` 骨架与 Electron Desktop 首次出现 | [e1443720](https://github.com/oldwinter/multica/commit/e1443720ebe1ed7cab033977bc0f43f4c1f2bddc)、[ff026b12](https://github.com/oldwinter/multica/commit/ff026b122d755b5f5ee96f16c7263ddc68428e70) |
| 02-03 | 第一代 Expo Mobile 初始化 | [01c82b29](https://github.com/oldwinter/multica/commit/01c82b296d286f170f3d58ddd111e8bfafad2806) |
| 02-10 | 早期代码重组为 monorepo，第一代 `packages/core` 出现 | [6ef58a0c](https://github.com/oldwinter/multica/commit/6ef58a0cab97e9579a7c9d5a9702248c70768a1c) |

## 2026 年 3 月：产品转向与执行主链形成

| 日期 | 事件 | 证据 |
| --- | --- | --- |
| 03-20 | 明确转向“AI-native task management platform”；引入 Go/Chi/sqlc/PostgreSQL、工作区、任务、智能体和实时 Hub，删除旧框架及第一代 Desktop/Mobile | [d4f5c5b1](https://github.com/oldwinter/multica/commit/d4f5c5b16fdb75b223b5e77a9948843bd8436554) |
| 03-23 | task service 和守护进程 REST 协议出现 | [5a3a72c4](https://github.com/oldwinter/multica/commit/5a3a72c4110f1512570e45c5295623f11bb8b136) |
| 03-24 | 守护进程并入 Cobra `multica` CLI | [707b5ac6](https://github.com/oldwinter/multica/commit/707b5ac6e759e4f1819d1961bd997ecfb085e06c) |
| 03-25 | 每 task 隔离执行环境；事件总线、WebSocket 工作区隔离和全局 store 迁移 | [678266ec](https://github.com/oldwinter/multica/commit/678266ec87da8dd01adabb356b9395eb27de36b2)、[9127e543](https://github.com/oldwinter/multica/commit/9127e543d5c2a0cdd249b5c508b6df4a2d08ab67) |
| 03-25 | 当前 HEAD 可达的首个标准版本 tag `v0.1.8` | [v0.1.8](https://github.com/oldwinter/multica/releases/tag/v0.1.8) |
| 03-29 | 合并队列和 task 生命周期护栏；同一 agent/issue 复用工作目录与 bare clone cache | [b112d1f1](https://github.com/oldwinter/multica/commit/b112d1f1aeeda36f4613d6131c843d8cd469fb56)、[ce2b263e](https://github.com/oldwinter/multica/commit/ce2b263ea5e003f372a6919c4ade2386b64b5b81)、[cdc1ac70](https://github.com/oldwinter/multica/commit/cdc1ac708e0569ad1bca618cd70f33bfd1704bc0) |

## 2026 年 4 月：共享边界、Desktop 与实时系统

| 日期 | 事件 | 证据 |
| --- | --- | --- |
| 04-09 | 今天的 `core`、`ui`、`views` 三层被分别抽取；`views` 通过导航 adapter 同时服务 Web/Desktop | [e1e7f683](https://github.com/oldwinter/multica/commit/e1e7f68330e9864af36ce63520355239148db6d7)、[35828492](https://github.com/oldwinter/multica/commit/35828492d565f986686590194a748fd5960893d0)、[f41a0cf4](https://github.com/oldwinter/multica/commit/f41a0cf4236ccd6fc9896775126e7d41461b2f9d) |
| 04-09 | 第二代 Desktop 以 electron-vite 骨架回归 | [74cc1d48](https://github.com/oldwinter/multica/commit/74cc1d488ee4f99f0dadf90c1149d817609ed3e4) |
| 04-10 | Fumadocs 文档站加入 | [17ae320d](https://github.com/oldwinter/multica/commit/17ae320dd255769fb7ccfcfcbbdc281fbf3c6edc) |
| 04-15 | `v0.1.35` 与 `v0.2.0` 同日，发布序列进入 0.2 | [v0.1.35](https://github.com/oldwinter/multica/releases/tag/v0.1.35)、[v0.2.0](https://github.com/oldwinter/multica/releases/tag/v0.2.0) |
| 04-26 | Redis relay 连接池隔离，随后实现分片实时 relay | [141c294c](https://github.com/oldwinter/multica/commit/141c294cdb98888b574bf7201955011919478cce)、[18524d80](https://github.com/oldwinter/multica/commit/18524d80d0edf2ed329a46fec8ae84d27489c0a0) |
| 04-28 | 守护进程 WebSocket task 唤醒加入，轮询不再是唯一通知路径 | [9db91e89](https://github.com/oldwinter/multica/commit/9db91e89f55952af14cbafe03d40a8575da2115c) |

## 2026 年 5—7 月：移动端、渠道与平台化

| 日期 | 事件 | 证据 |
| --- | --- | --- |
| 05-14 | `v0.3.0` 发布 | [v0.3.0](https://github.com/oldwinter/multica/releases/tag/v0.3.0) |
| 05-22 | 第二代 Mobile 以 iOS 首版回归 | [fd0fe1d0](https://github.com/oldwinter/multica/commit/fd0fe1d08a18784462fc1ceb363adfbb31601878) |
| 06-03 | Lark Bot MVP 建立迁移与 service 边界 | [8c98940b](https://github.com/oldwinter/multica/commit/8c98940b797e4cc98ea458ab473b0d8002300a15) |
| 06-24 | 渠道基础被抽象为平台无关层和 Supervisor/Router 引擎 | [ce28d0aa](https://github.com/oldwinter/multica/commit/ce28d0aa0edad0e28948c3ea2e34a857cb1d9cbe)、[3e21e58d](https://github.com/oldwinter/multica/commit/3e21e58df03cdb12b98efa471e7d49557bc5079b) |
| 06-25 | Slack Socket Mode adapter 接入共享渠道引擎 | [cb6616f5](https://github.com/oldwinter/multica/commit/cb6616f5304aafb75aa23f177c5cc1a4b803e4bd) |
| 07-13 | `v0.4.0` 发布 | [v0.4.0](https://github.com/oldwinter/multica/releases/tag/v0.4.0) |

## 2026 年 8 月—9 月 1 日：下游产品与高频同步

| 日期 | 事件 | 证据 |
| --- | --- | --- |
| 08-03 | Twin review workspace UI 与 Tension Map 设计文件进入下游 | [8c5e86ed](https://github.com/oldwinter/multica/commit/8c5e86ed682f8be122570cb9cd866610ebd84528) |
| 08-06—08-07 | 钉钉与企业微信进入共享渠道引擎 | [c3577cb0](https://github.com/oldwinter/multica/commit/c3577cb04bcfc9a1d50b3e2dc6aa6187f8d5f6f0)、[040bb97b](https://github.com/oldwinter/multica/commit/040bb97b0f0282a6556a31768bd847e974b9e68d) |
| 08-12 | LM Wiki 与可演进 Twin 生命周期落地 | [99a0fbf6](https://github.com/oldwinter/multica/commit/99a0fbf671e7acc5d840cf3183ba6781b5b6b83d) |
| 08-15—08-16 | Rooms 与 Tension/Relay/Field 命名皮肤加入 | [4cbabf31](https://github.com/oldwinter/multica/commit/4cbabf315e474445f3ae6a7ebb2275826dc464a2)、[f5b275ab](https://github.com/oldwinter/multica/commit/f5b275ab359fe9d3348032b54d8e02f0be8a9fd1) |
| 08-19 | 上游插件系统重建出 Action API、Surface 与独立 SDK；Telegram 接入渠道引擎 | [97195885](https://github.com/oldwinter/multica/commit/97195885fae5e8a69182380e1a002692047348c6)、[67c6aa05](https://github.com/oldwinter/multica/commit/67c6aa05c4ac5f1fef6d84e2d0099f5aee2d2d1c) |
| 08-24 | 首批下游 tag：`v0.4.32-oldwinter.1`、`.2` | [v0.4.32-oldwinter.1](https://github.com/oldwinter/multica/releases/tag/v0.4.32-oldwinter.1)、[v0.4.32-oldwinter.2](https://github.com/oldwinter/multica/releases/tag/v0.4.32-oldwinter.2) |
| 08-28—08-29 | Skill Evolution 从领域契约、持久化、归因到受治理生命周期逐步组合 | [c4be0cfc](https://github.com/oldwinter/multica/commit/c4be0cfced8c3fd510d6cf25ae7f50988757c933)、[4f116cdd](https://github.com/oldwinter/multica/commit/4f116cddccbc55b67e82652ae53bdbedcebd1d7d) |
| 08-31 | 上游 `v0.4.37` 与下游 `v0.4.36-oldwinter.1` 均可达 | [v0.4.37](https://github.com/oldwinter/multica/releases/tag/v0.4.37)、[v0.4.36-oldwinter.1](https://github.com/oldwinter/multica/releases/tag/v0.4.36-oldwinter.1) |
| 09-01 | 合并 `upstream/main`、协调 `origin/main` 并记录 v0.4.37 同步 | [43cc9a99](https://github.com/oldwinter/multica/commit/43cc9a99914ee0344dfffe2d47fd61138b1c48d0)、[949b9d41](https://github.com/oldwinter/multica/commit/949b9d417ca11f75dd974e88e6258a2922b560f7)、[fccc7487](https://github.com/oldwinter/multica/commit/fccc7487e90eceb269762fbf9cbdc9cc25b8041a) |
| 09-01 | 合并 `upstream/main`、协调 `origin/main`，记录 v0.4.37 同步并达到本章快照 | [43cc9a99](https://github.com/oldwinter/multica/commit/43cc9a99914ee0344dfffe2d47fd61138b1c48d0)、[949b9d41](https://github.com/oldwinter/multica/commit/949b9d417ca11f75dd974e88e6258a2922b560f7)、[685cf788](https://github.com/oldwinter/multica/commit/685cf788777e24724b0d82ffe88c56459341fb2a) |

## 发布 tag

快照可达 145 个 tag。其中 142 个是 `v0.x.y` 标准名，3 个带 `-oldwinter.N` 下游后缀。

| 系列 | 标准 tag 数 | 范围 | 创建日期范围 | 备注 |
| --- | ---: | --- | --- | --- |
| 0.1 | 28 | `v0.1.8`—`v0.1.35` | 03-25—04-15 | HEAD 可达序列从 1.8 开始 |
| 0.2 | 33 | `v0.2.0`—`v0.2.32` | 04-15—05-13 | `v0.2.8` 与 `v0.2.9` 指向同一提交 |
| 0.3 | 43 | `v0.3.0`—`v0.3.43` | 05-14—07-10 | 序列中没有 `v0.3.7` |
| 0.4 | 38 | `v0.4.0`—`v0.4.37` | 07-13—08-31 | 另有 3 个下游 tag |

复现命令：

```bash
SNAPSHOT=685cf788777e24724b0d82ffe88c56459341fb2a
git tag --merged "$SNAPSHOT" --sort=version:refname
git tag --merged "$SNAPSHOT" --list 'v[0-9]*' \
  | grep -Ev -- '-oldwinter\.' | wc -l
git tag --merged "$SNAPSHOT" --list '*-oldwinter.*'
git for-each-ref --merged="$SNAPSHOT" \
  --format='%(refname:short)|%(objectname)|%(creatordate:short)' refs/tags
```

## 提交节奏

按 author date 分组，5,315 个提交分布如下：

| 月份 | 提交 |
| --- | ---: |
| 2026-01 | 207 |
| 2026-02 | 944 |
| 2026-03 | 471 |
| 2026-04 | 1,130 |
| 2026-05 | 625 |
| 2026-06 | 459 |
| 2026-07 | 634 |
| 2026-08 | 839 |
| 2026-09（截至 01 日） | 6 |

其中 622 个为 merge commit，4,693 个为非 merge commit；共有 179 个不同的提交日期。命令为：

```bash
git log --format='%ad' --date=format:%Y-%m "$SNAPSHOT" | sort | uniq -c
git rev-list --count --merges "$SNAPSHOT"
git rev-list --count --no-merges "$SNAPSHOT"
git log --format='%ad' --date=short "$SNAPSHOT" | sort -u | wc -l
```

架构节点背后的因果关系见[架构演进](architecture-evolution.md)，版本号与两次客户端“重生”的细节见[项目掌故](../lore.md)。
