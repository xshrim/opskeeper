# I003-R001 AIEngine（AI引擎）执行框架设计与实现

**迭代：** I003-ai-engine  
**需求状态：** 已完成
**设计文档：** [AIEngine（AI引擎）设计](../../design/ai-engine.md)  
**验收报告：** [R001-requirement-acceptance.md](R001-requirement-acceptance.md)

## 1. 背景与目标

当前 AI 对话、故障诊断、Skill 执行和自动巡检存在不同的执行入口。需要建立统一的 AIEngine（AI引擎），让所有需要模型能力的业务都通过同一套执行框架完成 Agent 执行、Tool Calling、推理循环、Prompt 编排、流式响应、上下文加载、知识检索和工作流编排。

业务调用方只选择 AIProvider 和模型，不直接访问 Provider 地址或凭据。AIEngine 必须支持前端交互式诊断，也支持后台自动巡检和可恢复的长任务。

详细架构、接口和生命周期见 [AIEngine 设计文档](../../design/ai-engine.md)，本文只维护本迭代的需求和验收边界。

## 2. 范围

本需求包含：

- 统一 AIEngine 执行入口和执行 Profile；
- Agent 执行、推理循环、Tool Calling、同步和流式响应；
- 上下文资源工具化，并接入 Connector 和远程 MCP Server；
- 指定 Skill、专家 Agent、知识库和工作流；
- 工具调用过程、输入输出、错误和证据的可追溯记录；
- 权限、预算、取消、超时、失败恢复和敏感信息保护；
- 将 Skill 作为可选的 Prompt/工具/契约适配器接入 AIEngine；Diagnosis、Inspection 和 Workflow 直接使用统一 Agent Runtime。巡检策略不强制绑定 Skill，可选绑定 AgentProfile。
- AI 诊断页面可选择具备诊断能力的 AIProvider 和模型，并通过 AIEngine 完成实际故障诊断。
- 诊断页面展示执行状态、受控工具动作、证据链、模型错误原因和中断状态。

## 3. 非目标

- 不允许 Agent 绕过 Scope、Resource、RBAC、Tool Policy 或审计链路；
- 不开放任意 HTTP 请求或未审批的高风险写操作；
- 不在本需求中实现基于价格、质量或反馈的动态模型评分；
- 不把大模型节点暴露为独立业务资源；
- 不要求本迭代立即拆分独立微服务；
- 不以模型输出直接修改确定性巡检评分。

## 4. 功能需求

### 4.1 统一执行

- 提供统一 AIEngine 入口，支持同步执行、流式执行和主动取消；
- 支持交互式对话、故障诊断、Skill、自动巡检和工作流执行 Profile；
- 请求能够指定 Scope、AIProvider、模型、任务、消息、上下文、Skill、Agent、工作流和执行预算；
- 返回统一执行 ID、结果、状态、Token 用量、Tool Call 数量和实际错误原因；
- 支持执行幂等标识，拒绝同一执行 ID 的并发重复运行。

### 4.2 上下文和工具

- 用户选择的资源必须经过 Scope 和资源使用权限校验；
- AIEngine 能够为 PostgreSQL、Kubernetes、监控资源和远程 MCP Server 建立受控工具集合；
- 模型开始分析前自动采集必要的基础上下文；
- Agent 后续工具调用必须经过统一 Tool Gateway、白名单、Schema、策略和超时控制；
- 默认工具只读，高风险写操作必须进入既有审批流程。

### 4.3 Skill、Agent、知识库和工作流

- 支持只指定 Skill、只指定专家 Agent 以及 Skill + Agent 组合；
- Skill 和 Agent 必须进行版本、权限、能力和输入输出契约校验；
- 支持按 Scope 和权限检索知识库，并提供可追溯引用；
- 支持持久化工作流中的 Agent、Skill、Tool、Retrieval、Condition、Parallel 和 Approval 节点；
- 工作流支持超时、有限重试、暂停、取消、审批和断点恢复。

