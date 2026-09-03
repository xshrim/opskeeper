# AIProvider + AIEngine 设计

**状态：** I003 实施版本  
**适用范围：** I003 后续 AIEngine 改造  
**最后更新：** 2026-09-02

## 1. 设计结论

第一版只保留两个核心概念：

```text
AIProvider（模型服务商与模型目录）
        |
AIEngine（统一执行、工具、上下文、流式响应和审计）
```

不再保留 `AIEndpoint`。业务调用方可以直接选择一个 AIProvider 和其中的模型；未显式选择时，AIEngine 根据当前 Scope 和执行场景的默认标签解析 Provider，并负责校验标签对应的能力要求和执行预算。

`AIProvider` 的中文名为**模型服务商**，`AIEngine` 的中文名为**AI 引擎**。Provider 下的单个模型没有独立资源 ID，不维护 `id`、`display_name` 或 `version` 属性；模型的唯一标识就是发送给上游服务的 `name`。

## 2. 目标与边界

### 2.1 目标

- 前端 AI 诊断可以按“模型服务商 -> 模型”直接选择模型。
- 默认、诊断、巡检和工作流是 Scope 级 Provider 默认标签，不是模型或 Provider 的用途许可。
- 所有模型调用、上下文加载、Connector/MCP 工具调用和 Skill/Agent 执行均经过 AIEngine。
- AIEngine 固定本次执行的 Provider 和模型，避免执行过程中出现不可解释的模型漂移。
- 支持同步结果、流式响应、取消、超时、预算、断线续读和完整审计。
- 模型能力显式声明，并在执行前校验，而不是依赖上游错误后才发现能力不足。

### 2.2 非目标

- 第一版不把每个模型建成独立 Resource。
- 第一版不做跨 Provider 的自动模型聚合和隐式路由。
- 第一版不允许业务绕过 AIEngine 直接访问 Provider 地址或凭据。
- 第一版不把任意 Provider 请求参数原样透传给上游。

### 2.3 AIEngine 强制执行能力

以下能力是 AIEngine 的必选契约，而不是某个诊断页面或特定模型的可选增强。任何
新的业务适配器（对话、诊断、巡检或工作流）都必须复用同一套语义；适配器不得把
执行降级为“单次模型请求 -> 单次工具调用 -> 最终回答”。

1. **ReAct（Reasoning and Acting）。** 每轮执行按
   `Reasoning summary -> Action -> Observation` 交替推进。Reasoning summary 是
   面向用户的、脱敏且限长的阶段摘要；Action 是经过授权和策略检查的工具调用；
   Observation 是工具返回的结果、错误、超时或环境事实。只有在 Observation 完成
   后才能开始依赖该结果的下一轮推理。
2. **智能体循环（Agentic Loop / Multi-turn Tool-Use Loop）。** Runtime 必须持续
   驱动模型回合和工具回合，直到模型给出无待执行工具的最终回答，或触发明确终止
   信号（`stop`、取消、超时、预算耗尽或受控错误）。不得在第一轮工具返回后强制
   结束，也不得在没有终止信号时静默截断。
3. **流式阶段性输出（Progress Streaming）。** 在模型回合开始、阶段摘要、工具
   请求、工具开始、工具完成/失败、Observation 汇总和下一轮恢复等边界，Runtime
   必须按实际发生顺序持久化并通过 SSE 推送事件。耗时工具前后都要有可消费的进度
   反馈，客户端无需等待最终回答即可看到执行过程。
4. **动态规划与自我纠错（Dynamic Planning & Self-Correction）。** 每个工具
   Observation 都必须进入下一轮模型输入。模型可根据错误、空结果、部分结果、权限
   拒绝、超时或环境变化改换工具、修正参数、缩小范围、重试或说明无法完成；Runtime
   不得吞掉错误、伪造结果或无条件沿用旧计划。
5. **长程/多轮任务执行（Long-horizon Execution）。** Runtime 必须支持数十乃至
   上百次连续的“推理-行动-反馈”步骤，并以 `MaxIterations`、`MaxToolCalls`、
   Token、输出大小、总耗时和取消信号实施硬上限。达到任一上限时要发出带原因的预算
   事件和终态，且不得再发起新的工具副作用。
6. **环境反馈驱动推理（Environment-driven Reasoning）。** 代码运行、编译器和
   测试输出、Git 状态、容器/集群状态、实例指标以及 Connector/MCP 返回值都是后续
   决策的主要输入。静态 Prompt 只规定角色、目标和边界；最新环境 Observation 优先
   于过时的 Prompt 假设。所有环境事实必须可追溯到对应事件和工具审计记录。

上述能力必须在同步和流式执行中保持一致。`Request.Stream=false` 只改变传输方式，
不改变循环、终止、纠错、预算或审计语义。

## 3. 核心概念

### 3.1 AIProvider

AIProvider 是一个可连接的模型服务配置，包含服务地址、协议、凭据引用、服务级限制和模型目录。Provider 本身不生成回答，具体回答由其目录中的某个模型完成。

### 3.2 Model

Model 是 AIProvider 配置中的一个目录条目，不是独立资源。`name` 是上游 API 使用的模型名称，也是该 Provider 内的唯一键。

### 3.3 AIEngine

AIEngine 是唯一的业务调用入口，负责：

- 解析 Scope、用途、Provider 和模型；
- 组装 Prompt、Skill、Agent、知识和上下文；
- 执行推理循环和 Tool Calling；
- 输出同步结果或 SSE 事件；
- 管理超时、取消、预算和失败策略；
- 持久化执行事件和 Tool Call 审计。

### 3.4 执行运行时与场景适配器

AIEngine 内部只有一个通用 Agent 执行运行时，负责模型调用、推理循环、上下文工具、预算、取消、流式事件和输出契约。业务场景通过适配器把自己的输入转换为 `aiengine.Request`：

```text
Diagnosis Adapter     Inspection Adapter     Workflow Adapter
       \                    |                    /
                    AIEngine Runtime
             (Agent Loop + Tool Gateway + Audit)
                              |
                         AIProvider + Model
```

