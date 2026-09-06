# 统一资源接入设计

## 1. 文档目的

本文定义 OpsKeeper 对基础设施、数据库、中间件和可观测资源的统一接入方式。目标是让同一套资源工具既可以作为本项目提供的 MCP Server 工具，也可以作为 AIEngine 的 Direct 内置工具使用。

本文描述长期设计原则和运行时边界；具体实施顺序、任务状态和验收证据以 [I004 统一资源接入迭代](../iterations/I004-unified-resource-access/iteration.md) 及其需求文档为准。

## 2. 核心目标

资源可以使用两种接入方式：

```text
Direct
  逻辑资源 + 连接配置/凭据
  -> AIEngine 加载该资源类型的内置工具集
  -> 工具直接连接目标资源

Agent
  逻辑资源 + 关联 MCPServer 资源
  -> AIEngine 通过 MCP tools/list 发现工具
  -> 通过 MCP tools/call 调用目标资源
```

两条路径必须遵循以下不变量：

- 工具名称、业务入参、业务出参和错误语义保持一致；
- Direct 和 MCP 不维护两套资源业务实现；
- AIEngine 不读取或向模型暴露资源连接凭据；
- MCP Server 是否由 OpsKeeper 提供，不改变 AIEngine 的调用协议；
- 权限、资源状态、超时、响应大小、审计和取消由外层运行时统一治理。

## 3. 术语与边界

### 3.1 逻辑资源

逻辑资源是资源目录中的 `Host`、`Docker`、`Kubernetes`、`PostgreSQL`、`Redis` 等资源实例。它代表用户要访问的目标和授权对象，不因为使用 Agent 接入就变成 `MCPServer`。

### 3.2 MCPServer 资源

`MCPServer` 是一个传输资源，保存 MCP endpoint、传输类型、请求凭据、工具白名单、超时和响应限制。它可以被一个或多个逻辑资源作为 Agent 传输入口关联。

MCPServer 不是 Docker、Kubernetes 或数据库资源的替代品，也不是全局插件。权限主体仍然是用户选中的逻辑资源；MCPServer 只记录实际传输路径。

### 3.3 工具集

工具集是某种资源类型能够提供的一组工具及其实现，例如 Docker 工具集包含 `docker_info`、`docker_images` 和 `docker_container_logs`。工具集不携带某个资源实例的凭据；连接配置由调用上下文提供。

## 4. 工具契约

### 4.1 最小公共定义

公共工具只定义模型和协议需要的业务信息：

```text
name
description
input schema
invoke
```

工具名称按功能命名并保持稳定，例如：

```text
docker_info
docker_images
docker_containers
docker_container_logs
docker_container_inspect
docker_container_stats

kubernetes_cluster_info
kubernetes_namespaces
kubernetes_pods
kubernetes_workloads
kubernetes_pod_logs
kubernetes_resource_get
```

工具名称已经表达资源域，不再在工具契约中重复声明 `resource_kind`。

### 4.2 不属于工具契约的字段

以下内容不作为工具入参，也不要求远端 MCP 工具提供：

- `resource_kind`；
- `tool_contract_version`；
- `capabilities`；
- `read_only`；
- `managed` 或 `external` MCP 标记。

工具是否存在本身就表示对应能力。当前工具全部为只读工具，AIEngine 不需要根据工具声明的布尔值决定是否调用；未来的写操作必须进入受控操作和审批链，不能依赖远端声明的 `read_only`。

AIEngine 内部可以保留 `resource_id`、适配器来源和调用审计字段，但这些是运行时绑定信息，不是公共工具的业务参数。

### 4.3 连接参数

资源连接串、Docker TLS 文件、kubeconfig、数据库密码、API Token 等属于资源配置和凭据上下文。它们不能由模型自由指定。

现有 MCP 工具如果为了独立运行而接受连接字段，公共调用仍可保留原字段和结果格式；MCP Server 从自身配置注入，Direct 适配器从逻辑资源配置和凭据注入，并覆盖模型提交的同名字段。后续如需改善模型可见 Schema，可以把连接字段标记为适配器内部字段，但不得让模型借此改变目标资源。

### 4.4 业务结果

公共工具返回稳定的结构化业务结果。MCP 适配器将其编码为 MCP structured content/text；Direct 适配器将其包装为 AIEngine ToolResult 和证据。适配器不得改变业务字段、成功条件或错误分类。

结果中的资源正文、日志、对象描述和远端文本均视为不可信数据，不得覆盖系统提示、工具权限或资源授权。

