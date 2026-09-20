# 项目概览

OpsKeeper 是面向 Kubernetes 业务应用和各类中间件的 AI 运维值守平台。系统以平台、团队、项目三级 Scope 组织用户、资源和权限，提供资源目录、证据驱动诊断、自动巡检和审计能力。

本文只描述项目级目标和边界。当前实现事实以 `design/` 下的设计文档和代码为准；迭代范围、任务状态和验收结果以 `iterations/` 下对应迭代文档为准。

## 文档入口

- 项目设计：[总体架构](architecture.md)、[授权设计](authorization.md)、[资源模型](resource-model.md)、[统一资源接入](resource-access.md)、[大模型领域对象](llm-domains.md)、[Engine 设计](ai-engine.md)、[图标系统](icon-system.md)
- 当前计划：[I005 Skill 与智能巡检](../iterations/I005-skill-intelligent-inspection/iteration.md)（技能正文和智能巡检设计暂缓）
- 跨迭代事项：[backlog](../backlog.md)
- 工程约束：[文档规范](../standards/documentation.md)、[Go 规范](../standards/go-coding-conventions.md)、[版本控制规范](../standards/version-control.md)