Skill 不是执行运行时。Skill 只提供可选的专家指令、工具声明、输入/输出 Schema 和目标资源约束；`SkillService.ResolvePlan` 将已发布版本解析为只读 `ExecutionPlan`，由 Runtime 注入请求。Skill 不持有模型客户端、Connector、ADK Runner 或执行 Store。未指定 Skill 的诊断、对话和巡检仍直接使用 AgentProfile 或内置指令进入 AIEngine。这样所有场景共享同一套模型固定、推理循环、Tool Calling、预算、取消、流式事件和审计行为，不会因为是否使用 Skill 产生两套执行语义。

代码分层约束：

- `backend/aiengine` 持有唯一的通用 `AgentRunner` 和 `Runtime`；它不依赖 Skill、Diagnosis 或具体 Connector。
- `backend/skill` 只实现 Skill 资源/版本解析，并输出 `aiengine.ExecutionPlan`；不得依赖模型、Connector、执行 Store，也不得定义 Runner 或执行入口。
- `backend/diagnosis` 只负责会话、诊断计划、Evidence 和报告，并直接依赖 `aiengine.Engine`。
- `backend/inspection` 和 `backend/aiengine/workflow_service.go` 同样通过 `aiengine.Engine` 执行 Agent，不直接访问模型客户端。巡检策略可以选择一个 AgentProfile；未选择时使用内置巡检解释契约，不再把 Skill 作为必选执行依赖。

当前实现由 `aiengine.Runtime` 统一管理生命周期、上下文、取消和计划解析：无 Skill 的请求直接进入 `aiengine.AgentRunner`；带 `skill_resource_id` 或 `skill_version_id` 的请求先经 `PlanResolver` 注入执行计划，然后仍进入同一个 `aiengine.AgentRunner`。Diagnosis、Inspection 和 Workflow 都直接使用该路径；巡检未配置 AgentProfile 时使用内置契约。不存在 Skill Runner 回退路径或第二套 Agent Loop。

### 3.5 统一对话与可选资源上下文

AI 诊断工作台不再根据用户文本把会话拆分为“普通对话”与“诊断模式”，也不维护第二个 Runner 或通用聊天 Agent。每一条消息都由同一个 AIEngine Runtime 执行；工作台请求使用 `PurposeDiagnosis` 仅用于按当前 Scope 解析该场景的默认 Provider，而不强制模型输出诊断报告。

资源上下文完全由用户在会话中显式选择：未选择资源时，AIEngine 直接完成普通问答；选择资源时，只有当前用户具备 `resource:use` 权限且资源处于活动状态的资源会出现在选择列表，并被解析为受控只读工具。模型根据问题自主决定是否调用工具。后端在会话创建、上下文解析和每次工具调用时都再次执行 `resource:use` 授权校验，以应对绕过界面、权限在会话期间撤销或历史会话保留资源 ID 的情形。

无资源上下文的回答不需要 Evidence 或诊断假设；已选择资源但模型未调用工具时，回答会保留原文，同时标注资源相关结论尚待核验。只有产生工具证据时，系统才持久化 Evidence、受支持假设和相应的可追溯建议。

### 3.6 AgentProfile

AgentProfile 是资源目录中的可复用专家智能体配置，不是模型资源，也不直接保存
Provider 地址或凭据。它包含版本、专家指令、适用资源类型、所需模型能力、工具白名单
以及可选的输入/输出 JSON Schema，并沿用 `resource:read` 与 `resource:use` 权限。

一次执行可以选择只使用 Skill、只使用 AgentProfile，或同时使用 Skill 和 AgentProfile。
组合执行时，AgentProfile 指令作为专家约束放在 Skill 指令之前，二者都必须满足输入和
输出契约；AgentProfile 的工具白名单是最终限制，模型能力要求与执行场景要求取并集。
只使用 AgentProfile 时，AIEngine 使用其指令和契约创建临时执行，不伪造 Skill 版本记录，
但仍经过统一的模型解析、上下文工具策略、生命周期事件和审计链路。

AgentProfile 的创建、更新、启用和停用复用统一 Resource API（`kind=AgentProfile`），
因此沿用现有资源目录的 Scope、RBAC 和审计行为；执行请求只传递
`agent_profile_id`，不会把提示词或工具白名单直接从客户端注入运行时。

AgentProfile 的契约快照通过以下版本 API 管理：

- `POST/GET /api/v1/agent-profiles/{profileID}/versions`：创建或查询版本；
- `POST /api/v1/agent-profiles/{profileID}/versions/{versionID}/publish`：发布版本；
- `POST /api/v1/agent-profiles/{profileID}/versions/{versionID}/disable`：停用版本。

执行时优先使用该 Profile 的最新已发布版本；没有已发布版本时才使用 Resource 初始配置。
版本发布后不可修改，新的变更必须创建新版本。

## 4. AIProvider 数据模型

### 4.1 Resource 配置

AIProvider 仍然作为统一 Resource 保存。凭据通过 `resource.credential_id` 关联加密凭据，API 响应不得返回密钥明文。

```json
{
  "provider_type": "openai_compatible",
  "protocol": "openai_chat_completions",
  "base_url": "https://api.example.com/v1",
  "timeout_seconds": 60,
  "max_concurrency": 8,
  "rate_limit_per_minute": 600,
  "enabled": true,
  "default_model": "deepseek-chat",
  "models": [
    {
      "name": "deepseek-chat",
      "context_window_tokens": 128000,
      "max_output_tokens": 128000,
      "temperature": 0.7,
      "temperature_mutable": true,
      "capabilities": [
        "text",
        "tool_calling",
        "structured_output",
        "stream"
      ],
      "enabled": true
    }
  ]
}
```

### 4.2 Provider 属性

