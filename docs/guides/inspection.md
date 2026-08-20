# 自动巡检、健康评分和通知

自动巡检由三个独立进程协作：API 管理策略与查询记录，Scheduler 根据策略生成任务，Worker 领取并执行任务。生产环境应至少各运行一个 Scheduler 和 Worker；多个副本是安全的。

## 创建策略

策略属于平台、团队或项目作用域。它只能选择当前作用域可覆盖且调用者拥有资源权限的目标和 Skill。策略包含标准 Cron、IANA 时区、明确目标或标签选择器、Skill 集、超时、重试次数、并发与工具/Token 预算。维护窗口内不创建定时任务。

Scheduler 以策略时区按分钟计算 Cron；同一个“策略 + 调度窗口”在数据库中唯一。它同时使用 PostgreSQL advisory lock，因此多个 Scheduler 轮询不会产生重复任务。

## 执行、评分与恢复

Worker 领取任务时获得租约，并在执行期间发送心跳。异常退出后，租约到期即可由其他 Worker 重新领取。普通失败在未耗尽重试次数前会以指数退避回队；耗尽后运行标记失败。运行启动即保存策略与目标快照，后续资源或策略变化不影响这一次执行。

确定性 Connector 检查先执行；健康分数只由其规则严重度与权重计算，最低目标分数作为运行分数。LLM 仅可解释已产生的异常，不能创建、删除或改变健康分数。没有异常时不请求 LLM；存在异常但模型不可用时，运行仍成功完成确定性部分，`llm_status` 会显示 `degraded`。

Finding 有两类键：目标与规则构成持续身份，用于保持、恢复与重新打开；目标、规则与调度窗口构成观测指纹，用于同一窗口的去重。一个成功运行没有再次观测到旧规则时，会将对应 Finding 标记为已恢复。

## Webhook 通知

首版通知渠道为 HTTPS Webhook。请求为 JSON，带有 `X-OpsKeeper-Timestamp`（Unix 秒）和 `X-OpsKeeper-Signature`（对 `timestamp + "." + body` 的 HMAC-SHA256）。实现限制响应体为 64 KiB、默认超时 10 秒；渠道的速率限制、重试和投递记录由持久队列实施。

Webhook 只应接收并验证通知，不能据此触发无人值守变更。邮件、短信、容量预测和自动处置不在 T13 范围内。

## 本地运行

先在一个终端运行 API，再分别运行 Scheduler 与 Worker：

```bash
make api-run
make scheduler-run
make worker-run
```

三者均从 `.env` 读取数据库、凭据加密、Connector 限制及 `OPSK_INSPECTION_*` 设置。迁移必须在部署新 API/Worker/Scheduler 前由受控发布流程单实例执行，详见 [自动化发布](delivery.md)。
