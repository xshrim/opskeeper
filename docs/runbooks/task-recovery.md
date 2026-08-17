# Worker、Scheduler 和 Redis 故障恢复

1. 检查 PostgreSQL readiness、Redis readiness、Worker/Scheduler Pod 重启和 `opskeeper.errors` 指标。
2. Redis 不可用时，授权缓存会回源 PostgreSQL；不要为恢复缓存而放宽授权。恢复 Redis 后允许缓存自然重建。
3. Worker 中断时不直接修改运行状态。等待租约过期，由新 Worker 通过 `SKIP LOCKED` 重新领取。
4. Scheduler 恢复后根据幂等调度窗口补建运行；不手工复制任务行。
5. 通知失败由持久化投递记录重试。复核幂等键后再处理最终失败，避免重复 Webhook。

验证完成条件：无超过两个租约周期的 `running` 任务，新调度窗口只有一条运行，通知幂等键无重复成功记录。