| 属性 | 必填 | 说明 |
|---|---:|---|
| `provider_type` | 是 | 服务商类型，如 `openai`、`anthropic`、`gemini`、`openai_compatible` |
| `protocol` | 是 | 请求协议适配器，如 `openai_chat_completions`、`openai_responses`、`anthropic_messages` |
| `base_url` | 是 | 服务地址；禁止由业务请求覆盖 |
| `timeout_seconds` | 否 | Provider 默认请求超时 |
| `max_concurrency` | 否 | Provider 级并发上限 |
| `rate_limit_per_minute` | 否 | Provider 级速率限制 |
| `enabled` | 是 | Provider 是否允许新执行 |
| `default_model` | 否 | Provider 默认模型，必须存在于 `models[]` 且已启用 |
| `models` | 是 | 模型目录，至少一个条目 |

Provider 本身不保存诊断、巡检或工作流标签。标签属于 Scope 与 Provider 的绑定关系，表示某个场景下的默认选择；同一个 Provider 可以在同一个 Scope 同时拥有多个场景标签。

### 4.3 Model 属性

模型条目只使用以下属性，不增加 `id`、`display_name` 和 `version`：

| 属性 | 必填 | 说明 |
|---|---:|---|
| `name` | 是 | 上游实际模型名称，Provider 内唯一 |
| `context_window_tokens` | 是 | 输入上下文和输出预算可使用的总窗口 |
| `max_output_tokens` | 否 | 模型允许的最大输出 Token；未填写时默认 `128000`，仍受 Provider 实际模型能力和上下文窗口约束 |
| `temperature` | 否 | 模型默认温度，建议范围 0-2 |
| `temperature_mutable` | 否 | 是否允许执行请求覆盖温度 |
| `capabilities` | 是 | 模型能力集合 |
| `enabled` | 是 | 模型是否可被新执行选择 |
| `priority` | 否 | 仅在显式开启故障切换或候选排序时使用 |

模型能力使用可扩展集合，而不是为每项能力增加数据库列：

```json
[
  "text",
  "vision",
  "audio_input",
  "audio_output",
  "tool_calling",
  "structured_output",
  "stream",
  "deep_thinking",
  "long_context"
]
```

第一版公共能力定义如下：

| 能力 | 含义 |
|---|---|
| `text` | 文本输入和文本输出 |
| `vision` | 图片输入 |
| `audio_input` | 音频输入 |
| `audio_output` | 音频输出 |
| `tool_calling` | 函数或工具调用 |
| `structured_output` | JSON 或 JSON Schema 结构化输出 |
| `stream` | 流式输出 |
| `deep_thinking` | 深度思考或推理模式 |
| `long_context` | 超长上下文支持 |

### 4.4 参数归属原则

| 参数 | 归属 | 原因 |
|---|---|---|
| 服务地址、协议、凭据 | AIProvider | 同一服务连接共享 |
| Provider 并发、限流和请求超时 | AIProvider | 账号或服务地址级限制 |
| 模型名称 | Model | 上游模型唯一标识 |
| 上下文窗口 | Model | 不同模型通常不同 |
| 最大输出长度 | Model | 不同模型限制不同 |
| 默认温度 | Model | 不同模型的推荐值不同 |
| 支持文本、视觉、音频、工具、流式和深度思考 | Model | 能力属于具体模型 |
| 当前执行温度、输出预算 | AIEngine Request | 运行时参数，受模型和权限约束 |

## 5. Scope 场景默认标签

Provider 的 `default_model` 和模型能力保存在 Provider；`default`、`diagnosis`、`inspection`、`workflow` purpose 保存在 Scope 与 Provider 的绑定关系中（数据库列名为 `tag`）。Purpose 表示该场景的默认 Provider，不表示模型用途许可。

默认 Provider 按级别逐级回退：先检查当前级别的具体标签，再检查当前级别的 `default` 标签；当前级别两者都没有时，才检查上级级别的具体标签和 `default` 标签。

建议新增表：

```sql
CREATE TABLE scope_ai_provider_bindings (
    scope_id uuid NOT NULL REFERENCES scopes(id),
    provider_resource_id uuid NOT NULL REFERENCES resources(id),
    tag text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, tag, provider_resource_id),
    CHECK (tag IN ('default', 'diagnosis', 'inspection', 'workflow'))
);
```

每个 Scope 的每个标签最多只能出现在一个 Provider 上。数据库使用唯一索引保证约束：

```sql
CREATE UNIQUE INDEX scope_ai_provider_one_tagged_provider
    ON scope_ai_provider_bindings(scope_id, tag);
```

同一个 Provider 可以拥有多个标签，例如：

```text
项目 A / Provider X: default, diagnosis
项目 A / Provider Y: inspection
项目 A / Provider Z: workflow
```

标签能力要求由 AIEngine 固定定义：

| 标签 | 默认模型必须具备的能力 |
|---|---|
| `default` | `text` |
| `diagnosis` | `text`、`tool_calling`、`stream` |
| `inspection` | `text`、`tool_calling`、`structured_output` |
| `workflow` | `text`、`tool_calling`、`structured_output` |

管理员添加或修改标签时，后端读取 Provider 的 `default_model`，确认该模型存在、启用并满足标签能力要求；不满足时拒绝保存并返回缺失能力列表。Provider 修改 `default_model` 或模型能力时，也必须重新校验已有标签，不能让绑定进入失效状态。

解析顺序：

```text
请求显式 Provider
  -> 当前 Scope 对应场景标签的 Provider
  -> 父级 Scope 对应场景标签的 Provider
  -> 平台级对应场景标签的 Provider
```

显式 Provider 必须满足：

- 当前 Actor 对该 Provider 具有 `resource:use`；
- Provider 处于启用状态；
- 目标模型存在、启用且满足当前执行的能力要求。

## 6. AIEngine 请求契约

```go
type Request struct {
    ExecutionID          string
    ActorID              string
    ScopeID              string
    Purpose                Purpose
    AIProviderResourceID string
    ModelName            string
    ModelOverride        string
    Task                 string
    Messages             []Message
    Context              ContextRequest
    SkillResourceID      string
    AgentID              string
    KnowledgeBaseID      string
    WorkflowID           string
    Parameters           GenerationParameters
    Budget               Budget
    Stream               bool
}

type GenerationParameters struct {
    Temperature      *float64
    MaxOutputTokens  *int
    ReasoningEffort  string
}
```

