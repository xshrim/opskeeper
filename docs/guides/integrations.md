# 集成指南

## 1. 资源接入

资源统一使用 `scope_id`、`kind`、`subtype`、`config` 和加密凭据。Direct 资源由平台 Connector 连接；MCP 资源通过 `agent_ref` 连接外部只读 MCP Server。两种方式都必须经过 Scope、资源状态、`resource:use`、超时、并发、响应大小和审计校验。

Application 是项目级聚合对象，只暴露其实例关联的受控工具，不把整套底层资源工具交给模型。目标定位使用人类可读名称、编码、命名空间或工作负载信息；数据库 UUID 不能作为外部运行时定位符。

## 2. MCP 服务

项目提供 Docker、Kubernetes 和 Host 三类只读 MCP 服务。它们均支持 Streamable HTTP 和 SSE，并可用 Bearer Token 保护；默认监听地址和完整参数见对应实现及 `.env.example`。

| 服务 | Make 命令 | 默认端口 | 典型能力 |
| --- | --- | ---: | --- |
| Docker MCP | `make docker-mcp-run` | 8811 | Engine、镜像、容器、日志、inspect、stats |
| Kubernetes MCP | `make kubernetes-mcp-run` | 8812 | 集群、工作负载、Pod、事件、日志、受控文件读取 |
| Host MCP | `make host-mcp-run` | 8813 | 主机信息、指标、进程和日志 |

所有服务只执行固定的只读工具，不接受任意 Shell、SQL、Docker 命令、Kubernetes 路径或写操作。凭据、Token、私钥和 kubeconfig 不写入日志或结果。

### 最小权限

- Docker：只读 Engine 权限；不提供容器启停、删除或 exec。
- Kubernetes：限定命名空间和允许的资源类型；禁止 Secret、RBAC 和写操作。
- Host：只读 `/proc`、`/sys`、`/etc/os-release` 或受控 SSH 目标；命令行脱敏并限长。

## 3. 内置诊断 Skill

内置 Skill 当前覆盖 Kubernetes、PostgreSQL、Redis 和 Kafka。它们使用固定结构化工具和受控 Connector，不执行模型生成的任意协议命令。

| Skill | 主要内容 |
| --- | --- |
| Kubernetes | 工作负载、Pod、事件、探针、资源限制和发布状态 |
| PostgreSQL | 健康、慢查询、活动 SQL、锁、连接、表结构和扩展 |
| Redis | 健康、内存、客户端、复制、慢日志和数据库信息 |
| Kafka | Broker、Topic、分区、ISR、消费组和积压 |

Connector 的默认限制为超时 10 秒、并发 8、单次证据 4 MiB。能力缺失、权限不足、超时和空结果必须标记为不可用，不能推断为健康。

## 4. 关系型诊断

PostgreSQL、MySQL、Oracle 和 OceanBase 提供统一的只读诊断结果：慢查询、活动查询、锁等待、连接统计和表结构。各数据库的采集视图、保留窗口和权限不同，结果中的 `available`、`capacity_known` 和 `wait_duration_known` 用来区分“没有结果”和“无法采集”。

SQL 文本最多 4 KiB，列表默认最多 100 条。OceanBase 使用其自身 `GV$OB_*` 和 SQL Audit 视图，不回退为 MySQL 或 Oracle 查询。

## 5. 外部 LLM

Provider 保存厂商地址、模型目录和加密凭据；Engine 负责模型调用、上下文、工具、预算、取消、流式事件和审计。Provider、Engine、Skill 和 Persona 是独立领域对象，不是通用 Resource。

模型不能直接访问基础设施。每次工具调用仍需经过 Scope/资源权限、Connector 策略、预算和审计；Provider 或模型不可用时，确定性诊断仍应保留并明确标记 LLM 降级。
