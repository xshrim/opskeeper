# ADR-0009：Kubernetes 映射到 Project/Application 并使用通用资源级授权

- 状态：已接受，T08 已完成
- 日期：2026-08-16

## 背景

Kubernetes 中存在 Namespace、Workload、Pod、Service、Ingress、Endpoint 等大量内部对象。如果全部登记为独立资源，资源目录会复制 Kubernetes API，用户还需要维护并不属于业务管理核心的对象。另一方面，项目成员可能只负责项目中的某个 Application 或其他具体资源，仅使用 project Scope 角色会让其获得整个项目的资源权限。

## 决策

1. Kubernetes 集群本身是 `Kubernetes` 资源，保存非敏感连接配置并关联加密 kubeconfig。
2. Namespace 不登记为资源，发现时映射为已有或新建 Project。
3. Deployment、StatefulSet、DaemonSet、Job 和 CronJob 统一映射为 `Application` 资源，原始类型保存在 `kubernetes.workload_kind`。
4. Pod 副本映射为 Application 配置中的 `instances`；Service、Ingress 和 EndpointSlice 聚合为 Application 的访问与运行信息，不创建独立资源记录。
5. 导入使用 Kubernetes UID、来源 Kubernetes 资源、目标 Scope 和资源类型形成幂等身份；重复发现更新同一 Application，失联对象先标记 `unknown`。
6. 组织权限保持 platform、team、project 三级 Scope，不增加 application Scope。
7. 增加通用 `resource_roles` 和 `resource_role_bindings`。项目观察员默认只获得项目及祖先范围的资源可见性；具体 Application 或其他资源的使用权限由 `ResourceViewer` 或 `ResourceOperator` 显式追加，资源管理仍由 Scope 管理员或操作员角色负责。
8. Scope 角色和资源角色采用允许权限并集；拥有原有 ProjectViewer/Operator/Admin 的用户仍按整个项目授权。

## 后果

- 资源目录围绕 Project、Application 和可连接的外部能力，而不是复制 Kubernetes 对象树。
- Application 可直接为页面、AI 问答、诊断和巡检提供工作负载、Instance、Service、Ingress 和 Endpoint 上下文。
- 资源级授权适用于所有资源类型，不需要为 Application 建立特例或第四级权限层级。
- 周期调度、Kubernetes 事件、日志与实时查询属于后续 Connector、诊断和巡检任务。
