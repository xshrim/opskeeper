# ADR-0013：AIEngine 作为三级资源，LLMEndpoint 作为内部模型连接

- 状态：已接受
- 日期：2026-08-21

## 背景

用户在对话、诊断、Skill 和巡检中不应分别理解 Provider、模型名称、凭据和路由规则。将 Provider 和 Model 作为两个独立的用户选择对象，会导致连接测试、权限、默认值和审计都需要临时组合；把复杂动态路由直接引入第一版，又会增加不可解释的运行行为和维护成本。

系统需要一个统一的用户选择入口，同时保留平台、团队、项目三级 Scope、资源授权和后台任务能力。

## 决策

1. 废除面向用户的 `LLMProvider` 资源概念。Provider 类型、Base URL、模型名称、凭据、能力和上下文窗口组合为内部 `LLMEndpoint`。
2. `LLMEndpoint` 不作为独立资源展示，也不作为对话、诊断或 Skill 的选择项；它是 AIEngine 的内部可调用成员，可独立测试。
3. 新增 `AIEngine` 资源，使用平台、团队、项目三级 Scope。用户、业务场景和后台任务统一选择 AIEngine。
4. AIEngine 不设置单模态或多模态子类，统一作为能力入口。AIEngine 可以包含一个或多个 LLMEndpoint，第一版只支持固定 `priority` 顺序，以及 timeout、rate limit、server error 的受限故障转移。
5. AIEngine 能力取启用 Endpoint 的交集，上下文窗口取最小值；能力意图仅用于创建时表达业务需要，系统不得将其提升为 Endpoint 未提供的能力。
6. AIEngine 和 Skill 都是资源，但在前端分别提供“AI 引擎”和“Skill”侧边栏一级菜单；通用资源目录提供统一治理入口。
7. 所有执行记录保存请求的 AIEngine 和最终实际 Endpoint/模型，不能只记录 AIEngine ID。

## 影响

- 普通用户只需要选择 AIEngine；单模型和多模型配置具有相同的调用协议。
- LLMEndpoint 的内部化降低了资源目录复杂度，但要求服务端在保存和调用时持续校验 Endpoint Scope 和 AIEngine Scope。
- `LLMProvider` 旧数据需要迁移为单成员 AIEngine，旧默认值在兼容期只读解析，不再允许新写入。
- 第一版不具备质量评分、成本路由和学习型选择；如后续增加，必须保持硬约束过滤、会话模型粘性和可审计选择原因。

## 不变量

- 下级 AIEngine 只能引用自身 Scope 或祖先 Scope 的 Endpoint；兄弟和下级引用禁止。
- 一个 LLMEndpoint 只属于一个 AIEngine；跨 AIEngine 只能复制配置，不能共享内部对象或凭据引用。
- `resource:use` 不自动授予 `resource:read`，凭据永远不通过普通资源响应返回。
- 流式输出开始后不得切换模型；已发生不可确认幂等 Tool Call 后不得自动重放。
- Skill Runner、诊断编排和巡检仍通过 OpsKeeper Policy Enforcement 调用 ADK，不由 AIEngine 绕过工具授权或审批。
