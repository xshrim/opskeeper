# OpsKeeper 设计文档

OpsKeeper 是面向 Kubernetes 业务应用和各类中间件的 AI 运维值守平台。本仓库已经完成 T02 组织模型，具备 Go/Svelte 工程骨架、健康检查、PostgreSQL 迁移和三级 Scope/组织 API；T03 身份认证与三级 RBAC 待批准。

## 文档索引

- [总体架构设计](docs/architecture.md)
- [组织、资源与拓扑模型](docs/resource-model.md)
- [权限与安全设计](docs/authorization.md)
- [Kubernetes 导入、AI 诊断与自动巡检](docs/operations-workflows.md)
- [分阶段实施任务书](docs/implementation-tasks.md)
- [T01 验收记录](docs/t01-acceptance.md)
- [T02 验收记录](docs/t02-acceptance.md)
- [本地开发环境](docs/development.md)
- [Go 编码与工程组织通用规范](docs/go-coding-conventions.md)
- [Git 与远端仓库](docs/version-control.md)

## 核心设计原则

1. 平台按平台、团队、项目三级组织范围管理用户、资源和权限。
2. 资源类型不决定资源层级，资源实例可按实际共享边界归属任意层级。
3. 下级范围可引用上级资源，但不能直接引用同级其他范围或下级资源。
4. AI 诊断必须由 Skill 和受控工具获取证据，不能仅依赖模型自由生成结论。
5. 所有自动发现、诊断、巡检和变更动作均可追踪、可审计、可复现。
