# 组织、资源与拓扑模型

## 文档状态

T06 正在实施资源目录、凭据密文边界、关系约束、默认解析和有限拓扑查询。具体连接器、Kubernetes 自动发现和资源管理前端不属于 T06。

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
platforms(id, scope_id, name, code)
teams(id, scope_id, platform_id, name, code, labels)
projects(id, scope_id, platform_id, team_id, name, code, labels, source)
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

建议首批 `kind`：

| 分类 | 资源类型 |
|---|---|
| 基础设施 | KubernetesCluster、Namespace、Node、Workload、Pod、Service、Ingress |
| 业务 | BusinessApplication、Endpoint、CronApplication |
| 中间件 | PostgreSQL、Redis、Kafka、Elasticsearch、GenericMiddleware |
| AI | LLMProvider、Model、MCPServer、Skill |
| 可观测平台 | Prometheus、Loki、Tempo、Jaeger、Elastic、Datadog、GenericAPI |
| 运维支撑 | Credential、NotificationChannel、Runbook、ArtifactStore |

`config` 使用 JSONB 保存类型特有字段，并由每种资源的版本化 JSON Schema 校验；资源保存实际使用的 `schema_version`。Credential 资源只保存名称、作用域和用途等元数据，密文载荷单独存入 `resource_credentials`，普通资源配置仅保存凭据引用。登录用户的 `credentials` 表与外部资源凭据严格分开。

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
| `contains` | 集群包含 Namespace |
| `deployed_on` | 应用部署在 Workload 或集群上 |
| `depends_on` | 业务应用依赖 Redis、Kafka |
| `observed_by` | 应用由 Prometheus、Loki 观测 |
| `exposes` | Service/Ingress 暴露应用 |
| `uses_provider` | 诊断策略使用某个 LLM Provider |
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

Skill 选择遵循显式绑定优先，其次才是作用域默认配置：项目默认 > 团队默认 > 平台默认。默认配置只能有一个生效项，并保留最终解析结果供审计。

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
skills
skill_versions
scope_defaults
```

`external_uid + source_resource_id + scope` 建立唯一约束，保证 Kubernetes 和外部平台资源重复同步时执行更新而不是重复创建。

## 8. 拓扑查询

首版使用 PostgreSQL 递归 CTE 查询资源拓扑，不引入图数据库。查询必须限制最大深度、节点数量和关系类型。

典型拓扑：

```text
Project
└── BusinessApplication
    ├── deployed_on → Kubernetes Workload [项目]
    ├── depends_on → Redis [团队]
    ├── depends_on → PostgreSQL [团队]
    └── observed_by → Prometheus [平台]
```

当资源移动作用域或项目转移团队时，系统必须先执行关系影响分析。存在迁移后不可见的关系时，禁止直接迁移并输出待处理关系清单。
