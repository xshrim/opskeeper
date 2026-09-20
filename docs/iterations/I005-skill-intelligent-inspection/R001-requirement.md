# I005-R001 标准 Skill 与智能巡检

**迭代：** I005-skill-intelligent-inspection  
**需求状态：** 实施中  
**验收报告：** [R001-requirement-acceptance.md](R001-requirement-acceptance.md)

## 1. 需求背景

当前 Skill 已迁移为独立领域对象并具备独立身份、Scope 归属、版本、输入输出 Schema 和工具白名单；正文仍主要是简短 Instruction，标准正文发布、依赖执行和智能巡检契约暂缓实施。

## 2. 目标

- 让 Skill 成为独立目录中的完整领域对象，并支持平台、团队、项目 Scope 归属；（基础拆分已交付）
- 支持诊断、监控、优化、维护四种 Skill 分类；
- 支持 JSON、YAML、Markdown 三种正文格式，并统一解析为规范化文档；
- 保留输入/输出 JSON Schema、工具声明、版本和审计快照；
- 支持 Skill 版本依赖、查看、执行和复制；
- 让内置诊断 Skill 与自定义 Skill 使用同一标准格式；
- 让智能巡检冻结巡检对象、Skill 版本、依赖、资源上下文、Provider/Model 和通知配置；
- 保持确定性评分、资源权限、凭据隔离、Tool Catalog 和受控操作边界。

## 3. 范围

### 3.1 Skill 元数据

Skill 必须提供：名称、稳定 identifier、分类、标签、是否启用、`scope_id`、具体用户 maintainer、更新时间和当前发布版本摘要。Skill 使用 Scope/Organization 关系解析级别和团队/项目归属，不复制 `team_id` 或 `project_id`。identifier 不编码 Scope 或管理级别。

### 3.2 Skill 正文和版本

每个已发布版本必须包含：版本号、自然语言描述、输入 Schema、输出 Schema、正文格式、原始正文、规范化正文、工具引用、依赖版本、风险等级、发布状态、内容哈希和创建/发布时间。

JSON、YAML、Markdown 只表示同一规范的不同书写格式；发布时生成规范化 JSON 文档，运行时使用规范化快照。

### 3.3 巡检适用范围

四类 Skill 均可绑定巡检：

| 分类 | 是否可巡检 | 默认作用 |
|---|---|---|
| diagnosis | 可以 | 解释事实、发现异常、生成假设 |
| monitoring | 可以 | 周期指标、阈值、趋势和健康判断 |
| optimization | 可以 | 生成优化建议，不直接改资源 |
| maintenance | 可以 | 维护合规、版本、备份、证书和配置检查 |

诊断和监控 Skill 可以参与健康判断；优化和维护 Skill 默认只能产生建议、合规结果或待办，不得直接产生资源写操作，也不得单独改变健康分数。具体执行仍受 Skill 版本的输出契约和巡检策略约束。

### 3.4 目标资源和工具

Skill 不再使用 `target_kinds` 作为硬编码授权字段。目标资源由巡检策略选择，运行时通过资源 Scope、关系上下文、Tool Catalog、Connector/MCP 能力和 `resource:use` 权限判断工具是否可用。Skill 正文可以引用工具和资源角色，但不能注册工具、扩大工具参数或直接指定凭据/连接地址。

## 4. 非目标

- 不允许 Markdown、YAML、JSON 或模型输出改变工具白名单；
- 不允许自由文本直接执行 SQL、Shell、Redis CLI、Kubernetes exec 或资源变更；
- 不把调度和通知配置保存到 Skill 版本正文中；
- 不使用 LLM 自由输出替代确定性健康评分；
- 不增加 `resource_level`、`maintainer_type`、`maintainer_id` 或 identifier 中的 Scope 编码；
- 不以 `target_kinds` 固定限制 Skill 的资源身份。

## 5. 任务清单

