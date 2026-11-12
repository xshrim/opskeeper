# 运维指南

## 1. 首次使用

1. 启动 API/App，并确认 `/health/ready` 成功。
2. 首次打开登录页，设置 `admin` 密码。
3. 创建日常管理账号、团队、项目和最小权限用户组。
4. 登记资源，先执行连接测试，再建立资源关系。

管理员不应长期使用 bootstrap 账号。资源密文由服务端加密保存，API 不返回明文。

## 2. 用户操作流程

1. 在当前平台、团队或项目 Scope 下选择目标。
2. 登记 Kubernetes、主机、Docker、数据库、消息队列和可观察性资源。
3. 在资源详情中执行连接测试并确认能力状态。
4. 在诊断中选择有权限的应用或资源，检查 Evidence 和未检查范围。
5. 为确定性检查创建巡检策略，配置 Webhook 通知。
6. Medium/High 风险操作先看 dry-run，再由不同用户审批。

没有 Evidence 的内容只能作为待验证推断；权限拒绝、超时和能力缺失都应保留在结果中。

## 3. 巡检和通知

API 管理策略，Scheduler 创建任务，Worker 使用租约领取并执行。生产环境至少运行一个 Scheduler 和一个 Worker；Worker 可扩展，Scheduler 默认单副本。

- 同一“策略 + 调度窗口”只生成一条运行记录。
- Worker 异常退出后，租约到期，其他 Worker 可重新领取。
- 失败任务按退避策略重试，耗尽后标记失败。
- 健康评分只使用确定性规则；LLM 只能解释 Finding，不能改变分数。
- Webhook 使用 HMAC-SHA256 签名、10 秒超时和持久化投递记录。

本地运行：

```bash
make api-run
make scheduler-run
make worker-run
```

## 4. 日常检查

- API readiness、Worker/Scheduler 错误和任务租约。
- 数据库连接、锁等待、慢查询和迁移日志。
- 资源权限、停用用户、过期会话和审计记录。
- 巡检失败、Webhook 投递和 LLM 降级状态。
- 高风险操作的审批人、参数摘要和执行结果。

故障时保留 Request ID、执行 ID、镜像 digest 和时间窗口，不要反复提交或放宽权限来“试错”。

## 5. 审计保留

审计事件只允许追加；清理前必须完成不可变导出、校验和验证并携带变更单。示例：

```sql
SELECT prune_audit_events(
  now() - interval '365 days',
  's3://audit-archive/2025/manifest-sha256.txt',
  'change-ticket-1234'
);
```

该函数拒绝删除 30 天内的数据，并记录保留执行结果。数据库恢复、任务恢复、密钥轮换和升级回滚见 [runbooks/](../runbooks/)。

当前版本的限制和容量边界统一维护在 [known-limitations.md](../known-limitations.md)，本页不重复列出。
