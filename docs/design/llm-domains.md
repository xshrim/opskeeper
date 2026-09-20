# 大模型领域对象

## 状态

本文描述当前实现。I005 的技能增强和智能巡检设计暂缓，不属于本次领域拆分交付范围。

## 边界

Provider（厂商）、Skill（技能）和 Persona（专家）是独立于通用 Resource 的领域对象；Engine（引擎）是统一的大模型运行时，不是可配置的 Resource。

- Provider 保存上游模型服务配置、模型目录和加密凭据。
- Skill 保存可版本化的执行计划、输入输出契约和工具白名单。
- Persona 保存可版本化的专家指令和输出契约。
- Engine 统一负责模型调用、上下文、工具调用、预算、取消、流式事件和审计。

Provider、Skill、Persona 都使用 `scope_id` 归属平台、团队或项目，并沿用 Scope RBAC 的继承和可见性规则；不增加专用 `resource_level` 字段，也不把级别或归属编码进 identifier。维护人使用具体用户标识字段。

通用资源页面不包含 Provider、Skill 或 Persona。历史资源行由 0067 硬切迁移清理；新的管理入口分别位于“大模型”（渠道、引擎）、“技能”和“专家”页面。

## 权限

独立领域对象使用独立权限前缀：

| 对象 | 读取 | 管理 | 使用/执行 |
| --- | --- | --- | --- |
| Engine | `engine:read` | `engine:manage` | `engine:use` |
| Provider | `provider:read` | `provider:manage` | `provider:use` |
| Skill | `skill:read` | `skill:manage` | `skill:execute` |
| Persona | `persona:read` | `persona:manage` | `persona:use` |

首版权限仍由平台、团队、项目角色通过 Scope 继承授予，不新增对象级授权表。版本记录从属于各自的独立对象，执行时固定已发布版本并写入 Engine 审计。

## API 与数据

独立目录接口为 `/api/v1/engines`、`/api/v1/providers`、`/api/v1/skills` 和 `/api/v1/personas`；版本接口分别使用对应对象 ID。数据库中的独立主表为 `engines`、`providers`、`skills` 和 `personas`，绑定表为 `engine_scope_bindings`、`provider_scope_bindings`，版本表为 `skill_versions` 和 `persona_versions`。

## 大模型工作区

生产页面采用“引擎分层目录 + 厂商详情抽屉”组合布局。引擎位于页面上方，厂商位于下方；厂商抽屉同时承担详情查看和编辑，不再区分两个页面模式。引擎和厂商都支持图标选择。

引擎与厂商的场景标签固定为 `general`（通用）、`diagnosis`（诊断）、`inspection`（巡检）和 `workflow`（工作流）。数据库和 API 仍使用稳定的 `tag` 字段及英文值，中文“场景标签”仅作为用户界面和文档术语。每个 scope 内同一标签只有一个默认绑定，子 scope 查询时优先使用本级绑定，没有本级绑定则继承父 scope；没有场景专用绑定时由 `general` 绑定兜底。绑定只能由对应 scope 的管理权限写入。Provider 的模型能力也使用固定标签列表（文本、流式、工具、结构化输出、推理、向量、音频、视觉、生图），Provider 能力取默认模型能力。结构化输出是模型的基础契约能力，不作为场景默认标签要求；“生图”使用稳定值 `image_generation`。

Engine 内置能力词条使用中文 Agent 能力术语：智能体循环、上下文编排、工具调用、工具网关、技能编排、专家路由、结构化输出、检索增强、工作流编排和流式事件。能力词条目录允许由有权限的管理员持续扩展。

## Engine 运行时适配现状

当前项目已经有一层通用运行时契约：`engine.Engine` 负责统一的执行、流式事件和取消接口，`engine.Runner`/`StreamingRunner` 负责一次执行，`ModelBuilder` 将 Provider/Model 解析隔离在 `llm` 包之外。这样可以让诊断、技能、巡检和工作流只依赖统一的 Engine，不直接依赖某个厂商 SDK。

