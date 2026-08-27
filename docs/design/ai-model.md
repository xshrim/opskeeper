# AIModel 与大模型节点（LLMEndpoint）设计

## 文档状态

本文是 AIModel 功能的目标设计。当前仓库已有的 `LLMProvider` 实现和 `provider_resource_id + model_name` 调用字段属于历史实现；新功能实施时按本文迁移，不在新代码中继续扩展 `LLMProvider`。本设计不要求第一版实现动态评分、机器学习路由、价格路由或复杂灰度发布。

### 术语约定

本文及产品界面统一将 `LLMEndpoint` 的中文名称写作“**大模型节点**”。`LLMEndpoint`、`Endpoint`、`endpoint`、`llm_endpoints` 等英文标识仅用于代码、API、数据库和协议字段，不随界面文案改名。

## 1. 目标与边界

用户在对话、AI 诊断、Skill、巡检和其他需要大模型的场景中只选择一个 AIModel。AIModel 可以只包含一个模型连接，也可以包含多个模型连接；调用者不需要理解 Provider、API 地址、凭据或具体模型名称。

本设计解决以下问题：

- 将 Provider、模型名称、凭据和连接参数合并为一个可调用、可测试的内部对象；
- 让平台、团队和项目可以按 Scope 管理可复用的 AIModel；
- 用固定优先级和受限故障转移提供简单、可解释的模型聚合；
- 让单模型和多模型使用经过同一条调用链；
- 保留实际选中模型、故障转移原因和权限判断的审计证据。

本设计明确不包含：

- 让另一个大模型决定使用哪个大模型；
- 基于价格、质量或历史反馈的动态评分；
- 将每个具体模型暴露为独立的三级资源；
- 将 AIModel 设计成用户私有、绕过 Scope 的临时配置；
- 让 LLM 直接访问基础设施或绕过 Skill Runner、Policy Enforcement 和审计。

## 2. 核心概念

```text
AIModel Resource (三级 Scope 资源)
    ├── 大模型节点（LLMEndpoint，内部对象）
    ├── 大模型节点（LLMEndpoint，内部对象）
    └── 大模型节点（LLMEndpoint，内部对象）

对话 / 诊断 / Skill / 巡检
              ↓
       ai_model_resource_id
              ↓
        ResolveAIModel
              ↓
        选择大模型节点
              ↓
          BuildModel
              ↓
       ADK Agent / Runner
```

### 2.1 AIModel

AIModel 是用户真正选择的能力入口，也是统一资源目录中的一种资源：

- 有平台、团队或项目 Scope 归属；
- 有名称、描述、状态、标签和生命周期；
- 使用通用 `resource:read`、`resource:use`、`resource:update`、`resource:delete` 权限；
- 可以配置为某个 Scope 的默认大模型能力；
- 可以被对话、诊断、Skill、巡检和后台执行引用；
- 可以被连接测试、审计和资源关系引用。

AIModel 的用户界面是侧边栏一级菜单，而不是要求用户在通用资源表单中编辑复杂的模型成员。资源目录仍然可以展示和筛选 `AIModel`，但点击编辑时进入专用 AIModel 页面。

#### 统一能力契约

资源页面将原“大模型”类型改名为“AI 模型”，不再设置“单模态”和“多模态”子类。AIModel 是统一的调用入口，输入媒介和调用能力由启用大模型节点的实际能力交集决定。

`tool_calling`、`structured_output`、`vision`、`audio`、`stream`、`long_context`、`deep_thinking` 和上下文窗口都是可叠加能力字段。模型参数包含温度，默认值为 `0.7`，范围为 `0` 到 `2`。用户可以在创建向导中选择业务需要的能力意图，但不能手工伪造最终能力；服务端在连接测试和保存时根据成员交集校验。

一个只包含文本大模型节点的 AIModel 和一个同时包含文本、视觉或音频大模型节点的 AIModel 使用同一套资源、权限、路由和审计模型。诊断、巡检和 Skill 是业务调用方，也不单独建子类。

### 2.2 大模型节点（LLMEndpoint）

大模型节点是 AIModel 内部的最小可调用对象，不作为独立资源类型展示，不直接出现在对话或诊断的选择器中。一个大模型节点（LLMEndpoint）始终包含一个完整的 Provider + Model + Credential 组合：

```text
LLMEndpoint {
    id
    ai_model_id
    provider_type
    base_url
    model_name
    credential_id
    context_window
    capabilities
    timeout_seconds
    status
    last_test_result
}
```

其中：

