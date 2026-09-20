# 组织、资源与拓扑模型

## 文档状态

资源目录、凭据密文边界、关系约束、默认解析和有限拓扑查询已实现；资源管理控制台、Project/Application 映射、具体资源授权、Kubernetes/Prometheus/Loki Connector 和受控执行记录已实现。Provider、Engine、Skill、Persona 已从通用资源目录中拆分为独立领域对象，边界见[大模型领域对象](llm-domains.md)。Kubernetes Discovery 和受控 Operation 已由 0068 迁移移除。

## 1. 设计原则

资源的类型和作用域是两个独立维度。例如：

- 独立领域对象通常是平台级，但团队可以维护团队专属 Provider、Engine、Skill 或 Persona，项目也可拥有项目级对象。
- Kubernetes 集群通常是团队级，但公共基础设施集群可以是平台级，项目独占集群可以是项目级。
- Redis 通常是团队共享资源，但也可以是平台公共服务或项目独享实例。
- 可观测平台通常是平台级或团队级，也允许项目拥有独立监控平台。

因此资源模型统一引用 `scope_id` 表达归属，作用域类型由 Scope 节点确定，不为每种资源预设固定层级。

## 2. 作用域与组织模型

```text
scopes(id, tenant_id, scope_type, parent_scope_id, status)
platforms(id, scope_id, name, code, icon)
teams(id, scope_id, platform_id, name, code, icon, labels)
projects(id, scope_id, platform_id, team_id, name, code, icon, labels, source)
```

`Scope` 是统一的权限和资源归属节点，平台、团队、项目是它的三种业务表现。资源、角色绑定、巡检策略、诊断会话和审计记录统一引用 `scope_id`，不再分别保存多态组织外键。

```text
platform scope
└── team scope
    └── project scope
```

约束：

- 平台 Scope 没有父节点，团队 Scope 的父节点必须是平台，项目 Scope 的父节点必须是团队。
- 团队必须属于当前平台。
- 项目必须属于一个团队，并冗余 `platform_id` 方便过滤和完整性校验。
- 项目编码在团队内唯一，团队编码在平台内唯一。
- `scopes.status` 是组织启用状态的唯一持久化事实来源；组织 API 返回的 `status` 从对应 Scope 派生，不在组织表重复保存。
- 停用父 Scope 后，其后代在授权和新建业务中视为无效，即使后代自身仍为 `active`；T04 实现完整的祖先状态判定。
- 组织删除采用停用和软删除，不允许遗留无归属资源。

## 3. 统一资源模型

所有资源必须使用同一套结构，不得因为 Direct、Agent 或其他子类型创建不同资源结构体。`subtype` 是接入方式的权威字段，Agent 关联通过统一的 `agent_ref` 字段表达；连接参数统一放入 `config`。`config` 为空对象或 null 表示未配置资源级连接覆盖；非空时其中的字段按该资源工具的公共参数传递。资源的 Direct/Agent 接入语义、MCPServer 关联、公共工具和 Engine 解析规则以[统一资源接入设计](resource-access.md)为准。

```text
resources {
  id, tenant_id,
  scope_id,
  kind,
  subtype,
  agent_ref,
  name,
  external_uid,
  source_resource_id,
  labels,
  schema_version,
  config,
  status,
  credential_ciphertext,
  credential_key_version,
  credential_purpose,
  created_at, updated_at, deleted_at
}
```

首批可登记的 `kind`：

| 分类 | 资源类型 |
|---|---|
| 基础设施 | Kubernetes |
| 业务 | Application |
| 中间件 | PostgreSQL、Redis、Kafka、Elasticsearch、GenericMiddleware |
| AI 接入 | MCPServer |
| 可观测平台 | Prometheus、Loki、Tempo、Jaeger、Elastic、Datadog、GenericAPI |
| 开发交付 | Repository、Artifact |
| 运维支撑 | NotificationChannel、Runbook |

Kubernetes 的 Namespace 映射为 Project，Deployment、StatefulSet、DaemonSet、Job 和 CronJob 映射为 Application。Pod 副本映射为 Application 内的 Instance；Service、Ingress 和 Endpoint 信息也聚合在 Application 配置中。这些 Kubernetes 对象都不单独登记或维护为资源。Provider、Engine、Skill、Persona 和 LLM 的具体 Model 都不登记为资源；Model 是 Provider 的配置字段，Provider 的连接密文保存在独立 `providers` 表中。资源连接密文直接保存在资源行中，不把 Credential 当作资源登记。

Kubernetes 来源的 Application 在 `kubernetes.workload_kind` 中保留 Deployment、StatefulSet、DaemonSet、Job 或 CronJob 类型。该字段描述来源工作负载，不改变资源类型，也不产生新的权限层级。

手工创建的 Application 必须归属于 Project Scope；从平台或团队视图发起时，界面必须先选择团队下的项目。其接入方式只能为虚拟机、容器化或云原生，并至少配置一个实例。虚拟机实例关联 Host 与唯一定位进程的关键字表达式（支持 `&`、`|`、逗号/空格分隔和引号保护）；容器化实例关联 Docker 与容器名称；云原生实例关联 Kubernetes 与命名空间及合并显示的“工作负载类型 · 名称”，后端仍分别保存工作负载类型和名称。实例日志来源可以是受控文件路径或已关联日志平台的查询语句；容器化和云原生未指定文件路径时使用标准输出。关联资源可以是 Direct 或 Agent，Application 不保存或判断该接入方式。

