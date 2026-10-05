# CLI 命令参考

`multica` 使用 Cobra。当前命令树的事实来源是[root 注册](../../server/cmd/multica/main.go)和各 `cmd_*.go` 文件；运行某个已安装版本时，以 `multica help`、`multica <command> --help` 和该二进制的 `multica version` 为准。

## 全局形式

```text
multica [--server-url URL] [--workspace-id ID] [--profile NAME] [--debug] <command>
```

四个 persistent flag 由所有子命令继承。`--profile` 隔离 CLI 配置、守护进程状态和 task 工作区；脚本不应假设不同 profile 共用同一个 daemon。

## 当前命令树

下表按 [main.go](../../server/cmd/multica/main.go) 的注册分组列出当前顶层命令和直接子命令。尖括号表示必填参数，方括号表示可选参数；完整 flag 仍应查看对应命令的 `--help`。

| 顶层命令 | 直接子命令或形式 |
| --- | --- |
| `issue` | `list`、`get <id>`、`pull-requests <id>`、`children <id>`、`create`、`update <id>`、`assign <id>`、`status <id> <status>`、`reorder <id>`、`timeline <id>`、`runs <issue-id>`、`run-messages <task-id>`、`usage <issue-id>`、`rerun <id>`、`cancel-task <task-id>`、`search <query>` |
| `issue comment` | `list <issue-id>`、`add <issue-id>`、`delete <comment-id>`、`resolve <comment-id>`、`unresolve <comment-id>` |
| `issue subscriber` | `list <issue-id>`、`add <issue-id>`、`remove <issue-id>` |
| `issue label` | `list <issue-id>`、`add <issue-id> <label-id>`、`remove <issue-id> <label-id>` |
| `issue property` | `list <issue-id>`、`set <issue-id>`、`unset <issue-id>` |
| `issue metadata` | `list <issue-id>`、`get <issue-id>`、`set <issue-id>`、`delete <issue-id>` |
| `project` | `list`、`get <id>`、`create`、`update <id>`、`delete <id>`、`status <id> <status>`、`resource …` |
| `project resource` | `list <project-id>`、`add <project-id>`、`update <project-id> <resource-id>`、`remove <project-id> <resource-id>` |
| `label` | `list`、`get <id>`、`create`、`update <id>`、`delete <id>` |
| `property` | `list`、`get <id-or-name>`、`create`、`update <id-or-name>`、`archive <id-or-name>`、`unarchive <id-or-name>` |
| `agent` | `list`、`get <id>`、`create`、`update <id>`、`copy <source-agent-id>`、`archive <id>`、`restore <id>`、`tasks <id>`、`avatar <id>`、`skills …`、`env …`、`mcp …` |
| `agent skills` | `list <agent-id>`、`set <agent-id>`、`add <agent-id>` |
| `agent env` | `get <agent-id>`、`set <agent-id>` |
| `agent mcp` | `list <agent-id>`、`add`、`enable`、`disable`、`remove`；后四项接收 `<agent-id> <server-id>` |
| `autopilot` | `list`、`get <id>`、`create`、`update <id>`、`delete <id>`、`trigger <id>`、`runs <id>`、`trigger-add`、`trigger-list`、`trigger-update`、`trigger-delete`、`trigger-rotate-url` |
| `workspace` | `list`、`create`、`get [workspace]`、`update [workspace]`、`switch <workspace>`、`member …`、`mcp …` |
| `workspace member` | `list [workspace]`、`invite <email> [workspace]` |
| `workspace mcp` | `list [workspace]`、`add <server-name> [workspace]`、`update <server-id> [workspace]`、`remove <server-id> [workspace]` |
| `repo` | `list`、`add [url]...`、`remove [url]...`、`checkout <url>` |
| `skill` | `list`、`get <id>`、`create`、`update <id>`、`delete <id>`、`import`、`refresh <id>`、`search <query>`、`files …` |
| `skill files` | `list <skill-id>`、`upsert <skill-id>`、`delete <skill-id> <file-id>` |
| `squad` | `list`、`get <squad-id>`、`create`、`update <squad-id>`、`delete <squad-id>`、`member …`、`activity <issue-id> <outcome>` |
| `squad member` | `list <squad-id>`、`add <squad-id>`、`set-role <squad-id>`、`remove <squad-id>` |
| `chat` | `history`、`thread [id]` |
| `wiki` | `list`、`get <page-id>`、`search <query>`、`propose <page-id>` |
| `daemon` | `start`、`stop`、`restart`、`status`、`probe-runtimes`、`logs`、`disk-usage` |
| `runtime` | `list`、`usage <runtime-id>`、`activity <runtime-id>`、`update <runtime-id>`、`rename <runtime-id> <name>`、`delete <runtime-id>`、`profile …` |
| `runtime profile` | `list`、`create`、`update <profile-id>`、`delete <profile-id>`、`set-path <profile-id>`、`unset-path <profile-id>` |
| `auth` | `status`、`logout` |
| `user profile` | `get`、`update` |
| `setup` | `cloud`、`self-host` |
| `attachment` | `download <attachment-id>`、`upload <path>` |
| `config` | `show`、`set <key> <value>` |
| 无子命令顶层入口 | `login`、`update`、`version` |