- `provider_type` 支持 `openai` 和 `openai_compatible` 等适配器类型；
- `base_url` 是非敏感连接地址，必须通过对应 Provider 适配器校验；
- `model_name` 是该大模型节点的唯一模型名称；
- `credential_id` 只引用加密凭据，API、日志、模型上下文和前端不返回凭据明文；
- `context_window` 是该模型可接受的最大上下文窗口；
- `temperature` 是模型采样温度，默认值为 `0.7`，允许范围为 `0` 到 `2`；
- `capabilities` 是服务端声明并通过测试验证的能力集合；
- 不保存 `input_price` 和 `output_price`，第一版不做成本路由；
- `status` 至少包括 `active`、`disabled` 和 `unavailable`。

一个大模型节点（LLMEndpoint）只属于一个 AIModel，不允许被多个 AIModel 共享。用户在另一个 AIModel 中添加相同 Provider、模型和凭据时，系统创建新的大模型节点副本；“从已有模型连接复制”只复制配置，不建立共享引用。修改某个 AIModel 内的大模型节点只影响该 AIModel 的使用者，不会隐式改变其他 AIModel。大模型节点的凭据也随副本独立保存或重新绑定，避免 API Token 的隐式共享。

## 3. AIModel 配置

AIModel 的非敏感资源配置建议为：

```json
{
  "strategy": "priority",
  "default": false,
  "endpoints": [
    {
      "endpoint_id": "endpoint-a",
      "priority": 100,
      "enabled": true
    },
    {
      "endpoint_id": "endpoint-b",
      "priority": 80,
      "enabled": true
    }
  ],
  "fallback_on": [
    "timeout",
    "rate_limit",
    "server_error"
  ]
}
```

### 3.1 默认 AIModel

每个平台、团队和项目 Scope 最多只能有一个默认 AIModel。`default` 是资源配置中的布尔字段：启用某个模型的默认开关时，服务端在同一 Scope 内以事务方式先取消其他 AIModel 的默认状态，再设置当前模型；关闭开关则该 Scope 暂时没有默认模型。平台、团队和项目分别维护自己的默认值，父级默认值不会覆盖子级默认值。

默认开关只能由对应 Scope 的管理员操作：平台管理员、团队管理员或项目管理员。普通资源管理员、操作员和查看者即使可以查看或编辑模型连接，也不能改变默认模型。默认模型必须处于启用状态；停用模型前应先取消默认状态。

创建向导在基础信息步骤提供“默认模型”开关，列表中以独立的开关列展示当前状态。所有默认变更都写入审计日志，业务调用只读取最终解析出的 Scope 默认模型，不直接选择大模型节点。

### 3.2 内置模型厂商目录

添加模型步骤使用内置模型厂商目录。下拉菜单以图标和名称展示以下厂商：OpenAI 兼容、OpenAI、Anthropic、Gemini、Grok、DeepSeek、Qwen、Kimi、GLM、MiniMax、MiMo、LongCat、Doubao、OpenRouter、SiliconFlow 和 Ollama。选择厂商后自动填充对应的默认模型地址，用户仍可按实际部署修改地址。厂商选择只提供连接预设，模型连接测试仍以实际大模型节点响应为准。

第一版只支持 `priority` 策略：优先级数值越大越先调用；优先级相同时使用稳定的 `Endpoint ID` 排序，避免配置顺序变化造成不可解释的选择。

约束：

- 至少包含一个大模型节点；
- `Endpoint ID` 不能重复；
- `priority` 为有限整数；
- 禁用所有成员的 AIModel 不可发布或不可设置为默认；
- `fallback_on` 只允许 `timeout`、`rate_limit`、`server_error`；
- 不允许通过配置传入任意 URL、HTTP 方法、请求头或模型协议命令；
- AIModel 能力不由用户手工填写，而由启用成员自动计算。

一个只包含一个大模型节点的 AIModel 是合法配置。它提供统一的用户选择、权限和审计入口，同时不产生额外的故障转移行为。

## 4. 能力交集

AIModel 对外公布的能力必须是所有启用大模型节点能力的交集：

```text
capabilities(ai_model) = intersection(capabilities(endpoint_i))
context_window(ai_model) = min(context_window(endpoint_i))
```

例如：

```text
大模型节点 A: chat, stream, tool_calling, structured_output, 128000
大模型节点 B: chat, stream, structured_output, 32000

AIModel: chat, stream, structured_output, 32000
```

因此，Skill 或诊断在选择 AIModel 时只需要校验 AIModel 能力。运行时仍必须对最终选中的大模型节点再校验一次，防止成员状态、配置或测试结果在读取后发生变化。

能力至少包括：

```text
chat
stream
tool_calling
structured_output
vision
long_context
```

能力声明不是无条件信任。保存大模型节点时验证结构，连接测试或首次调用时验证协议；测试失败时大模型节点进入不可用候选状态，不能继续作为健康成员使用。

