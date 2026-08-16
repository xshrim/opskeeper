# Custom Skill 沙箱部署基线

本目录是需要执行自定义代码的 Skill 的 Kubernetes Job 安全基线。API、Worker 和 Scheduler 不执行用户提供的命令、脚本或 MCP `stdio` 命令；它们只能创建已经通过审批的 Job 规格并由受限集群工作负载运行。

`serviceaccount.yaml` 禁止自动挂载令牌，`role.yaml` 不授予任何 API 权限，`networkpolicy.yaml` 默认拒绝所有出站网络。若某项明确审批的 Job 需要访问目标，部署方必须另行创建仅含目标 CIDR、端口和 DNS 所需范围的 NetworkPolicy，并将该策略与操作审批记录关联。不得以“调试”为由使用 `privileged`、hostPath、hostNetwork、root 用户或宽泛集群管理员令牌。