`Purpose` 表示本次执行场景（`default`、`diagnosis`、`inspection` 或 `workflow`），用于选择 Scope 默认 Provider 和确定能力要求。`AIProviderResourceID` 可以为空；为空时按“当前级别具体 purpose -> 当前级别 `default` -> 上级级别具体 purpose -> 上级级别 `default`”解析，不为空时只使用指定 Provider，不要求该 Provider 拥有当前场景 purpose。凭据资源中的 `purpose` 仍表示凭据用途说明，两者语义不同。

`ModelName` 是用户明确选择的模型；为空时按以下顺序解析：

```text
请求中的 ModelName
  -> AIProvider.default_model
  -> AIProvider.models[] 中唯一启用模型
```

如果 `ModelOverride` 由受控内部流程传入，仍然必须经过同样的模型存在性和能力校验。模型一旦固定，本次执行不得因为普通错误或流式中断而静默切换。

## 7. 模型参数和能力校验

AIEngine 在调用模型前生成一份不可变的选择快照：

```json
{
  "provider_resource_id": "provider-id",
  "provider_type": "openai_compatible",
  "protocol": "openai_chat_completions",
  "model_name": "deepseek-chat",
  "context_window_tokens": 128000,
  "temperature": 0.7,
  "capabilities": ["text", "tool_calling", "stream"],
  "selection_reason": "scope_default_diagnosis"
}
```

校验规则：

1. 请求温度为空时使用模型默认温度；请求温度不为空时必须满足 `temperature_mutable` 和合法范围。
2. 请求输出预算不能超过模型 `max_output_tokens`，也不能使输入和输出总量超过 `context_window_tokens`。
3. 需要工具时模型必须声明 `tool_calling`。
4. 需要 SSE 时模型必须声明 `stream`。
5. 需要图片或音频时分别要求对应多媒体能力。
6. 需要结构化结果时要求 `structured_output`。
7. 深度思考请求必须要求 `deep_thinking`，并由协议适配器决定实际的 `reasoning_effort` 参数。

能力不足属于确定性配置错误，不触发 Provider 或模型自动切换。

## 8. 执行生命周期

```text
接收 Request
  -> 校验 Actor、Scope、Purpose 和 Provider 使用权限
  -> 解析 Provider 和 Model
  -> 固定选择快照
  -> 解析上下文资源
  -> 注册 Connector、MCP、Skill、知识库和工作流工具
  -> 编排 Prompt
  -> 执行 Agent Loop
       -> 模型响应
       -> 工具调用策略检查
       -> 执行工具并记录审计
       -> 将工具结果回传模型
       -> 直到完成、取消、超时或预算耗尽
  -> 返回结果或 SSE 事件
  -> 持久化完成/失败/取消事件
```

生命周期事件至少包括：

```text
execution.started
provider.resolved
context.loaded
prompt.composed
model.started
assistant.delta
assistant.progress
assistant.answer_started
tool.requested
tool.started
tool.completed
tool.failed
model.resumed
assistant.completed
execution.completed
execution.failed
execution.cancelled
```

每个事件包含 `execution_id`、单调递增 `sequence`、事件时间、状态和脱敏 Payload。

### 8.1 Agentic Loop 与 ReAct 执行契约

AIEngine 的 AgentRunner 必须以可持续推进的 **Agentic Loop（多轮 Tool-Use Loop）** 为基本执行模型，而不是把一次模型请求和一次工具调用拼接成固定流程。每一轮都必须遵循以下状态转换：

```text
Reasoning summary（阶段性分析摘要）
  -> Action（选择一个或多个已授权工具及参数）
  -> Observation（接收工具结果、错误或超时）
  -> 重新评估目标与上下文
  -> 下一轮 Reasoning / Action
  -> 明确完成信号后 Final Answer
```

运行时要求：

1. **交替循环。** 模型每次产生工具调用后，AIEngine 必须先完成工具策略校验和执行，再把脱敏的工具结果作为下一轮模型输入；在收到工具结果之前不得生成最终完成状态。一次模型响应包含多个彼此独立的工具调用时，可以并行执行，不要求在每个工具之间插入额外模型回合；但每个调用都必须有独立的开始、完成或失败事件，并在该批调用全部返回后以一个批次级 Observation 边界进入下一轮模型决策。
2. **明确终止。** 循环只能在模型返回无待执行工具的最终回答、达到取消/超时/预算上限，或触发受控错误时结束。`execution.completed` 只能在最终回答已经产生并持久化之后发送。
3. **动态规划与纠错。** 工具错误、部分结果、空结果、权限拒绝、超时和环境状态变化都必须作为 Observation 回传模型。模型可以据此更换工具、修正参数、缩小范围、重试或明确告知无法完成；运行时不得把失败结果静默吞掉，也不得无条件沿用原计划。
4. **环境反馈优先。** 代码运行结果、编译器/测试输出、Git 状态、容器或集群状态、Connector/MCP 返回值等外部环境事实，是后续推理的主要输入。静态 Prompt 只定义角色、边界和目标，不得替代最新环境观察。Context Resolver 在资源解析阶段得到的初始快照会以脱敏、限长的 `environment_facts` 字段注入首轮输入；后续事实必须以工具 Observation 为准。
5. **长程任务。** 循环必须支持数十乃至上百次连续的“分析-行动-观察”步骤。`MaxIterations`、`MaxToolCalls`、Token、输出大小、超时和取消是硬上限；达到上限时发送可解释的预算事件和终态，不得悄悄截断或继续调用工具。
6. **可恢复性。** 每个阶段和工具步骤都要持久化事件，客户端可用事件游标续读；断线重连不得重复展示事件，也不得在没有幂等保护时重复产生工具副作用。

这里的 Reasoning summary 是面向用户的、经过脱敏和长度限制的阶段性进展（例如“正在确认可用工具”“已获得容器清单，继续查询目标容器日志”），不是模型的私有 Chain-of-Thought。AIEngine 不得要求或展示隐藏思维链、逐 token 的内部推理或敏感 Prompt；模型未提供安全可展示摘要时，运行时应发送稳定的阶段状态文本。

