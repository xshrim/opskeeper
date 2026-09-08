# I004-R001 需求验收报告：统一资源接入

**迭代：** I004-unified-resource-access  
**需求：** R001 统一资源接入  
**验收结论：** 部分验收：T01、T02、T03 已完成，其余任务待实施

## 1. 需求级验收结论

<!-- 实施完成后填写结论和日期。 -->

## 2. 验收环境和范围

<!-- 记录代码提交、数据库版本、Docker/Kubernetes/PostgreSQL/Redis 环境和测试范围。 -->

## 3. 任务验收汇总

| 任务 | 名称 | 结果 | 证据 |
|---|---|---|---|
| T01 | 公共工具契约与执行基础 | 已通过 | `cd backend && go test ./tool`；公共包不依赖 MCP、AIEngine 或 HTTP API |
| T02 | 资源接入模型与上下文解析 | 已通过 | `cd backend && go test ./resource ./aiengine ./mcp ./connector ./httpapi ./migrations`；`go test -race ./resource ./aiengine ./mcp`；`subtype`/`agent_ref` 关联和 Direct/Agent Provider 路由测试通过 |
| T03 | Docker 工具集统一 | 已通过 | `cd backend && go test ./connector ./tool/... ./mcpserver/docker/...`；公共 Docker 实现由 MCP 薄适配器和 Direct Provider 共用，Direct 注册 6 个工具并隐藏连接字段；MCP Schema 与日志过滤回归通过 |
| T04 | Host 工具集接入 | 待实施 |  |
| T05 | Kubernetes 工具集统一 | 待实施 |  |
| T06 | PostgreSQL 工具集统一 | 待实施 |  |
| T07 | Redis 工具集统一 | 待实施 |  |
| T08 | AIEngine 与证据链收敛 | 待实施 |  |
| T09 | Kafka、Prometheus、Loki 迁移 | 待实施 |  |
| T10 | 其他数据库和中间件迁移 | 待实施 |  |
| T11 | 管理界面与接入校验 | 待实施 |  |
| T12 | 删除旧路径与全量验收 | 待实施 |  |

## 4. T01 任务验收报告

### 验收目标

建立协议无关的公共工具定义、调用上下文、统一结果/错误模型和按名称注册表，为 Docker 工具集的后续复用提供稳定边界。

### 验证步骤和结果

- `cd backend && go test ./tool`：通过。
- `tool.Definition` 只包含名称、描述和输入 Schema，不包含资源类型、工具版本、能力、只读标记或 MCP 管理类型字段。
- `tool.Invocation` 将业务参数与适配器拥有的 opaque connection context 分离；注册表调用前复制参数，避免工具修改调用方 map。
- 注册表支持注册、重复检测、Schema 校验、排序枚举、覆盖注册和按名称调用；调用不存在工具返回 `tool_unavailable` 分类。
- `tool` 源码仅使用 Go 标准库，不依赖 MCP SDK、AIEngine、HTTP API 或资源目录。

### 遗留问题

Docker 公共工具迁移、Direct 适配器和 MCP Server 薄适配器已在 T03 完成。T01 的协议无关工具边界保持不变。

<!-- 后续任务完成后继续增加对应验收章节。 -->

## 5. T02 任务验收报告

### 实施内容

- 资源模型以 `subtype` 表达 Direct/Agent，以 `agent_ref` 关联 MCPServer；两者是唯一权威字段。
- 0034 迁移完成过渡数据回填，0035 迁移收敛到 `subtype`/`agent_ref`，并恢复已有 `served_by_mcp` 关系和关联索引。
- Resource API 支持创建和更新接入方式及 MCPServer 关联；服务层校验接入方式、关联对象类型、活动状态、权限范围、自关联和字段冲突。
- ContextResource 携带 `subtype`、`agent_ref`、凭据引用和资源配置；凭据与配置不进入上下文 JSON 序列化。
- Context Resolver 按 `subtype` 选择唯一 Provider：Direct 仅选择声明 Direct 的 Connector，Agent 仅选择声明 Agent 的 MCP Provider；Agent 缺少 MCP Provider 时明确失败，不回退到 Direct。
- MCP Provider 对 Agent 使用关联 MCPServer 资源进行发现和调用，但工具定义、事实、审计和权限主体仍使用逻辑资源 ID。
- API 与 Worker 均注册 MCP Context Provider，确保后台诊断和 HTTP 请求使用同一解析路径。

### 验收边界

- T02 未修改 Docker/Kubernetes 工具的业务参数、输出或功能；公共工具迁移属于 T03/T05。
- 历史 Agent 资源若无法从 `served_by_mcp` 关系恢复传输资源，会保留 `agent` 但关联为空，由 API 修复后才能执行，不会静默改成 Direct。
- Docker Direct 工具集使用公共 Docker 函数，并按逻辑资源的 Direct 配置绑定连接；其他资源仍以现有 Connector 能力为准。

## 6. T03 任务验收报告

### 实施内容

- Docker DTO、连接输入、六个只读工具和日志过滤逻辑迁移到 `backend/tool/docker`，不依赖 MCP SDK 或 AIEngine。
- Docker MCP Server 仅保留 MCP Tool 注册、输入解码和结果编码，通过公共 Docker 函数执行。
- Connector 增加 Docker Direct Provider，按逻辑资源配置和已授权凭据绑定连接，注册六个稳定工具。
- Direct Schema 隐藏连接字段并在调用前覆盖模型参数；MCP 独立运行 Schema 与既有参数保持不变。

### 验证步骤和结果

- `cd backend && go test ./connector ./tool/... ./mcpserver/docker/...`：通过。
- Direct Provider 工具集、逻辑资源 ID、配置优先于凭据和连接字段隔离测试通过。
- Docker MCP 工具发现、Schema、连接测试和工具调用测试通过；`docker_container_logs` 的 `&`/`|` 语义继续由公共实现覆盖。

## 7. 需求级遗留事项

<!-- 将未完成的低优先级资源、驱动限制或外部环境依赖转入 backlog 或后续迭代。 -->

## 8. 用户确认和最终结论

<!-- 封板时填写用户确认、提交基线和最终结论。 -->
