# ADR-0011：Agent、Runner 与 MCP 使用指定官方运行时

- 状态：已接受
- 日期：2026-08-16

## 背景

OpsKeeper 后续需要支持主流大模型、声明式 Skill、受控 Tool Calling、诊断 Agent 和 MCP。Agent 循环、Runner、模型事件与 Tool 协议如果完全自行实现，会增加流式事件、会话状态、工具编排和模型兼容性的维护成本；同时，通用运行时不能替代 OpsKeeper 自身的 Scope、资源授权、预算、审计和敏感信息边界。

项目需要固定基础运行时，避免不同任务分别引入不兼容的 Agent 或 MCP 框架。

## 决策

1. LLM 接入必须支持主流大模型，OpenAI-compatible Chat/Tool Calling 是首批必须通过验收的兼容协议。
2. Agent 编排、Skill Tool 调用和 Runner 执行内核必须使用 [`google/adk-go`](https://github.com/google/adk-go) 的最新稳定主版本 v2。对应任务开始时先核对最新稳定版本并在 `go.mod`、`go.sum` 中固定；本 ADR 建立时最新稳定标签为 `v2.2.0`。
3. OpsKeeper 在 ADK Runner 外建立 Policy Enforcement 层，负责用户与目标资源授权、Skill 与 Tool 白名单、参数 Schema、超时、调用次数、Token 和输出预算、脱敏、Evidence 及审计；不得另写一套替代 ADK 的 Agent/Runner 循环。
4. OpenAI-compatible Provider 必须实现 ADK 的模型接口。若 `adk-go` 当前版本未提供满足要求的实现，可以参考 [`achetronic/adk-utils-go`](https://github.com/achetronic/adk-utils-go) 的 OpenAI LLM 等实现，将经过裁剪、审查和测试的必要代码纳入本仓库。
5. 禁止在项目中 import `achetronic/adk-utils-go`，也不将其加入 `go.mod`。复制或改写参考代码时必须核对许可证、保留必要归属，并由 OpsKeeper 自己维护安全修复和兼容性测试。
6. MCP Client、Server 连接、能力发现和 Tool 调用必须使用 [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) 的最新稳定版本。对应任务开始时固定版本；本 ADR 建立时最新稳定标签为 `v1.7.0`。
7. ADK 和 MCP SDK 类型限制在对应基础设施边界内。组织、资源、授权、Skill 版本、执行记录和诊断模块使用 OpsKeeper 自有领域类型，避免 SDK 升级扩散到业务模型。
8. T10 实施 ADK Agent、Runner、AIEngine（内部 LLMEndpoint）和 Skill；历史 LLMProvider 仅保留兼容解析。T14 才引入 MCP SDK 并实施 MCP 调用。不得为了登记 MCPServer 资源而在 T10 提前加入未使用的 MCP 运行时依赖。

## 验收约束

- T10 必须使用 ADK v2 的 Agent、Runner 和 Tool 能力完成至少一个多轮 Tool Calling 流程，并通过 OpenAI-compatible 模拟服务验证。
- T10 必须验证未声明 Tool、无权资源、非法参数、超时、超预算和模型异常均被 ADK 外层的 OpsKeeper Policy 阻断或收敛。
- 依赖检查必须确认仓库没有 import `achetronic/adk-utils-go`。
- T14 必须通过官方 MCP Go SDK 验证能力发现、Tool 白名单、调用、超时、响应限制和恶意内容隔离。

## 后果

- Agent 和 Tool Calling 的通用运行时能力由 ADK v2 提供，OpsKeeper 专注于运维领域模型和安全控制。
- OpenAI-compatible 支持通过 ADK 模型边界实现，不把某一家模型 SDK 的类型扩散到 Skill 和诊断模块。
- MCP 使用官方协议实现，避免自行维护 JSON-RPC 和传输细节。
- SDK 升级必须经过模拟 Provider、Tool Calling、权限隔离、流式事件和 MCP 兼容性回归测试。
