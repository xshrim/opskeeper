# PostgreSQL First 规约

OpsKeeper 平台自身的持久化、缓存和异步处理默认只依赖 PostgreSQL。不得为平台内部功能默认引入 Redis、Kafka/Redpanda、Elasticsearch、MongoDB、独立时序数据库或对象存储服务。

这条规则不限制资源目录接入和诊断用户已有的 Redis、Kafka、S3、Elasticsearch 等外部资源；它们不是 OpsKeeper 自身的部署依赖。

## 1. 优先实现

| 需求 | 首选 PostgreSQL 能力 |
| --- | --- |
| 短期共享缓存 | `UNLOGGED` 表、TTL 和可重建数据 |
| 消息队列、任务队列、可靠投递 | Job/Outbox 表、事务、租约与 `FOR UPDATE SKIP LOCKED` |
| 全文检索 | 原生全文检索；需要中文分词时评估 `pg_textsearch` 或兼容扩展 |
| 文档数据 | `jsonb` 和 GIN 索引 |
| 时序数据 | TimescaleDB；不具备该扩展时使用原生分区表和时间索引 |
| 向量检索 | pgvector |
| 二进制对象 | `bytea`；仅在对象大小或吞吐超过数据库边界时评估 S3 |

引入独立中间件前，必须以容量、延迟、隔离或协议语义限制为依据建立 ADR，并说明 PostgreSQL 方案为何不足。

## 2. 当前替代状态

- Redis 授权缓存由 `cache_entries` 替代。该表为 `UNLOGGED`，保存短 TTL、可安全重建的数据。
- S3/MinIO Repository Bundle 存储由 `repository_bundles.bundle` 的 `bytea` 替代，直接保存原始 Git Bundle，不使用 FlatBuffers。
- Redpanda/Kafka 任务与投递由 `inspection_jobs`、`notification_deliveries` 等 PostgreSQL 表替代，使用事务、租约和 `FOR UPDATE SKIP LOCKED`。
- 默认本地部署只启动含 pgvector 的 PostgreSQL；Redis、Redpanda、MinIO 均不是默认 Compose 服务。

Redis 缓存和 S3-compatible 存储仍是显式配置的可选后端，用于已有外部基础设施或 PostgreSQL 已超过适用边界的部署；启用它们同样需要在部署说明中记录理由。
