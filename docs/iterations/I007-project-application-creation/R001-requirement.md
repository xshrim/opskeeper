# 项目与应用配置模型重构

## 目标

将项目身份配置与应用运行时绑定解耦，建立一个可以在新增项目和既有项目中复用的应用向导。

## 规则

1. 项目只填写所属团队、名称、编号、描述和图标，不区分项目来源；编号为用户显式填写的必填字段，描述作为项目正式字段保存。
2. 新增项目是一个独立配置页面，下方维护待创建的项目应用列表。
3. 添加应用向导先选择 Host、Docker 或 Kubernetes，再完成应用和实例绑定。
4. Host、Docker 支持多实例逐个资源绑定；Kubernetes 读取工作负载并批量生成应用及实例，应用名可以覆盖。
5. 这一迭代不配置服务依赖、应用依赖或其他资源关联。

## 数据约束

应用必须保存 `runtime_kind`。应用实例的目标资源类型必须与应用运行时一致，且 selector 必须包含 Host 的 `process_keyword`、Docker 的 `container_name`，或 Kubernetes 的 `namespace`、`workload_kind`、`workload_name`。
