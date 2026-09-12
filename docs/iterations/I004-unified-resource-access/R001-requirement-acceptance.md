# I004-R001 需求验收报告：统一资源接入

**迭代：** I004-unified-resource-access  
**需求：** R001 统一资源接入  
**验收结论：** 部分验收：T01-T09 已完成，其余任务待实施

## 1. 需求级验收结论

T01-T09 已完成验收，确认日期更新为 2026-09-13。T10-T14 按任务表继续实施。

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
| T08 | Redis 工具集统一 | 已通过 | `go test ./...`、真实 Redis 六工具集成测试、前端专用创建/编辑流程、`npm run check/test/build`、MCP Server 编译与固定工具契约测试通过 |
| T09 | Nacos 工具集统一 | 已通过 | 公共 Nacos API 工具、Direct/Agent/MCP 适配器、前端资源流程、资源目录排序和 0039 迁移完成；契约测试通过 |
| T10 | AIEngine 与证据链收敛 | 待实施 |  |
| T11 | Kafka、Prometheus、Loki 迁移 | 待实施 |  |
| T12 | 其他数据库和中间件迁移 | 待实施 |  |
| T13 | 管理界面与接入校验 | 待实施 |  |
| T14 | 删除旧路径与全量验收 | 待实施 |  |

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
- 前端专用创建/编辑流程已验收：`PostgreSQLConnectionStep`/`PostgreSQLReviewStep` 提供 Direct/Agent 配置、连接测试、总结核验和编辑回填；Agent 模式不保存 Direct 数据库凭据。
- 移除旧的 `connector.inspect_postgresql` 聚合快照、PostgreSQL Inspector 接口、Worker 调用和旧工具名注册。

### 验证步骤和结果

- `cd backend && go test ./...`：通过。
- `cd frontend && npm run check && npm run build`：通过。
- `git diff --check`：通过。
- 本机 Docker PostgreSQL 16（仅执行受控只读查询）执行：`cd backend && set -a && . ../.env && set +a && go test -tags=integration ./tool/postgresql ./mcpserver/postgresql/server -run 'TestRealPostgreSQL' -count=1`：通过。验证全部 12 项 Direct 工具，以及 PostgreSQL MCP 的 `tools/list` 和 `postgresql_health` 实际调用。
- 实际运行发现 `pg_settings.unit` 和 `short_desc` 可为空，性能/VACUUM 公共查询已用 `COALESCE` 规范化后复测通过。

## 8.1 T08 Redis 工具集统一验收

### 实施内容

- 公共包 `backend/tool/redis` 固定提供 `redis_health`、`redis_memory`、`redis_clients`、`redis_replication`、`redis_slowlog`、`redis_database_info` 六项只读工具。
- Direct Provider 与 Redis MCP Server 共享工具名、Schema、DTO 和错误边界；Agent 通过 `agent_ref` 调用 MCPServer，连接字段由服务端注入且不会暴露给模型。
- 前端新增 Redis Direct/Agent 配置、草稿连接测试、创建/编辑、总结核验和详情工具列表，样式与 PostgreSQL/Docker/Kubernetes 一致；0038 迁移更新 Redis 资源 schema 与内置 Skill 工具契约。
- 前端专用创建/编辑流程已验收：`RedisConnectionStep`/`RedisReviewStep` 覆盖 Host、Port、Database、凭据、MCPServer 关联、连接测试、总结核验和编辑回填；Agent 模式清理 Direct 凭据关联。
- 仅调用 PING、固定 INFO 分区、DBSIZE、SLOWLOG GET 20；不接受任意 Redis 命令、不扫描全量 Key，慢日志只返回命令名和参数数量。

### 验证步骤和结果

- `cd backend && go test ./...`：通过。
- `cd frontend && npm run check && npm run test -- --run && npm run build`：通过（52 个测试）。
- `cd backend && go test ./mcpserver/redis/server ./mcp ./connector`：通过，Redis MCP Server 与 Agent 参数注入契约编译/回归通过。
- `cd backend && go test -tags=integration ./tool/redis -run 'TestRealRedis' -count=1`：本机 Redis `127.0.0.1:6383` 六项工具真实连接通过。
- `git diff --check`：通过。

## 9. 需求级遗留事项

## 8.2 T09 Nacos 工具集统一验收

### 实施内容

- 新增 Nacos 资源，支持 Direct/Agent 子类型、统一连接配置、凭据及 MCPServer 关联。
- 公共包 `backend/tool/nacos` 提供服务端状态、命名空间、服务列表、服务实例、配置元数据和配置详情六项固定只读 API 工具。
- Direct Provider、Nacos MCP Server 与 Agent 使用统一工具名称、业务 Schema 和结果 DTO；连接字段由服务端注入，禁止模型覆盖。
- 前端新增 Nacos 创建、编辑、连接测试、总结核验和详情工具列表；资源目录调整为用户指定顺序。
- 前端专用创建/编辑流程已验收：`NacosConnectionStep`/`NacosReviewStep` 覆盖 Host、Port、Scheme、Context Path、用户名/密码、Access Token、MCPServer 关联、连接测试、总结核验和编辑回填。
- 0039 迁移新增 Nacos 资源 Schema。

