# 生产运行手册

本目录保存生产故障诊断、恢复、巡检和应急处置步骤。Runbook 应明确适用范围、前置权限、风险、验证方法和回退方式。

- [PostgreSQL 备份与 PITR 恢复](database-recovery.md)
- [凭据与密钥轮换](credential-rotation.md)
- [Worker、Scheduler 和 Redis 故障恢复](task-recovery.md)
- [升级与回滚](upgrade-rollback.md)

构建、数据库迁移和应用发布流程参见[自动化发布](../guides/delivery.md)。
