# I005 Skill 与智能巡检

**迭代编号：** I005  
**目录标识：** I005-skill-intelligent-inspection  
**状态：** 设计暂缓（独立领域基础已交付）  
**计划开始：** 2026-09-18  
**计划封板：** 待定

## 1. 迭代目标

把 Skill 从“数据库中的简短指令和工具白名单”收敛为可查看、可复制、可版本化、可执行的独立技能对象，并让巡检策略以 Skill、资源上下文和固定模型版本为输入执行可追溯的智能巡检。本分支先交付 Provider、Engine、Skill、Persona 的独立领域拆分、Scope/RBAC 和管理页面；标准 Skill 正文执行与 Skill 驱动智能巡检暂缓，待后续迭代单独实施。

本迭代复用 Resource 作为目标上下文、Scope/RBAC、Engine、Connector/MCP、PostgreSQL Job Queue、Finding 和 Webhook 边界；Skill 自身通过独立目录和权限管理，不进入资源目录。

## 2. 已确认的设计决策

1. Skill 是独立一级对象，管理入口位于技能页；资源只作为 Skill 的目标上下文。Skill 版本、正文、工具声明和依赖属于 Skill 目录管理能力。
2. Skill 使用与其他领域对象一致的 `scope_id` 归属。Scope 类型决定平台、团队或项目级别。
3. Skill 的团队和项目归属由 Scope 关系解析，不复制 `team_id`、`project_id` 字段。
4. `identifier` 只表达 Skill 的稳定身份，不编码平台、团队、项目或管理级别。复制 Skill 时生成新的唯一 identifier。
5. `maintainer` 只保存具体用户 ID；团队维护通过该用户所属权限和 Scope 管理，不增加 `maintainer_type` 或 `maintainer_id` 复合字段。
6. `target_kinds` 不作为 Skill 的硬编码字段。Skill 描述工具、上下文角色和输入输出契约；执行前由目标资源、关系上下文、Tool Catalog 和权限共同决定是否可执行。
7. 四类 Skill 都可以绑定到巡检策略：`diagnosis`、`monitoring`、`optimization`、`maintenance`。其中诊断和监控可以产生健康判断；优化和维护默认为建议/合规检查，不得自动执行资源变更，也不能单独改变健康分数。
8. 内置诊断 Skill 也使用标准 Skill 文档格式。内置版本由发布包安装为平台级、已发布、不可直接修改的 Skill；用户通过复制创建自己的草稿。

## 3. 本分支交付边界

已交付：

- Provider、Engine、Skill、Persona 独立表、API、Scope 归属和独立权限前缀；
- 通用资源页移除 Provider、Skill、Persona 等旧资源类型；
- “模型”页面中的 Engine 目录、Provider 渠道、模型能力、场景默认绑定和连接测试；
- 技能、专家和统一权限管理页面的基础管理体验；
- Engine Runtime、Provider/Model 解析、图标命名空间和旧发现/受控操作入口清理。

暂缓：

- JSON/YAML/Markdown 标准 Skill 正文的完整发布与依赖执行；
- 内置诊断 Skill 的统一版本安装链路；
- Skill 驱动智能巡检、Finding、通知模板和巡检策略新模型。

## 4. 原设计范围（后续实施）

- Skill 元数据、标准正文格式、JSON Schema、工具引用和版本不可变性；
- JSON、YAML、Markdown 三种正文的统一解析和规范化；
- Skill 版本依赖和无环校验；
- 受控 Tool Catalog 与资源角色绑定；
- Skill 查看、执行和复制；
- 巡检对象、Skill 版本、Provider/Model、调度、预算、Finding 和通知路由；
- 诊断、监控、优化、维护四类 Skill 的巡检约束；
- 结构化 LLM 输出、Evidence 引用、确定性评分和 Finding 生命周期；
- 通知模板、通知级别、渠道、幂等投递和恢复通知；
- 技能页 Skill 管理和巡检中心配置体验。

## 5. 非目标

- 不允许 Skill 正文、Front Matter、模型输出或远端资源内容直接增加工具权限；
- 不允许 Skill 或智能巡检直接执行任意 SQL、Shell、Redis 命令、Kubernetes exec 或未审批写操作；
- 不把调度、通知渠道和巡检对象固化到 Skill 本身；
- 不引入 Redis、Kafka、独立对象存储或独立任务系统作为 OpsKeeper 默认依赖；
- 不以 LLM 自由输出直接替代确定性健康评分；
- 不把 `target_kinds` 作为 Skill 身份或固定硬编码字段。

## 6. 需求清单

| 需求 | 名称 | 任务 | 状态 | 需求文档 | 验收报告 |
|---|---|---:|---|---|---|
| R001 | 标准 Skill 与智能巡检 | T01-T08 | 实施中 | [R001-requirement.md](R001-requirement.md) | [R001-requirement-acceptance.md](R001-requirement-acceptance.md) |

## 7. 任务和依赖

| 任务 | 名称 | 依赖 | 主要产出 | 状态 |
|---|---|---|---|---|
| T01 | Skill 独立元数据与正文契约 | 无 | 分类、标签、稳定 identifier、用户 maintainer 和独立 Scope 模型 | 基础已交付，正文契约暂缓 |
| T02 | Skill 版本解析与依赖 | T01 | 不可变版本、发布校验、正文解析、无环版本依赖 | 待批准 |
| T03 | 内置诊断 Skill 标准化 | T01-T02 | 内置诊断 Skill 使用标准格式并可作为 Scope 默认 | 待批准 |
| T04 | Tool Catalog 与资源上下文 | T01-T03 | 受控工具引用、关联资源角色、Direct/Agent 解析和权限边界 | 待批准 |
| T05 | Skill 执行与 Engine 接入 | T02-T04 | 查看、执行、复制、Evidence、结构化输出和审计 | 待批准 |
| T06 | Skill 驱动智能巡检 | T02-T05 | 巡检对象、四类 Skill 约束、Provider/Model 快照、调度和 Finding | 待批准 |
| T07 | 通知模板、级别和渠道 | T06 | 通知路由、Webhook 投递、重试、幂等、恢复通知 | 待批准 |
| T08 | 技能页和巡检中心 | T01-T07 | Skill 管理、版本正文、执行/复制和巡检配置页面 | 待批准 |

## 8. 进入条件

- I004 统一资源接入的 Resource、Direct/Agent、Connector/MCP 和 Scope/RBAC 作为运行时边界；
- I003 Engine、Provider/Model、结构化输出和审计能力可复用；
- 当前巡检 Scheduler、Worker、Finding、Webhook 和 PostgreSQL Job Queue 保持可用；
- 内置诊断 Skill 的工具仍由 Connector/Tool Catalog 控制，不能通过正文自由注册。

## 9. 退出条件

- 四类 Skill 均可创建、查看、版本化、复制和执行；
- 内置诊断 Skill 与自定义 Skill 使用同一标准格式和执行路径；
- Skill 的 Scope、team/project 归属、maintainer 和权限边界可复核；
- Skill 正文无法改变工具白名单、目标资源、凭据和审批边界；
- 巡检运行冻结 Skill、依赖、目标、资源上下文、Provider 和 Model；
- 诊断/监控与优化/维护在评分、建议和变更权限上有明确隔离；
- Finding、健康评分、通知级别、模板、渠道和恢复行为有可复核测试；
- 技能页成为 Skill 管理的唯一入口，巡检中心只管理策略和运行；
- 所有未完成事项转入 backlog 或下一迭代，并完成验收记录。

## 10. 封板记录

待补充。