### 8.2 阶段性流式输出

当 `Request.Stream=true` 时，AIEngine 必须在循环过程中实时发出可消费的中间事件，而不是等所有工具完成后一次性返回。事件至少覆盖：

| 阶段 | 事件 | Payload 最低要求 |
|---|---|---|
| 执行启动 | `execution.started` | profile、执行时间 |
| 阶段变更 | `phase.changed` | `phase`、可选 `detail`、`elapsed_ms` |
| 模型回合开始 | `model.started` | iteration、目标摘要、`elapsed_ms` |
| 阶段性文本 | `assistant.delta` / `assistant.progress` | 脱敏文本、iteration、`kind`、是否最终文本、动作计数 |
| 最终回答开始 | `assistant.answer_started` | 已确认当前模型回合不包含工具调用；iteration、`final=true`、原因 |
| 工具决策 | `tool.requested` | 工具名、资源、脱敏参数、iteration、动作计数 |
| 工具执行 | `tool.started` | 工具名、资源、调用序号、`started_at`、动作计数 |
| 环境观察 | `tool.completed` / `tool.failed` | 状态、脱敏结果或错误、`duration_ms`、`elapsed_ms`、动作计数 |
| 回合继续 | `model.resumed` | iteration、观察摘要、`elapsed_ms`；并行批次包含全部工具结果 |
| 最终回答 | `assistant.completed` | 最终文本 |
| 执行终止 | `execution.completed` / `execution.failed` / `execution.cancelled` | 终态、错误/预算原因 |

事件必须按实际发生顺序写入持久化 Store 并通过 SSE 推送。前端应按 `sequence` 将阶段文本、工具决策、工具结果和下一轮分析合并为一条时间线；不得把工具记录统一移动到最终回答之后，也不得用轮询快照覆盖已经收到的更新事件。耗时工具开始前应先推送进度，完成或失败后应立即推送观察摘要，然后才允许进入下一轮模型请求。

`assistant.answer_started` 是最终回答语义边界，不等同于“第一个 token 到达”：普通 Provider 的流式 token 在工具调用被确认前可能仍属于工具决策说明，因此 AIEngine 只有在当前模型回合完整结束且确认没有函数调用时才发送该事件。它位于最终 `assistant.completed` 之前，保证消费者不会因首个增量文本误折叠执行过程；需要逐字显示的客户端仍应继续消费 `assistant.delta`。

阶段事件中涉及动作进度时，统一使用以下字段（字段缺省时按事件类型推导）：

| 字段 | 类型 | 语义 |
|---|---|---|
| `action_index` | integer | 当前动作在本次执行中的 1-based 序号 |
| `action_count` | integer | 已请求或已完成的动作总数，包含并行批次中的每个工具调用 |
| `remaining_actions` | integer/null | 运行时已知、尚未结束的动作数；未知时为 `null`，不得伪造精确值 |
| `duration_ms` | integer | 当前模型回合或工具调用的耗时；仅在完成/失败后稳定 |
| `elapsed_ms` | integer | 从 `execution.started` 到当前事件的单调耗时 |
| `iteration` | integer | 当前模型回合序号，从 1 开始 |
| `kind` | string | `analysis`、`tool_decision`、`observation`、`final` 或 `budget` |

`action_count` 和 `remaining_actions` 必须以服务端事件为准，前端不得通过本地轮询猜测。并行工具调用时，所有 `tool.requested`/`tool.started`/`tool.completed` 事件共享同一个 `iteration`，但每个调用拥有独立的 `action_index`、调用序号和耗时；只有该批次全部结束后，才能发送带汇总 Observation 的 `model.resumed`。

### 8.3 诊断页面验收契约

诊断页面订阅 `model.started`、`assistant.progress`、`assistant.delta`、`tool.*` 和 `model.resumed` 事件，并在同一条“实时执行过程”时间线上按事件 ID 展示。模型回合开始、工具决策、工具执行、Observation 和下一轮分析必须能够在页面上逐步出现；连续或同一并行批次的工具调用默认聚合为一个可折叠批次，收起时只显示最新动作、批次数量、状态和耗时，展开批次后再逐个展开单个动作的脱敏入参和出参，避免大量相同工具调用平铺占用回答区域。工具明细不得等最终回答持久化后才首次出现。页面刷新快照时须合并本地已收到的 SSE 事件，只有确认终态或新的助手消息已经持久化后才能清理实时占位内容。取消/超时应以 `execution.cancelled` 和 `diagnosis.cancelled` 结束，失败应以 `execution.failed` 和 `diagnosis.failed` 结束；成功则在报告持久化后发送 `report.ready`。

### 8.4 诊断进度叙事与前端展示契约

诊断场景的实时过程应让用户看到“当前判断 -> 正在做什么 -> 得到什么 -> 下一步为什么这样做”，而不是只看到一串工具名称。`assistant.progress` 的文本应使用自然语言说明阶段目标和决策依据；工具事件负责提供可折叠的调用明细；`model.resumed` 负责明确 Observation 已经回到模型并触发重新规划。推荐的展示形式如下（工具名和耗时仅为示例）：

```text
收到，用户反馈响应慢和超时，属于典型性能问题。我现在开始诊断流程，首先进行首次评估：获取实例/表信息、性能与 VACUUM 配置、扩展情况，同时查看慢查询和当前活动查询。

postgresql_e9483c86__getCurrentActiveQueries 已完成 982ms · 还有 4 个动作 · 总耗时 1.02s

初步评估已完成。数据库表都非常小，慢查询记录为空，当前无活动查询，说明数据量和查询本身不是直接瓶颈。接下来我并行检查连接状态、锁等待、实例指标和应用容器状态。

postgresql_e9483c86__pg_core_connection_saturation 已完成 1.6s · 还有 5 个动作 · 总耗时 2.73s

数据库侧检查结果很关键：连接利用率仅 18%，无锁等待、无阻塞查询、无慢查询，数据库本身并不是瓶颈。下一步转向应用层日志和连接池配置。
```

