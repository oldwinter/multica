# 数据库变更

Multica 使用 PostgreSQL 与 sqlc。迁移文件在 [`server/migrations/`](../../server/migrations/)，查询源在 [`server/pkg/db/queries/`](../../server/pkg/db/queries/)，生成代码在 `server/pkg/db/generated/`。数据库规则是硬约束，不因表是新建、数据量小或功能尚未上线而放宽。

## 设计规则

### 不创建数据库外键

不得增加 `FOREIGN KEY`、`REFERENCES`、`ON DELETE CASCADE` 或 `ON UPDATE CASCADE`。应用层必须显式完成：

- 关联对象存在性和工作区归属校验；
- 删除父对象前后的依赖清理；
- 权限和 membership 检查；
- 多步操作必须共同成功时的事务边界。

需要原子性时在 service/应用层事务中完成校验、清理和父对象变更。关联列仍按 `<table>_id` 命名，但命名不代表存在数据库 FK。

### 每个索引都并发创建

任何迁移创建索引都使用：

```sql
CREATE INDEX CONCURRENTLY ...
-- 或
CREATE UNIQUE INDEX CONCURRENTLY ...
```

包括新表上的索引。每个 concurrent index 必须独占一个、只含单条语句的 migration 文件；PostgreSQL 不允许它位于显式事务或多命令字符串中。仓库迁移 runner 因此在显式事务外执行迁移文件。

### 迁移必须双向且身份稳定

- 文件名为 `NNN_descriptive_name.up.sql` 和匹配的 `.down.sql`。
- 选择未使用的下一 numeric prefix；不要扩充历史重复前缀名单。
- 已发布迁移的完整 stem 和 SQL 内容不能改写，否则现有 `schema_migrations` ledger 可能把它视为另一项迁移或重跑 DDL。
- 条件跳过的迁移仍会写入 ledger，因此 ledger 只证明顺序，不证明 SQL 的每个对象都已创建。
- 后续操作可能缺失的对象时使用 `IF EXISTS` / `IF NOT EXISTS`；如果对象缺失会破坏运行时，介绍该对象的变更必须写明恢复办法。

迁移 lint 在 [`server/internal/migrations/migrations_lint_test.go`](../../server/internal/migrations/migrations_lint_test.go) 检查方向配对、numeric prefix 和冻结的历史身份。下游迁移碰撞与 ledger 兼容背景见 [`docs/downstream/upstream-sync.md`](../../docs/downstream/upstream-sync.md)。

## 推荐工作流

### 1. 设计应用层生命周期

先回答：

- 哪个 workspace 拥有每一行，所有查询如何强制 `workspace_id`；
- 父对象删除时由哪个 service 清理哪些依赖；
- 哪些步骤必须在同一事务提交或回滚；
- 回滚是否会丢数据，能否安全降级；
- 旧安装与全新数据库分别如何到达目标 schema。

### 2. 添加迁移

普通表/列变更可以在一对迁移中完成。若还要创建两个索引，则每个索引再使用各自独立的 `.up.sql`/`.down.sql` 对；不要把 `CREATE TABLE` 和 concurrent index 塞进同一文件。

示意：

```text
528_add_example_column.up.sql
528_add_example_column.down.sql
529_example_workspace_index.up.sql
529_example_workspace_index.down.sql
```

实际 prefix 以当前目录中的最大已用值和 lint 结果为准。

### 3. 修改 sqlc 查询

SQL 写在 `server/pkg/db/queries/*.sql`，配置见 [`server/sqlc.yaml`](../../server/sqlc.yaml)。修改后运行：

```bash
make sqlc
```

该命令使用仓库固定的 sqlc 版本生成 `server/pkg/db/generated/`。不要手工编辑生成文件；提交 SQL 源和对应生成 diff。CI 会重新生成并用 `git diff --exit-code` 检查漂移。

### 4. 接入应用代码

- handler 只处理协议边界、身份和权限；跨表生命周期放 service。
- path 参数可能是 UUID 或可读 ID 时，先用 loader，再把已解析实体 `ID` 传给 write query。
- 纯 UUID 请求字段用 `parseUUIDOrBadRequest` 并在失败时立即返回。
- server 内其他位置使用 `util.ParseUUID` 时必须检查 error。
- 删除/清理通过显式查询执行，不依赖 cascade。

### 5. 验证迁移和行为

```bash
make migrate-up
make migrate-down
make migrate-up
make sqlc
(cd server && go test ./internal/migrations -count=1)
make test
```

至少验证全新数据库。涉及已发布迁移、下游编号碰撞或条件 DDL 时，还要在带真实旧 ledger 的数据库上升级，并运行受影响功能测试；不要只证明空库能迁移。

## checklist

- [ ] 没有 `FOREIGN KEY`、`REFERENCES` 或 cascade。
- [ ] 关系校验和依赖清理有明确应用层所有者，必要时在事务中。
- [ ] 每个 index 使用 `CONCURRENTLY`，且独占单语句迁移。
- [ ] 每个 migration stem 同时有 `.up.sql` 和 `.down.sql`。
- [ ] prefix 唯一；没有修改已发布迁移的 stem 或内容。
- [ ] 条件 DDL 后续可在对象缺失时安全运行，并记录恢复办法。
- [ ] 每个查询过滤 `workspace_id`，UUID 来源和解析方式正确。
- [ ] 已运行 `make sqlc`，生成代码与 SQL 一起提交。
- [ ] 新库、旧 ledger 和受影响行为得到与风险相称的验证。

## 相关页面

- [添加功能](adding-a-feature.md)
- [测试指南](testing.md)
- [模式与约定](patterns-and-conventions.md)
- [系统架构](../overview/architecture.md)
- [迁移 runner](../../server/internal/migrations/migrations.go)
- [Makefile 数据库命令](../../Makefile)
