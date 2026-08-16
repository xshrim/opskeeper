# 待确认事项与未解决问题

本文集中跟踪尚未形成完整任务边界的待确认事项、跨阶段风险、外部环境前置条件和已知技术债。它不是实施任务书、验收记录或临时笔记。

## 使用边界

- 已经具备目标、范围和验收标准的工作写入[分阶段实施任务书](implementation-tasks.md)。
- 影响长期技术方向且已经作出决定的内容写入 [ADR](../adr/README.md)。
- 已经完成验证的结果写入 `acceptance/`，生产故障处置步骤写入 `runbooks/`。
- 本文中的每项必须包含责任阶段或触发条件和明确关闭条件；已解决事项保留状态和结论，不直接删除。
- 如果后续启用 GitHub Issues，代码缺陷和短周期执行项迁移到 Issue；本文继续保留跨阶段决策和外部依赖。

状态使用：`待确认`、`待规划`、`待实施`、`受外部条件约束`、`已关闭`。

## 当前事项

| ID | 优先级 | 类型 | 状态 | 责任阶段或触发条件 | 事项与关闭条件 |
|---|---|---|---|---|---|
| OI-001 | 高 | 工程门禁 | 待实施 | T03 开始前 | 仓库尚无 GitHub Actions；增加执行 `make quality`、依赖一致性和数据库集成测试的 PR 工作流，并在 GitHub 启用 `main` 保护后关闭。 |
| OI-002 | 中 | 数据库迁移 | 待规划 | 首次不可逆数据变换前 | 当前迁移加载器要求 `.down.sql`；设计并实现显式不可逆迁移声明，确保生产前滚原则不需要伪造危险的 Down SQL 后关闭。 |
| OI-003 | 中 | 生产工程 | 待规划 | T03-T14 各任务审批时 | 安全响应头、扫描、指标和恢复能力不能全部推迟到 T15；将与当前功能直接相关的加固项写入各阶段验收标准，T15 只保留综合验收后关闭。 |
| OI-004 | 中 | 部署配置 | 受外部条件约束 | 每个测试、预发布和生产环境部署前 | 确认 Ingress/反向代理的直接来源 IP/CIDR 及其转发头策略，设置 `OPSK_TRUSTED_PROXIES` 并验证伪造头测试后按环境关闭。 |
| OI-005 | 中 | 产品与身份 | 已关闭 | 2026-08-15，T03 | 首版采用本地身份与会话；OIDC/LDAP 不属于 T03，已在 [ADR-0005](../adr/0005-identity-session-baseline.md) 和 T03 任务范围中记录，后续如需企业 SSO 另立任务。 |
| OI-006 | 高 | Skill 产品与运行时 | 待规划 | T12 验收后、后续 Skill 管理能力任务开始前 | 当前内置 Skill 仅由数据库中的简短 Instruction、Schema 和 Tool 白名单组成，尚非可阅读、可编辑、可版本化的 Markdown Skill 文档。按下文确定的“Markdown 正文 + 数据库版本 + 受控工具目录”方案完成设计、实现和验收后关闭。 |

## OI-006：Markdown Skill 文档、编辑体验与受控工具目录

### 当前状态

T12 已实现安全基线：`SkillVersion` 保存名称、简短 Instruction、输入/输出 Schema、Tool 白名单和发布状态；ADK-Go Agent/Runner 只获得该版本声明的 Function Tool。PostgreSQL、Redis、Kafka 使用固定只读快照，Kubernetes 使用资源允许列表读取。Connector 内部执行预定义查询或 API 调用，模型不能提交任意 SQL、Redis 命令、Kafka 管理操作或 Kubernetes 路径。

这保证了资源授权、凭据隔离、审计、调用预算和只读边界，但 Skill 正文仍然过薄：用户不能在控制台直接阅读一份完整的诊断说明、创建 Markdown 草稿或编辑新的 Skill；中间件的一次性快照工具也不如细粒度查询能力透明和可组合。

### 已确定的目标方案

一个运行时 Skill 由以下三个部分组成，且三者职责不可混淆：

