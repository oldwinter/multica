# 可清理机会

本页只收录能从当前源码、测试或仓库文档核实的维护机会。每项把“已核实事实”和“建议动作”分开；建议不是已批准计划，也不自动表示现有行为是缺陷。

## 协议兼容与事实源

### Kiro 同时发送 `content` 与 `prompt`

**已核实事实：** [Kiro 适配器](../server/pkg/agent/kiro.go)注明公开文档使用 `content`，Kiro CLI 2.1.1 仍要求标准 ACP `prompt`，因此 `session/prompt` 当前同时发送两个字段。相邻 TODO 要求在 Kiro 统一 canonical payload 后删除一个字段；[测试](../server/pkg/agent/kiro_test.go)固定了当前双字段兼容行为。

**建议动作：** 先用受支持 Kiro 版本矩阵确认 canonical 字段和最低版本，再删除兼容字段及对应测试分支。没有版本证据前不要提前清理。

### 工作区 slug 到 UUID 每次查询

**已核实事实：** [工作区中间件](../server/internal/middleware/workspace.go)的 `resolveWorkspaceUUID` 有 TODO：slug 不可变，可以用短 TTL 缓存 slug→UUID 查询。

**建议动作：** 在保留 workspace membership/role 每请求校验的前提下，只缓存身份解析；为失效、未知 slug、数据库错误和多实例行为补测试，避免把授权结果一起缓存。

### 文本附件预览 allowlist 有两份

**已核实事实：** 服务端 [`isTextPreviewable`](../server/internal/handler/file.go)和客户端[预览工具](../packages/views/editor/utils/preview.ts)分别维护文本预览类型。两边注释都要求保持同步，并说明漂移会造成 415 fallback 或 Eye 入口消失。

**建议动作：** 生成共享清单或增加跨语言契约测试。服务端仍应独立执行 allowlist，不能只信任客户端判断。

### CLI 配置 round-trip 丢弃未知字段

**已核实事实：** [CLI 配置结构](../server/internal/cli/config.go)注明 Go `encoding/json` 在 load/save round-trip 时会丢弃结构体中未知字段。[`TestCLIConfig_UnknownFieldsArePreserved`](../server/internal/cli/config_test.go)因当前限制被显式跳过。

**建议动作：** 选择 `json.RawMessage`、保留未知 map 或其他可验证方案，先启用现有测试，再检查原子写入、`0600` 权限和已知字段验证没有回退。

### 运行时安装文档与命令清单可能漂移

**已核实事实：** [默认运行时命令名清单](../scripts/agent-cli-command-names.txt)当前包含 26 个命令名；[`CLI_INSTALL.md`](../CLI_INSTALL.md)的最终检测与错误提示只列出 9 个。前一处被普通测试入口用作防止执行环境中的真实 CLI，后一处面向安装用户；仓库没有在这两处之间建立自动同步。

**建议动作：** 明确 `CLI_INSTALL.md` 的列表是示例还是完整支持集。若是完整集，从 manifest 生成；若只是示例，修改文案并链接完整运行时文档，避免读者推断未列工具不受支持。

## Mobile 待完成体验

### 任务 reaction 只有展示，没有新增入口

**已核实事实：** [Mobile reaction row](../apps/mobile/components/issue/issue-reaction-row.tsx)在无 reaction 时不渲染，并注明新增 reaction 延后到任务正文的长按 affordance。

**建议动作：** 先确认与 Web/Desktop 一致的权限、计数和去重语义，再实现适合手机的新增入口，并覆盖空状态、长按可发现性和辅助功能。

### Blockquote 样式等待模拟器验证

**已核实事实：** [Mobile Markdown 样式](../apps/mobile/lib/markdown/markdown-style.ts)把 blockquote `borderWidth` 设为 3，但注释说明依赖库 schema 没有明确这是左侧条还是完整边框，需要模拟器验证。

**建议动作：** 在受支持 iOS 模拟器和明暗模式中记录实际渲染；如果是完整边框，再根据库能力调整样式，而不是仅凭注释推断视觉缺陷。

## 已记录的产品与设计债务

### Desktop 离开工作区的顺序例外

**已核实事实：** [CLAUDE.md](../CLAUDE.md)规定删除工作区必须等待服务端成功后再导航；离开工作区当前却为避免 `member:removed` 实时竞态而先清理/导航、后执行 mutation，并明确标记为已知债务、不可复用模式。

**建议动作：** 设计单一 responder 和 self-initiated guard 来消除实时竞态，再把 leave 流程收敛到“等待服务端后清理”。先添加竞态回归测试，不要直接交换两行调用顺序。

### Raw color 岛仍需渐进迁移

**已核实事实：** [设计系统](../DESIGN.md)把现有 landing 与 docs raw-color 岛定义为设计债务，要求在相关表面被触及时迁移到语义变量；[token contract](../packages/ui/styles/TOKEN_CONTRACT.md)定义了共享语义 token 约束。

**建议动作：** 按被修改的表面小批迁移并运行 token contract、明暗模式和 skin 检查，不做与当前功能无关的全仓机械替换。

## 明确不列为缺陷的设计选择

以下实现需要持续评审，但现有证据只表明它们是明确选择，不能仅凭存在就标成 bug：

- Electron renderer 的 `webSecurity: false`：源码记录了权衡，评估必须结合导航、内容来源、preload 与 IPC 利用路径。
- 守护进程 local-directory 模式原地执行：这是用户显式选择的模式，不是 task 隔离逻辑遗漏。
- agent output 的模式脱敏：它是纵深控制，不应被当成授权或完备 secret 检测器，也不能仅因“不完备”就替代具体泄露路径分析。

这些边界的实现背景见[安全边界](security/index.md)。

## 相关页面

- [功能域导航](features/index.md)
- [安全边界](security/index.md)
- [CLI 命令参考](reference/cli-commands.md)
- [分支背景](background/index.md)
- [下游扩展](background/downstream-extensions.md)
- [参与贡献](how-to-contribute/index.md)
