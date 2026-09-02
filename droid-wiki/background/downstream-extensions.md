# 下游扩展

本页概括 `oldwinter/multica` 在上游产品旁维护的主要扩展面。归类依据是[上游同步账本](../../docs/downstream/upstream-sync.md)和仓库中的下游规格；它描述维护上下文，不表示这些目录只能由某个人修改。

## 主要扩展

| 扩展 | 作用 | 主要代码入口 | 设计依据 |
| --- | --- | --- | --- |
| Room | 多智能体参与者、轮次、预算、产物、记忆、推荐和 outcome 闭环 | [Room handler](../../server/internal/handler/room.go)、[Room service 层](../../server/internal/service/)、[共享视图](../../packages/views/rooms/) | [Room outcome 规格](../../docs/downstream/specs/rooms-outcome-loop.md)、[关注与复用闭环](../../docs/downstream/specs/rooms-attention-and-reuse-loop.md) |
| Twin 与 LM Wiki | 工作区知识来源、修订与提案，Twin 激活、简报编译、使用策略和反馈 | [Twin service](../../server/internal/service/twin.go)、[LM Wiki service](../../server/internal/service/lm_wiki.go)、[Twin 视图](../../packages/views/twins/)、[Wiki 视图](../../packages/views/wiki/) | [Twin/Wiki 知识执行闭环](../../docs/downstream/specs/twin-wiki-knowledge-execution-loop.md) |
| 命名皮肤 | `tension`、`relay`、`field` 皮肤，独立的 skin 与 light/dark/system appearance | [设计系统](../../DESIGN.md)、[语义 token](../../packages/ui/styles/tokens.css)、[外观适配](../../packages/views/appearance/) | [命名皮肤体验闭环](../../docs/downstream/specs/named-skins-experience-loop.md) |
| 搜索与任务增强 | 拼音检索、来源上下文、属性、状态、时间线和下游工作流增强 | [搜索 handler](../../server/internal/handler/search.go)、[任务 handler](../../server/internal/handler/issue.go)、[任务视图](../../packages/views/issues/) | [下游同步账本](../../docs/downstream/upstream-sync.md) |
| Skill Evolution | 证据、评估、提案、修订、发布、循环计划和 Room 候选衔接 | [后端模块](../../server/internal/skillevolution/)、[共享视图](../../packages/views/skill-evolution/)、[SQL 查询](../../server/pkg/db/queries/workspace_skill_evolution.sql) | [松耦合自改进 Skill](../../docs/downstream/specs/loosely-coupled-self-improving-skills.md) |

这些功能通常不是单目录功能。例如 Room 同时涉及迁移、sqlc、handler/service、调度任务、实时事件、共享视图、权限和本地化；Twin/Wiki 又会参与 task 简报和 Skill Evolution 的证据链。修改前应沿[功能域导航](../features/index.md)检查完整切面。

## 降低与上游的接缝

同步账本建议“拥有一个叶子，在稳定点注册”。实际含义是：

- 把下游领域逻辑放进独立 handler/service/package，而不是持续扩大上游热点文件；
- 在 router、scheduler、导航、locale 或 schema 的稳定注册点做窄接入；
- 对共享类型和数据库列给出明确兼容语义，避免双方各自增加隐式分支；
- 生成文件只由各自源文件生成；
- 把冲突解决的产品意图记录回同步账本，而不是只留下 Git 冲突结果。

## 修改扩展时的检查项

### Room、Twin、Wiki 与 Skill Evolution

- 检查 workspace、actor 和 attribution 是否贯穿所有写入。
- 检查后台 job、重试、幂等键和实时事件是否与 UI 状态一致。
- 检查已有下游数据库能否继续迁移，不能只测空库。
- 检查功能关闭、预览和启用状态是否有清晰的默认分支。

### 命名皮肤

- 组件只能消费语义 token，不增加产品级 raw color。
- skin 与 appearance 是两个独立偏好；同步、持久化和系统模式不能混为一个字段。
- 新 token 要同时覆盖所有 skin 的明暗模式，并检查 focus、状态色和 200% zoom。

### 搜索与任务增强

- 搜索索引、过滤语义和结果权限必须保持工作区作用域。
- API 响应变化要经过共享 Zod schema，并允许已安装 Desktop 客户端面对后端漂移。
- 任务状态、属性和来源上下文变更要检查 CLI、内置 Skill 与 source map。

## 不应从“下游”推导的结论

- 下游功能不等于实验性功能；是否稳定要看具体行为、迁移和测试。
- 下游目录不等于正式 owner 名单；历史贡献上下文见[维护上下文](../maintainers.md)。
- 上游出现相似功能时，不应直接保留两套实现；先比较产品生命周期与已发布数据，再决定替换、迁移或整合。

## 相关页面

- [分支背景](index.md)
- [迁移与上游同步](migrations-and-upstream-sync.md)
- [功能域导航](../features/index.md)
- [维护上下文](../maintainers.md)
- [可清理机会](../cleanup-opportunities.md)
