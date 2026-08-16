# ADR-0012：MCP、操作审批与自定义代码隔离边界

- 状态：已接受
- 日期：2026-08-17

## 决策

MCP Server 是受 Scope 与资源权限保护的 `MCPServer` 资源，而不是全局插件。API 和 Worker 只通过官方 MCP Go SDK 连接 HTTPS Streamable HTTP 服务，使用显式工具白名单，保存能力快照，并把所有外部内容标记为不可信。

系统把读与写分离。ReadOnly/Low 操作允许在 Resource Filter 检查后直接排队；Medium/High 需要另一位有 `operation:approve` 权限的用户在有效期内批准准确参数哈希。删除和写 SQL 不在本阶段开放。操作请求、审批、执行均是独立的持久化记录并进入审计日志。

声明式 Skill 在 ADK Runner 内执行受控工具。任何自定义代码都不能由 API、Worker、Scheduler 或 MCP `stdio` 启动，只能进入无特权、非 root、只读根文件系统、最小 ServiceAccount、默认无网络的 Kubernetes Job。

## 后果

MCP 的灵活性受到资源登记、网络策略和白名单限制；这会增加配置工作，但避免外部服务描述、响应或模型文本提升权限。操作审批引入用户协作和等待时间，但保留了可审计、可重放防护的变更闭环。自定义代码的执行能力依赖 Kubernetes 可用性，不作为单体进程的降级路径。
