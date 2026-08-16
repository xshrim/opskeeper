# 内置诊断 Skill

## 1. 目的与边界

T12 提供 Kubernetes、PostgreSQL、Redis 和 Kafka 的内置只读诊断 Skill。它们不保存独立连接信息，也不接受模型生成的任意协议命令；每次执行都通过目标资源解析受控 Connector，并沿用资源级授权、超时、并发、响应大小、Tool 次数和 Token 预算。

Connector 返回的是固定的结构化诊断快照。确定性规则负责将事实转换为 Finding；模型只能解释 Finding、关联 Evidence 和提出待验证假设，不能执行变更或替代规则结论。

四个内置 Skill 的 Connector 调用均受默认 **10 秒** 超时、8 个并发槽位和 4 MiB 证据上限约束；每个快照的 `capabilities` 和 `unavailable` 字段分别说明实际可用和已降级的能力。

## 2. 内置 Skill 与固定能力

| Skill | 目标资源 | 固定只读能力 | 典型事实 |
|---|---|---|---|
| Kubernetes 工作负载诊断 | `Application`、`Kubernetes` | 允许列表内的 Kubernetes 对象读取 | 工作负载、Pod、事件、探针、资源限制、发布状态 |
| PostgreSQL 健康诊断 | `PostgreSQL` | `postgresql_inspect` | 版本、活跃会话、长时活跃查询、等待锁、复制数量、数据库容量 |
| Redis 健康诊断 | `Redis` | `redis_inspect` | 内存、客户端、复制、慢命令、拒绝连接；热 Key 安全降级 |
| Kafka 健康诊断 | `Kafka` | `kafka_inspect` | Broker、Topic、分区、ISR、离线副本、消费组成员与累计积压 |

迁移在默认平台安装内置 Skill，并以已发布 v2 作为当前版本；为保证版本内容不可变，初始 v1 会保留为已禁用的历史版本。管理员可将已发布版本设为 Scope 默认；运行时仍必须验证目标资源类型、Scope 和资源权限。

每个内置版本强制模型返回 JSON，顶层固定为 `facts`、`findings`、`evidence`、`hypotheses`、`confidence`（0–1）和 `recommendations`。Connector 证据与确定性 Finding 先产生，模型只能在此基础上解释、归因和给出不执行的建议。

## 3. 最小权限清单

### Kubernetes

使用专门 ServiceAccount，只授予目标命名空间的 `get`、`list`、`watch`（如确有必要）权限，资源限定为 `deployments`、`statefulsets`、`daemonsets`、`jobs`、`cronjobs`、`pods`、`events`、`services`、`ingresses` 和 `endpointslices`。不得授予 `create`、`update`、`patch`、`delete`、`exec`、`portforward` 或 Secret 读取权限。

### PostgreSQL

使用独立登录角色，配置 `default_transaction_read_only=on`，只授予连接目标数据库及读取 `pg_stat_activity`、`pg_locks`、`pg_stat_replication`、`pg_database` 所需统计视图权限。不得授予 DDL、DML、`pg_read_file`、复制、超级用户、`BYPASSRLS` 或跨库所有权。

### Redis

使用 ACL 用户，仅允许 `PING`、`INFO` 和 `SLOWLOG GET`；禁止 `@write`、`@dangerous`、`CONFIG`、`MODULE`、`SCRIPT`、`DEBUG`、`FLUSH*`、`SHUTDOWN` 和 `MONITOR`。如上游不允许慢日志读取，Connector 将 `slowlog` 标记为不可用。Redis 没有低成本且仅靠只读 ACL 即可获取热 Key 的 API；默认不会扫描整个键空间，而是明确返回 `hot_keys` 不可用。

### Kafka

使用只读主体，授予 Cluster Describe、Topic Describe、Group Describe 和 OffsetFetch 所需 ACL；禁止 Create、Alter、Delete、Produce、事务和配置修改。固定快照读取 Broker/Topic/分区/ISR、消费组和已提交 Offset，并据此计算累计积压。支持 TLS 及 SASL/PLAIN；消费组或积压无法被版本或 ACL 支持时必须返回能力降级，不能声明为正常。

## 4. 故障与降级

认证/授权失败、超时、限流、版本不兼容和统计视图缺权会保留 Connector 错误分类。内置规则只对已获得的事实作结论；缺失事实进入 `unavailable`，而不是推断目标健康或故障。

## 5. 验证边界

`make backend-integration-test` 会在本地 Compose 提供的 PostgreSQL 和 Redis 上实际采集健康快照，并在隔离 Schema 中验证内置 Skill 迁移可应用、回滚、再应用。Kubernetes 规则以动态客户端兼容的 JSON 故障夹具和黄金 Finding 验证；Kafka 分区/ISR/离线副本及认证配置使用 `kafka-go` 的结构化夹具验证。真实 Kafka 集群由部署环境提供，未配置消费组 ACL、旧 Broker API 或无法读取 Offset 时，运行时将该能力明确写入 `unavailable`，不会以测试替代真实连接结论。
