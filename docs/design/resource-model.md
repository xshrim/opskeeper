# 组织、资源与拓扑模型

## 文档状态

T06 已实现资源目录、凭据密文边界、关系约束、默认解析和有限拓扑查询；T07 已实现资源管理控制台基础页面；T08 已完成 Kubernetes 发现、Project/Application 映射和具体资源授权；T09 已完成 Kubernetes、Prometheus、Loki Connector 和连接检查；T10 已实现 LLM Provider 模型配置、不可变 SkillVersion、作用域默认解析和受控执行记录。

## 1. 设计原则

资源的类型和作用域是两个独立维度。例如：

- Skill 通常是平台级，但团队可以维护团队专属 Skill，项目也可拥有项目 Runbook Skill。
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

```text
resources {
  id, tenant_id,
  scope_id,
  kind,
  name,
  external_uid,
  source_resource_id,
  labels,
  schema_version,
  config,
  status,
  credential_id,
  created_at, updated_at, deleted_at
}
```

首批可登记的 `kind`：

| 分类 | 资源类型 |
|---|---|
| 基础设施 | Kubernetes |
| 业务 | Application |
| 中间件 | PostgreSQL、Redis、Kafka、Elasticsearch、GenericMiddleware |
| AI | AIModel、MCPServer、Skill |
| 可观测平台 | Prometheus、Loki、Tempo、Jaeger、Elastic、Datadog、GenericAPI |
| 开发交付 | Repository、Artifact |
| 运维支撑 | NotificationChannel、Runbook |

Kubernetes 的 Namespace 映射为 Project，Deployment、StatefulSet、DaemonSet、Job 和 CronJob 映射为 Application。Pod 副本映射为 Application 内的 Instance；Service、Ingress 和 Endpoint 信息也聚合在 Application 配置中。这些 Kubernetes 对象都不单独登记或维护为资源。LLM 的具体 Model 是 Provider 的配置字段，也不单独作为资源。连接凭据由独立的 `resource_credentials` 管理，不把 Credential 当作资源登记。

Kubernetes 来源的 Application 在 `kubernetes.workload_kind` 中保留 Deployment、StatefulSet、DaemonSet、Job 或 CronJob 类型。该字段描述来源工作负载，不改变资源类型，也不产生新的权限层级。

`resource_schemas` 同时保存 `display_name`、`description` 和 `icon`，前端据此展示中文名称、说明和类型图标。`config` 使用 JSONB 保存非敏感类型字段，并由每种资源的版本化 JSON Schema 校验；资源保存实际使用的 `schema_version`。

敏感字段按照类型定义进入加密凭据，例如：

| 资源类型 | 非敏感配置 | 加密凭据 |
|---|---|---|
| Kubernetes | Context、API Server | kubeconfig |
| PostgreSQL | Host、Port、Database、Username | Password |
| Redis | Host、Port、Database、Username | Password |
| Kafka | Brokers、TLS | Username、Password |
| AIModel | 路由策略、能力意图、大模型节点能力交集和上下文窗口 | AIModel 内部大模型节点的 API Token |
| Repository | URL、Provider、默认分支 | Username、Token、SSH 私钥 |
| Artifact | URL、Provider、Namespace | Username、Password、Token |
| Prometheus | URL | Username、Password、Token |
| Loki | URL、Tenant ID | Username、Password、Token |

登录用户的 `credentials` 表与外部资源凭据严格分开。前端使用类型化表单收集字段，提交时由 API 将非敏感字段写入 `config`，将敏感字段写入加密凭据并保存 `credential_id`；用户不需要手写配置 JSON。

Connector 不是新的资源类型。它根据资源 `kind + schema_version` 解析适配器，读取资源配置和关联的加密凭据，并声明该资源支持的查询能力。连接测试结果独立保存在 `resource_connection_checks`，资源本身仍是配置和授权的权威对象。

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
| `uses_ai_model` | 诊断策略、Skill 或巡检使用某个 AIModel |
| `uses_skill` | 项目或资源使用某个 Skill |
| `served_by_mcp` | Skill 通过 MCP Server 获取能力 |

创建关系时必须验证目标资源是否处于源资源允许引用的可见链上。自动发现的推测关系保存为 `confirmed=false`，并记录来源和置信度。

## 6. 同名资源和覆盖规则

下级资源可以与上级资源同名，但不做隐式覆盖。所有引用均保存资源 ID，界面展示名称时附带作用域标识，例如：

```text
redis-orders [项目]
redis-shared [团队: 支付团队]
prometheus-main [平台]
```

模型与 Skill 选择遵循显式指定优先，其次才是作用域默认配置：项目默认 > 团队默认 > 平台默认。AI 默认项固定 AIModel 资源 ID，由模型内部按优先级解析实际大模型节点；Skill 默认项固定 Skill 资源 ID 和已发布 SkillVersion ID。每次执行再次把最终解析结果写入执行记录，后续默认配置变化不影响历史审计。

## 7. 主要数据表

```text
platforms
teams
projects
scopes
resources
resource_relations
resource_schemas
resource_credentials
resource_sync_states
discovery_runs
discovery_items
skill_versions
llm_scope_defaults
skill_scope_defaults
skill_executions
skill_tool_calls
scope_defaults
```

`external_uid + source_resource_id + scope` 建立唯一约束，保证 Kubernetes 和外部平台资源重复同步时执行更新而不是重复创建。

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
