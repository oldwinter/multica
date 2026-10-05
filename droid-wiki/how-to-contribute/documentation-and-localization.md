# 文档与本地化

产品文档在 [`apps/docs/content/docs/`](../../apps/docs/content/docs/)，Web/Desktop 翻译在 [`packages/views/locales/`](../../packages/views/locales/)。命名、术语和中文 voice 的唯一权威来源是[英文 conventions](../../apps/docs/content/docs/developers/conventions.mdx)与[中文 conventions](../../apps/docs/content/docs/developers/conventions.zh.mdx)；修改翻译、路由命名或中文产品文案前必须先查它们。

## 文档修改

- Fumadocs 页面使用 MDX；现有产品页通常以英文 `.mdx` 为基准，并配套 `.zh.mdx`、`.ja.mdx`、`.ko.mdx`。
- 改命令、flag、环境变量、API 字段或产品行为时，搜索并同步所有受影响的语言页面，不要只改代码旁的一处说明。
- 示例命令和代码标识符保持原样并实际核对仓库 script；不要把 `issue`、`task`、字段名或 slash command 混译。
- 链接尽量指向实际源码、配置或同一组相关文档，避免复制会漂移的长列表。
- 代码注释一律英文；中文只用于用户文案、翻译和中文文档。

本地预览与验证：

```bash
pnpm dev:docs
pnpm -C apps/docs typecheck
pnpm -C apps/docs test
pnpm -C apps/docs build
```

## Locale 文件

每个 namespace 在 `packages/views/locales/en/`、`zh-Hans/`、`ja/`、`ko/` 中保持对应 JSON。共享 copy 放 namespace 顶层；Web-only 放 `web` 段；Desktop-only 放 `desktop` 段。

Key 使用三层语义命名：

```text
feature.component.action
```

例如 `issues.toolbar.batch_update_success`、`inbox.empty.title`。变量插值使用 `{{var}}`。i18next 复数使用 `_one` / `_other`；中文没有语法单复数，只填写 `_other`，并使用合适量词。

运行 views 测试检查 locale 结构和 parity：

```bash
pnpm -C packages/views test
```

对应测试见 [`packages/views/locales/parity.test.ts`](../../packages/views/locales/parity.test.ts)。

## 核心术语

| 英文概念 | 简体中文 | 说明 |
| --- | --- | --- |
| Issue | 任务 | 用户提交并由智能体处理的工作单元 |
| Task | `task` | 智能体的一次执行；不可与“任务”混为同一词 |
| Workspace | 工作区 | 日常概念，完整翻译 |
| Agent | 智能体 | 日常产品概念 |
| Project | 项目 | 不保留英文 |
| Autopilot | 自动化 | 不译为“自动驾驶” |
| Runtime | 运行时 | 日常产品概念 |
| Daemon | 守护进程 | 日常产品概念 |
| Skill | `skill` | Multica 专有词，中文正文保留小写英文 |

品牌和通用缩写不翻译，例如 Multica、GitHub、Slack、API、CLI、URL、OAuth、JSON、SQL。角色 `owner` / `admin` / `member` 与任务状态 `backlog` / `todo` / `in_progress` / `in_review` / `done` / `blocked` / `cancelled` 是 schema 标识符，在中文 UI 中保持小写英文。

API/DB 字段、代码引用和字面命令始终保持英文，例如 `issue_status`、`task_id`、`multica issue ...` 和 `/issue`。表示运行时异常的普通名词 “issue” 应译为“异常”，不是产品实体“任务”。

## 中文 voice

- 中文使用全角标点；引号使用直双引号 `"..."`；省略号使用三个点 `...`。
- 中英文相邻时加一个空格，纯中文词组不额外加空格。
- 简洁直接，避免“对于 X 来说”“作为 X”“我们的”等翻译腔。
- 错误文案温和且明确；按钮以动词开头，通常 2–4 字；tooltip 使用完整短句；placeholder 用示例式提示。
- 拿不准时，先查 conventions，再参考 `apps/docs/content/docs/*.zh.mdx` 和现有 `zh-Hans` namespace。

## Built-in skill 与 source map

内置 skill 位于 [`server/internal/service/builtin_skills/`](../../server/internal/service/builtin_skills/)。如果修改以下任何契约，必须在同一 PR 更新相关 skill：

- CLI command 或 flag；
- API 字段；
- skill 已描述的产品行为、权限、状态或工作流。

每个受影响目录同时检查：

```text
server/internal/service/builtin_skills/<skill>/SKILL.md
server/internal/service/builtin_skills/<skill>/references/*-source-map.md
```

`SKILL.md` 说明给智能体的实际操作方式，source map 把说明锚定到实现和事实来源；只更新其中一个会让说明或追溯关系漂移。当前例子包括 [working-on-issues](../../server/internal/service/builtin_skills/multica-working-on-issues/)、[autopilots](../../server/internal/service/builtin_skills/multica-autopilots/) 和 [wiki](../../server/internal/service/builtin_skills/multica-wiki/)。

## checklist

- [ ] 修改前已查英文/中文 conventions，而不是依赖旧 glossary。
- [ ] 文档命令、字段、路由和代码示例与当前源码一致。
- [ ] 受影响的英文、中文、日文、韩文页面或 locale namespace 已同步。
- [ ] Issue/Task、skill、角色、状态和品牌术语没有混用。
- [ ] 插值、复数、量词、标点和中英空格符合约定。
- [ ] Web-only/Desktop-only copy 放在正确 namespace 段。
- [ ] CLI/API/产品行为变化已同步 `SKILL.md` 与 source map。
- [ ] 已运行 docs 和 views locale 所需的 typecheck、test 或 build。

## 相关页面

- [参与贡献](index.md)
- [添加功能](adding-a-feature.md)
- [模式与约定](patterns-and-conventions.md)
- [测试指南](testing.md)
- [开发规范（中文）](../../apps/docs/content/docs/developers/conventions.zh.mdx)
- [Docs package scripts](../../apps/docs/package.json)
