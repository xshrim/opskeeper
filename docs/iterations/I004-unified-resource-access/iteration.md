# I004 统一资源接入迭代

**迭代编号：** I004  
**目录标识：** I004-unified-resource-access  
**状态：** 实施中  
**计划开始：** 待定  
**计划封板：** 待定

## 1. 迭代目标

建立统一资源接入机制，使同一套资源工具可以同时用于 Direct 内置工具集和 MCP Server 工具。用户选择资源作为 AIEngine 上下文后，系统根据资源接入方式自动选择直接连接或 MCP 工具发现，不再维护重复的 Connector 工具和 MCP Server 业务实现。

本迭代优先完成 Host、Docker、Kubernetes 和 Application 的完整闭环，再按相同机制扩展 PostgreSQL、Redis、Kafka、Prometheus、Loki 及其他数据库和中间件。

## 2. 迭代范围

- 建立协议无关的公共资源工具层、工具注册和统一结果/错误边界；
- 在资源模型中明确 Direct/Agent 接入方式及 Agent 对应的 MCPServer；
- 将 Host、Docker、Kubernetes、Application、PostgreSQL、Redis 接入同一套 Direct/Agent 解析流程；
- 将已有 Docker、Kubernetes MCP Server 改为公共工具层的薄适配器；
- 让 AIEngine 按逻辑资源的接入方式注册内置工具或远程 MCP 工具；
- 保持资源权限、凭据隔离、超时、取消、响应限制和审计边界；
- 建立 Direct/MCP 一致性测试和真实资源集成验证；
- 按优先级迁移 Kafka、Prometheus、Loki、RabbitMQ、Elasticsearch、MySQL、Oracle、OceanBase、TongRDS 等资源。

不在本迭代范围内：把 MCP Server 分成 managed/external 两种运行时类型；让模型指定连接地址或凭据；开放任意命令、写 SQL 或未审批的资源变更；把 AIProvider、MCPServer、Skill、AgentProfile 当作诊断资源工具集；为兼容旧工具名长期保留重复实现。

## 3. 需求清单

| 需求 | 名称 | 任务 | 状态 | 需求文档 | 验收报告 |
|---|---|---:|---|---|---|
| R001 | 统一资源接入 | T01-T13 | 实施中 | [R001-requirement.md](R001-requirement.md) | [R001-requirement-acceptance.md](R001-requirement-acceptance.md) |

## 4. 需求和任务总览

| 任务 | 名称 | 优先级 | 依赖 | 主要产出 | 状态 |
|---|---|---:|---|---|---|
| T01 | 公共工具契约与执行基础 | P0 | 无 | 协议无关的工具定义、连接上下文、结果/错误模型和注册表 | 已完成 |
| T02 | 资源接入模型与上下文解析 | P0 | T01 | `subtype`、`agent_ref`、Direct/Agent 选择和统一授权入口 | 已完成 |
| T03 | Docker 工具集统一 | P0 | T01-T02 | 迁移现有 Docker MCP 工具，增加 Direct 工具集并删除重复实现 | 已完成 |
| T04 | Host 工具集接入 | P0 | T01-T02 | Host Direct 工具集、Agent MCP 代理和连接测试 | 待批准 |
| T05 | Kubernetes 工具集统一 | P0 | T01-T02 | 迁移现有 Kubernetes MCP 工具，接入 Direct 工具集和资源连接配置 | 已完成 |
| T06 | Application 资源接入 | P0 | T01-T05 | 项目归属、虚拟机/容器化/云原生实例关联、日志读取和唯一性验证 | 已完成 |
| T07 | PostgreSQL 工具集统一 | P0 | T01-T02 | 固定诊断快照及后续只读工具共用 Direct/MCP 实现 | 待批准 |
| T08 | Redis 工具集统一 | P0 | T01-T02 | Redis 连接、状态和诊断工具共用 Direct/MCP 实现 | 待批准 |
| T09 | AIEngine 与证据链收敛 | P0 | T03-T08 | 工具注册、别名、证据、事件、审计和错误统一 | 待批准 |
| T10 | Kafka、Prometheus、Loki 迁移 | P1 | T01-T02、T09 | 中间件和可观测工具集按同一机制接入 | 待批准 |
| T11 | 其他数据库和中间件迁移 | P2 | T10 | RabbitMQ、Elasticsearch、MySQL、Oracle、OceanBase、TongRDS | 待批准 |
| T12 | 管理界面与接入校验 | P1 | T02-T09 | Direct/Agent 配置、MCPServer 关联、连接测试和错误展示 | 待批准 |
| T13 | 删除旧路径与全量验收 | P0 | T03-T12 | 删除重复实现、完成迁移、测试和文档验收 | 待批准 |

## 5. 进入条件

- I003 AIEngine 执行内核、上下文工具层、权限和审计能力可作为统一运行时基础；
- 当前 Docker、Kubernetes MCP Server、Connector 和资源服务的参数、结果、凭据读取方式已盘点；
- 明确首批测试环境：Docker Engine、Kubernetes API、PostgreSQL 和 Redis；
- 任何工具迁移前先列出原工具名、Schema、结果字段、错误和限制，迁移后通过一致性测试确认；
- 新增或迁移的工具默认只读；写操作必须另行进入受控操作迭代。

## 6. 退出条件

- Host、Docker、Kubernetes、Application、PostgreSQL 和 Redis 均支持 Direct 和 Agent 两种接入方式；
- 同一资源工具在 Direct 和 MCP 路径使用一致的工具名、业务参数、结果字段和错误语义；
- AIEngine 根据 `subtype` 只选择一条执行路径，不发生 Direct/Agent 自动回退或双重调用；
- MCPServer 不再按 managed/external 分支，项目提供的 MCP Server 与外部 MCP Server 使用同一 MCP 调用路径；
- 资源权限、凭据隔离、超时、取消、响应限制、脱敏和审计在两条路径均通过测试；
- 旧 `connector.*` 资源业务工具和 MCP Server 中的重复实现已删除或明确迁移；
- 后端单元、集成、竞态、API 和必要的前端测试通过，验收报告记录可复核证据；
- 未完成的低优先级资源已明确转入后续迭代或 backlog。

## 7. 封板记录

### T05 完成记录

- **确认日期：** 2026-09-10
- **用户确认：** I004-T05 Kubernetes 工具集统一已完成。
- **合并基线：** `main` @ `86c3714`
- **验证证据：** Go 全量测试、前端测试、`svelte-check`、生产构建和差异检查均通过。
- **遗留事项：** I004 其他任务按任务表继续实施，不影响 T05 完成状态。

### T06 完成记录

- **确认日期：** 2026-09-12
- **用户确认：** I004-T06 Application 资源接入验收通过。
- **合并基线：** 当前分支 `codex/t06-application-resource`，未提交工作区。
- **验证证据：** `cd backend && go test ./...`、`cd frontend && npm run check && npm run test -- --run`、`cd frontend && npm run build`、`git diff --check` 通过；Application 关联 Direct/Agent Host、Docker、Kubernetes、Loki 的透明调用与无回退回归通过。
- **验收范围：** 项目归属、三种实例模式、多实例、Host 表达式及唯一性、Docker 容器唯一性、Kubernetes workload 到 Pod 解析、固定 Application 工具边界、关联资源的 Direct/Agent 透明执行、受控候选发现。
- **已知边界：** 未运行的 Kubernetes workload（包括零副本和无当前 Pod 的 Job/CronJob）会导致连接校验失败；这是当前状态验证要求 Instance 存在的结果。