`workspace get`、`workspace member list`、`config show` 和 `config set` 是测试明确固定的兼容入口，见[命令兼容测试](../../server/cmd/multica/cmd_compat_test.go)。

## 配置文件与 profile

默认 profile 使用：

```text
~/.multica/config.json
```

命名 profile 使用：

```text
~/.multica/profiles/<name>/config.json
```

task 管理的调用使用私有 `MULTICA_TASK_CONFIG_ROOT`，不会把 task 临时凭据写入普通 profile。配置保存采用临时文件加原子替换，目标权限为 `0600`；实现见[CLI 配置存储](../../server/internal/cli/config.go)。

`config set` 当前支持：

- 连接与选择：`server_url`、`app_url`、`workspace_id`
- 守护进程身份与目录：`device_name`、`runtime_name`、`workspaces_root`
- 并发与轮询：`max_concurrent_tasks`、`poll_interval`、`heartbeat_interval`
- 超时：`agent_timeout`、`codex_semantic_inactivity_timeout`、`codex_handshake_timeout`
- 更新与重载：`disable_auto_update`、`auto_update_check_interval`、`disable_auto_reload`

空字符串用于清除持久值。多数 duration 字段必须是正的 Go duration；`agent_timeout=0s` 是例外，明确关闭墙钟时限，而空字符串表示回退到环境变量或默认值。验证逻辑见[`server/cmd/multica/cmd_config.go`](../../server/cmd/multica/cmd_config.go)。

## 守护进程配置优先级

守护进程参数的已测试优先级是：

```text
命令行 flag > MULTICA_* 环境变量 > 当前 profile 的 config.json > 内置默认值
```

测试覆盖见[优先级测试](../../server/cmd/multica/cmd_daemon_precedence_test.go)。不要把配置文件中的零值简单理解为“关闭”：不同字段有清除、禁用或回退三种语义。

profile 还参与 daemon 的 PID、端口、状态和 workspace root 身份。相关测试固定了 profile 冲突、哈希端口碰撞以及旧 daemon 不返回 profile 字段时的兼容行为，见[daemon 身份测试](../../server/cmd/multica/cmd_daemon_identity_test.go)。

## 修改命令时更新什么

1. **命令注册与帮助**：在相应 `cmd_*.go` 注册命令/flag；若顶层分组变化，同步 [main.go](../../server/cmd/multica/main.go)和[帮助模板](../../server/cmd/multica/help.go)，并更新相邻命令测试。
2. **兼容入口**：重命名、移动或删除命令前更新[兼容测试](../../server/cmd/multica/cmd_compat_test.go)，并明确旧脚本是否必须继续工作。
3. **配置语义**：同时更新 `config show`、`config set`、daemon flag/env 解析、[配置测试](../../server/internal/cli/config_test.go)和[优先级测试](../../server/cmd/multica/cmd_daemon_precedence_test.go)。
4. **profile 与生命周期**：涉及 daemon 身份、端口或状态文件时运行 `server/cmd/multica/cmd_daemon_identity_test.go` 和相邻 daemon 测试。
5. **内置运行时命令名**：新增默认智能体 CLI 时更新[运行时命令名清单](../../scripts/agent-cli-command-names.txt)，避免普通测试意外执行用户机器上的真实 CLI。
6. **技能与文档**：如果命令、flag、API 字段或产品行为被内置 Skill 记录，同步 [builtin_skills](../../server/internal/service/builtin_skills/) 中的 `SKILL.md` 和 source map。

窄范围验证可以从以下命令开始：

```bash
cd server
go test ./cmd/multica ./internal/cli
```

只有实际运行后才能声称检查通过；涉及 daemon 子进程时，默认测试必须使用测试创建的 fake 或 missing executable，不能解析用户安装的真实智能体 CLI。

## 相关页面

- [功能域导航](../features/index.md)
- [安全边界](../security/index.md)
- [本地开发](../overview/getting-started.md)
- [参与贡献](../how-to-contribute/index.md)
- [可清理机会](../cleanup-opportunities.md)