### 4.4 流式和审计

- 流式响应能够传递模型增量、工具状态、上下文、工作流和最终状态事件；
- 事件具有执行 ID、递增序号、时间戳、状态和结构化内容；
- 全程记录工具名称、来源、资源、入参、出参、状态、耗时、错误、重试和策略决策；
- 支持取消、超时、模型失败、工具失败和中断原因的明确展示；
- 工具结果、Prompt、事件和审计内容必须支持敏感信息脱敏。

## 5. 非功能需求

### 5.1 安全与权限

- 每次执行重新校验 Actor、Scope、AIProvider、模型、上下文资源和工具权限；
- 外部资源内容视为不可信数据，不能覆盖系统安全规则；
- 凭据、Token、密码和 Authorization Header 不得进入模型上下文、普通日志或前端响应；
- 普通用户只能查看符合权限范围的脱敏执行记录。

### 5.2 可靠性与可控性

- 所有执行必须有超时、取消和预算边界；
- 模型调用、工具调用和工作流节点必须传播取消信号；
- 外部依赖必须有超时、并发限制、有限重试和响应大小限制；
- 长任务支持执行快照、断线续读或断点恢复；
- 已产生外部副作用的操作不得被伪造为未执行或自动无条件重试。

### 5.3 可观测性与可维护性

- 执行、模型、工具、工作流和错误均可按执行 ID 关联追踪；
- 事件和审计记录具有稳定结构，便于 API、前端和后台任务复用；
- 新增 Runner、Model Adapter 或 Tool 实现不应修改统一执行契约；
- 旧模型资源和调用入口不保留；相关业务统一使用 AIProvider + AIEngine。

## 6. 任务清单

| 任务 | 名称 | 交付目标 | 依赖 | 状态 |
|---|---|---|---|---|
| T01 | AIEngine 统一执行内核 | Request、Result、Event、Profile、预算、取消和统一 Runtime | 无 | 已完成 |
| T02 | 上下文工具层 | Context Resolver、Tool Registry、Policy Gateway、Connector/MCP 工具 | T01 | 已完成 |
| T03 | Skill 与 Agent Profile | Skill 版本、专家 Agent、组合 Prompt 和契约校验 | T01-T02 | 已完成 |
| T04 | 流式事件与工具调用审计 | 事件持久化、SSE 续读、Tool Call 脱敏审计 | T01-T03 | 已完成 |
| T05 | 知识库与工作流编排 | 知识检索、引用、持久化 DAG、审批和恢复 | T01-T04 | 已完成 |
| T06 | AI 诊断页面接入 AIEngine | Provider/模型选择、真实诊断执行、流式状态和证据展示 | T01-T04 | 已完成 |

## 7. 依赖与风险

- 依赖现有 AIProvider、Resource、Authorization、Connector、MCP、Skill、Diagnosis、Inspection 和 Audit 边界；
- 外部模型和 MCP 服务的协议差异、延迟和错误格式需要通过 Adapter/Tool Gateway 隔离；
- 长时间执行、工具副作用和流式断线可能造成重复调用，必须依赖幂等键、快照和审计；
- 数据库表、HTTP API 和独立 Worker 的新增应在对应 T02-T05 任务中分别评审，不在 T01 提前扩张。

## 8. 需求验收标准

- 对话、诊断、Skill 和巡检能够使用同一个 AIEngine 入口；
- 统一入口支持同步、流式、取消、超时、重复执行保护和明确错误原因；
- 上下文资源能够在权限约束下自动形成工具并执行基础采集；
- 指定 Skill、Agent、知识库和工作流时能够执行对应能力并保留关联信息；
- 工具调用的输入、输出、状态、耗时、错误和证据能够追溯且已脱敏；
- 预算、权限、只读策略、审批、重试和取消边界能够通过测试验证；
- 后端单元/集成测试、API 测试、必要的前端交互测试和安全检查通过；
- 每个任务都有独立验收记录，未完成能力明确转入后续任务或 backlog。
