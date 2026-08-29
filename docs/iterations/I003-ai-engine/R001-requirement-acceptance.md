# I003-R001 AIEngine 执行框架需求验收报告

**迭代：** I003-ai-engine  
**需求：** R001 AIEngine 执行框架设计与实现  
**状态：** 实施中（T01、T02、T03 已完成）

本报告将在 T01-T05 完成后填写，至少记录：

- 每个任务的提交基线和实际变更文件；
- AIEngine、AIProvider、Tool Gateway、Skill、Agent、知识库和工作流的集成验证；
- PostgreSQL、Connector、远程 MCP Server 的工具调用验证；
- 流式输出、取消、超时、断线续读和断点恢复验证；
- 入参出参脱敏、权限校验、审计和错误原因验证；
- 后端测试、API 测试、前端交互测试和遗留问题。

## T01 AIEngine 统一执行内核

**状态：** 已完成

**验收结论：** 通过（2026-08-28）

本任务已建立 `backend/aiengine` 统一执行契约和运行时，并将 Skill Runner 接入统一执行路径。

实现范围：

- `Request`、`Result`、`Event`、`Runner`、`StreamingRunner` 和 `Engine` 接口；
- `interactive`、`diagnosis`、`skill`、`inspection`、`workflow` 五种执行 Profile；
- 默认循环、Tool Call、Token、输出大小和执行时长预算，并限制最大超时时间；
- 同步执行、流式事件、执行 ID、生命周期事件、事件时间戳和单调序号；
- 执行取消、超时、重复执行 ID、Runner 不可用和错误状态映射；
- `skill.Runner.AIEngineAdapter()` 适配；新请求通过 `AIProviderResourceID` 和 `ModelName` 解析，并在执行开始固定 Provider 和最终模型。

验证记录：

- `cd backend && go test ./...`：通过；
- `cd backend && go test -race ./aiengine`：通过；
- `cd backend && go vet ./aiengine`：通过；
- `cd frontend && npm run check`：通过；
- `cd frontend && npm run build`：通过。

用户已确认执行 T01 验收。验收确认 T01 已满足统一执行内核的任务边界。T01 尚未引入数据库迁移、HTTP API 或工具网关；上下文资源解析、Connector/MCP Tool、Skill/Agent Profile、审计持久化和工作流编排分别在 T02-T05 实施。

### T01 补强：执行循环预算

2026-08-28 根据验收后的技术复核，补充落实 `MaxIterations` 的实际执行限制：Skill Runner 按 ADK 非 partial 模型回合计数，超过预算时返回 `iteration_budget`；AIEngine Skill 适配器会传递该预算。该补强不改变 T01 已验收边界，也不引入 T02 的模型解析或工具层职责。

## T02 上下文工具层

**状态：** 已完成

当前已完成的阶段性实现：

- `ToolRegistry`：按资源注册、查询和枚举工具定义；
- `PolicyGateway`：统一执行授权回调、并发限制、超时、响应大小限制和工具状态事件；
- `AuthorizeResourceUse`：复用现有 `ResourceFilter`/`ScopeFilter` 完成上下文资源使用授权；
- `ResourceContextResolver`：去重解析上下文资源、校验活动状态和授权，并将 Provider 产生的工具注册到 Registry；
- AIEngine Runtime wiring：执行开始后、Runner 调用前解析 Request.Context，传递 ResolvedContext/ToolGateway，并发出 `context.loaded` 或 `context_resolution` 错误；
- Skill Runner 适配：新请求使用 `AIProviderResourceID` 与 `ModelName`，由 AIProvider Resolver 解析最终模型；
- Connector Provider：为 Kubernetes、Prometheus、Loki、PostgreSQL、Redis 和 Kafka 复用既有受限 Connector 能力；
- MCP Provider：执行资源发现，转换白名单工具定义并通过既有 MCP Service 调用；MCP 工具默认不标记为只读，是否执行由策略层决定；
- 工具输入输出和外部资源结果均按不可信数据处理；共享 Runtime 重复解析资源时按资源和工具名幂等刷新注册。
- 应用接入：API 容器构造带 Connector/MCP Provider 的 ContextTooling 和 AIEngine Runtime；`/skill-executions` 在 Runtime 注入时经统一入口执行。Worker 的巡检解释步骤也优先经统一 Runtime 执行，并保留旧 Runner 回退。
- HTTP Handler：新增受认证和 `DiagnosisStart` 权限保护的 `POST /api/v1/ai-executions` 与 `POST /api/v1/ai-executions/{executionID}/cancel`，分别进入统一执行和取消流程；结构化输入、AIProvider、模型、上下文和预算均可由请求传递。
- 执行持久化：新增 `ai_execution_events`、`ai_execution_tool_calls` 迁移；Runtime 生命周期事件和 Skill Runner 工具调用写入 Store，工具参数与结果递归脱敏。
- SSE 续读：新增 `GET /api/v1/ai-executions/{executionID}/events`，支持 `Last-Event-ID`/`after` 游标并在断线后继续读取已持久化事件。
- Tool Call 审计查询：新增 `GET /api/v1/ai-executions/{executionID}/tool-calls`，仅返回递归脱敏后的参数、结果、状态和错误；查询受 `DiagnosisRead` 权限保护。