| 任务 | 名称 | 依赖 | 主要产出 | 状态 |
|---|---|---|---|---|
| T01 | Skill 独立元数据与正文契约 | 无 | 分类、标签、identifier、maintainer、归属查询和标准文档结构 | 基础已交付，正文契约暂缓 |
| T02 | Skill 版本解析与依赖 | T01 | 三种格式解析、校验、发布和版本依赖 | 待批准 |
| T03 | 内置诊断 Skill 标准化 | T01-T02 | 内置 Skill 标准文档、默认版本和复制入口 | 待批准 |
| T04 | Tool Catalog 与资源上下文 | T01-T03 | 工具引用、资源角色、权限和可用性判断 | 待批准 |
| T05 | Skill 执行与 Engine 接入 | T02-T04 | Skill 执行、Evidence、结构化输出、查看和复制 | 待批准 |
| T06 | Skill 驱动智能巡检 | T02-T05 | 策略、目标、Provider/Model、调度、评分和 Finding | 待批准 |
| T07 | 通知模板、级别和渠道 | T06 | 模板、路由、级别、Webhook 投递、重试和恢复 | 待批准 |
| T08 | 技能页和巡检中心 | T01-T07 | 管理、执行、复制和巡检配置界面 | 待批准 |

## 6. 任务说明

### T01 Skill 资源元数据与正文契约

#### 目标

定义唯一 Skill 独立对象模型和三种正文格式的共同语义。

#### 实施范围

- 复用 Resource 作为目标上下文并使用独立 Skill 目录和 `scope_id`，不把 Skill 登记到 Resource 目录；
- 增加分类、标签、稳定 identifier、用户 maintainer 和归属查询；
- 定义标准 Skill Document 的 metadata/spec 结构；
- 定义四类 Skill 的巡检约束；
- 不增加 `resource_level`、Scope 编码 identifier 或 `target_kinds` 硬限制。

#### 验收标准

- 相同 Skill 可通过 JSON、YAML、Markdown 表达相同规范化文档；
- 技能详情通过现有 Scope/Organization 查询显示 Scope 类型及团队/项目上下文；
- identifier 在全局唯一且不包含 Scope/管理级别；
- 四类分类和元数据边界有单元测试。

#### 风险和回滚

只扩展 Skill 独立 Schema，不修改已发布版本内容；验证失败时保留已有版本和旧执行路径。

### T02 Skill 版本解析与依赖

#### 目标

将 Skill 正文、工具和依赖固化为不可变、可复现的发布版本。

#### 实施范围

- 三种格式解析和规范化；
- JSON Schema 校验、内容哈希和发布状态；
- 版本依赖、拓扑排序、循环和 Scope 校验；
- 发布版本冻结原始正文和规范化正文。

#### 验收标准

- 草稿可校验但不能执行；
- 发布版本不可原地修改；
- 循环依赖、停用依赖和越权依赖被拒绝；
- 运行记录可还原当时的 Skill 版本。

#### 风险和回滚

保留现有整数版本兼容层；新解析失败时不得影响已有内置 Skill 的读取。

### T03 内置诊断 Skill 标准化

#### 目标

让 Kubernetes、PostgreSQL、Redis、Kafka 等内置诊断 Skill 使用标准 Skill 文档并可作为默认 Skill。

#### 实施范围

- 内置文档使用标准格式；
- 发布包安装平台级 Skill 和已发布版本；
- 内置正文和工具只能由代码/迁移发布，不允许资源页原地覆盖；
- 用户可以复制内置 Skill 创建自定义草稿。

#### 验收标准

- 内置 Skill 与自定义 Skill 经过同一解析和执行流程；
- 内置版本拥有内容哈希和不可变历史；
- Scope 默认解析可选择内置 Skill；
- 内置 Skill 不能写入资源变更工具。

#### 风险和回滚

以新增版本和新增迁移为主，不重写已应用迁移；旧 Skill 版本继续可查询但可停用。

### T04 Tool Catalog 与资源上下文

#### 目标

让 Skill 正文中的工具引用只能在关联资源、权限和连接能力全部满足时执行。

#### 实施范围

- Tool Catalog 和工具 Schema 快照；
- target、observed_by、deployed_on、depends_on 等资源角色；
- Direct/Agent 统一上下文解析；
- 凭据不进入模型上下文；
- 缺失资源或能力返回 unavailable，不推断健康。

#### 验收标准

- 正文不能新增未注册工具；
- 模型不能覆盖资源 ID、URL、凭据和工具白名单；
- Resource Filter 同时约束 Skill、目标和关联资源；
- Direct/MCP 路径工具名、结果和错误语义一致。

#### 风险和回滚

继续使用现有 Connector/MCP Provider；工具目录不完整时只允许已注册工具执行。