`resource_schemas` 同时保存 `display_name`、`description` 和 `icon`，前端据此展示中文名称、说明和类型图标。`config` 使用 JSONB 保存非敏感类型字段，并由每种资源的版本化 JSON Schema 校验；资源保存实际使用的 `schema_version`。

敏感字段按照类型定义进入资源连接密文，例如：

| 资源类型 | 非敏感配置 | 资源连接密文 |
|---|---|---|
| Kubernetes | Context、API Server | kubeconfig |
| PostgreSQL | Host、Port、Database、Username | Password |
| Redis | Host、Port、Database、Username | Password |
| Kafka | Brokers、TLS | Username、Password |
| Repository | URL、Provider、默认分支 | Username、Token、SSH 私钥 |
| Artifact | URL、Provider、Namespace | Username、Password、Token |
| Prometheus | URL | Username、Password、Token |
| Loki | URL、Tenant ID | Username、Password、Token |

登录用户的 `credentials` 表与资源连接密文严格分开。前端使用类型化表单收集字段，提交时由 API 将非敏感字段写入 `config`，将敏感字段加密后写入同一资源行；用户不需要手写配置 JSON。

Connector 不是新的资源类型。它根据资源 `kind + schema_version` 解析适配器，读取资源配置和资源自身的连接密文，并声明该资源支持的查询能力。连接测试结果独立保存在 `resource_connection_checks`，资源本身仍是配置和授权的权威对象。

## 4. 可见性与引用规则

资源可见范围遵循从上到下继承：

| 当前上下文 | 可见资源 |
|---|---|
| 平台 | 平台级资源 |
| 团队 | 平台级资源、当前团队资源 |
| 项目 | 平台级资源、所属团队资源、当前项目资源 |

该表描述在某个目标作用域内创建资源、关系或策略时可以消费的资源集合，不限制管理员查看下级数据。具有后代读取权限的平台或团队角色仍可查询其管辖范围内的团队、项目及资源。

可见不等于可修改。下级用户对上级资源默认只有使用权，是否可查看配置、执行连接测试或管理资源由权限单独决定。

资源关联允许：

- 项目资源关联本项目、所属团队或平台资源。
- 团队资源关联本团队或平台资源。
- 平台资源关联平台资源。

默认禁止：

- 项目引用同团队的其他项目资源。
- 一个团队引用另一个团队的资源。
- 上级资源直接依赖某个下级范围资源。

确需跨团队共享时，应将资源提升到平台级，或以后引入显式 `ResourceShareGrant`，不能绕过作用域规则直接连边。

## 5. 资源关系模型

```text
resource_relations {
  id,
  source_resource_id,
  target_resource_id,
  relation_type,
  attributes,
  discovery_source,
  confidence,
  confirmed,
  created_by,
  created_at
}
```

关系类型包括：

| 关系 | 示例 |
|---|---|
| `deployed_on` | Application 部署在 Kubernetes 上 |
| `depends_on` | 业务应用依赖 Redis、Kafka |
| `observed_by` | 应用由 Prometheus、Loki 观测 |
| `exposes` | Service/Ingress 暴露应用 |
| `served_by_mcp` | Skill 通过 MCP Server 获取能力 |

创建关系时必须验证目标资源是否处于源资源允许引用的可见链上。Provider、Engine、Skill、Persona 不进入资源关系图；它们通过独立 API、Scope 默认绑定和执行快照引用资源上下文。自动发现的推测关系保存为 `confirmed=false`，并记录来源和置信度。

## 6. 同名资源和覆盖规则

下级资源可以与上级资源同名，但不做隐式覆盖。所有引用均保存资源 ID，界面展示名称时附带作用域标识，例如：

```text
redis-orders [项目]
redis-shared [团队: 支付团队]
prometheus-main [平台]
```

模型与 Skill/Persona 选择遵循显式指定优先，其次才是作用域默认配置：项目默认 > 团队默认 > 平台默认。AI 默认项固定 Scope + 场景标签对应的 Provider，执行时解析并固定模型；Skill 和 Persona 只提供可选 Prompt、工具和契约来源。巡检策略未指定 Persona 时使用内置巡检契约。每次执行再次把最终解析结果写入执行记录，后续默认配置变化不影响历史审计。

## 7. 主要数据表

```text
platforms
teams
projects
scopes
resources
resource_relations
resource_schemas
resource_sync_states
providers
engines
engine_scope_bindings
skills
personas
skill_versions
persona_versions
inspection_policies (可选 persona_id)
provider_scope_bindings
skill_scope_defaults
scope_defaults（仅保留资源默认项）
```

Engine 的统一执行事件与工具审计保存在 `engine_execution_events` 和
`engine_execution_tool_calls`；Skill 不拥有独立执行表。

`external_uid + source_resource_id + scope` 建立唯一约束，保证外部同步资源重复写入时执行更新而不是重复创建。独立大模型对象使用各自的 Scope 唯一约束；它们不共享资源目录的 `resource_schemas` 或资源关系。

## 8. 拓扑查询

首版使用 PostgreSQL 递归 CTE 查询资源拓扑，不引入图数据库。查询必须限制最大深度、节点数量和关系类型。

典型拓扑：

```text
Project
└── Application
    ├── deployed_on → Kubernetes [团队]
    ├── depends_on → Redis [团队]
    ├── depends_on → PostgreSQL [团队]
    └── observed_by → Prometheus [平台]
```

当资源移动作用域或项目转移团队时，系统必须先执行关系影响分析。存在迁移后不可见的关系时，禁止直接迁移并输出待处理关系清单。