T02 的上下文解析、Connector/MCP 工具接入、权限策略和第一阶段审计链路已完成。更完整的事件快照、SSE 断线恢复语义和审计查询增强属于 T04 的范围，不作为 T02 的遗留阻塞项。

### T02 继续验收记录（2026-08-29）

- 使用管理员 `admin` 登录 API 成功（`/auth/login`、`/auth/me` 均为 HTTP 200）。
- 真实 Docker MCP Server `http://127.0.0.1:3100/mcp` 发现成功，返回 4 个工具：`docker:list_containers`、`docker:inspect_container`、`docker:container_logs`、`docker:container_stats`。
- 真实 MCP 工具调用成功，返回容器清单数据；MCP 工具名称带 `:` 的命名空间格式已验证可用。
- 真实 AIEngine 执行请求成功进入上下文解析：持久化 `context.loaded` 事件显示 1 个 MCP 资源、4 个工具和 1 个上下文事实。随后 AIProvider 上游因当前测试凭证无效返回 401，Runtime 保留实际错误原因并记录 `execution.failed`。
- 已修正 Skill Runner 适配缺口：`ResolvedContext.Tools` 现在通过统一 `ToolGateway` 注册为模型可调用的函数工具，并复用资源授权、并发/超时/响应限制、事件和审计路径。使用 `.env` 测试模型重试时，上游明确返回 `Function call is not supported for this model`，证明 MCP 函数声明已进入模型请求；该测试模型自身不支持函数调用，因此未执行实际 Tool Call。
- SSE 在完整 HTTP 中间件链下已验证可用；`after=0`、`after=2` 以及 `Last-Event-ID: 2` 均只返回游标之后的事件。此前的 `stream_unsupported` 是包装后的 `ResponseWriter` 未直接实现 `http.Flusher`，现已改用 `http.ResponseController` 兼容中间件包装。
- 未登录调用 AIEngine 返回 HTTP 401；跨 Scope 上下文资源不会泄露，返回资源不可见错误。
- `ai_execution_events` 已有真实生命周期记录；`ai_execution_tool_calls` 结构和查询 API 已验证，但当前测试模型不支持函数调用，尚未产生真实 Tool Call 审计行。待配置支持 Tool Calling 的 AIProvider 后补做该项，并核对参数/结果递归脱敏为 `[REDACTED]`。
- 失败结果事件类型已修正：Runner 返回失败结果时写入 `execution.failed`，而不是错误地写入 `execution.completed`；已补充单元回归测试。

本轮自动验证：`go test ./...`、目标包 `go test -race`、`go vet`、`git diff --check` 均通过。`make lint` 的唯一遗留是本机未安装 `helm`，与本任务代码无关。

### T01/T02 自动复验（2026-08-29）

- 后端全量 `go test -count=1 ./...`：通过。
- T01/T02 目标包 `go test -count=1` 和 `go test -race`（`aiengine`、`skill`、`httpapi`、`llm`、`mcp`、`connector`）：通过。
- 后端 `go vet ./...`：通过。
- I003 集成套件（迁移、组织、身份、授权、资源、发现、连接器、诊断、巡检、受控操作、E2E），使用 `-count=1` 强制重跑：全部通过。
- SiliconFlow 外部模型测试 `make llm-provider-test`：通过，普通和 SSE 两种模式均成功。
- 前端 `npm run test`、`npm run check`、`npm run build`：全部通过，Svelte 检查 0 错误、0 警告。
- 前端 Playwright `npm run test:e2e`：14 个场景全部通过，包含 AI 引擎菜单隐藏、Skill 页面和诊断工作台恢复场景。
- 全局旧概念扫描：活动后端 Go 代码、前端源码和 E2E 夹具无 `AIEndpoint`、`LLMEndpoint`、`LLMProvider`、`endpoint:manage` 或 `model:manage` 引用；迁移历史中的名称仅用于历史回滚和连续性。
- `make lint` 的 Helm 校验未执行成功，原因是测试环境未安装 `helm`（`helm: 未找到命令`）；其余 Go、前端和 shell 校验通过。

