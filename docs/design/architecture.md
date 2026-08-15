# OpsKeeper 总体架构设计

## 文档状态

本文同时维护已经验收的架构事实和已经确定的目标边界，但二者必须明确区分：

- **当前实现**：截至 T02 验收后，仓库中已经存在并通过验收的能力。
- **目标设计**：尚未实现的设计约束，必须标注对应任务编号；任务获批前不代表已经交付。
- 任务状态、实施细节和验收边界以[分阶段实施任务书](../planning/implementation-tasks.md)为准。

## 1. 建设目标

OpsKeeper 以业务项目为最终运维对象，以团队为基础设施和共享中间件的管理边界，以平台为公共能力和治理边界，为线上服务提供资源管理、自动发现、智能诊断、自动巡检和运维审计能力。

平台重点管理：

- 部署在 Kubernetes 上的业务应用和工作负载。
- 部署在任意环境中的 PostgreSQL、Redis、Kafka 等中间件。
- Prometheus、Loki、Tempo、Elastic 等外部可观测平台。
- LLM Provider、模型、MCP Server、Skill 等 AI 运维能力。
- 平台、团队和项目三级用户、角色及授权关系。

首版按单平台部署设计，数据模型预留 `tenant_id`，但不在首版暴露多租户能力。

## 2. 组织层级

```text
Platform
├── Team A
│   ├── Project A1
│   └── Project A2
└── Team B
    └── Project B1
```

- 平台：全局治理边界，承载公共 Skill、模型、监控平台和平台管理员。
- 团队：人员及基础设施共享边界，通常拥有独立 Kubernetes 集群、共享中间件和监控数据源。
- 项目：业务系统边界，承载业务应用、服务、工作负载及其依赖关系。
- 一个项目只能直属一个团队。资源归属层级由资源实例的共享范围决定，而不是由资源类型硬编码决定。

## 3. 目标总体架构

下图描述完整目标形态。当前已经实现 Browser、嵌入式 Web、Go API、PostgreSQL、Redis 和 Organization；其他节点按 T03-T15 逐步交付。

```mermaid
flowchart LR
    UI[Browser] <--> API[Go API Server + Embedded Svelte Web]
    API --> Auth[Scope RBAC]
    API --> Catalog[Resource Catalog]
    API --> AI[AI Orchestrator]
    API --> PG[(PostgreSQL)]
    API --> Redis[(Redis)]
    Scheduler[Inspection Scheduler] --> Jobs[PostgreSQL Job Queue]
    Jobs --> Worker[Worker Pool]
    Worker --> Discovery[Kubernetes Discovery]
    Worker --> Skills[Skill Runner]
    Skills --> K8S[Kubernetes APIs]
    Skills --> Middleware[Middleware APIs]
    Skills --> Observability[Metrics Logs Traces]
    Skills --> MCP[MCP Servers]
    AI --> LLM[LLM Providers]
    AI --> Skills
    Worker --> PG
```

## 4. 技术架构

| 层次 | 技术选型 | 状态 | 说明 |
|---|---|---|---|
| 前端基础 | Svelte 5、TypeScript、Vite | 已实现，T01-T02 | 独立开发，生产时嵌入 Go API |
| 前端数据访问 | TanStack Query | 目标，T07 | 管理控制台的数据获取和缓存 |
| 后端基础 | Go、`chi`、`pgx` | 已实现，T01-T02 | REST API、健康检查、迁移和组织模型 |
| 查询生成 | `sqlc` | 候选，按真实复杂度引入 | 不作为所有 Store 的强制前置条件 |
| Kubernetes 客户端 | `client-go` | 目标，T08 | 集群发现、导入和同步 |
| 数据库 | PostgreSQL 16 | 已实现，T01-T02 | 当前保存组织数据，后续扩展资源、任务、证据和审计 |
| 缓存 | Redis 7 | 已接入健康检查；业务用途未实现 | 目标用于缓存、限流和可恢复短期状态 |
| 实时交互 | SSE | 目标，T11 | 推送诊断过程、工具调用和巡检进度 |
| 日志 | `slog` text/json | 已实现，T01 | 全部 Go 进程统一结构化字段 |
| 指标和链路 | OpenTelemetry | 目标，随业务增量接入，T15 完整验收 | 平台自身日志、指标和链路 |
| 本地部署 | Docker Compose | 已实现，T01 | PostgreSQL 和 Redis 开发环境 |
| 生产部署 | 单镜像、Kubernetes、Helm、Ingress | 单镜像已实现；集群部署目标为 T15 | API 和 Worker 可独立扩容 |