但当前还不是“可选择多个 Agent 框架”的完整实现。生产实例目前只有一个 `Runtime`，其默认 `AgentRunner` 将 Google ADK Go 的 Agent Loop、工具调用、流式响应和结构化输出直接接入 `backend/engine/runner.go`。Provider 层已经可以适配多个模型厂商和 OpenAI-compatible 接口，但这属于模型接入多样化，不等于 Agent 引擎框架可切换。

后续如果需要多引擎，建议保持上层 `engine.Engine` 契约不变，增加 Engine Adapter/Factory：每个实际框架实现统一的 `Runner` 和 `StreamingRunner`，由 Engine 配置选择 `adk`、`eino`、`ingenimax` 或其他适配器；工具策略、Scope 权限、上下文解析、预算、审计和事件格式仍由 OpsKeeper 外层控制，避免被某个框架重新接管。四个候选框架、整体迁移与双引擎共存的详细评估见 [Engine 多 Agent Runtime 候选方案](ai-engine.md#19-多-agent-runtime-候选方案)。

该多引擎方案已经完成设计确认，但当前暂缓实施：不新增 Runtime Adapter，不引入候选框架依赖，不修改执行数据库结构，也不改变现有 ADK 执行链路。启动实施时应先完成契约冻结和 ADK Adapter 化，再进行 Ingenimax 验证。

## Go 生态候选

| 项目 | 类型 | 与当前 Engine 的关系 | 结论 |
| --- | --- | --- | --- |
| Google ADK Go | Agent 框架 | 当前已经使用，且与 Google GenAI/ADK 模型接口衔接 | 继续作为首个内置适配器 |
| CloudWeGo Eino | Go 原生 Agent/Graph 框架 | 可作为第二个 Agent Adapter，适合图编排、工具和多 Agent | 最值得优先评估的替代框架 |
| LangChainGo | Go Agent/Chain/RAG 框架 | 能覆盖链、工具、检索和 Agent，但需要额外收敛其运行状态与事件模型 | 适合快速验证和集成较多第三方组件 |
| Google Genkit Go | Flow/Tool/Model 应用框架 | 更偏应用流程和可观测性，可作为轻量流程型适配器 | 适合工作流场景，不宜直接替代核心 Agent Loop |
| OpenAI Go SDK | 厂商模型 SDK | 负责 Responses/Chat 等模型 API，不提供 OpsKeeper 所需的完整 Agent 运行时 | 作为 Provider Adapter，不作为 Engine 框架 |
| Anthropic Go SDK | 厂商模型 SDK | 负责 Claude Messages 等模型 API，不负责 Scope、工具策略和审计 | 作为 Provider Adapter，不作为 Engine 框架 |
| Vercel AI SDK | TypeScript/JavaScript SDK | 主要面向 JS/TS 服务和前端流式协议，暂无同等定位的官方 Go Agent Runtime | 不建议作为 Go Engine 依赖，可参考其流式协议 |

因此，当前答案是：项目已经支持“统一 Engine 抽象下接入多个模型 Provider”，但尚未支持“运行时可配置地选择多个 Agent 框架”。后续落地多引擎时，ADK Go 继续作为默认和兼容基线；Ingenimax/agent-sdk-go 在版本和行为稳定后优先作为功能型第二适配器；Eino 作为 Go 原生图编排候选，Microsoft Agent Framework Go 作为工作流、检查点和人工审批候选。OpenAI 和 Anthropic 应放在 Provider/Model 适配层，而不是与这些 Agent 框架并列的 Engine Runtime。集成应采用 `ai-engine.md` 第 19 节定义的 Adapter-first 方案，不允许不同框架各自接管权限、工具审计或执行主状态。

Provider 的创建和更新接口为 `POST/PATCH /api/v1/providers`，凭据单独加密保存；Engine 的创建和更新接口为 `POST/PATCH /api/v1/engines`。

统一运行时审计表为 `engine_execution_events` 和 `engine_execution_tool_calls`。历史迁移文件和已经应用的迁移名称保留原名，0060 迁移负责把运行数据和版本关系迁移到独立领域表。
