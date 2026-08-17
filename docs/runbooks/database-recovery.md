# PostgreSQL 备份与 PITR 恢复

**适用范围**：业务数据库误删、损坏或迁移后不一致。
**前置权限**：数据库管理员、Kubernetes 发布管理员、已批准变更单。

1. 停止新发布和写入流量，记录事件时间、当前镜像 digest 和 `schema_migrations`。
2. 保留故障实例和 WAL；不在原实例上试验恢复。
3. 在隔离 PostgreSQL 实例中恢复最近全量备份，重放 WAL 到事件前的目标时间。
4. 以只读方式核对关键表计数、最新审计事件、Skill/LLM 版本及巡检任务终态。
5. 运行与目标应用 digest 相同的 `opskeeper-migrate up`；不运行 `down`。
6. 更新 runtime Secret 指向恢复实例，先启动单个 API 副本做 readiness 和只读冒烟检查。
7. 恢复 API、Worker、Scheduler，观察错误率、队列租约和重复通知，再恢复流量。

回退方式是将 Secret 重新指向保留的原实例并恢复上一镜像，前提是原实例未被继续写入。每季度至少在隔离环境执行一次备份恢复演练，记录 RPO/RTO 和校验和。
