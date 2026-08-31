# I003 AIEngine 执行框架迭代

**迭代编号：** I003  
**目录标识：** I003-ai-engine  
**状态：** 待封板
**计划开始：** 待定  
**计划封板：** 待定

## 1. 迭代目标

在现有 AIProvider、Connector、MCP、Skill、AI 诊断和自动巡检能力之上，建立统一的 `AIEngine` 执行框架。所有需要大模型能力的业务都通过 AIEngine 执行 Agent、Tool Calling、推理循环、Prompt 编排、流式响应、知识库检索和工作流编排；业务层只选择 AIProvider 和模型，不直接调用 Provider 地址或凭证。

AIEngine 必须同时支持交互式 AI 对话/故障诊断和后台自动监控巡检，并具备可恢复、可观测、可审计的执行生命周期。

## 2. 迭代范围

- 建立统一 AIEngine 执行内核和执行 Profile；
- 将现有 ADK Agent、Skill Plan Resolver、Diagnosis Orchestrator 和 Inspection AI 扩展点接入统一执行链；
- 根据用户选择的上下文资源自动建立受控工具集合，并主动采集基础资源信息；
- 将 Connector 和远程 MCP Server 工具纳入统一 Tool Gateway；
- 支持指定 Skill、指定专家 Agent 以及 Skill + Agent 组合；
- 建立知识库检索、引用和权限过滤能力；
- 建立可持久化的工作流/DAG 编排和断点恢复能力；
- 通过流式事件记录模型输出、工具调用、工作流步骤和错误；
- 全程记录工具调用入参、出参、状态、耗时和错误，并执行敏感信息脱敏；
- 将 AI 对话、故障诊断、Skill 执行和自动巡检统一到 `ai_provider_resource_id + model_name` 调用入口。

不在本迭代范围内：开放式模型自主编写任意 HTTP 请求；绕过 Resource/RBAC/Policy Enforcement 的工具执行；未经审批的高风险写操作；基于价格或质量反馈的动态模型评分；把大模型节点暴露为独立业务资源。

## 3. 需求清单

| 需求 | 名称 | 任务 | 状态 | 需求文档 | 验收报告 |
|---|---|---:|---|---|---|
| R001 | AIEngine 执行框架设计与实现 | T01-T06 | 已完成 | [R001-requirement.md](R001-requirement.md) | [R001-requirement-acceptance.md](R001-requirement-acceptance.md) |

## 4. 任务总览

| 任务 | 名称 | 目标 | 依赖 | 状态 |
|---|---|---|---|---|
| T01 | AIEngine 统一执行内核 | 统一 Request、Result、Event、Agent Loop、预算、取消和执行 Profile | 无 | 已完成 |
| T02 | 上下文工具层 | 自动解析上下文资源，接入 Connector 和远程 MCP Tool Gateway | T01 | 已完成 |
| T03 | Skill 与 Agent Profile | 支持指定 Skill、专家 Agent 和组合执行 | T01-T02 | 已完成 |
| T04 | 流式事件与工具调用审计 | 完整记录流式输出、工具入参出参、错误、耗时和恢复 | T01-T03 | 已完成 |
| T05 | 知识库与工作流编排 | 支持知识检索、引用、DAG、并行、审批和断点恢复 | T01-T04 | 已完成 |
| T06 | AI 诊断页面接入 AIEngine | 前端可选择 AIProvider/模型，通过 AIEngine 完成真实诊断并展示状态、证据和错误 | T01-T04 | 已完成 |

## 5. 进入条件

- I001 已封板，I002 的 AIEndpoint、AI 诊断和主题相关改动已合并到 `main`；
- 当前 AIProvider 作为模型选择入口，AIEngine 负责场景默认绑定、能力校验和执行固定；
- 现有 Connector、MCP、Skill、Diagnosis、Inspection 和审计边界已完成盘点；
- 每个任务开始前必须明确 API、数据模型、权限边界、失败策略和测试范围，并获得该任务批准；
- 任何新增工具默认只读，写操作必须通过既有受控操作审批链。

## 6. 退出条件

- T01-T06 全部完成验收，或明确记录未完成项及转移迭代；
- 对话、诊断、Skill 和巡检都可以使用同一个 AIEngine 入口；
- 选择 PostgreSQL、Kubernetes、监控平台或远程 MCP Server 资源后，AIEngine 可以自动建立并调用受控工具；
- 指定 Skill、Agent 和知识库后，执行记录能够追溯完整的 Prompt、工具、证据和最终模型节点；
- 工具调用入参出参经过脱敏、大小限制和权限控制后可审计查看；
- 流式执行支持取消、失败原因、断线续读和执行快照；
- 工作流支持节点状态、重试、并行、条件、人工确认和断点恢复；
- 后端单元/集成测试、API 测试和必要的前端 Playwright 场景通过。

## 7. 封板记录

待迭代完成后填写。
