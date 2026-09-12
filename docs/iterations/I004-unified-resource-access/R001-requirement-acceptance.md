# I004-R001 需求验收报告：统一资源接入

**迭代：** I004-unified-resource-access  
**需求：** R001 统一资源接入  
**验收结论：** 部分验收：T01-T07 已完成，其余任务待实施

## 1. 需求级验收结论

T01-T07 已完成验收，确认日期为 2026-09-12。T08-T13 继续按任务表实施。

## 2. 验收环境和范围

- **PostgreSQL：** 本机 Docker PostgreSQL 16，`127.0.0.1:5432`，仅对已有 OpsKeeper 数据库执行固定只读查询；连接串、用户名和密码未写入报告或测试输出。
- **MCP：** `httptest` 启动 PostgreSQL MCP HTTP Server，经项目 MCP 客户端完成 `tools/list` 与 `postgresql_health` 调用。
- **前端：** Svelte 类型检查与生产构建。

## 3. 任务验收汇总

| 任务 | 名称 | 结果 | 证据 |
|---|---|---|---|
| T01 | 公共工具契约与执行基础 | 已通过 | `cd backend && go test ./tool`；公共包不依赖 MCP、AIEngine 或 HTTP API |
| T02 | 资源接入模型与上下文解析 | 已通过 | `cd backend && go test ./resource ./aiengine ./mcp ./connector ./httpapi ./migrations`；`go test -race ./resource ./aiengine ./mcp`；`subtype`/`agent_ref` 关联和 Direct/Agent Provider 路由测试通过 |
| T03 | Docker 工具集统一 | 已通过 | `cd backend && go test ./connector ./tool/... ./mcpserver/docker/...`；公共 Docker 实现由 MCP 薄适配器和 Direct Provider 共用，Direct 注册 6 个工具并隐藏连接字段；MCP Schema 与日志过滤回归通过 |
| T04 | Host 工具集接入 | 已通过 | `make host-mcp-test`、`cd backend && go test ./...`、`cd frontend && npm run check`；Host Direct 与 Host MCP Agent 共用五个 Linux 只读工具，SSH 支持密码/私钥和 known_hosts，连接目标遵循工具参数 > HOST_MCP_* 环境变量 > 本机，文件日志支持 tail/since/until/keyword，资源前端支持 Direct/Agent 配置和连接测试 |
| T05 | Kubernetes 工具集统一 | 已通过 | `cd backend && go test ./...`、`cd frontend && npm run check && npm test -- --run`；14 个只读 Kubernetes 工具由公共实现同时提供 Direct 与 MCP/Agent 路径，连接参数遵循工具入参 > 环境变量 > 默认 kubeconfig，MCP HTTP 支持可选 Bearer Token；Kubernetes 资源前端添加、编辑、总结核验和详情展示已接入 |
| T06 | Application 资源接入 | 已通过 | `cd backend && go test ./...`、`cd frontend && npm run check && npm run test -- --run`、`cd frontend && npm run build`、`git diff --check`；Application 项目归属、三种接入方式、多实例唯一性、结构化表单、受控候选发现和日志工具已通过验收 |
| T07 | PostgreSQL 工具集统一 | 已通过 | 公共 PostgreSQL 工具、Direct Provider、PostgreSQL MCP Server、Agent 参数注入、专用管理界面及数据库迁移已完成；真实 PostgreSQL 16 上 12 项 Direct 工具、MCP `tools/list` 和 `postgresql_health` 调用通过 |
| T08 | Redis 工具集统一 | 待实施 |  |
| T09 | AIEngine 与证据链收敛 | 待实施 |  |
| T10 | Kafka、Prometheus、Loki 迁移 | 待实施 |  |
| T11 | 其他数据库和中间件迁移 | 待实施 |  |
| T12 | 管理界面与接入校验 | 待实施 |  |
| T13 | 删除旧路径与全量验收 | 待实施 |  |

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

## 7. T06 Application 资源接入验收

### 实施内容