## T03 Skill 与 Agent Profile

**状态：** 已完成（2026-08-29）

本任务已完成第一版 Skill/Agent 组合执行链：

- 新增 `AgentProfile` Resource 类型及迁移 `0024_agent_profiles`；配置包含版本、专家指令、适用资源类型、模型能力要求、工具白名单和输入/输出 Schema；
- AgentProfile 复用 `resource:read`、`resource:use` 和 Scope 过滤，不新增绕过现有 RBAC 的权限路径；
- AIEngine Runtime 根据 `agent_profile_id` 在执行前解析并授权 Profile，发出 `agent_profile.resolved` 事件；Profile 不可用、未授权、禁用或契约非法时在模型调用前失败；
- 支持只使用 Skill、只使用 AgentProfile、以及 Skill + AgentProfile 组合；组合时专家指令和 Skill 指令按固定顺序编排，输入和输出 Schema 均校验；
- Profile 声明的模型能力与 Purpose 能力要求取并集，模型能力不足时返回缺失能力；Profile 工具白名单作为最终工具限制；
- Agent-only 执行不伪造 Skill 版本或 Skill 执行外键记录，但继续使用 AIEngine 的模型解析、上下文工具策略、生命周期事件和审计链路；
- `/skill-executions` 请求已支持传递 `agent_profile_id`，统一 AIEngine 路径优先执行。

验证记录：

- `cd backend && go test -count=1 ./...`：通过；
- AgentProfile 解析器测试：通过，覆盖有效配置、资源权限、禁用状态和工具契约校验；
- AIEngine Profile 注入测试：通过，确认 Profile 在 Runner 前解析并产生生命周期事件；
- Agent-only Skill Runner 测试：通过，确认无 Skill Resource 时可完成模型执行并返回结果；
- `0024_agent_profiles` 迁移加载及校验测试：通过。
- `0025_agent_profile_versions` 迁移加载、版本发布表结构和 Resolver 读取已发布版本测试：通过。
- AgentProfile 管理 API（创建/查询/发布/停用版本）已接入 Resource 权限边界；前端 Agent 专家管理页面 E2E：通过。

T03 增强项已完成：新增 AgentProfile 版本表、版本发布/停用 API、运行时优先读取已发布版本，并在前端增加 Agent 专家独立管理页面。Resource 元数据仍通过统一 Resource API 管理。

## T04 流式事件与工具调用审计

**状态：** 已完成（2026-08-29）

本任务已完成：Tool Call 审计记录实际 Provider、模型、开始/结束时间、耗时和错误码；审计入参、出参递归脱敏；SSE 在发送 `execution.completed`、`execution.failed` 或 `execution.cancelled` 后主动关闭连接，客户端可使用最后事件序号续读。并修正 Skill Runner 上下文工具回调返回值，确保 ADK 继续执行真实 MCP Tool，而不是仅增加调用计数。

本轮自动验证：`go test -count=1 ./...`、目标包 `go test -race`、`go vet ./...`、迁移集成测试、前端 `npm run check/test/build` 均通过。

## T05 知识库与工作流编排

**状态：** 实施中（执行闭环已接入，端到端验收待补）

已完成第一阶段：

- 新增 `KnowledgeQuery`、知识片段、引用和 `KnowledgeRetriever` 契约；资源型 KnowledgeBase 提供确定性的关键词检索基线，结果带不可信标记和引用信息。
- 新增 Workflow/WorkflowRun 类型、节点类型白名单、节点/边引用校验、DAG 无环校验、节点超时/重试边界和显式运行状态机。
- 新增迁移 `0027_knowledge_workflow`：注册 KnowledgeBase 与 Workflow Resource Schema，并创建 `ai_workflow_runs` 持久化运行快照表及索引、更新时间触发器。
- Workflow Resource 写入时即执行 DAG 基础校验；WorkflowRun 创建、查询、列表、状态迁移、暂停恢复和取消 API 复用 Scope/Resource 权限边界。
- 新增知识库检索 API：`POST /api/v1/knowledge-bases/{id}/search`。
- 新增 `WorkflowService`：Agent/Skill 节点统一构造 `aiengine.Request`，Tool 节点经过
  `PolicyGateway`，Retrieval 节点经过 `KnowledgeRetriever`；`start` 和审批后的 `resume`
  现在会真正执行节点并返回最新运行状态。
