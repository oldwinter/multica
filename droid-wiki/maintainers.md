# 维护上下文

本页提供默认分支历史中的代码区域贡献信号，帮助修改者寻找评审上下文。它不是 CODEOWNERS、审批规则、值班表或当前职责分配，也不表示提交数较多的人必须负责未来修改。

## 快照与方法

- 历史范围：本地 `main` 在下述固定快照可达的历史
- 快照提交：`685cf788777e24724b0d82ffe88c56459341fb2a`
- 快照提交日期：2026-09-01；收集日期：2026-09-01
- 统计单位：触及指定路径的 commit 数；同一个 commit 可以计入多个区域。
- 身份字段：Git author name（`%aN`），不收集或发布 email。

快照中未发现 `.mailmap` 或 CODEOWNERS 文件，因此名称 alias 不做猜测性合并。例如 `Bohan Jiang` 与 `Jiang Bohan`、`Jiayuan Zhang` 与 `Jiayuan` 保持为不同历史身份。表中只列每个区域前三个名称；“区域提交数”是该路径全部 commit 数，不是前三名之和。

## 区域历史信号

| 区域与统计路径 | 历史名称前三 | 区域提交数 |
| --- | --- | ---: |
| CLI：[`server/cmd/multica`](../server/cmd/multica/) | Bohan Jiang（95）、yushen（26）、LinYushen（24） | 285 |
| 认证与中间件：[`server/internal/middleware`](../server/internal/middleware/) | Jiayuan Zhang（8）、Naiyuan Qing（7）、LinYushen（7）；Bohan Jiang 也为 7 | 41 |
| Handler 与 service：[`server/internal/handler`](../server/internal/handler/)、[`server/internal/service`](../server/internal/service/) | Bohan Jiang（316）、Jiayuan Zhang（128）、Multica Eve（127） | 1,077 |
| 守护进程与智能体：[`server/internal/daemon`](../server/internal/daemon/)、[`server/pkg/agent`](../server/pkg/agent/) | Bohan Jiang（325）、Multica Eve（93）、Jiayuan Zhang（43） | 807 |
| 数据库与迁移：[`server/migrations`](../server/migrations/)、[`server/internal/migrations`](../server/internal/migrations/)、[`server/pkg/db`](../server/pkg/db/) | Bohan Jiang（140）、Multica Eve（83）、Jiayuan Zhang（73）；Naiyuan Qing 也为 73 | 572 |
| Web：[`apps/web`](../apps/web/) | Naiyuan Qing（339）、Bohan Jiang（102）、Jiayuan Zhang（99） | 835 |
| Desktop：[`apps/desktop`](../apps/desktop/) | Naiyuan Qing（170）、Jiayuan Zhang（102）、Jiang Bohan（54） | 477 |
| Mobile：[`apps/mobile`](../apps/mobile/) | Bohan Jiang（17）、Jiayuan Zhang（17）、Naiyuan Qing（11） | 81 |
| Docs：[`apps/docs`](../apps/docs/) | Bohan Jiang（108）、Multica Eve（27）、Jiayuan Zhang（18）；Naiyuan Qing 也为 18 | 266 |
| 共享 core：[`packages/core`](../packages/core/) | Jiayuan Zhang（227）、Bohan Jiang（182）、Naiyuan Qing（158） | 854 |
| 共享 views 与 UI：[`packages/views`](../packages/views/)、[`packages/ui`](../packages/ui/) | Naiyuan Qing（484）、Bohan Jiang（351）、Jiayuan Zhang（294） | 1,628 |

## 如何使用这些数据

适合的用途：

- 在修改陌生区域前寻找历史设计讨论和相关 commit；
- 判断一次变更是否跨越多个长期演进区域，需要哪些背景材料；
- 为高冲突上游同步寻找曾处理相似冲突的提交；
- 发现 alias 或组织变化后，提示维护者补充正式的 ownership 机制。

不适合的用途：

- 自动指定 reviewer、审批人或事故责任人；
- 用提交数判断代码质量、当前熟悉程度或可用时间；
- 把历史名称映射为 email、账号或现实身份；
- 绕过仓库现有评审流程。

实际评审人应根据当前团队安排、最近相关变更、变更风险和正式规则确定。若未来加入 `.mailmap` 或 CODEOWNERS，应重新生成本页，并仍把历史统计与正式 ownership 分开。

## 复现原则

统计可用只输出 author name 的 Git 历史命令复现，例如：

```bash
git log 685cf788777e24724b0d82ffe88c56459341fb2a \
  --format='%aN' -- server/cmd/multica \
  | sort | uniq -c | sort -rn
```

更新时应同时记录新的 `origin/main` 完整提交 ID 和日期，不要把不同快照的计数混在一张表中。

## 相关页面

- [分支背景](background/index.md)
- [下游扩展](background/downstream-extensions.md)
- [迁移与上游同步](background/migrations-and-upstream-sync.md)
- [功能域导航](features/index.md)
- [参与贡献](how-to-contribute/index.md)
