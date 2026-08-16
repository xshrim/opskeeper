# MCP、受控操作与自定义 Skill 沙箱

本指南说明 T14 的运行边界和操作流程。MCP、Skill 与操作审批共享“资源授权优先、外部内容不可信、变更可追溯”的原则；它们不是让模型或用户绕过现有资源、凭据和 Scope 边界的通道。

## MCP 服务

MCP 服务先作为 `MCPServer` 资源登记在一个 Scope 中。当前生产接入只支持 `streamable_http`，且 URL 必须是没有用户信息的 HTTPS 公网地址。`stdio`、HTTP、localhost、私有 IP、链路本地地址、组播地址均被 API 进程拒绝。运行时会在拨号时再次解析并拒绝非公网地址，避免配置时与连接时的 DNS 结果不一致。

资源配置示例：

```json
{
  "transport": "streamable_http",
  "url": "https://mcp.example.com/mcp",
  "tool_allowlist": ["inventory.read", "alerts.list"],
  "timeout_seconds": 10,
  "max_response_bytes": 1048576
}
```

`tool_allowlist` 是精确、不可包含空格的工具标识列表，不能使用通配符。发现操作通过官方 `modelcontextprotocol/go-sdk` 的初始化和 `tools/list` 调用创建版本快照；调用也通过该 SDK 的 `tools/call` 完成，项目不实现 JSON-RPC 协议栈。

发现结果、工具描述、工具 schema、MCP 响应和其中任何“指令”都标为不可信数据。它们可以展示给用户、作为证据保存或交给模型做受限总结，但不能改变系统提示、工具白名单、资源权限或审批要求；前端按普通文本/JSON 呈现，不能作为 HTML 注入。

调用前会同时检查：调用用户当前的 `resource:use` Resource Filter、MCPServer 资源状态、该资源 Scope、已登记工具白名单、超时和响应大小。资源权限不足时不发起网络连接。MCP Tool 不能直接执行高风险操作；任何状态变更必须先创建操作请求。

## 受控操作流程

当前首批操作标识为：

| 操作 | 作用 | 风险与执行方式 |
|---|---|---|
| `kubernetes.restart_workload` | 重启指定工作负载 | 先生成 dry-run 请求；Medium/High 必须由另一位审批人批准；执行仅排队到受限 Job。 |
| `kubernetes.scale_workload` | 调整工作负载副本数 | 同上；请求必须包含精确命名空间、工作负载和期望副本等参数。 |

高危删除（操作名包含 `delete`）和写 SQL（操作名包含 `sql`）在服务端永久拒绝，不能通过提高风险、MCP 或审批绕过。

1. 有 `resource:use` 的用户选择已经有权访问的目标资源，填写操作、精确 JSON 参数、影响范围、回滚建议与 idempotency key，创建请求。
2. 服务端规范化 JSON 并生成 SHA-256 参数哈希。ReadOnly/Low 请求可直接进入可执行状态；Medium/High 请求默认 `pending`，有效期为 30 分钟。
3. 具备同一 Scope `operation:approve` 的另一位用户查看目标、参数哈希、影响、回滚建议和有效期，批准或拒绝。请求者不能自批。
4. 执行前服务端再次检查状态、未过期、审批哈希与当前参数哈希一致、当前 Resource Filter 仍允许目标、以及幂等键。任一条件不成立即拒绝执行。
5. 通过检查后创建一条唯一的执行记录并进入 `queued`。请求、审批、执行开始及结果都持久化，并写入安全审计事件。

批准仅覆盖其看到的参数哈希。参数发生变化时旧审批无法被重放，必须创建/审批新的请求。

## 自定义代码 Skill 沙箱

声明式 Skill 继续在 ADK Runner 中执行受控工具；它们不能运行任意脚本。需要代码执行的扩展不得在 API、Worker 或 Scheduler 主进程中运行，也不得使用 MCP `stdio` 让主进程启动外部命令。

这类扩展只能由已批准操作创建 Kubernetes Job 规格。生成的 Job 使用固定受审核 runner 入口，而不是用户提供的 `command`；Job 默认具有：

- `runAsNonRoot`、`RuntimeDefault` seccomp、只读根文件系统、禁用特权提升、删除全部 Linux capabilities；
- CPU / 内存 requests 与 limits、失败不重试、完成后 TTL 清理；
- 不挂载 ServiceAccount token；
- 仅使用无权限的 `opskeeper-sandbox` ServiceAccount；
- 默认拒绝全部出站网络。

基础 Kubernetes 清单位于 [deploy/sandbox](../../deploy/sandbox/README.md)。若确有经过审批的外部访问需求，平台管理员必须为该 Job 添加按目的 CIDR、端口和 DNS 范围精确限制的独立 NetworkPolicy，并在审批记录中说明；不得改为特权、host 网络、hostPath、root 用户或集群管理员凭据。

## 审计与故障排查

审计动作包括 `mcp.discover`、`mcp.tool.call`、`operation.request.create`、`operation.request.approved`、`operation.request.rejected` 和 `operation.execution.start`。审计内容记录资源/请求标识、Scope、参数哈希、风险和状态，不记录凭据、完整敏感参数或 MCP 响应正文。

MCP 发现失败时，会保存失败快照及受限错误摘要，便于排查连接、TLS、DNS、超时或服务端协议问题。调用失败不应被解释为资源健康；应在诊断或操作界面中明确标记“不可用/未验证”。