目标任务引擎使用 PostgreSQL Job 表和 `FOR UPDATE SKIP LOCKED`，确保 Redis 故障不会造成任务丢失，在 T10-T13 实施。规模确实超过数据库任务队列边界后，再评估 Temporal，不将其作为当前预设依赖。

## 5. 后端模块

首版采用模块化单体和独立 Worker，不提前拆分微服务：

| 模块 | 核心职责 | 状态 |
|---|---|---|
| Organization | 平台、团队、项目及 Scope 关系 | 已实现，T02 |
| Identity | 用户、凭据、登录、会话和身份同步 | T03 正在实施基础登录与会话；用户组和同步目标为 T05 |
| Authorization | 三级 RBAC、权限继承和数据范围校验 | 目标，T04/T05 |
| Resource Catalog | 资源、凭据、关系、标签、状态和拓扑查询 | 目标，T06 |
| Discovery | Kubernetes 发现、差异预览、导入和周期同步 | 目标，T08 |
| Connector | Kubernetes、中间件、监控平台和 LLM Provider 适配 | 目标，T09-T10 |
| Skill Registry | Skill 定义、版本、能力要求和权限声明 | 目标，T10 |
| AI Orchestrator | 上下文构建、Skill 选择、工具编排和证据归纳 | 目标，T10-T11 |
| Diagnosis | 对话会话、消息、工具调用、假设和诊断报告 | 目标，T11 |
| Inspection | 巡检策略、调度、执行、评分和异常项 | 目标，T13 |
| Notification | Webhook、邮件及其他通知渠道 | 目标，T13 |
| Audit | 管理操作、模型调用、工具调用和审批记录 | 基础目标 T05，完整目标 T15 |

后端代码默认按业务特性组织。每个特性包拥有自身的类型、业务规则、持久化实现和测试，并在同包内通过文件划分职责；不会把所有业务的数据库实现集中到全局 adapter 层。数据库连接生命周期、缓存客户端、消息传输和外部平台协议等真正跨特性的能力，才建立独立基础设施包。

包之间优先直接依赖清晰的具体类型。只有消费者需要测试替身、已经存在多个真实实现或边界需要稳定契约时，才定义小接口；不要求所有模块机械地通过接口调用。耗时任务进入 Worker，普通资源查询和管理请求由 API Server 直接处理。

完整约束见 [Go 编码与工程组织通用规范](../standards/go-coding-conventions.md)。

## 6. 目标前端信息架构

当前前端只有工程骨架和健康状态界面。以下业务页面均为目标设计，从 T07 开始按对应后端能力逐步交付。

| 页面 | 主要能力 |
|---|---|
| 平台总览 | 全局健康状态、异常趋势、团队分布和任务状态 |
| 团队工作台 | 团队资源、集群、中间件、项目及共享关系 |
| 项目工作台 | 应用拓扑、依赖、诊断入口、巡检结果和负责人 |
| 资源中心 | 按平台/团队/项目范围浏览、创建、测试和同步资源 |
| 集群导入 | Namespace 扫描、项目映射、资源预览和确认导入 |
| AI 诊断 | 流式对话、资源选择、执行时间线、证据和审批 |
| Skill 中心 | Skill 版本、适用目标、工具权限和测试记录 |
| 巡检中心 | 策略、执行历史、异常、健康评分和通知 |
| 权限中心 | 平台、团队、项目角色和成员授权 |
| 审计中心 | 登录、资源变更、诊断、工具执行和审批日志 |

用户进入团队或项目后，前端必须显示当前作用域，所有资源选择器默认限定在当前作用域可见范围内。

## 7. 核心 API 边界

所有公开页面、健康检查和业务 API 统一挂载在 `OPSK_BASE_PATH` 下，默认值是 `/opskeeper`，也可以设置为根路径 `/`。以下示例使用默认值：

当前已实现健康检查和 Team/Project 组织 API。目标 API 按任务逐步增加：