```text
Markdown Skill 文档
  ├── YAML Front Matter：名称、目标资源类型、风险等级、工具标识、Schema 引用
  └── 正文：目标、诊断步骤、证据解释、判断口径、输出约束

SkillVersion（数据库中的不可变发布版本）
  ├── content_md 与已解析的结构化元数据
  ├── 输入 / 输出 Schema、Tool 白名单、Draft / Published / Disabled 状态
  ├── 来源、版本、发布时间和审计信息
  └── 已发布版本不得原地修改；变更必须创建新版本

Connector / Tool Catalog（Go 代码）
  ├── 受审核的具体查询或 API 操作
  ├── 所需 Connector 能力、最小权限、严格参数、超时和结构化结果
  └── 禁止接受 Markdown 或模型生成的任意命令直接执行
```

Markdown 负责面向人和模型说明“为何、何时、以何种顺序”收集证据；Tool Catalog 负责规定“实际能够执行什么”。例如 `postgresql.waiting_locks` 可以在 Markdown 中被引用，但 SQL 必须仍由 PostgreSQL Connector 内置、审核并以只读账户执行。

### 计划范围

1. **数据与版本模型**：为 `SkillVersion` 增加 Markdown 正文及来源元数据；把现有内置 Skill 转换为完整 Markdown 文档。发布时冻结 Markdown、Front Matter、Schema 和 Tool 清单；数据库与导入文件的权威关系必须明确，不能维护两份会漂移的正文。
2. **受控工具目录**：将当前 `postgresql_inspect`、`redis_inspect`、`kafka_inspect` 等粗粒度快照，逐步拆分为有稳定标识的细粒度只读能力，例如连接概览、会话、长时查询、锁、复制和容量。每项能力都必须定义参数 Schema、最小权限、超时、最大结果、降级行为和 Evidence 结构。
3. **控制台体验**：增加 Skill 列表、详情、Markdown 渲染预览、草稿编辑器、版本历史、版本差异、复制内置 Skill 创建草稿、校验和发布流程。内置已发布版本只读；用户通过草稿或新版本扩展，不直接覆盖历史版本。
4. **Runner 接入**：ADK-Go Agent 的 Instruction 使用已发布版本的 Markdown 正文；ADK Function Tool 仅从该版本白名单和 Tool Catalog 映射产生。继续保留 RBAC、Scope/资源过滤、调用审计、超时、并发、Token 与响应大小预算。
5. **安全与测试**：禁止 Markdown、Front Matter、模型输出和 Connector 返回的自由文本改变工具权限或执行任意命令；补充 Markdown 解析、版本不可变性、工具目录参数校验、权限拒绝、发布回滚和控制台端到端测试。

### 非目标

- 不把 Markdown 中的 SQL、Shell、Redis CLI 或 Kubernetes 命令直接交给执行器。
- 不允许模型自行注册工具、提高权限、绕过 Resource/Connector/凭据边界。
- 不在此事项中引入自动修复、自动扩缩容、写 SQL、配置修改或任意命令执行。

### 关闭条件

以下条件全部满足后，OI-006 才可关闭：

1. 内置与自定义 Skill 均可在控制台查看完整 Markdown，并能查看适用资源、允许工具、输入输出契约、最小权限和版本历史。
2. 有权限的用户可创建、编辑和校验 Draft；发布会生成不可变版本，已发布版本不能被原地覆盖。
3. Runner 实际使用已发布 Markdown 作为 ADK 指令，并仅加载该版本声明且 Tool Catalog 已注册的受控工具。
4. 至少 PostgreSQL、Redis、Kafka、Kubernetes 各有可组合的细粒度只读工具，并覆盖成功、权限不足、能力缺失和参数非法等行为测试。
5. 通过控制台、API 与集成测试证明：Markdown 或模型文本无法触发未声明工具、任意协议命令或越权资源访问。
6. 完成对应阶段的验收记录，并将过时的 T12 临时说明更新为最终模型链接。

## 已关闭事项

OI-005 已于 2026-08-15 关闭。后续关闭事项时继续记录关闭日期、关联任务/ADR/PR 和最终结论。