- 支持并行节点的受控分支执行（最多 32 个声明式分支），节点超时、有限重试和取消会传播到
  分支；工作流事件写入 AIEngine 事件存储，包含开始、节点开始/完成/失败、审批等待、终态。
- 新增 `GET /api/v1/workflow-runs/{id}/events`，按运行 ID 直接提供与 AIEngine 执行事件一致的
  SSE 续读能力，支持 `after` 和 `Last-Event-ID`。

验证记录：

- `cd backend && go test -count=1 ./...`：通过；
- `go test -count=1 -tags=integration ./aiengine ./migrations`：通过，真实 PostgreSQL 已验证工作流运行记录创建、输入/状态快照、状态迁移、终态完成时间和终态恢复保护；
- `make migrate`：迁移 0027 成功应用；
- `go vet ./...`、`git diff --check`：通过。

后续 T05 工作：补充 HTTP API 的真实权限集成测试、事件查询验收和向量检索实现；当前节点执行器已经接入统一 AIEngine、Tool Gateway 和知识检索契约。

### T05 自动验收记录（2026-08-29）

- `go test -count=1 ./...`：通过。
- `go test -race ./aiengine ./httpapi ./resource ./migrations`：通过。
- `go vet ./...`：通过。
- `OPSK_TEST_DATABASE_URL=$OPSK_DATABASE_URL go test -count=1 -tags=integration ./aiengine ./migrations`：通过，真实 PostgreSQL 验证 WorkflowRun 创建、`created_by`、输入/状态快照、状态迁移、终态保护。
- `OPSK_TEST_DATABASE_URL=$OPSK_DATABASE_URL go test -count=1 -tags=integration ./e2e`：通过，真实临时 schema 验证资源导入、诊断和巡检端到端链路。
- 工作流节点执行器单元测试通过，覆盖 Agent、Retrieval、Tool、Parallel 分支和节点输出快照。

补充验证：`go test -count=1 -tags=integration ./...` 中 AIEngine、e2e、资源、诊断、巡检、
Connector、LLM 和权限相关测试通过；迁移包中的项目成员历史迁移断言，以及组织包的回滚
断言在当前共享数据库上失败，单独串行重跑仍可复现，属于既有迁移测试/数据库状态问题，
与本次 T05 工作流代码无直接调用关系。

尚未自动通过的项目：当前仓库没有带登录态的 Workflow HTTP API 集成测试夹具，因此
`/workflows/{id}/runs`、`start`、`resume`、`cancel`、权限隔离和
`/workflow-runs/{id}/events` 的真实 HTTP 验收仍需启动当前分支 API 后执行；该项不宣称已通过。

### T04 真实模型与 HTTP 验收记录（2026-08-29）

- `make llm-provider-test` 使用 `.env` 中的 GLM-4-Flash 通过普通和 SSE 两种模式完成真实文本生成，均返回有效输出和 token 用量。
- 直接向同一 Provider 发送函数声明，模型返回 `finish_reason=tool_calls` 和有效函数参数，确认模型支持 Tool Calling。
- 通过 API 登录管理员 `admin`，登记真实 MCP 资源 `http://127.0.0.1:3100/mcp`，发现并调用 `docker:list_containers` 成功，返回当前 Docker 容器清单。
- 真实 AIEngine 请求使用 `ai_provider_resource_id`、`model_name`、AgentProfile 和 MCP 上下文执行；事件顺序包含 `execution.started`、`agent_profile.resolved`、`context.loaded`、`tool.requested`、`tool.started`、`tool.completed` 及终态事件，工具完成事件包含实际 `duration_ms`。
- `GET /api/v1/ai-executions/{id}/tool-calls` 返回审计记录，包含 Provider ID、`GLM-4-Flash`、MCP Resource ID、工具名、`started_at`、`completed_at`、`duration_ms`、脱敏后的参数和出参；未出现 API Key 或其他凭证。
- `GET /api/v1/ai-executions/{id}/events?after=3` 只返回序号 4 之后的事件并在终态后关闭；`Last-Event-ID: 5` 只返回序号 6、7，确认断线续读不重复发送历史事件。
- 一次真实执行的模型输出未满足 AgentProfile JSON Schema，Runtime 正确记录 `execution.failed` / `output_schema`；工具调用本身仍成功并完整入审计，说明工具失败与最终输出校验失败可区分追踪。
- 增加回归测试，确保上下文 Tool 的 `BeforeToolCallback` 返回 `nil` 继续执行 ADK 函数工具；此前返回参数会短路真实 MCP 调用。
