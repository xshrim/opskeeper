# AIProvider + AIEngine 设计

**状态：** I003 实施版本  
**适用范围：** I003 后续 AIEngine 改造  
**最后更新：** 2026-08-29

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

### 3.4 AgentProfile

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
      "max_output_tokens": 8192,
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
| `max_output_tokens` | 否 | 模型允许的最大输出 Token |
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
model.delta
tool.started
tool.completed
tool.failed
execution.completed
execution.failed
execution.cancelled
```

每个事件包含 `execution_id`、单调递增 `sequence`、事件时间、状态和脱敏 Payload。

## 9. 上下文、Tool Calling、Skill 和 Agent

AIEngine 不直接读取数据库、容器或 MCP Server。所有外部信息必须通过 Context Resolver 和 Tool Gateway：

- PostgreSQL、Redis、Kafka、Kubernetes、Prometheus、Loki 等 Connector 提供受限工具；
- 远程 MCP Server 先发现工具，再通过 MCP Service 调用；
- 工具执行前校验 Actor、Scope、资源 `resource:use`、工具白名单和策略；
- 工具执行受并发、超时和响应大小预算约束；
- Connector/MCP 返回内容被视为不可信数据，不能改变系统权限或 Prompt 规则；
- 指定 Skill 或 Agent 只负责 Prompt 和工具契约，实际模型调用仍由 AIEngine 完成；
- 知识库检索和工作流节点以受控工具形式接入，不允许绕过审计。

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
| `endpoint:manage` | 第一版移除，不再作为运行时权限 |
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

## 16. 数据库改造

从当前 `AIEndpoint` 架构迁移到第一版时：

1. 删除或归档 `AIEndpoint` 资源和相关 Scope 默认表；
2. 将 Provider 的模型目录补齐温度、上下文窗口和能力集合；
3. 创建 `scope_ai_provider_bindings`；
4. 将默认 AIEndpoint 转换为对应 Scope 的默认 AIProvider；
5. 删除 `endpoint:manage` 权限和旧接口；
6. 更新诊断、Skill、巡检和 AIEngine 请求字段为 `ai_provider_resource_id` 与 `model_name`；
7. 清空不再兼容的旧 AI 资源数据，不保留运行时兼容分支。

由于项目已明确不保留旧 AI 资源，本迁移可以在维护窗口中执行硬切换，失败时回滚数据库迁移，不要求保留旧业务数据。

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
8. 业务 API、前端和后台 Worker 均不再依赖 `AIEndpoint`。
