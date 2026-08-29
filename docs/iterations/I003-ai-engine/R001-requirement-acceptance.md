# I003-R001 AIEngine 执行框架需求验收报告

**迭代：** I003-ai-engine  
**需求：** R001 AIEngine 执行框架设计与实现  
**状态：** 实施中（T01 已完成，T02 实施中）  

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

**状态：** 实施中（核心链路已通过，待有效模型凭证完成 Tool Call 实测）

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

T02 尚待完成：完整的 MCP 远程端到端集成验收，以及 API/Worker 真实部署环境中的权限上下文验证。执行事件持久化、SSE 游标续读和 Tool Call 脱敏持久化已完成第一阶段实现；更完整的事件快照、SSE 断线恢复语义和审计查询权限将在 T04 继续增强。

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