## 5. Scope 与共享规则

AIModel 使用现有三级 Scope。由于大模型节点是 AIModel 的私有子对象，大模型节点不再有独立 Scope，也不能作为跨资源引用主体：

| AIModel Scope | 内部大模型节点来源 |
|---|---|
| 平台 | 该平台 AIModel 自己创建的大模型节点 |
| 团队 | 该团队 AIModel 自己创建的大模型节点 |
| 项目 | 该项目 AIModel 自己创建的大模型节点 |

上级 Scope 的 AIModel 可以被下级 Scope 使用，但下级只能使用上级 AIModel 的完整能力，不能直接读取其大模型节点，也不能把大模型节点拼装进自己的 AIModel：

```text
团队 A AIModel 不能引用团队 B 大模型节点
项目 A AIModel 不能引用项目 B 大模型节点
平台 AIModel 不能引用团队或项目大模型节点
```

保存配置、连接测试和实际调用时都要校验大模型节点确实属于目标 AIModel。大模型节点被禁用、删除或连接检查失败时，所属 AIModel 进入需要修复或不可用状态，不能静默改用另一个大模型节点。

AIModel 的共享遵循资源模型：上级 Scope 的 AIModel 可以被下级 Scope 使用，下级 AIModel 不会向上级可见。`resource:read` 和 `resource:use` 分离；能够使用 AIModel 不代表能够看到 Base URL、凭据或内部请求配置。

## 6. 路由与故障转移

```text
请求
  → 校验 AIModel resource:use
  → 读取启用的大模型节点
  → 过滤 Scope 不合法、禁用和不可用成员
  → 按 priority 排序
  → 调用第一个大模型节点
  → 成功则结束
  → 仅在允许的可恢复错误上尝试下一个大模型节点
```

可以自动切换的错误：

- 连接超时；
- Provider 限流；
- Provider 暂时不可用或 HTTP 5xx；
- 尚未向客户端发送任何模型输出时的连接建立失败。

不可自动切换的错误：

- 用户输入或参数校验失败；
- 权限不足；
- AIModel 能力不足；
- 大模型节点配置或凭据错误；
- 已经开始流式输出；
- 已经执行了不可确认幂等性的 Tool Call 或高风险操作。

一次执行固定一个最终大模型节点。切换只发生在调用尚未产生不可逆外部效果时；不能把两个模型的流式结果拼接成一个响应。每次切换都记录原因、候选顺序和最终结果。

## 7. 统一调用协议

所有使用大模型的业务场景只接收：

```text
ai_model_resource_id
```

不再向业务层暴露 `provider_resource_id` 或 `model_name`。统一解析接口可以抽象为：

```go
type ResolvedLLM struct {
    AIModelResourceID string
    EndpointID        string
    ProviderType      string
    ModelName         string
    APIKey            string
    Capabilities      []string
    ContextWindow     int
    FallbackUsed      bool
    FallbackReason    string
}

ResolveAIModel(ctx, scopeID, modelResourceID, requirements) (ResolvedLLM, error)
BuildModel(ctx, ResolvedLLM) (model.LLM, error)
```

调用链：

```text
对话 / 诊断 / Skill / 巡检
    → ai_model_resource_id
    → ResolveAIModel
    → ResolvedLLM
    → BuildModel
    → ADK Agent / Runner
```

Skill 的 `requirements` 来自 SkillVersion 声明；诊断、对话和巡检可以声明 `tool_calling`、`structured_output`、上下文窗口和流式需求。若 AIModel 交集能力不满足要求，应在调用前失败，而不是调用第一个成员后才发现不支持。

## 8. 默认值

Scope 默认值指向 AIModel，而不是大模型节点（LLMEndpoint）：

```text
项目默认 AIModel > 团队默认 AIModel > 平台默认 AIModel
```

所有后台任务、巡检和没有明确用户偏好的诊断都使用 Scope 默认 AIModel。用户可以在个人设置中选择一个自己可使用的 AIModel作为交互式默认值，但个人默认值不改变 Scope 默认，也不能被后台任务引用。

调用解析顺序：

```text
显式 ai_model_resource_id
  > 用户个人默认 AIModel
  > 当前项目默认 AIModel
  > 团队默认 AIModel
  > 平台默认 AIModel
```

迁移期间可以兼容旧的 `provider_resource_id + model_name` 默认配置：系统为每个旧默认组合创建单大模型节点 AIModel，或在解析层临时包装为单成员 AIModel；新接口和新数据不再写入旧字段。

## 9. 测试、健康和审计

测试分为两层：

1. 大模型节点测试：发送最小请求，验证地址、凭据、模型名称和协议适配器可用；测试结果保存状态、延迟、错误分类和时间，不保存响应正文或凭据。
2. AIModel 测试：按优先级调用成员，验证能力交集、首选成员和允许的故障转移；测试不能修改生产对话状态，也不能执行 Tool Call。

