# I004-R001 统一资源接入

**迭代：** I004-unified-resource-access  
**需求状态：** 实施中  
**设计文档：** [统一资源接入设计](../../design/resource-access.md)  
**验收报告：** [R001-requirement-acceptance.md](R001-requirement-acceptance.md)

## 1. 需求背景

当前资源工具存在两套实现：Connector 为 Direct 资源定义 AIEngine 工具，Docker 和 Kubernetes MCP Server 又分别实现 MCP 工具。两套实现的工具名称、参数、结果和错误处理不一致，新增资源需要重复编写业务逻辑，AIEngine 也无法只根据资源接入方式稳定选择工具来源。

统一资源接入要求把资源业务工具从协议适配器中抽离。同一套工具实现既能由本项目的 MCP Server 暴露，也能由 AIEngine 作为 Direct 内置工具集调用。用户勾选资源后，Direct 使用资源配置和凭据连接目标，Agent 使用关联 MCPServer 发现并调用远端工具。

## 2. 目标

- 建立协议无关的公共资源工具层；
- 保持工具名称、业务入参、业务出参和错误语义一致；
- 让 Host、Docker、Kubernetes、Application 优先完成 Direct/Agent 闭环，随后迁移 PostgreSQL、Redis；
- 让项目提供的 Docker、Kubernetes MCP Server 与外部 MCP Server 使用同一 AIEngine MCP 路径；
- 保持逻辑资源权限为唯一授权主体，隔离 MCP 传输资源和凭据；
- 删除重复 Connector/MCP 资源业务实现，减少后续资源接入成本。

## 3. 范围

### 3.1 首批优先资源

| 资源 | Direct 工具集 | Agent 工具来源 | 首批要求 |
|---|---|---|---|
| Host | 主机信息、健康和受限只读检查 | 关联 MCPServer 的发现工具 | 完成公共实现及连接测试 |
| Docker | Docker Engine 信息、镜像、容器、日志、检查和统计 | Docker MCP 或外部 MCP 发现工具 | 复用现有 Docker MCP 工具定义和结果 |
| Kubernetes | 集群、命名空间、节点、Pod、工作负载、服务、事件、日志和资源读取 | Kubernetes MCP 或外部 MCP 发现工具 | 复用现有 Kubernetes MCP 工具定义和结果 |
| PostgreSQL | 连接、会话、慢查询、锁、复制和容量诊断 | 关联 MCPServer 的 PostgreSQL 工具 | 保持现有只读快照语义，逐步细化工具 |
| Redis | 连接、内存、客户端、复制和慢日志诊断 | 关联 MCPServer 的 Redis 工具 | 保持现有只读快照语义和不可用能力说明 |

### 3.2 后续资源

- P1：Kafka、Prometheus、Loki；
- P2：RabbitMQ、Elasticsearch、MySQL、Oracle、OceanBase、TongRDS；
- 可观测平台的 Tempo、Jaeger、Elastic、Datadog、Alertmanager 按同一模式接入；
- AIProvider、MCPServer、Skill、AgentProfile、Repository、Artifact 不在本需求中作为直连诊断工具集实现；Application 作为项目级聚合资源纳入 T06。

## 4. 非目标

- 不把工具的业务输入改成携带 resource kind、工具版本、能力或只读标识；
- 不要求远端 MCP 工具通过本项目的自定义契约校验；
- 不区分 managed MCP Server 和 external MCP Server 的 AIEngine 调用路径；
- 不允许模型指定 Direct 连接地址、证书、Token、密码或 kubeconfig；
- 不开放任意 Shell、SQL、Redis 命令、Docker 写操作或 Kubernetes 变更；
- 不保留为旧架构服务的长期双轨工具实现。

## 5. 任务清单