- Application schema v2 使用 `access_mode` 与 `instances`，服务层要求项目 Scope、活动的 Host/Docker/Kubernetes 关联和按接入方式完整的实例定位字段。
- 虚拟机实例通过唯一的 `process_keyword` 字符串表达式确认唯一 PID，表达式支持 `&`、`|`、逗号/空格和引号保护；未输入关键字时不扫描全量进程，候选选择后即时复核唯一性。容器化实例通过容器名称确认唯一容器；云原生实例确认 namespace/workload 存在并可解析到至少一个受控 Pod，前端以“类型 · 名称”合并候选，日志读取再按 workload selector 解析受控 Pod。
- Application 工具只注册 `application_instances` 和 `application_logs`，实例索引是唯一可选目标参数，复用底层公共只读工具；日志支持 Host 文件、Docker/Pod 文件或 stdout，以及活动 Loki 的查询语句。统一调用器按关联资源自身的接入方式选择 Connector 或 MCP Provider，Application 不区分 Direct/Agent。
- 资源管理界面要求团队与项目，提供三种接入方式和多实例结构化编辑；Host、Docker、Kubernetes 和 Loki 的活动 Direct/Agent 资源均可作为关联候选。I004 任务表已将原 T06 及后续任务顺延为 T07-T13。

### 验证步骤和结果

- `cd backend && go test ./...`：通过。
- Direct/Agent 透明关联回归：`Application` 对两类 Host、Docker、Kubernetes 发出相同的固定工具调用；Agent Loki 使用相同受控 `query_logs` 调用；统一调用器在缺少 Agent Provider 时明确失败，不回退到 Direct。
- `cd frontend && npm run check && npm run test -- --run`：通过，52 个测试通过。
- `git diff --check`：通过。

### 已知边界

- Application 本身不提供 MCP 传输；它固定关联资源 ID 与业务目标参数，统一调用器据此选择关联资源的唯一执行 Provider。Agent 缺少受控工具时明确失败，不会回退到 Direct。
- Kubernetes Job/CronJob 可能没有当前 Pod，或 workload 对应多个 Pod；连接验证要求当前至少存在一个 Pod，工具限制解析数量并在返回中保留 `partial`/`errors`。

## 8. T07 PostgreSQL 工具集统一验收

### 实施内容

- 公共包 `backend/tool/postgresql` 固定提供 12 个只读工具：健康、活跃会话、长查询、等待锁、复制、容量、用户表统计、指定用户表列、性能与参数、VACUUM 参数、已安装扩展和数据库概要。
- Direct Provider 使用 PostgreSQL 逻辑资源的 config 和加密 credential 注入连接；工具 schema 不暴露主机、数据库、账号或密码。
- Agent 资源仅通过 `agent_ref` 对应的 MCPServer 发现和调用同名工具。Agent schema 仅保留业务参数，连接字段由逻辑资源服务器端注入，且 Agent 解析失败不会回退到 Direct。
- 新增 PostgreSQL MCP 可执行程序、草稿连接测试 API、Direct/Agent 专用资源创建、编辑、总结核验和详情展示；0037 迁移更新资源 schema 和内置 Skill 工具契约。
- 移除旧的 `connector.inspect_postgresql` 聚合快照、PostgreSQL Inspector 接口、Worker 调用和旧工具名注册。

### 验证步骤和结果

- `cd backend && go test ./...`：通过。
- `cd frontend && npm run check && npm run build`：通过。
- `git diff --check`：通过。
- 本机 Docker PostgreSQL 16（仅执行受控只读查询）执行：`cd backend && set -a && . ../.env && set +a && go test -tags=integration ./tool/postgresql ./mcpserver/postgresql/server -run 'TestRealPostgreSQL' -count=1`：通过。验证全部 12 项 Direct 工具，以及 PostgreSQL MCP 的 `tools/list` 和 `postgresql_health` 实际调用。
- 实际运行发现 `pg_settings.unit` 和 `short_desc` 可为空，性能/VACUUM 公共查询已用 `COALESCE` 规范化后复测通过。

## 9. 需求级遗留事项

<!-- 将未完成的低优先级资源、驱动限制或外部环境依赖转入 backlog 或后续迭代。 -->

## 10. 用户确认和最终结论

Application T06 验收通过；I004-R001 仍处于实施中，后续任务未完成。