### T05 Skill 执行与 Engine 接入

#### 目标

支持 Skill 查看、人工执行、复制和结构化审计。

#### 实施范围

- Skill 执行请求和运行快照；
- Evidence、Tool Call、LLM 输出和审计；
- 输出 Schema 和 Evidence 引用校验；
- `deterministic`、`hybrid`、`llm_assisted` 执行模式；
- 复制为新的 Skill 对象和 draft 版本。

#### 验收标准

- 执行固定 Skill、目标、Provider、Model 和工具快照；
- 输出不合法或证据引用不存在时执行进入失败/降级；
- 复制不复用原 Resource ID、identifier、发布状态或 Scope 默认；
- 所有执行可按资源权限查看。

#### 风险和回滚

保留现有 Engine 执行接口和 SkillVersion 解析，按新版本能力逐步切换。

### T06 Skill 驱动智能巡检

#### 目标

让巡检策略以资源上下文、Skill 版本和固定模型为核心执行。

#### 实施范围

- 目标资源 ID、标签选择器和关联资源范围；
- Skill 绑定、版本固定/跟随最新；
- Provider、Model、预算、调度和维护窗口；
- 确定性规则、LLM Findings、健康分数和恢复；
- 复用 PostgreSQL Job、租约、心跳和重试。

#### 验收标准

- 四类 Skill 均可绑定巡检；
- 诊断/监控可以参与健康判断，优化/维护默认只产生建议；
- 每次运行保存目标、Skill、依赖、上下文和模型快照；
- LLM 不能直接修改健康分数或执行写操作；
- Finding 可以打开、升级、确认、恢复和抑制。

#### 风险和回滚

保留旧巡检策略读取和确定性评分；新策略运行异常时不得影响旧策略任务领取。

### T07 通知模板、级别和渠道

#### 目标

将 Finding、运行失败、Skill 不可用和恢复事件可靠通知到配置渠道。

#### 实施范围

- 通知模板版本和变量 Schema；
- `info`、`warning`、`critical`、`recovery`、`system_error`；
- 通知路由、Webhook 渠道、幂等、重试、冷却和去重；
- 复用 PostgreSQL 投递队列。

#### 验收标准

- 同一 Finding 在冷却窗口内不重复发送；
- 恢复通知能关联原始 Finding；
- 渠道失败不会丢失投递记录；
- 模板不能执行代码或读取未声明字段。

#### 风险和回滚

首版只保留 HTTPS Webhook；新模板/路由不影响已有渠道配置。

### T08 技能页和巡检中心

#### 目标

让 Skill 管理位于技能页，巡检中心只负责策略和运行。

#### 实施范围

- Skill 对象筛选、详情、版本、正文、工具、依赖和维护者；
- 查看、执行、复制和发布；
- 巡检策略、目标、Skill、模型、调度、通知配置；
- 运行历史、Finding、Evidence 和投递状态。

#### 验收标准

- 不存在第二套 Skill 权威管理入口；
- 移动和桌面布局可查看核心状态；
- 无权限、草稿、停用、运行中、失败和恢复状态清晰；
- 前端调用后端权限和状态，不自行判断授权。

#### 风险和回滚

保留现有 Skill/Inspection 路由兼容层，页面迁移失败时可回退到旧列表。

## 7. 需求级验收标准

1. Skill 的资源身份、`scope_id`、Scope/Organization 归属上下文、用户 maintainer 和 identifier 可查询，且不重复存储管理级别、team_id 或 project_id。
2. JSON、YAML、Markdown 正文可转换为同一规范化 Skill Document。
3. 内置诊断 Skill 与自定义 Skill 使用同一版本、工具和执行契约。
4. 四类 Skill 均可巡检，但优化和维护默认不参与健康评分、不执行资源变更。
5. Tool Catalog、Resource Filter、Direct/Agent 和凭据隔离边界通过测试。
6. Skill 执行和巡检运行具备固定版本、Evidence、结构化输出和审计链。
7. Finding、健康评分、通知模板、通知级别、渠道和恢复行为可复现。
8. 技能页成为 Skill 管理入口，巡检中心完成策略和运行管理。

## 8. 变更记录

- 2026-09-18：创建 I005，确认 Skill Scope 归属、四类巡检适用范围、标准内置 Skill、用户 maintainer 和非硬编码目标资源策略。