| 任务 | 名称 | 依赖 | 主要产出 | 状态 |
|---|---|---|---|---|
| T01 | 公共工具契约与执行基础 | 无 | 公共工具定义、连接上下文、结果/错误模型、注册表 | 已完成 |
| T02 | 资源接入模型与上下文解析 | T01 | Direct/Agent 字段、MCPServer 关联、统一解析和权限 | 已完成 |
| T03 | Docker 工具集统一 | T01-T02 | Docker 公共工具、MCP 薄适配器、Direct 适配器 | 已完成 |
| T04 | Host 工具集接入 | T01-T02 | Host Direct/Agent 工具和连接测试 | 待批准 |
| T05 | Kubernetes 工具集统一 | T01-T02 | Kubernetes 公共工具、MCP 薄适配器、Direct/Agent 适配器 | 已完成 |
| T06 | Application 资源接入 | T01-T05 | 项目归属、三种接入方式、实例唯一性、受控日志工具和管理界面 | 已完成 |
| T07 | PostgreSQL 工具集统一 | T01-T02 | PostgreSQL 公共工具、Direct/MCP 适配器、专用管理界面 | 已完成 |
| T08 | Redis 工具集统一 | T01-T02 | Redis 公共工具、Direct/MCP 适配器 | 已完成 |
| T09 | Nacos 工具集统一 | T01-T02 | Nacos 服务注册、配置中心、命名空间 API 工具和 Direct/MCP 适配器 | 实施中 |
| T10 | AIEngine 与证据链收敛 | T03-T09 | 工具注册、别名、证据、事件、审计和错误统一 | 待批准 |
| T11 | Kafka、Prometheus、Loki 迁移 | T01-T02、T10 | P1 工具集接入和一致性测试 | 待批准 |
| T12 | 其他数据库和中间件迁移 | T11 | P2 工具集接入 | 待批准 |
| T13 | 管理界面与接入校验 | T02-T10 | 接入方式、MCPServer 关联、连接测试和错误展示 | 待批准 |
| T14 | 删除旧路径与全量验收 | T03-T13 | 删除重复实现、迁移、回归和验收报告 | 待批准 |

## 6. 任务说明

### T01 公共工具契约与执行基础

#### 目标

建立不依赖 MCP SDK、AIEngine 和 HTTP API 的公共资源工具包，统一工具名称、业务参数、结果、错误和资源客户端边界。

#### 实施范围

- 建立公共工具定义和按名称注册机制；
- 建立连接上下文、调用上下文、结构化结果和错误分类；
- 将参数限制、输出截断、敏感字段清理放在公共实现；
- 明确工具业务参数与连接凭据的边界；
- 为 Docker/Kubernetes 迁移提供第一套可复用接口。

#### 验收标准

- 公共工具不依赖 MCP SDK、AIEngine 或资源 HTTP 服务；
- 工具定义不包含 resource kind、contract version、capabilities、read-only 等强制字段；
- 同一个工具可以由两个不同适配器调用并得到等价业务结果；
- 参数非法、资源不可用、响应超限和取消均有稳定错误分类。

#### 风险和回滚

公共类型一旦被 MCP 和 Direct 同时使用，后续修改会影响两条路径。通过先迁移 Docker/Kubernetes 测试和保留明确的公共 DTO 降低风险；若任务失败，暂不删除旧适配器，但不得把临时字段扩散到工具契约。

### T02 资源接入模型与上下文解析

#### 目标

让 AIEngine 根据逻辑资源的 Direct/Agent 接入方式选择唯一工具来源，并把 MCPServer 关联纳入资源权限边界。

#### 实施范围

- 规范化 `subtype` 和 `agent_ref`；
- 校验 Direct 资源配置/凭据和 Agent 关联 MCPServer；
- Context Resolver 接入 Direct ToolSet Resolver 和 MCP Provider；
- 工具注册以逻辑资源 ID 隔离同名工具；
- 统一权限、状态、取消、超时和审计上下文。

#### 验收标准

- Direct 资源不读取 MCPServer；Agent 资源不直接读取 Direct 凭据；
- 关联 MCPServer 必须是活动、可见且允许使用的 MCPServer 资源；
- 资源权限不足时不发起外部连接；
- Direct/Agent 不自动互相回退；
- 同名工具在多个资源之间不会串用连接或结果。

#### 风险和回滚

现有资源使用 `subtype` 表达 Direct/Agent，迁移时可能存在字段不一致。先建立单一解析函数和数据迁移校验，发现无法转换的资源置为明确错误，不在运行时同时维护两个权威字段。

### T03 Docker 工具集统一

#### 目标

以现有 Docker MCP Server 工具为基准，使其同一套实现同时服务 MCP 和 Direct。

#### 实施范围

- 迁移 `docker_info`、`docker_images`、`docker_containers`、`docker_container_logs`、`docker_container_inspect`、`docker_container_stats` 的业务实现；
- 保持现有业务入参、日志 keyword 过滤语义、结果字段和限制；
- Docker MCP Server 只保留 MCP 注册、连接配置注入和结果编码；
- 增加 Docker Direct 工具集和资源连接测试；
- 删除 Connector 或其他位置的重复 Docker 业务逻辑。

#### 验收标准

- MCP 和 Direct 使用相同工具名、Schema、业务结果和错误语义；
- `docker_container_logs` 的 `&`/`|` 多关键字支持在两条路径一致；
- Direct 不能被模型传入的 Docker host 或证书字段改变目标；
- 真实 Docker Engine 完成工具调用和审计验证。

