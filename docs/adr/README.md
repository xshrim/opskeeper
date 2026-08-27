# 架构决策记录

本目录保存影响长期技术方向的架构决策记录（ADR）。每份记录使用 `NNNN-short-title.md` 命名，说明背景、候选方案、最终决定及其影响；已被替代的记录保留原文并标明状态。

## 决策索引

| 编号 | 决策 | 状态 |
|---|---|---|
| [0001](0001-package-by-feature.md) | Go 后端采用模块化单体和按业务特性组织 | 已接受 |
| [0002](0002-embedded-web-single-image.md) | 前后端源码分离，生产时嵌入 Web 并使用单镜像 | 已接受 |
| [0003](0003-controlled-database-migrations.md) | 使用内嵌迁移器和独立 Migration Job | 已接受 |
| [0004](0004-fixed-service-names-and-base-path.md) | 固定服务名称，仅配置 HTTP Base Path | 已接受 |
| [0005](0005-identity-session-baseline.md) | 本地身份使用 Argon2id 和不透明双 Token 会话 | 已接受 |
| [0006](0006-rbac-scope-filtering-baseline.md) | T04 采用内置角色、向下继承和服务端 Scope 过滤 | 已接受 |
| [0007](0007-access-management-audit-baseline.md) | T05 使用 Scope 组、权限子集授权、revision 缓存和追加式审计 | 已接受 |
| [0008](0008-resource-catalog-and-relation-boundaries.md) | T06 使用统一资源目录、Scope 关系约束和密文凭据边界 | 已接受 |
| [0009](0009-kubernetes-project-application-and-resource-rbac.md) | Kubernetes 映射到 Project/Application 并使用通用资源级授权 | 已接受 |
| [0010](0010-connector-capability-and-runtime-boundary.md) | Connector 使用能力接口、资源配置和受控运行边界 | 已接受 |
| [0011](0011-adk-and-mcp-runtime-dependencies.md) | Agent、Runner 与 MCP 使用指定官方运行时 | 已接受 |
| [0012](0012-mcp-approval-and-sandbox-boundary.md) | MCP、操作审批与自定义代码隔离边界 | 已接受 |
| [0013](0013-ai-model-and-internal-llm-endpoint.md) | AIModel 作为三级资源，大模型节点（LLMEndpoint）作为内部模型连接 | 已接受 |

现行架构事实仍以 [`design/`](../design/) 为准。ADR 解释为什么形成当前决策，不替代操作指南、工程规范或实施计划。

## 状态约定

- `提议`：正在评估，尚不能作为实现依据。
- `已接受`：当前有效决策。
- `已弃用`：不再建议用于新实现，但历史系统可能仍依赖。
- `已替代`：由另一份 ADR 取代，并保留链接关系。

新增 ADR 只能记录会长期约束多个模块、构建发布或运维边界的决定。局部重构、命名调整和可由代码直接看出的实现细节不创建 ADR。
