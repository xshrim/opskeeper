# 三级权限与安全设计

## 1. 权限目标

权限系统同时回答三个问题：用户能在哪个范围内操作、能操作哪个具体资源、能执行什么动作。

```text
Permission = Subject + Role + Scope + Conditions
```

- Subject：用户或用户组。
- Role：角色包含的权限集合。
- Scope：平台、团队或项目。
- Conditions：可选的资源类型、标签、环境或风险级别限制。

组织权限始终保持 platform、team、project 三级 Scope，不为 Application 增加第四级 Scope。需要把项目成员限制到某个具体 Application 或其他资源时，使用资源角色绑定。

## 2. 作用域继承

角色绑定在指定范围生效，并向下继承：

- 平台角色可覆盖平台及其全部团队和项目。
- 团队角色覆盖本团队及其项目，不影响其他团队。
- 项目角色只覆盖当前项目。
- 下级授权不能赋予超出授权人自身范围和权限的能力。

继承只扩大用户可操作的数据范围，不改变资源所有权。团队管理员不能修改平台资源，除非另有平台级授权。

## 3. 内置角色

| 角色 | 绑定范围 | 主要权限 |
|---|---|---|
| PlatformAdmin | 平台 | 全局组织、资源、权限、凭据和策略管理 |
| PlatformOperator | 平台 | 全局查看、诊断、巡检和审批，不管理平台权限 |
| PlatformViewer | 平台 | 全局只读和审计查看 |
| TeamAdmin | 团队 | 团队成员、团队资源、项目及团队策略管理 |
| TeamOperator | 团队 | 团队及项目资源查看、诊断和巡检执行 |
| TeamViewer | 团队 | 团队及其项目只读 |
| ProjectAdmin | 项目 | 项目成员、项目资源和项目策略管理 |
| ProjectOperator | 项目 | 项目诊断、巡检和低风险工具执行 |
| ProjectViewer | 项目 | 项目资源和报告只读 |

平台还应支持自定义角色，但自定义角色只能组合系统定义的权限点，不能创建任意脚本权限。

### 3.1 授权管理边界

- PlatformAdmin 可以在平台及任意下级范围授予角色。
- TeamAdmin 可以在本团队及其项目授予团队级或项目级角色，不能授予平台角色。
- ProjectAdmin 只能在当前项目授予项目级角色。
- 授权人不能授予自己不具备的权限，也不能把角色绑定到自身管辖范围之外。
- 一个用户可以在不同范围拥有多个角色。例如用户可以是 Team A 的 TeamOperator，同时是 Team B 某个项目的 ProjectViewer。

MVP 采用仅允许、无显式拒绝的模型。同一目标上的有效权限取所有适用角色的并集，以保持继承和故障排查简单；如以后增加拒绝规则，必须明确拒绝优先级并提供权限解释器。

## 4. 权限点

权限采用 `domain:action` 命名：

```text
organization:read
team:manage
project:manage
member:grant
resource:read
resource:create
resource:update
resource:delete
resource:use
credential:manage
credential:test
relation:manage
discovery:run
discovery:import
diagnosis:start
diagnosis:read
inspection:manage
inspection:execute
operation:approve
audit:read
```

权限说明：

| 权限 | 含义 |
|---|---|
| `organization:read` | 查看组织、平台和 Scope 信息 |
| `team:manage` | 创建、编辑和停用团队 |
| `project:manage` | 创建、编辑和停用项目 |
| `member:grant` | 管理用户、用户组和角色授权 |
| `resource:read` | 查看资源列表、配置和详情 |
| `resource:create` | 创建资源 |
| `resource:update` | 编辑资源配置 |
| `resource:delete` | 删除或停用资源 |
| `resource:use` | 使用资源执行连接测试或业务调用 |
| `credential:manage` | 管理凭据及其关联配置 |
| `credential:test` | 测试凭据连接 |
| `relation:manage` | 管理资源之间的关联关系 |
| `discovery:run` | 启动集群或资源发现 |
| `discovery:import` | 导入发现结果 |
| `diagnosis:start` | 启动 AI 诊断 |
| `diagnosis:read` | 查看诊断记录和结果 |
| `inspection:manage` | 管理自动巡检策略 |
| `inspection:execute` | 执行自动巡检 |
| `operation:approve` | 审批受控操作 |
| `audit:read` | 查看审计日志 |

`resource:use` 与 `resource:read` 分离。AIEngine 执行时直接使用有权限的 AIProvider 和模型；Provider 的地址和凭据仍不向业务请求暴露。

## 5. 授权数据模型

```text
users(id, ...)
groups(id, ...)
group_members(group_id, user_id)
roles(id, name, builtin, scope_levels)
role_permissions(role_id, permission, conditions)
role_bindings(id, subject_type, subject_id, role_id, scope_id)
resource_roles(id, name, builtin)
resource_role_permissions(role_id, permission)
resource_role_bindings(id, subject_type, subject_id, role_id, resource_id)
```

`scope_id` 指向统一 Scope 树。角色绑定、资源、诊断会话和巡检策略使用同一种祖先关系判断，不分别实现三套鉴权逻辑。