对应的最小事件序列为：

```text
assistant.progress  {kind:"analysis", text:"收到……开始诊断流程……", iteration:1,
                     action_count:0, remaining_actions:6, elapsed_ms:120}
tool.requested      {tool:"postgresql…getCurrentActiveQueries", action_index:1,
                     action_count:1, remaining_actions:6, iteration:1}
tool.started        {tool:"postgresql…getCurrentActiveQueries", action_index:1,
                     action_count:1, remaining_actions:6, iteration:1}
tool.completed      {tool:"postgresql…getCurrentActiveQueries", duration_ms:982,
                     elapsed_ms:1102, action_index:1, action_count:1,
                     remaining_actions:4, iteration:1}
model.resumed       {iteration:1, kind:"observation", observations:[…],
                     action_count:1, remaining_actions:4, elapsed_ms:1110}
assistant.progress  {kind:"analysis", text:"初步评估已完成……接下来并行检查……",
                     iteration:2, action_count:1, remaining_actions:5, elapsed_ms:1180}
```

文本约束：

- 可以展示阶段摘要、工具用途、环境观察摘要、耗时和动作计数，但不得展示隐藏 Chain-of-Thought、逐 token 推理、系统 Prompt、凭据或未脱敏工具参数/结果。
- 阶段摘要必须是事实性、可验证、限长的陈述；如果模型没有提供安全摘要，运行时使用稳定模板（如“正在整理工具结果并重新评估下一步”），不能用空白或虚构内容替代。
- 阶段摘要应采用自然语言，禁止“阶段 1”“阶段 2”或其他机械编号；没有新观察或路径变化时不得重复上一条摘要。工具决策摘要应与同一轮的临时流式文本去重，用户只看到一份阶段说明。
- `总耗时` 使用 `elapsed_ms`，`已完成` 使用当前工具的 `duration_ms`；两者不能互换。
- `还有 N 个动作` 仅在运行时确实知道 N 时展示；否则显示“后续动作待评估”，避免把动态规划误报成预先固定的计划。
- 工具失败时使用“返回错误，正在重新评估”一类中性叙事，并在可折叠明细中保留脱敏后的错误分类和 Observation；不得把失败吞掉后继续显示“已完成”。

## 9. 上下文、Tool Calling、Skill 和 Agent

### 9.1 长程执行上下文治理

长程诊断不能把所有历史消息和原始工具输出无限累积到下一次请求。AIEngine 对每次模型请求执行以下治理策略：

1. **上下文窗口感知。** Provider Model 的 `context_window_tokens` 和 `max_output_tokens` 会随模型解析结果传入运行时；`BeforeModelCallback` 在每轮调用前按序列化请求大小估算输入 Token，动态压缩上下文并计算本轮可用输出上限。
2. **滑动窗口与历史压缩。** 保留系统约束、初始任务和最近的完整回合；较早普通文本和工具响应被替换为压缩标记。模型调用与函数响应按成对消息保留，避免裁剪出非法的 Tool Calling 历史。
3. **Observation 压缩。** 工具结果先递归脱敏，再限制文本字段和整体字节数；错误、状态、统计等结构化字段优先保留，超大日志不会原样复制到每一轮 Prompt。
4. **结构化诊断状态。** 执行级状态维护 `confirmed_facts`、`hypotheses`、`eliminated_causes`、`open_questions` 和 `next_actions`，以短状态快照注入下一轮系统指令，减少模型反复阅读旧历史。
5. **Observation 按需回读。** 完整脱敏结果保存在执行级 Observation Store；模型只收到 `observation_id` 与摘要。具备长上下文窗口的模型可通过受控 `read_observation(observation_id, offset, limit)` 分页读取精确内容，读取动作同样计入工具预算和审计事件。
6. **分层预算。** `MaxOutputTokens` 是单轮输出上限，`context_window_tokens` 是 Provider 输入/输出总窗口，`MaxTokens` 是执行累计 Token 上限；`MaxIterations`、`MaxToolCalls`、`MaxOutputBytes` 和 `Timeout` 分别约束回合、工具、最终文本和总耗时。任一硬上限触发时发送带原因的终态事件。

这些限制用于保护 Provider 上下文容量、避免单次异常日志耗尽预算、控制成本并保证 SSE 事件和审计记录可持续写入；它们不会把长程任务降级成单轮执行，因为历史压缩与 Observation 回读为后续推理保留了可追溯路径。

AIEngine 不直接读取数据库、容器或 MCP Server。所有外部信息必须通过 Context Resolver 和 Tool Gateway：

当一次执行选择了多个资源，而这些资源暴露了同名工具时，Context Resolver 保留每个资源自己的工具实现；AgentRunner 只在模型声明侧为冲突名称生成稳定的资源限定别名（例如 `query_logs__resource_2`）。网关、审计记录、Observation 和前端时间线继续使用规范工具名及 `resource_id`，因此别名不会改变权限边界或隐藏实际调用目标。

- PostgreSQL、Redis、Kafka、Kubernetes、Prometheus、Loki 等 Connector 提供受限工具；
- 远程 MCP Server 先发现工具，再通过 MCP Service 调用；
- 工具执行前校验 Actor、Scope、资源 `resource:use`、工具白名单和策略；
- 工具执行受并发、超时和响应大小预算约束；
- Connector/MCP 返回内容被视为不可信数据，不能改变系统权限或 Prompt 规则；
- 指定 Skill 或 Agent 只负责 Prompt 和工具契约，实际模型调用仍由 AIEngine 完成；
- 知识库检索和工作流节点以受控工具形式接入，不允许绕过审计。

T05 的基础契约已经在 `backend/aiengine` 建立：`KnowledgeQuery` 强制携带
Scope 和检索词，`KnowledgeRetriever` 返回不可信知识片段及可追溯引用；`Workflow`
以版本化节点和有向边表示，写入或执行前进行节点类型、引用、重试/超时边界和 DAG
无环校验。`WorkflowRun` 使用显式的 pending/running/waiting_approval/succeeded/
failed/cancelled 状态迁移，为后续持久化快照、审批恢复和断点续跑提供状态机边界。

