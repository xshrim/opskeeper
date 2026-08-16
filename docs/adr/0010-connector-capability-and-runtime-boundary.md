# ADR-0010：Connector 使用能力接口、资源配置和受控运行边界

- 状态：已接受，T09 已完成
- 日期：2026-08-16

## 背景

Kubernetes、Prometheus 和 Loki 使用不同协议，但后续 Skill 和诊断需要按“读取 Kubernetes、查询指标、查询日志、读取告警”等能力工作。如果每个调用方直接读取资源 JSON、解密凭据并访问上游，权限、超时、限流、错误分类和敏感信息边界会散落在多个模块中。另一方面，Connector 只是资源的执行方式，不应成为第二套资源目录或复制资源配置。

## 决策

1. Connector 是独立的外部协议与能力层，不是资源类型。`resources` 和关联的加密凭据是连接配置的权威来源。
2. 注册表按资源 `kind + schema_version` 解析适配器；重叠版本注册被拒绝，未知类型或版本返回 `unsupported`。
3. 上层依赖小型能力接口：`kubernetes_read`、`query_metrics`、`query_logs`、`query_traces` 和 `get_alerts`。首批实现 Kubernetes、Prometheus 和 Loki。
4. 每次调用统一受超时、有限重试、全局并发、查询窗口、结果数量和响应大小限制；只重试明确标记为临时的失败。
5. 返回值统一携带来源资源、能力、采集时间、查询窗口、摘要、原始 JSON 和 `partial` 标记，作为后续 Evidence 候选。
6. 失败使用稳定类别并向用户返回安全消息。上游正文、凭据、Token 和 kubeconfig 不进入连接检查记录或审计详情。
7. 公共 HTTP 在 T09 只开放连接测试和最近结果，并分别要求 `resource:use`、`resource:read`。实际查询只保留为 Go API，待 Runner 在 T10 统一实施工具白名单、目标授权和预算控制。
8. 连接检查持久化在 PostgreSQL，保存状态、分类、安全消息、耗时、能力、操作者和时间；不把瞬时结果写回资源配置。

## 后果

- 新协议可以通过注册适配器扩展，调用方不需要了解凭据格式和上游 API。
- Scope 与资源级授权继续以现有资源为目标，不增加 Connector 权限对象。
- 单进程全局并发限制可以控制当前阶段风险；按租户、资源或上游隔离的配额和熔断在获得真实负载数据后再引入。
- T10 Runner 是查询能力对 LLM 和 Skill 的唯一出口，避免当前阶段提前公开可被任意组合的查询 HTTP API。