### 验证步骤和结果

- `cd backend && go test ./...`：通过。
- `cd frontend && npm run check && npm run test -- --run && npm run build`：通过（53 个测试）。
- `cd backend && go test ./tool/nacos ./connector ./mcpserver/nacos/server ./mcp`：通过。
- `cd backend && go test ./...`：通过；`cd frontend && npm run check && npm run test -- --run && npm run build`：通过（53 个测试）。
- `git diff --check`：通过。
- 本机未运行 Nacos 服务，`127.0.0.1:8848` 连接失败；因此真实 Nacos 集群 API 验证列为环境限制。公共 API 请求、分页边界、参数校验和错误路径已通过 `httptest` 契约测试。

<!-- 将未完成的低优先级资源、驱动限制或外部环境依赖转入 backlog 或后续迭代。 -->

## 10. 用户确认和最终结论

Application T06、PostgreSQL T07、Redis T08 和 Nacos T09 验收通过；三项资源的专用前端创建/编辑、连接测试、总结核验和详情展示均已验证。Repository T10 的 local Bundle 和工具链已完成，但 S3 后端及标准 Git clone/pull 服务尚未完成，因此 T10 当前为部分完成，I004-R001 仍处于实施中。

## 8.3 T10 Repository 工具集统一验收

### 实施内容

- Repository 保留 `Git`、`Bundle` 两种子类型：Git 按需实时读取远端仓库，Bundle 通过 `.bundle` 上传到 OpsKeeper；资源目录继续按统一 Direct/Agent 样式展示。
- 新增 `backend/tool/repository` 公共只读工具：`repository_branches`、`repository_checkout`、`repository_status`、`repository_tree`、`repository_file`、`repository_search`、`repository_metadata`。工具固定使用受控 Git 参数，限制分支、路径、文件大小、搜索结果和执行超时，不开放任意 Shell/Git 命令。
- Connector Direct Provider 和 Repository MCP Server 使用同名工具；Agent 资源通过 `agent_ref` 发现远端 MCP 工具，连接字段由服务端注入，Direct/Agent 不互相回退。
- 新增 `POST /api/v1/resources/{resourceID}/bundle` multipart 上传 API，并在 Repository 详情页提供文件上传控件。服务端执行 `git bundle verify`，导入 bare 仓库；同名分支用强制 ref 更新覆盖，新分支自动追加，串行锁保证并发上传不会破坏仓库。
- 前端新增 Repository 专用创建/编辑流程：Git 配置 URL/默认分支，Bundle 配置 local/S3 后端及对应参数，均提供独立的配置校验和总结核验步骤；编辑已有 Repository 时会回填配置，Bundle 文件上传仍在详情页提供。
- 新增 Repository 存储配置：`OPSK_REPOSITORY_STORAGE_BACKEND=local|s3`、`OPSK_REPOSITORY_LOCAL_ROOT`、`OPSK_REPOSITORY_S3_ENDPOINT`、`OPSK_REPOSITORY_S3_BUCKET`、`OPSK_REPOSITORY_S3_PREFIX`、`OPSK_REPOSITORY_S3_ACCESS_KEY`、`OPSK_REPOSITORY_S3_SECRET_KEY`、`OPSK_REPOSITORY_S3_USE_SSL`、`OPSK_REPOSITORY_MAX_BUNDLE_BYTES`。默认 local，上传大小默认上限 512 MiB；S3 通过 S3-compatible API 实际读写 Bundle 对象。
- 0040 迁移将 Repository Schema 切换为 v2，移除旧敏感字段模型；上传成功后回写 `path/storage_key`，供 AI 诊断工具稳定读取仓库内容。

### 验证步骤和结果

- `cd backend && go test ./...`：通过，包含 Repository 工具临时 Git 仓库读操作回归测试。
- `cd frontend && npm run check`：通过。
- `git diff --check`：通过。
- 通过固定 Git 命令和路径校验验证：绝对路径、`..` 穿越、非法分支、超大文件和空搜索均被拒绝；Bundle 上传验证失败时不会更新已有仓库。

### 已知边界

- Bundle 上传在 S3 模式下以对象 `<prefix>/<resource-id>/repository.bundle` 保存完整 Bundle；每次上传会下载旧对象、合并分支、重新生成完整 Bundle 后原子覆盖对象。MinIO 和 pgsty/silo 均按 S3-compatible API 接入，支持 endpoint、bucket、access key、secret key 和 HTTP/HTTPS。
- 当前尚未提供标准 Git Smart HTTP/SSH clone/fetch 服务，因此客户端还不能直接对 Bundle Repository 执行标准 `git clone`/`git pull`。
- Repository MCP 可执行程序提供同一公共工具的 MCP 入口；实际 Agent 连接仍需在资源上关联可用的 MCPServer。