当前最小管理 API：

```text
POST /api/v1/knowledge-bases/{id}/search
POST /api/v1/workflows/{id}/runs
GET  /api/v1/workflows/{id}/runs
GET  /api/v1/workflow-runs/{id}
GET  /api/v1/workflow-runs/{id}/events
POST /api/v1/workflow-runs/{id}/start
POST /api/v1/workflow-runs/{id}/resume
POST /api/v1/workflow-runs/{id}/cancel
PATCH /api/v1/workflow-runs/{id}
```

KnowledgeBase 和 Workflow 本体使用 Resource API 管理；`0027_knowledge_workflow`
增加对应 Schema，并为 WorkflowRun 保存工作流版本、执行 ID、当前节点、尝试次数、
输入和状态快照。运行 API 在每次操作前重新读取 Workflow Resource 并检查资源权限，
因此不能通过猜测运行 ID 跨 Scope 访问数据。`WorkflowService` 将 Agent/Skill 节点转换为
统一 `aiengine.Request`，Tool 节点强制经过 `PolicyGateway`，Retrieval 节点强制经过
`KnowledgeRetriever`。`start` 和 `resume` 会执行节点并写入节点输出快照；节点失败按有限
重试策略终止运行，审批节点进入 `waiting_approval`，恢复后从未完成节点继续。并行节点使用
配置中的 `branches` 声明式分支，每次最多 32 个分支，并传播取消。所有工作流生命周期和
节点状态事件会写入 AIEngine 事件存储。

节点配置只允许引用 Provider/模型、资源 ID、Skill/Agent ID 或检索参数，禁止注入 Provider
地址、凭据或任意代码。Condition 节点只支持受限的 `state` 键与 `equals` 值比较。

工作流节点配置示例：

```json
{
  "id": "diagnose",
  "type": "agent",
  "name": "诊断摘要",
  "config": {
    "purpose": "diagnosis",
    "agent_profile_id": "profile-resource-id",
    "task": "根据前置节点证据生成诊断摘要",
    "context": {"resource_ids": ["postgres-resource-id"]},
    "stream": false
  }
}
```

`tool` 节点的配置为 `resource_id`、`name` 和可选 `arguments`；`retrieval` 节点的配置
为 `knowledge_base_id`、`query` 和可选 `top_k`；`parallel` 节点的配置为 `branches` 数组，
每个分支使用同样的节点契约。Agent/Skill 节点执行结果、工具输出和检索引用都会写入
WorkflowRun 的脱敏 `state.node_outputs` 快照。

## 10. 流式响应和断线续读

AIEngine 提供：

```text
POST /api/v1/ai-executions
GET  /api/v1/ai-executions/{id}/events
POST /api/v1/ai-executions/{id}/cancel
```

SSE 使用事件 `id` 作为游标，支持：

- `Last-Event-ID` 请求头；
- `after` 查询参数；
- 断线后从持久化事件表继续读取；
- 已发送事件不重复返回；
- 执行结束后发送完成或失败事件并关闭连接。

停止请求只设置执行取消信号，不删除已产生的事件和审计记录。

## 11. Tool Call 审计

每次工具调用持久化：

```text
execution_id
sequence
provider_resource_id
model_name
resource_id
tool_name
arguments
output
status
error
started_at
completed_at
duration_ms
```

审计表同时保存实际 Provider 和模型，避免在多模型配置下无法判断调用来源。
SSE 在发送终态事件后关闭连接；客户端通过 `Last-Event-ID` 或 `after` 从下一个序号继续读取，
不会重复已发送事件。

入参、出参和错误信息递归脱敏：

- `api_key`、`token`、`password`、`secret`、`authorization` 等字段替换为 `[REDACTED]`；
- 嵌套对象和数组继续递归处理；
- 明文凭据不进入事件、日志、审计和 API 响应；
- 审计查询只返回当前用户有权限查看的执行和资源。

## 12. 权限模型

第一版继续复用资源权限：

| 权限 | 作用 |
|---|---|
| `resource:read` | 查看 Provider 非敏感配置和模型目录 |
| `resource:use` | 使用 Provider 和其启用模型进行测试或 AIEngine 执行 |
| `resource:create` | 创建 AIProvider |
| `resource:update` | 修改 Provider 配置和模型目录 |
| `resource:delete` | 删除或停用 Provider |
| `diagnosis:start` | 启动诊断执行 |
| `diagnosis:read` | 查看诊断事件和审计 |
| `inspection:manage` | 配置巡检场景标签和 Scope 默认 Provider |

Scope 默认 Provider 的修改必须同时满足对应 Scope 的管理权限和 `resource:use`；下级管理员不能修改上级 Scope 的默认绑定。

## 13. HTTP API

### Provider 管理

资源 API 继续使用统一资源接口，`kind` 为 `AIProvider`。增加或明确以下接口：

```text
POST /api/v1/ai-providers/{id}/test
POST /api/v1/ai-providers/test-draft
GET  /api/v1/ai-providers/available?purpose=diagnosis&scope_id=...
```

`available` 只返回当前用户可使用、且模型能力满足该场景要求的 Provider 和已启用模型摘要；不返回地址、凭据或敏感配置。`purpose` 仅用于能力筛选和标记当前 Scope 的默认 Provider，不作为 Provider 的用途许可字段。

### Scope 默认绑定

```text
GET   /api/v1/scopes/{scopeID}/ai-provider-bindings
PUT   /api/v1/scopes/{scopeID}/ai-provider-bindings/{tag}
DELETE /api/v1/scopes/{scopeID}/ai-provider-bindings/{tag}
```

### AIEngine 执行

```text
POST /api/v1/ai-executions
GET  /api/v1/ai-executions/{executionID}/events
POST /api/v1/ai-executions/{executionID}/cancel
GET  /api/v1/ai-executions/{executionID}/tool-calls
```

业务请求只传 Provider ID、模型名称、用途、上下文和任务，不得传 `base_url`、API Key 或任意上游请求头。