#### 风险和回滚

Docker SDK、MCP SDK 和 AIEngine 当前类型不同。采用公共业务调用加两个薄适配器，先保留旧入口直到一致性测试通过，再删除重复实现。

### T04 Host 工具集接入

#### 目标

为 Host 建立首个没有现有重复 MCP 实现的 Direct 工具集，并验证 Agent 资源可以使用任意兼容 MCP Server 的远程工具。

#### 实施范围

- 定义受限的 Host 只读工具和连接配置；
- Direct 读取 Host 资源配置/凭据创建连接；
- Agent 通过 MCPServer 发现 Host 工具，不要求本地 Host 契约匹配；
- 连接测试、超时、输出限制和错误展示。

#### 验收标准

- Host 工具不能执行任意 Shell 或模型提供的命令；
- Direct 和 Agent 的资源授权、审计和取消行为一致；
- 连接信息不进入模型可控参数。

#### 风险和回滚

主机工具容易越界为任意命令执行。首批只实现明确的只读 API；未形成稳定边界前不接入 Shell、进程信号或文件写操作。

### T05 Kubernetes 工具集统一

#### 目标

以现有 Kubernetes MCP Server 工具为基准，实现 Kubernetes Direct/Agent 共用工具业务实现。

#### 实施范围

- 迁移集群信息、API 资源、Namespace、Node、Pod、工作负载、Service、ConfigMap、Ingress、Event、Pod 日志、资源读取和健康检查；
- 保持资源 allowlist、namespace、limit、continue、日志截断和敏感对象限制；
- MCP Server 只负责 MCP 输入输出和连接注入；
- Direct 从资源配置和凭据创建 Kubernetes client；
- 接入现有 Kubernetes discovery 和诊断证据。

#### 验收标准

- 两条路径的工具业务参数和结果字段一致；
- Secret 等禁止资源在 Direct 和 MCP 均被拒绝；
- 真实 Kubernetes 集群完成列表、日志和资源读取验证；
- 工具调用仍关联逻辑 Kubernetes 资源，而不是只记录 MCPServer。

#### 风险和回滚

Kubernetes 客户端配置包含 kubeconfig、Token 和证书，必须在适配器边界注入并脱敏；迁移期间先保留旧 MCP Handler，公共实现验证后再替换。

### T06 Application 资源接入

#### 目标

将 Application 作为项目级聚合资源纳入统一接入流程，以虚拟机、容器化或云原生之一关联现有 Host、Docker 或 Kubernetes 资源，并提供固定实例范围内的状态与日志读取能力。

#### 实施范围

- Application 必须归属于 Project Scope；从平台或团队 Scope 创建时显式选择团队和项目；
- 虚拟机实例通过 Host 进程关键字表达式关联（支持 `&`、`|`、逗号/空格隐式 AND 和引号保护），可读取 Host 路径或 Loki 查询；
- 容器化实例通过容器名称关联 Docker，可读取容器路径、默认标准输出或 Loki 查询；
- 云原生实例通过 namespace、合并显示的 `workload kind · name` 关联 Kubernetes，按 workload selector 解析 Pod 后读取路径或标准输出，也可使用 Loki 查询；
- 连接验证必须确认每个实例目标唯一存在；实例工具固定目标参数，不允许模型重新选择底层资源；
- Application 不新增 MCP 传输资源，采用 Application 语义工具复用底层公共只读工具的混合设计。

#### 验收标准

- 后端拒绝非项目 Scope、非法接入方式、缺少实例、错误资源类型、重复资源关联和不完整定位信息；
- 三种接入方式均可配置多个实例，虚拟机关键字交集恰好匹配一个进程，Docker 容器名称恰好匹配一个容器，Kubernetes workload 可解析到受控 Pod；
- `application_instances` 和 `application_logs` 以 Application ID 注册，日志路径、查询资源、tail 和 timestamps 受限，响应支持 partial/errors；
- 前端在非项目 Scope 下要求团队和项目，并提供结构化实例表单；
- Application 可关联 Direct 或 Agent 类型的 Host、Docker、Kubernetes 和 Loki；Application 仅固定资源 ID 与业务目标参数，由统一调用器按关联资源的接入方式选择唯一 Provider。Agent 缺少所需工具时明确失败，绝不回退到 Direct。

#### 风险和回滚

Kubernetes workload 可能对应多个 Pod 或短生命周期 Job。工具限制 Pod 数量并返回 partial；无法解析 selector 时连接检查失败。若 Application 工具未达到固定目标边界，保留底层工具而暂不开放 Application 工具。

### T07 PostgreSQL 工具集统一