运行时最少记录：

```text
requested_ai_model_id
selected_endpoint_id
selected_model_name
fallback_used
fallback_reason
latency_ms
prompt_tokens
completion_tokens
total_tokens
```

审计事件必须记录用户、Scope、AIModel、最终大模型节点、模型名称、是否切换和结果。日志和普通用户界面不显示 API Token、完整 Base URL 凭据、完整 Prompt 或敏感模型响应；具备审计权限的用户可以查看经过脱敏的路由元数据。

## 10. 数据边界与生命周期

AIModel 作为 `resources.kind = 'AIModel'` 保存公共元数据和非敏感配置。大模型节点（LLMEndpoint）可以先作为内部表保存：

```text
llm_endpoints {
    id,
    ai_model_id,
    provider_type,
    base_url,
    model_name,
    credential_id,
    context_window,
    capabilities,
    timeout_seconds,
    status,
    priority,
    enabled,
    last_test_status,
    last_tested_at,
    created_at,
    updated_at
}
```

由于大模型节点独占 AIModel，第一版不建立 `ai_model_endpoints` 多对多关系表。`ai_model_id`、`priority` 和 `enabled` 直接保存在 `llm_endpoints` 中。内部表不等于脱离权限：所有读取和调用仍必须先校验当前用户对 AIModel 的权限，再校验大模型节点的所属关系、启用状态和连接健康状态。

AIModel 的编辑采用更新配置并重新计算能力；实施时可以选择直接覆盖，或在需要可回滚时增加不可变发布版本。第一版不要求版本表，但已经开始被后台任务引用的执行必须保存配置快照，不能因为后续编辑而改变历史审计含义。

## 11. 前端信息架构

侧边栏提供两个一级能力入口：

```text
AI 模型
Skill
```

两者在后端都属于资源，前端都拥有专属配置页面。AI 模型页面不复制通用资源 CRUD，而是提供：

- AIModel 列表：名称、Scope、状态、能力、成员数、默认标记和最近测试结果；
- 新建/编辑 AIModel：名称、描述、Scope、能力意图、大模型节点成员、优先级和失败切换条件；
- 模型连接面板：Provider 类型、Base URL、模型名称、凭据、上下文窗口、能力和超时；
- 能力预览：显示由成员交集计算出的能力和最小上下文窗口；
- 测试入口：测试单个大模型节点、全部成员和整个 AIModel；
- 默认设置：将 AIModel 设置为当前 Scope 默认或个人默认；
- 资源治理入口：状态、权限、审计和资源目录链接。

普通用户在对话、诊断、Skill 和巡检表单中只看到有权使用的 AIModel。模型成员和实际模型名称是否显示，取决于用户的 `resource:read` 权限；调用始终只需要 `resource:use`。

通用资源目录仍然展示 AIModel，以便统一搜索、Scope 过滤、权限管理和审计；编辑操作跳转到 AI 模型专用页面。大模型节点（LLMEndpoint）不出现在通用资源目录和业务选择器中。

## 12. 实施顺序

1. 将现有 `LLMProvider` 迁移为单成员 AIModel 和内部大模型节点（LLMEndpoint），保留旧数据读取兼容层。
2. 新增 `AIModel` 资源 Schema、大模型节点内部存储和成员关系校验。
3. 实现 AIModel 能力交集、优先级调用和受限故障转移。
4. 将对话、诊断、Skill、巡检和后台默认配置统一改为 `ai_model_resource_id`。
5. 扩展执行记录和连接测试，保存最终大模型节点、模型和故障转移原因。
6. 增加 AI 模型一级菜单和专用配置页面，资源目录只提供治理入口。
7. 保留旧 Provider/Model 默认配置的只读兼容期，完成数据迁移后删除旧写入路径。

## 13. 验收标准

- AIModel 可以只包含一个或多个大模型节点（LLMEndpoint）；每个大模型节点只属于一个 AIModel；
- 大模型节点和 AIModel 都能独立测试，但用户只选择 AIModel；
- AIModel 能力严格取启用大模型节点的交集；
- AIModel 只按 priority 选择，并仅对规定的可恢复错误切换；
- 平台、团队和项目 Scope 规则不能被内部大模型节点引用绕过；
- 对话、诊断、Skill 和巡检使用同一个 `ai_model_resource_id` 解析入口；
- 执行记录包含请求的 AIModel、最终大模型节点、模型和故障转移信息；
- API、日志和普通 UI 不泄露凭据；
- AI 模型和 Skill 都作为资源治理，但各自拥有侧边栏一级菜单。
