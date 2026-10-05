# 项目掌故

这里的“掌故”必须能回到提交、tag 或当前源码。没有证据的名称由来不补写成故事。

## 13 个文件的第一天

根提交 [6b34ddc3](https://github.com/oldwinter/multica/commit/6b34ddc3dc7a2cf5b7054492f0bf0f46cc286e4d) 的主题是“Initial project setup with multi-component architecture”，但整棵树只有 13 个文件：

```text
src/agent/
src/client/
src/gateway/
src/shared/
src/index.ts
```

在快照中，仓库已经有 6,408 个跟踪文件。`git ls-tree -r --name-only <ref> | wc -l` 能直接复现这两个端点。

## 仓库在第 51 天换过一次“物种”

从 1 月 28 日根提交到 3 月 20 日正好相隔 51 天。[d4f5c5b1](https://github.com/oldwinter/multica/commit/d4f5c5b16fdb75b223b5e77a9948843bd8436554) 没有把旧 agent framework 慢慢改名，而是明确写下“Replace the agent framework codebase”，并删除旧 Desktop、Mobile、Gateway、core 和 skills 树，换成 Go/PostgreSQL 的任务管理平台。

所以在历史中看到同一个 `apps/desktop` 或 `packages/core` 路径，并不表示内部实现从 1 月连续演化到了今天。

## Desktop、Mobile 与 core 都有“第二代”

对三个 package manifest 查看 `--diff-filter=AD` 会得到完整的删建序列：

- Desktop：01-30 首建，03-20 删除，04-09 以 electron-vite 重建；
- Mobile：02-03 首建，03-20 删除，05-22 以 “Multica for iOS — first version” 重建；
- `packages/core`：02-10 首建，03-20 删除，04-09 作为 headless business logic 重建。

```bash
git log --reverse --diff-filter=AD \
  --format='%H|%ad|%s' --date=short "$SNAPSHOT" -- \
  apps/desktop/package.json apps/mobile/package.json packages/core/package.json
```

这也是为什么[架构演进](project-history/architecture-evolution.md)把“首次出现”和“当前世代首次出现”分开。

## 当前可达的版本序列从 v0.1.8 开始

本地仓库共有 161 个 tag，但只有 145 个从快照 HEAD 可达。对 **HEAD 可达** tag 做版本排序时，标准 `v0.x.y` 序列从 `v0.1.8` 开始，而不是 `v0.1.0`。早期 `v0.1.0`—`v0.1.7` 与若干 `desktop-v0.1.x` tag 存在于本地 refs，却不在这条默认分支祖先线上。

这说明发布统计必须使用：

```bash
git tag --merged "$SNAPSHOT" --sort=version:refname
```

而不是不加范围的 `git tag`。

## v0.2.8 和 v0.2.9 是同一个 Git 对象

可达 tag 中唯一一组共享 target 的标准版本是：

```text
v0.2.8
v0.2.9
    -> 9e47b83f02aed94eac92f5b44d7717ddbb3ac4e7
```

该提交的主题是“feat(agent): add Kimi CLI as agent runtime”。这不是推测：`git for-each-ref --points-at=<object> refs/tags` 会返回两个名字。

另一个版本小缺口是 `v0.3.0`—`v0.3.43` 中没有 `v0.3.7`，因此该系列范围看似有 44 个序号，实际只有 43 个可达 tag。

## Issue 与 task 被刻意保留为两个词

中文界面把产品实体 `issue` 译成“任务”，但智能体的一次执行 `task` 保留为小写英文 `task`。这不是偶然混用，而是[中文约定](https://github.com/oldwinter/multica/blob/685cf788777e24724b0d82ffe88c56459341fb2a/apps/docs/content/docs/developers/conventions.zh.mdx#L119-L137)明确规定的区分：一个任务下可以有多次 task，API/DB 字段仍分别是 `issue` 与 `task`。

同一约定还固定了 `Agent` → “智能体”、`Daemon` → “守护进程”、`Runtime` → “运行时”。历史页面因此不把产品“任务”和执行 `task` 写成同一个中文词。

## Twin 是“选择性借用”，不是另一个产品被整仓搬入

Twin 的来源记录没有藏在口述历史里。下游[产品目标](https://github.com/oldwinter/multica/blob/685cf788777e24724b0d82ffe88c56459341fb2a/docs/downstream/twin/product-goals.md)写明它从 Harness Studio 保留“把个人证据变成可审查文本工件，再用于指导委派工作”的产品意图，但不继承旧应用代码、拓扑或通过状态。

[词汇表](https://github.com/oldwinter/multica/blob/685cf788777e24724b0d82ffe88c56459341fb2a/docs/downstream/twin/glossary.md)把 `Twin` 定义为“evidence-backed, reviewable personal-work artifact”，并把每个旧术语标成 adopted、adapted 或 out-of-scope。诸如 Topic、Dispatch、Run 被适配到现有 issue、agent 和 execution record，而不是复制一个平行执行系统。

## 两个 Wiki 名字代表两套知识工作流

下游源码明确记录 Workspace Wiki “inspired by Karpathy's llm-wiki pattern”，但当前产品又把 Workspace Wiki 与 LM Wiki 分开：

- `/{slug}/wiki`：成员编辑的 Markdown 页面，有 workspace/project/user 三种范围；
- `/{slug}/twins` 下的 LM Wiki：从白名单来源编译出的不可变、有引用、需 owner/admin 审查的证据修订。

LM Wiki 不会隐式读取可变 Wiki 页面；它固定精确 revision。这个区别来自[`docs/downstream/wiki/README.md`](https://github.com/oldwinter/multica/blob/685cf788777e24724b0d82ffe88c56459341fb2a/docs/downstream/wiki/README.md)，而不是根据 UI 名称猜测。

## “Tension Map”不是泛称，而是有完整隐喻的设计名

根目录 [`DESIGN.md`](https://github.com/oldwinter/multica/blob/685cf788777e24724b0d82ffe88c56459341fb2a/DESIGN.md#L1-L18) 把视觉世界命名为 **Tension Map**：浅色结构面、深色工作面、绷紧的信号线，以及标记注意力负载的单一高色度强调色。三个 skin 也有正式名称：

- Tension：concrete white / carbon black / tension red；
- Relay：cool mineral gray / graphite / relay teal；
- Field：mineral white / deep moss-charcoal / field green。

该文件与 Twin UI 一同在 [8c5e86ed](https://github.com/oldwinter/multica/commit/8c5e86ed682f8be122570cb9cd866610ebd84528) 首次出现，命名皮肤实现则在 [f5b275ab](https://github.com/oldwinter/multica/commit/f5b275ab359fe9d3348032b54d8e02f0be8a9fd1) 落地。

## fork 的冲突史变成了架构规则

2026-08-17 的上游同步有 40 个冲突文件；同步账本把原因概括为 13 个本地提交面对 347 个上游提交。此后项目采用“own a leaf, register at a point”、不手工合并生成文件、已发布迁移身份不可变等规则。到 08-28 的 `v0.4.36` 同步，13 个上游提交产生 0 个文本冲突；09-01 的 `v0.4.37` 同步中，30 个上游提交与 205 个本地独有提交产生 5 个预测且实际冲突文件。

数字与处理过程都记录在[`docs/downstream/upstream-sync.md`](https://github.com/oldwinter/multica/blob/685cf788777e24724b0d82ffe88c56459341fb2a/docs/downstream/upstream-sync.md)。这段历史说明架构“深叶模块 + 薄注册点”的规则来自可量化的维护成本，而不只是风格偏好。

## 没有证据的故事

当前默认分支没有保存 `Multica`、`oldwinter` 或 `Room` 这些名称的明确词源说明。本页因此不声称它们来自某个缩写、人物或比喻。未来若提交、ADR 或产品文档补上来源，再把它作为有链接的事实加入即可。

完整日期顺序见[时间线](project-history/timeline.md)，目录世代和边界见[架构演进](project-history/architecture-evolution.md)。