#### 目标

将现有 PostgreSQL 只读诊断能力抽为公共工具，实现 Direct 和 MCP 的同一调用语义。

#### 实施范围

- 连接、会话、长查询、锁、复制、容量和健康诊断工具；
- 复用现有只读 SQL、结果规范化、限制和敏感字段清理；
- Direct 使用资源 config 与 credential；Agent 使用关联 MCPServer；
- 禁止模型传入任意 SQL。

#### 验收标准

- 工具只执行内置、受审核的只读查询；
- Direct/MCP 结果字段和错误分类一致；
- 密码、连接串和 SQL 中的敏感值不会进入事件或前端；
- 真实 PostgreSQL 完成连接和诊断调用。

#### 风险和回滚

查询版本和权限差异可能导致部分指标不可用。每项能力返回明确 unavailable/partial 信息，不以扩大数据库权限或开放任意 SQL 解决。

### T08 Redis 工具集统一

#### 目标

将现有 Redis 只读诊断能力抽为公共工具，实现 Direct 和 MCP 的同一调用语义。

#### 实施范围

- 连接、健康、内存、客户端、复制、慢日志和数据库信息工具，共 6 项固定只读工具；
- 保持当前不采样全量 Key、不执行任意 Redis 命令的安全边界；
- Direct 使用资源 config 与 credential；Agent 使用关联 MCPServer；
- 对不可用的 hot key 等能力返回明确说明。

#### 验收标准

- 工具不会执行模型提供的任意 Redis 命令；
- Direct/MCP 使用相同工具名、Schema、DTO 和错误语义；
- 真实 Redis 完成连接、诊断、超时和取消验证。

#### 风险和回滚

Redis 版本和权限会影响诊断能力。工具仅调用固定 INFO、PING、DBSIZE 和 SLOWLOG GET 20，不扫描全量 Key、不执行任意命令。

### T09 Nacos 工具集统一

#### 目标

新增 Nacos 资源，并将服务注册中心、配置中心和命名空间等只读 API 能力抽取为协议无关工具，实现 Direct 与 Agent/MCP 统一调用。

#### 实施范围

- Nacos 资源支持 Direct/Agent 子类型和统一连接配置（地址、协议、上下文路径、认证和超时）。
- 固定只读工具包括服务列表、服务实例、命名空间、配置元数据、指定配置详情和服务端状态；分页和结果大小必须受控。
- Direct Provider 与 Nacos MCP Server 复用同一公共工具包，Agent 通过 `agent_ref` 关联 MCPServer，模型不可覆盖服务端注入的连接字段。
- 不提供任意 URL、任意 HTTP 方法或配置写入能力。

#### 验收标准

- Nacos Direct/Agent 资源可创建、编辑、连接测试，并在资源详情中展示统一工具集。
- API 请求路径固定、分页有界、错误语义一致，禁止配置发布、删除或任意 API 调用。
- 前端资源目录顺序调整为 Application、Artifact、Repository、Host、Docker、Kubernetes、Nacos、Nginx、TongHttpServer、PostgreSQL、Oracle、MySQL、OceanBase、Redis、TongRDS、Kafka、RabbitMQ、ElasticSearch、LLM、MCPServer、Skill、Monitor。

### T10 AIEngine 与证据链收敛

#### 目标

让统一工具集完整进入 AIEngine 的上下文、工具调用、证据、事件和审计链路。

#### 实施范围

- Direct ToolSet Provider 和 MCP Provider 统一注册到 Context Resolver；
- 保持 `(resource_id, tool_name)` 注册隔离和模型别名规则；
- 工具结果统一进入 ToolResult、Observation 和 Evidence；
- 统一工具生命周期事件、脱敏、响应大小、取消和错误展示；
- 诊断后续轮次复用同一逻辑资源上下文，不重复回放历史工具结果。

#### 验收标准

- 一个用户问题最多产生一套工具调用和一条对应证据链；
- Direct/MCP 工具均能在 SSE 和持久化事件中追踪；
- 权限撤销、资源停用和请求取消均在下一次工具调用前生效；
- 工具失败不会被伪造成资源健康结论。

#### 风险和回滚

工具事件和诊断事件已有历史边界。先增加适配层测试并复用既有 Gateway，不修改对话渲染协议；发现事件重复时优先删除旁路发射路径。

### T10 Kafka、Prometheus、Loki 迁移

#### 目标

将现有 Connector 能力迁移到公共工具集，并按 Docker/Kubernetes 的模式支持 Direct/Agent。

#### 实施范围