资源读取与操作的最终过滤结果是“Scope 角色允许的全部资源”与“资源角色显式允许的资源”的并集。平台、团队和项目观察员默认只能读取自己可见的资源：平台资源向团队和项目可见，团队资源向项目可见；观察员不因可见而自动获得 `resource:use`。只有对具体资源追加 `ResourceViewer`、`ResourceOperator` 或 `ResourceAdmin` 后，主体才获得该资源对应的附加权限。观察员可追加自己可见范围内的资源，包括来自上级 Scope 的资源；服务端要求目标主体在与资源存在祖先/后代关系的 Scope 上拥有对应的 `PlatformViewer`、`TeamViewer` 或 `ProjectViewer`。

管理员和操作员默认具有本级及上级可见资源的读取/使用权限。资源编辑和删除只允许资源所在 Scope 及其上级 Scope 的相应管理员权限，项目级权限不能修改团队或平台资源。资源授权仍由具备 `member:grant` 的管理员创建，不能绕过 Scope 边界。

鉴权过程：

1. 根据用户和用户组取得全部角色绑定。
2. 判断绑定范围是否是目标资源范围的祖先或自身。
3. 判断角色是否包含所需权限点。
4. 判断资源类型、标签、环境和操作风险等条件。
5. 对敏感操作执行二次校验或审批。

结果可短期缓存到 Redis，但角色变更必须主动失效缓存。后端必须始终以数据库中的组织归属为事实来源。

## 5.1 T05 管理边界

- 平台管理员可以创建、查看和停用用户；用户创建必须同时写入 Argon2id 凭据，API 不返回密码或密码摘要。
- 组绑定一个 Scope，组成员继承组上的角色绑定；组成员变更会立即改变用户的有效权限。
- 平台、团队和项目管理员只能在自身 `member:grant` 范围内创建或撤销角色绑定。
- 授权人必须同时拥有被授予角色包含的每个权限点，不能把自身没有的权限转授给其他用户或组。
- 用户组、角色绑定和用户状态变更都写入安全审计；审计记录至少包含操作者、动作、目标、Scope、请求 ID、来源 IP、结果和时间。

授权查询使用 PostgreSQL 中的 `authorization_revision` 单调版本号构造 Redis 缓存键。任何影响授权的用户、Scope、组、成员、角色或绑定变化都会递增版本号；缓存读取失败时回源 PostgreSQL，绝不复用可能过期的成功结果。

## 6. 数据隔离

- API 查询必须注入授权后的 scope filter，不能先查询全部数据再在内存过滤。
- PostgreSQL 可使用 RLS 作为纵深防御，但应用层仍需显式鉴权。
- 用户不能通过已知资源 ID 访问无权资源。
- 项目可见不等于项目内所有资源可见；列表、按 ID 查询、关系、拓扑和发现记录必须合并执行 Scope 与资源 ID 过滤。
- 诊断会话、巡检结果、证据和审计记录继承目标资源的作用域。
- 包含多个目标的任务，其作用域取所有目标的最近公共祖先，并要求用户对每个目标均有权限。

## 7. 凭据与敏感信息

- 凭据独立保存，使用 KMS/Vault；最低要求为 AES-GCM 信封加密。
- 用户只能绑定和使用凭据，不通过 API 读取明文。
- 平台级凭据不能被团队管理员导出，团队级凭据不能被项目管理员导出。
- 日志、模型上下文和工具响应进入持久化前执行 Token、密码、连接串等脱敏。
- 凭据读取、测试、轮换和使用都写入审计日志。

## 8. AI 与工具权限

Skill 声明所需能力和风险级别：

| 风险 | 示例 | 执行规则 |
|---|---|---|
| ReadOnly | 查询日志、指标、Pod 状态 | 有 Skill 执行权限即可 |
| Low | 创建临时诊断任务 | 可按团队策略自动执行 |
| Medium | 重启 Pod、调整副本 | 必须人工审批 |
| High | 删除资源、执行写 SQL | 默认禁止，需特权审批策略 |

LLM 本身无权直接访问基础设施。Skill 是统一资源目录中的一种资源，读取、执行、版本发布和管理分别使用 `resource:read`、`resource:use`、`resource:update` 和 `resource:delete` 等通用权限；SkillVersion 从属于 Skill，不单独作为授权主体。Agent、Skill Tool 调用和 Runner 执行内核统一使用 ADK Go v2；所有调用仍必须经过 OpsKeeper Policy Enforcement Point，模型提出的操作参数需要结构化校验。ADK 负责通用编排，不替代 Scope、资源权限、预算、审批和审计。

## 9. 审计要求

审计记录至少包含用户、有效角色、作用域、目标资源、动作、请求 ID、来源 IP、前后摘要、结果和时间。

来源 IP 不直接信任客户端提交的转发头。API 只有在 TCP 直连来源属于 `OPSK_TRUSTED_PROXIES` 时才解析 `X-Forwarded-For` 或 `X-Real-IP`；未配置可信代理时始终使用连接来源。可信范围必须是受控代理节点地址，不能包含普通客户端网段。

模型相关审计还应记录 Provider、模型、Skill 版本、工具调用、Token 用量、脱敏输入摘要、证据 ID、审批人和最终结论。审计日志采用只追加写入策略，并设置独立保留周期。
