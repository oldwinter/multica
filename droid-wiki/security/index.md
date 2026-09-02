# 安全边界与信任模型

本页描述代码中可以核实的认证、授权、进程、桌面、存储和 webhook 边界，供实现与评审时导航。它不是安全审计、渗透测试、合规认证或“系统安全”的结论；列出某项控制也不证明它覆盖所有路径。修改敏感路径时仍需做针对性的威胁建模和测试。

## 边界总览

```text
浏览器 / Mobile / CLI
        │  用户会话、PAT、task token
        ▼
HTTP 认证中间件 ──► 工作区解析与成员/角色检查 ──► handler / service
        ▲                                              │
        │ daemon token                                 │ 短期 task 凭据
本机守护进程 ──► 每 task 执行环境 ──► 外部智能体 CLI ──► 输出与事件

Electron renderer ──► preload 允许的 IPC ──► 主进程 / 系统能力

外部提供商 ──► webhook token / 签名 / 限流 ──► 持久队列与 worker
```

## 身份认证边界

[常规认证中间件](../../server/internal/middleware/auth.go)区分多种凭据，而不是把所有 token 当作同一种用户会话：

- 请求带有 Bearer 凭据时先走 Bearer 路径，再考虑 cookie 会话。使用 cookie 的写请求需要通过 CSRF 校验。
- `mul_` 个人访问令牌按哈希查找；JWT 使用 HMAC 验证。代码中的验证行为不等于密钥生命周期已经过独立审计。
- `mat_` task token 绑定用户、智能体、task 与工作区。它携带的绑定关系是授权输入，不能用请求头把它扩展到其他工作区。
- `mdt_` daemon token 绑定守护进程和工作区上下文，具体兼容路径见[守护进程认证中间件](../../server/internal/middleware/daemon_auth.go)。
- `mcn_` Cloud PAT 通过 Fleet 验证；未配置 Fleet 验证能力时该路径失败关闭，并检查本地 owner 是否存在。

这几类凭据的权限和生命周期不同。新增端点时，应先确定允许的是人类会话、普通 PAT、task 身份还是 daemon 身份，不能仅以“已认证”代替授权设计。

## 工作区与人类操作边界

[工作区中间件](../../server/internal/middleware/workspace.go)负责解析工作区、验证成员关系和角色，并阻止 task token 通过其他 header 或 query 扩大其绑定作用域。仓库约定所有工作区查询都按 `workspace_id` 过滤，`X-Workspace-ID` 或工作区 slug 只是选择上下文，不是授权本身。

部分操作必须由人类发起。[`RequireHumanActor`](../../server/internal/handler/actor_guards.go)显式拒绝机器凭据。新增高影响写操作时，应检查：

1. 是否只要求登录，还是还要求成员、管理员或 owner 角色；
2. task/daemon 身份是否有业务理由访问；
3. 资源 path 参数是否先通过当前用户/工作区 loader 解析；
4. 是否存在跨工作区 ID 替换测试。

## 守护进程与本地执行边界

[执行环境构造](../../server/internal/daemon/execenv/execenv.go)为 task 建立独立工作目录、私有配置根和临时路径；task 管理的 CLI 调用使用私有 `MULTICA_TASK_CONFIG_ROOT`，避免读取或改写用户的常规 Multica profile。准备阶段的 helper 可被取消，[进程树运行器](../../server/internal/daemon/processtree/run.go)在取消时处理子进程树。

这些机制降低 task 之间的状态串扰，但不是操作系统级沙箱的同义词：

- 智能体 CLI 仍然是本机子进程，权限受启动守护进程的操作系统用户约束。
- 自定义参数会出现在子进程 argv 中；日志遮蔽不能保护 `ps` 或 `/proc`。
- 显式选择本地目录模式时，执行会在指定目录原地进行，这是有意提供的例外，不应被误写成隔离缺陷。
- 输出脱敏由[递归脱敏器](../../server/pkg/redact/redact.go)按模式处理文本和结构化值。它是纵深防御，不替代最小权限、凭据隔离或授权。

## Electron 主进程边界

[renderer web preferences](../../apps/desktop/src/main/renderer-web-preferences.ts)默认启用 sandbox 与 context isolation，并关闭 renderer 的 Node integration。[preload](../../apps/desktop/src/preload/index.ts)只向 renderer 暴露选择过的 IPC 表面；新增通道时仍需同时审查 preload、主进程 handler 和参数验证。

导航和外部打开是另一条边界：

- [导航守卫](../../apps/desktop/src/main/navigation-guard.ts)限制同窗口文档导航的 origin 与路径。
- [外部 URL 边界](../../apps/desktop/src/main/external-url.ts)只允许 HTTP(S) URL 用于外部打开和下载。
- renderer 配置中的 `webSecurity: false` 是源码明确记录的产品权衡，不能在没有利用路径和验证证据时直接标为漏洞；同时也不能因为有注释就免除对 renderer 内容、导航和 IPC 的评审。

## Secret、附件与对象存储

[secretbox](../../server/internal/util/secretbox/secretbox.go)使用 AES-256-GCM 封装被其调用的集成 secret。只有实际调用该封装的字段才能声称被应用层加密，不能推导为“数据库中的所有 secret 都加密”。

[S3 兼容存储](../../server/internal/storage/s3.go)支持签名、预签名和代理下载。附件 handler 还实现了上传大小限制、MIME 探测、短期能力、`nosniff` 和预览 CSP 等控制，主要入口见[文件 handler](../../server/internal/handler/file.go)。评审附件变更时应同时检查：

- 上传者是否能把对象绑定到目标工作区、task 或消息；
- 下载能力是否有期限，代理路径是否重新做授权；
- 服务端允许的文本预览类型是否与客户端入口一致；
- `Content-Type`、`Content-Disposition` 与浏览器预览 CSP 是否匹配预期。

## Webhook 与外部事件

[Autopilot webhook](../../server/internal/handler/autopilot_webhook.go)使用 URL bearer token，可选 HMAC，限制请求体大小，执行限流和去重，并把工作交给 PostgreSQL 持久队列。请求日志会对 Autopilot token 路径做遮蔽，见[请求日志中间件](../../server/internal/middleware/request_logger.go)。

[VCS webhook](../../server/internal/handler/vcs_webhook.go)按提供商校验签名，并读取受保护的集成 secret。新增 webhook 时至少需要明确：

1. 未认证入口如何证明来源；
2. 签名覆盖的原始字节与重放窗口；
3. body 上限、限流和幂等键；
4. secret 是否可能出现在 URL、日志或错误响应；
5. 入队前后的租户绑定如何保持。

## 评审清单

- [ ] 先写明 actor 类型、工作区和资源边界，再选择中间件。
- [ ] 机器凭据不能继承只适用于人类会话的能力。
- [ ] path/header/query 中的 ID 不会绕过 loader、成员或角色检查。
- [ ] 日志、argv、错误、重定向 URL 和事件 payload 不泄露新 secret。
- [ ] 子进程取消、超时、临时目录和进程树清理都有失败路径。
- [ ] Electron 新能力经过 preload 白名单和主进程参数验证。
- [ ] webhook 覆盖伪造、重放、超大 body、重复投递和队列恢复。
- [ ] 测试结论只覆盖实际运行的检查，不把实现阅读表述为审计结果。

## 相关页面

- [系统架构](../overview/architecture.md)
- [功能域导航](../features/index.md)
- [CLI 命令参考](../reference/cli-commands.md)
- [参与贡献](../how-to-contribute/index.md)
- [可清理机会](../cleanup-opportunities.md)