## 5. 公共工具实现层

公共实现层位于协议适配器之外，不依赖 MCP SDK、AIEngine、HTTP API Handler 或数据库资源服务。建议按资源类型组织：

```text
backend/tool/
  contract.go
  registry.go
  host/
  docker/
  kubernetes/
  postgresql/
  redis/
  kafka/
  prometheus/
  loki/
```

每个资源工具包负责：

- 工具名称、描述和输入 Schema；
- 强类型业务请求解析；
- 资源客户端创建和连接复用；
- 资源操作和结果规范化；
- 参数边界、查询范围、输出截断和错误分类；
- 敏感字段清理。

公共实现不得负责：

- 当前用户的 RBAC 和 Scope 判定；
- 读取资源目录或凭据库；
- MCP endpoint、JSON-RPC 或 HTTP 传输；
- AIEngine Agent Loop、事件和审计持久化；
- 把任意模型文本解释成命令。

公共实现可以使用连接上下文，例如 `DockerConnection` 或 `KubernetesConnection`，但连接上下文来自适配器，不是模型可以任意填写的业务参数。

## 6. 两种适配器

### 6.1 Direct 适配器

Direct 适配器挂在资源类型上，执行顺序如下：

```text
AIEngine Context Resolver
  -> 读取逻辑资源
  -> 校验 active、Scope 和 resource:use
  -> 读取 config 和关联凭据
  -> 创建对应 tool 连接上下文
  -> 注册该资源类型的公共工具集
  -> Policy Gateway 授权、限流、超时和审计
  -> 调用公共工具实现
```

Direct 资源必须具备该工具集所需的连接配置。缺少配置或凭据时，在工具注册或第一次调用前返回明确的配置错误，不回退到其他资源，也不使用模型提供的目标地址。

### 6.2 MCP 适配器

Agent 适配器执行顺序如下：

```text
AIEngine Context Resolver
  -> 读取逻辑资源及其 mcp_server_resource_id
  -> 校验逻辑资源和 MCPServer 的 Scope、状态及使用权限
  -> MCP tools/list
  -> 使用远端返回的工具名称、描述和 Schema
  -> 注册 MCP 代理工具
  -> Policy Gateway 授权、限流、超时和审计
  -> MCP tools/call
```

远端工具以其自身发现结果为准，不要求携带本项目的资源类型、版本、能力或只读元数据，也不与本地公共工具实现做语义比对。

仍然必须保留协议和安全边界：工具必须经过 MCP Server 的允许列表，调用前检查资源权限和状态，调用时应用超时与响应大小限制，远端结果标记为不可信并进入审计。

### 6.3 项目提供的 MCP Server

OpsKeeper 提供的 Docker、Kubernetes 等 MCP Server 仍是普通 MCP Server。其 Server 层只负责：

- 注册 MCP Tool；
- 解码 MCP 输入；
- 从 Server 配置提供连接上下文；
- 调用公共资源工具；
- 将公共结果编码为 MCP 结果；
- 映射公共错误。

AIEngine 不为这些 Server 增加特殊分支，也不区分 `managed` 和 `external`。公共工具库的复用是代码实现层的复用，对 MCP 协议调用方透明。

## 7. 资源模型

资源需要显式表达接入方式和 Agent 传输入口。目标字段为：

```text
access_mode: direct | agent
mcp_server_resource_id: UUID，可为空
config: JSONB
credential_id: UUID，可为空
```

约束：

```text
direct:
  access_mode = direct
  config/credential 满足对应资源工具集要求
  mcp_server_resource_id 为空

agent:
  access_mode = agent
  mcp_server_resource_id 必须存在
  关联资源的 kind 必须为 MCPServer
```

关联不能绕过 Scope 可见性和资源使用授权。一个 MCPServer 可以服务多个逻辑资源，但每次执行仍以逻辑资源 ID 作为权限、证据和审计主体。

如果现有资源模型继续使用 `subtype=Direct|Agent` 过渡，运行时只能把它解析为上述语义；最终设计以显式 `access_mode` 为准，不能让不同字段同时成为权威来源。

## 8. AIEngine 上下文解析

用户勾选资源后，Context Resolver 按资源的 `access_mode` 选择唯一路径：

```text
selected resource
  ├─ direct -> resource kind -> built-in ToolSet -> direct connection
  └─ agent  -> mcp_server_resource_id -> MCP discovery -> remote proxy tools
```