- Kafka Broker、Topic、分区、ISR、消费组和积压只读工具；
- Prometheus 指标和告警查询工具；
- Loki 日志查询工具，保留时间范围、limit 和关键字限制；
- 删除 `connector.*` 作为模型公开工具名的依赖。

#### 验收标准

- 每类资源均能按接入方式选择唯一工具路径；
- 工具参数和查询边界经过公共实现统一；
- 真实或可复现的服务端测试覆盖成功、超时、权限和响应超限。

#### 风险和回滚

监控系统返回格式差异较大。先固定结构化结果和原始内容大小上限；旧 Connector 仅在单类工具迁移未通过时暂留，不引入第二套长期名称。

### T11 其他数据库和中间件迁移

#### 目标

按资源类型逐步接入 RabbitMQ、Elasticsearch、MySQL、Oracle、OceanBase 和 TongRDS。

#### 实施范围

- 每类资源先完成健康/连接和一组最小只读诊断工具；
- 按资源类型复用公共客户端、结果和错误边界；
- 为没有项目内 MCP Server 的资源验证外部 MCP Agent 路径；
- 记录版本差异和不可用能力。

#### 验收标准

- 每类资源都有明确的 Direct 配置、凭据和工具清单；
- Agent 不要求本地实现存在，只按远端发现结果调用；
- 低优先级资源未完成部分有明确后续迭代记录。

#### 风险和回滚

不同数据库驱动和权限模型差异可能扩大范围。按资源类型独立验收，任何一类未达到边界不阻塞已完成资源，但不得把未完成能力标记为已支持。

### T12 管理界面与接入校验

#### 目标

让用户能够清晰配置 Direct 连接或关联 Agent MCPServer，并看到接入方式和失败原因。

#### 实施范围

- 资源表单显示接入方式；
- Direct 表单收集非敏感配置并关联凭据；
- Agent 表单选择当前 Scope 可见的 MCPServer；
- 接入方式切换时清理无效关联并校验必填项；
- 连接测试显示 Direct/MCP、逻辑资源和传输资源的实际结果。

#### 验收标准

- 用户不能选择不可见或非 MCPServer 的关联资源；
- 凭据明文不回显；
- 工具发现失败、工具不可用和目标连接失败可区分显示；
- 前端工具列表使用远端发现或本地工具集的实际名称，不硬编码第二套目录。

#### 风险和回滚

前端旧 `subtype` 字段和后端新接入字段可能短期并存。表单统一调用后端解析结果，迁移完成后删除前端兼容分支。

### T13 删除旧路径与全量验收

#### 目标

清理重复实现和旧工具名，确认统一接入在代码、数据和文档中成为唯一运行路径。

#### 实施范围

- 删除重复 Docker/Kubernetes MCP 业务代码和已迁移 Connector 工具；
- 删除 managed/external MCP 分支和无调用方的旧注册；
- 完成资源配置迁移和无效关联报告；
- 执行后端全量测试、竞态测试、API 测试、前端检查和真实资源集成测试；
- 更新设计、操作指南、资源表单和已知限制文档。

#### 验收标准

- 全仓库活动代码不再存在同一工具的两份业务实现；
- Docker、Kubernetes、Host、PostgreSQL、Redis Direct/Agent 测试全部通过；
- 资源权限、凭据、错误、证据和审计回归通过；
- 需求验收报告给出命令、环境、提交和剩余事项。

#### 风险和回滚

删除旧路径是不可逆的代码变更。必须先完成工具清单和调用方扫描、迁移测试及数据备份；发现未迁移调用方时先修复引用，不通过保留隐藏的运行时双轨来规避问题。

## 7. 需求级验收标准

- 用户可以为 Host、Docker、Kubernetes、PostgreSQL 和 Redis 选择 Direct 或 Agent 接入；
- Direct 资源自动加载对应内置工具集，Agent 资源自动发现关联 MCPServer 工具；
- 两种路径不需要改变公共工具的业务入参和出参；
- 项目提供的 Docker/Kubernetes MCP Server 和任意外部 MCP Server 在 AIEngine 中使用同一 MCP 调用流程；
- 远端工具无需通过本项目的自定义契约元数据校验，但仍受资源权限、工具白名单、超时、响应限制和审计保护；
- 工具实现、结果、错误和限制在 Direct/MCP 之间保持一致；
- 未完成资源、驱动限制和外部服务依赖均在验收报告或 backlog 中明确记录；
- 后端和前端质量门禁通过，且没有遗留未授权的凭据暴露或任意命令执行路径。

## 8. 变更记录

| 日期 | 变更 |
|---|---|
| 2026-09-06 | 初版：根据统一资源工具和 Direct/Agent 接入设计建立 I004。 |
