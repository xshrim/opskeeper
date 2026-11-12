# 管理员手册

## 首次初始化

1. 启动 API，确认其自动迁移已完成并通过 readiness。
2. 首次访问登录页时设置 `admin` 的密码；该操作仅在平台尚未初始化管理员时成功。
3. 登录后立即创建个人管理账号，将 bootstrap 凭据移出日常使用流程。
4. 创建团队、项目和最小权限用户组，再登记资源与凭据。

## 日常检查

- 确认 API readiness、Worker/Scheduler 日志和任务失败指标。
- 复核高风险操作请求；批准人不得与请求人相同。
- 检查权限绑定、停用用户、失效会话和审计导出。
- 检查巡检租约、Webhook 失败及 LLM 降级状态。

## 审计保留

`audit_events` 禁止普通 UPDATE/DELETE。只能在已完成不可变导出并验证校验和后，由数据库管理员执行：

```sql
SELECT prune_audit_events(
  now() - interval '365 days',
  's3://audit-archive/2025/manifest-sha256.txt',
  'change-ticket-1234'
);
```

函数拒绝 30 天内数据，并向不可变的 `audit_retention_runs` 写入截止时间、导出引用、变更单和删除数量。

用户、Scope、凭据、MCP 和操作审批详细边界分别见授权、资源和 MCP 指南。