工具注册键必须至少包含 `(logical_resource_id, tool_name)`。多个同类型资源可以拥有同名工具；模型声明中的函数名冲突只在执行绑定时生成稳定别名，不改变公共工具名、审计名或调用结果。

AIEngine 不因为资源类型没有内置工具就尝试把它转换成 MCPServer，也不因为 Agent 资源没有找到预期工具就调用 Direct 连接。工具不可用时，向模型和用户返回明确的资源工具不可用错误。

## 9. 权限、安全和审计

权限主体是逻辑资源：

```text
logical_resource_id
access_mode
transport_resource_id
tool_name
```

调用至少记录：

- Actor、Scope、执行 ID；
- 逻辑资源 ID和资源类型；
- Direct 或 Agent 接入方式；
- Agent 时的 MCPServer 资源 ID；
- 工具名称、模型名称和 Provider；
- 脱敏后的参数、结果、状态、耗时和错误。

凭据不得进入模型消息、工具普通日志、SSE 事件或审计正文。Direct 和 MCP 都必须经过同一套资源授权、并发、超时、取消、响应限制和审计路径。

MCP 工具和其返回内容默认是不可信的。MCP 工具是否只读不能由远端元数据决定；任何状态变更都必须使用受控操作请求和既有审批流程。

## 10. 错误与降级

错误至少分为：

- 资源配置错误：连接信息缺失或格式不合法；
- 凭据错误：凭据不存在、无法解密或认证失败；
- 资源不可用：目标服务不可达、超时或返回服务错误；
- 工具不可用：Direct 工具集未注册，或 MCP Server 未发现/不允许该工具；
- 权限拒绝：用户、Scope、逻辑资源或 MCPServer 不允许调用；
- 参数错误：工具业务参数不满足 Schema 或资源边界；
- 响应受限：输出超过统一大小或条数限制；
- 取消：请求或上游执行上下文已取消。

Direct 失败不得自动切换到 Agent，Agent 失败也不得自动切换到 Direct。两种方式的配置、权限和故障均应在用户界面中明确显示。

## 11. 资源工具范围

资源分为运维接入资源、可观测资源和平台管理资源。只有前两类进入统一资源工具体系：

| 类别 | 资源类型 | 工具接入范围 |
|---|---|---|
| 基础设施 | Host、Docker、Kubernetes | 主机信息、容器/集群对象、日志、状态和健康检查 |
| 数据库 | PostgreSQL、MySQL、Oracle、OceanBase、TongRDS | 连接、会话、锁、慢查询、容量和复制等只读诊断 |
| 中间件 | Redis、Kafka、RabbitMQ、Elasticsearch | 连接、节点、客户端、消费/队列、分片和健康等只读诊断 |
| 可观测 | Prometheus、Loki、Tempo、Jaeger、Elastic、Datadog、Alertmanager | 指标、日志、告警和追踪查询 |
| 平台管理 | AIProvider、MCPServer、Skill、AgentProfile | 不作为诊断目标工具集；由各自管理和执行模块处理 |
| 业务目录 | Application、Repository、Artifact | 不在本迭代实现资源直连工具；通过关系、发现或各自模块使用 |

## 12. 测试要求

每个资源工具集至少需要：

- 公共实现单元测试：参数、成功结果、错误和输出限制；
- Direct 适配器测试：资源配置、凭据注入、权限、取消和超时；
- MCP 适配器测试：`tools/list`、`tools/call`、Schema 原样传播、allowlist 和不可信结果；
- Direct/MCP 一致性测试：相同目标和业务参数得到等价工具名、字段和错误语义；
- 多资源测试：同名工具不会串用资源连接或审计对象；
- 安全测试：模型不能通过连接字段改变 Direct 目标，远端内容不能改变工具权限；
- 集成测试：真实 Docker、Kubernetes、PostgreSQL 和 Redis 环境各至少完成一次连接及工具调用验证。

## 13. 迁移与删除原则

统一工具集稳定后，应删除：

- `connector.*` 与 MCP Server 重复的资源业务工具实现；
- 按协议复制的参数解析、结果 DTO 和错误映射；
- 仅为区分 managed/external MCP Server 而存在的运行时分支；
- 没有调用方的旧工具名称和旧资源接入路径。

允许保留的仅是 HTTP、MCP、AIEngine 和资源服务适配代码。旧设计不需要兼容；若迁移发现旧数据无法直接转换，应通过一次性迁移或明确失败报告处理，而不是长期保留双轨运行时。