| 路径示例 | 状态 |
|---|---|
| `/opskeeper/health/live`、`/opskeeper/health/ready` | 已实现，T01 |
| `/opskeeper/api/v1/teams`、`/opskeeper/api/v1/teams/{teamId}/projects` | 已实现，T02 |
| `/opskeeper/api/v1/auth/login`、`/opskeeper/api/v1/auth/refresh`、`/opskeeper/api/v1/auth/logout`、`/opskeeper/api/v1/auth/me` | 实施中，T03 |
| `/opskeeper/api/v1/role-bindings`、`/opskeeper/api/v1/audit-logs` | 目标，T04-T05 |
| `/opskeeper/api/v1/platform/resources`、`/opskeeper/api/v1/resources/{resourceId}/relations` | 目标，T06 |
| `/opskeeper/api/v1/kubernetes-clusters/{id}/discoveries`、`/opskeeper/api/v1/discoveries/{id}/imports` | 目标，T08 |
| `/opskeeper/api/v1/diagnosis-sessions`、`/opskeeper/api/v1/diagnosis-sessions/{id}/events` | 目标，T11 |
| `/opskeeper/api/v1/inspection-policies`、`/opskeeper/api/v1/inspection-runs` | 目标，T13 |
| `/opskeeper/api/v1/skills` | 目标，T10 |

资源 ID 不代表访问权限。每个读取和写入请求都必须根据资源的实际作用域重新执行授权判断，禁止仅依赖前端或 URL 中的团队、项目参数。

## 8. 部署与可靠性

### 当前实现

- 前后端源码和依赖保持独立；本地既可使用 Go API 与 Vite 分离热更新，也可通过 `make run-front-api` 构建并嵌入前端后只运行 API。生产构建同样将 Vite 静态制品嵌入 `opskeeper-api`，由一个进程提供页面、静态资源和业务 API。
- 应用临时运行、二进制构建和最终镜像打包统一由根目录 Makefile 提供入口，不使用独立包装脚本。
- API、Worker、Scheduler 和 Migration 的服务名称及镜像内二进制文件名固定为 `opskeeper-api`、`opskeeper-worker`、`opskeeper-scheduler` 和 `opskeeper-migrate`；`OPSK_BASE_PATH` 只控制 API 的 HTTP 路径前缀。
- API、Worker、Scheduler 和 Migration 的日志格式由 `OPSK_LOG_FORMAT` 统一控制，支持 `text` 和 `json`，默认使用 `text`。
- API 默认不信任客户端转发头；只有 `OPSK_TRUSTED_PROXIES` 明确列出的直连代理才能提供客户端 IP，解析结果写入请求日志并供后续审计使用。
- 数据库迁移由滚动发布前的单实例 Migration Job 执行，使用 PostgreSQL advisory lock 防止并发；应用进程不自动迁移，Schema 演进遵循 Expand/Contract。

### 目标设计

- API Server 保持无状态，生产环境至少两个副本，在 T15 完成集群部署验收。
- Scheduler 在 T13 使用 PostgreSQL advisory lock 保证单一调度主节点。
- Worker 在 T10-T13 使用任务租约、心跳和幂等键恢复中断任务。
- 外部调用从各 Connector 引入时实施超时、限流、重试、熔断和最大响应体限制。
- PostgreSQL 备份、PITR 和高可用以及 Redis 可恢复数据边界在 T15 验收。
- 诊断、巡检和工具执行从 T11 起保存输入摘要、证据、版本和结果，支持复现。

### 数据库权限边界

- PostgreSQL Cluster 管理员与 OpsKeeper 业务数据库所有者必须是不同角色。管理员负责实例、角色和数据库生命周期，业务角色只拥有应用数据库，不具备超级用户、创建数据库或创建角色权限。
- 容器首次初始化时，`POSTGRES_*` 只定义基础设施管理员和管理连接数据库，`OPSK_DB_*` 由项目初始化脚本用于创建受限的业务角色和业务数据库。
- API、Worker、Scheduler 和数据库迁移只允许使用业务角色连接，不得注入或回退使用 PostgreSQL 超级用户凭据。
- 初始化环境变量和 `/docker-entrypoint-initdb.d/` 脚本只对空数据目录生效；已有实例的角色、密码和所有权变更必须通过受控的数据库管理操作完成。

数据库迁移和自动化发布的完整流程见[自动化发布](../guides/delivery.md)。

## 9. 交付阶段

交付顺序、审批状态和验收标准统一维护在[分阶段实施任务书](../planning/implementation-tasks.md)，本文不再维护另一套阶段编号。