## 14. 前端页面

### AI 引擎页面

当前版本暂时隐藏独立的“AI 引擎”菜单和页面。AIProvider 通过资源目录管理，AIEngine 能力由后端统一执行入口提供；AI 诊断和 Skill 页面只展示可用的 Provider/模型选择器。后续恢复独立页面时，页面应包含：

1. **模型服务商列表**：展示 Provider、服务地址摘要、场景默认标签、模型数量和健康状态。
2. **模型目录详情**：每个模型单行展示名称、温度、上下文窗口、能力标签和状态。
3. **AI 引擎能力**：展示 Agent 执行、Tool Calling、推理循环、Prompt 编排、流式响应、知识库、工作流和审计能力。
4. **Scope 默认绑定**：按 `default`、`diagnosis`、`inspection` 和 `workflow` 标签设置 Provider；同一 Scope 每个标签最多绑定一个 Provider。

### 创建模型服务商

创建流程：

1. **服务商信息**：服务商类型、协议、服务地址和凭据。
2. **模型目录**：逐行添加模型名称、温度、上下文窗口、最大输出和能力标签；添加前执行真实连接测试。
3. **默认与策略**：设置 Provider 默认模型、请求超时、并发和限流。
4. **发布总结**：展示 Provider、模型目录、能力、Scope 场景标签和连接测试结果。

AI 诊断对话框使用级联选择器：

```text
模型服务商 -> 模型
```

巡检任务使用相同的 Provider/Model 选择器，并优先选中当前 Scope 的 `inspection` 标签 Provider；用户显式选择其他有权限且能力满足的 Provider 时仍然允许执行。

## 15. 失败和切换策略

第一版默认不自动切换 Provider 或模型。以下错误直接失败并记录真实原因：

- 凭据错误；
- 模型不存在；
- 能力不满足；
- 参数非法；
- 权限不足；
- 上下文超限；
- 工具被拒绝。

如果后续启用显式故障切换，必须在 Request 或场景策略中明确候选 Provider/Model，并在事件中记录完整切换链。已经产生模型输出或工具副作用后不得静默切换。

## 16. 数据库契约

当前数据库只保留 AIProvider、Scope 场景绑定、Skill/AgentProfile 版本和
AIEngine 统一事件/工具审计表。Skill 不拥有执行表；历史版本中的旧 AI 资源和
Skill 专属执行表由硬切迁移删除，运行时不再读取或写入这些结构。

一次 AIEngine 执行的生命周期事件写入 `ai_execution_events`，每次工具调用写入
`ai_execution_tool_calls`。事件和工具审计都保存最终 Provider、模型、状态、耗时和
错误信息，并对入参/出参递归脱敏。

## 17. 非功能要求

- **可解释性：** 每次执行固定 Provider 和模型，并保存选择快照。
- **安全性：** Provider 地址和凭据不能由业务请求覆盖，所有敏感值递归脱敏。
- **隔离性：** 每次执行重新校验 Actor、Scope、Provider、模型和上下文资源权限。
- **可恢复性：** SSE 事件持久化，客户端可按游标续读。
- **可控性：** 执行支持最大迭代、最大工具调用、最大 Token、最大输出、超时和取消。
- **可观测性：** 记录执行状态、模型、工具、耗时、错误分类和 Token 使用量。
- **扩展性：** Provider 协议通过适配器扩展，模型能力使用集合扩展，不修改表结构。
- **一致性：** 同一模型在页面测试、诊断、Skill 和巡检中使用统一 Provider Adapter。

## 18. 第一版验收标准

1. 可以创建一个带多个模型的 AIProvider，每个模型具有独立温度、上下文窗口和能力集合。
2. AIEngine 可以按显式 Provider/Model 或 Scope 默认绑定执行。
3. AI 诊断和自动巡检未显式选择 Provider 时使用对应场景标签的默认 Provider；显式选择时只要求有权限且模型具备所需能力。
4. AIEngine 可以自动加载授权的数据库、Connector 和 MCP 上下文，并通过 Tool Gateway 调用工具。
5. 工具调用全过程记录入参、出参、状态、耗时和错误，审计结果完成递归脱敏。
6. 同步、SSE、取消、超时、预算、失败原因和断线续读测试通过。
7. 执行记录能够明确回答“使用了哪个 Provider 的哪个模型”。
8. 业务 API、前端和后台 Worker 均只依赖 AIProvider、模型名称和 AIEngine 统一执行契约。
9. 真实或模拟模型执行能够完成至少两轮“Reasoning summary -> Action -> Observation -> 下一轮 Reasoning”，并证明工具结果进入了下一轮模型输入。
10. 工具调用前后和每轮模型回合均能通过 SSE 实时收到有序阶段事件；前端按事件序列展示分析摘要、工具调用、工具结果和后续分析，不把工具过程延迟到最终回答之后。
11. 工具返回错误、空结果、部分结果、权限拒绝或超时后，模型能够基于 Observation 修正工具或参数，或输出明确的不可完成原因；错误不得被静默忽略。
12. 在配置的迭代、工具、Token、输出大小和时间预算内，执行可持续完成长程任务；达到任一硬上限时返回明确的预算终止事件和原因。
13. 执行过程中产生的环境反馈（命令、测试、编译、Git、容器、集群、Connector/MCP 结果）可追溯到对应 Observation，并参与后续模型决策。
14. 事件断线续读、取消和失败恢复测试通过；重连不重复展示事件，且不会在无幂等保护时重复产生工具副作用。
15. 诊断流式事件能够呈现“阶段摘要 -> 工具动作 -> Observation -> 下一轮分析”的连续叙事；每个动作提供稳定的动作序号、完成耗时和执行总耗时，剩余动作未知时明确标记为待评估而不是猜测。
16. 诊断前端能够按事件 `sequence` 实时展示阶段性文本、并行工具动作、成功/失败 Observation 和纠错后的下一步；示例中的“已完成 982ms、还有 N 个动作、总耗时”均来自服务端事件字段，不由客户端拼接或轮询推断。
