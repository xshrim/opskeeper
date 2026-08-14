# OpsKeeper 设计文档

OpsKeeper 是面向 Kubernetes 业务应用和各类中间件的 AI 运维值守平台。本仓库当前仅包含设计文档，尚未进入代码实现阶段。

## 文档索引

- [总体架构设计](docs/architecture.md)
- [组织、资源与拓扑模型](docs/resource-model.md)
- [权限与安全设计](docs/authorization.md)
- [Kubernetes 导入、AI 诊断与自动巡检](docs/operations-workflows.md)
- [分阶段实施任务书](docs/implementation-tasks.md)
- [本地开发环境](docs/development.md)
- [Git 与远端仓库](docs/version-control.md)

## 核心设计原则

1. 平台按平台、团队、项目三级组织范围管理用户、资源和权限。
2. 资源类型不决定资源层级，资源实例可按实际共享边界归属任意层级。
3. 下级范围可引用上级资源，但不能直接引用同级其他范围或下级资源。
4. AI 诊断必须由 Skill 和受控工具获取证据，不能仅依赖模型自由生成结论。
5. 所有自动发现、诊断、巡检和变更动作均可追踪、可审计、可复现。
