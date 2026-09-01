# 迁移与上游同步

本页把[上游同步账本](../../docs/downstream/upstream-sync.md)中的数据库兼容规则整理为操作清单。账本记录的 published migration identity 是用户数据库已经观察到的协议，Git 文件是否能无冲突合并只是其中一部分。

## 迁移身份规则

### 完整 stem 才是身份

迁移身份是文件名去掉 `.up.sql` 或 `.down.sql` 后的完整 stem，而不只是数字前缀。例如：

```text
123_add_room_state
123_add_upstream_state
```

它们数字相同，但身份不同。不能仅因为数字碰撞就重命名已经发布的 migration。

### 已发布 stem 不可随意改名

- 仓库的 migration lint 冻结已知重复 stem 和账本规则，见[迁移 lint](../../server/internal/migrations/migrations_lint_test.go)。
- 历史 alias 只用于 SQL 字节完全相同、仅文件名发生过变化的 migration。
- 已发布 SQL 如果需要修正，应新增向前 repair migration；不能改旧文件并假设所有数据库会重跑。
- 条件跳过的 migration 仍可能被记录在 `schema_migrations`，所以账本证明顺序，不证明条件 SQL 实际执行过。

## 仓库级数据库硬约束

除同步兼容外，新迁移还必须遵守 [CLAUDE.md](../../CLAUDE.md)：

- 不新增数据库外键、级联删除或级联更新；关系验证和关联清理由应用层显式完成，需要原子性时使用应用事务。
- 所有索引都使用 `CREATE INDEX CONCURRENTLY` 或 `CREATE UNIQUE INDEX CONCURRENTLY`。
- 每个 concurrent index 建立语句单独放在单语句 migration 文件中。
- 后续触碰条件对象的 migration 使用 `IF EXISTS` / `IF NOT EXISTS` 等幂等 DDL，并记录恢复路径。

## 同步前

1. 获取并核对 `upstream/main` 与 `origin/main`，确认当前分支基线。
2. 阅读[同步账本](../../docs/downstream/upstream-sync.md)中最近一次同步、下游 ownership map 和 migration ledger。
3. 分别列出上游与下游新增 migration stem，不只比较数字前缀。
4. 标记 router、service、共享包、locale、生成代码和迁移目录的冲突热点。
5. 记录一个“已有下游账本”测试数据库的版本状态，作为升级夹具。

## 冲突解决顺序

推荐按事实源到生成物处理：

1. **产品与生命周期意图**：确定最终权限、状态、失败和清理语义。
2. **迁移 SQL**：保留已发布 stem；需要调整时新增 forward repair。
3. **sqlc query 源**：解决 [queries](../../server/pkg/db/queries/) 中的 SQL。
4. **Go 与共享 schema**：适配最终数据库/API 形状。
5. **生成物**：运行 `make sqlc` 重新生成 [generated](../../server/pkg/db/generated/)，不手工合并生成 Go。
6. **客户端与文档**：处理共享视图、各 app wiring、locale、内置 Skill 和 source map。

冲突块不能简单取并集。两个分支可能分别实现互斥的状态机、清理顺序或兼容策略；最终代码必须只有一套可解释的语义。

## 迁移验证矩阵

至少覆盖两条数据库路径：

| 数据库起点 | 要证明的事情 |
| --- | --- |
| 全新空数据库 | 当前完整 migration 集可以从零建立 schema |
| 带有同步前下游 `schema_migrations` 账本的数据库 | 已发布下游用户可以无重命名、无重复执行地升级 |

对两条路径都应：

1. 运行 migration 到最新；
2. 再运行一次，验证重复执行入口幂等；
3. 运行 migration lint 和相关 contract test；
4. 执行受影响功能的 schema/handler/service 测试；
5. 检查 sqlc 生成物没有未提交漂移。

如果冲突涉及 Room、Twin/Wiki、Skill Evolution 等下游 schema，还要实际调用受影响功能路径；“migration 命令退出为零”不足以证明读写语义正确。

## 同步后记录

在[同步账本](../../docs/downstream/upstream-sync.md)中追加：

- 上游基线、下游基线和同步结果；
- 进入的上游提交范围；
- 冲突文件以及按什么产品意图解决；
- 新增、重复或 alias migration stem；
- fresh database 与 prior-downstream-ledger 的验证结果；
- 未完成的风险与下次同步提示。

不要把临时调查笔记当作账本替代品。账本应让下一位维护者在没有本次终端历史的情况下重建关键决策。

## 相关页面

- [分支背景](index.md)
- [下游扩展](downstream-extensions.md)
- [数据库变更](../how-to-contribute/database-changes.md)
- [参与贡献](../how-to-contribute/index.md)
- [维护上下文](../maintainers.md)
