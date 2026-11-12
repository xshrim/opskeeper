# 文档总览

文档按“当前事实、可执行步骤、强制规则、历史证据”分层。一个事实只保留一个权威位置，其他地方只链接，不复制完整规则。

## 从哪里开始

| 目标 | 入口 |
| --- | --- |
| 本地开发、测试、环境变量和镜像 | [guides/development.md](guides/development.md) |
| 构建、Compose、Helm、迁移、发布和回滚 | [guides/deployment.md](guides/deployment.md) |
| 管理员、用户、巡检、通知和运行检查 | [guides/operations.md](guides/operations.md) |
| MCP、资源接入、内置 Skill 和关系型诊断 | [guides/integrations.md](guides/integrations.md)、[guides/diagnostics.md](guides/diagnostics.md) |
| 故障恢复 | [runbooks/](runbooks/) |
| 当前架构和领域设计 | [design/](design/) |
| 编码、UI、日志、Git 和文档规则 | [standards/](standards/) |
| 长期决策 | [adr/README.md](adr/README.md) |
| 迭代范围和验收记录 | [iterations/](iterations/) |

## 目录职责

```text
docs/
├── README.md             # 本索引
├── design/               # 当前系统设计和领域边界
├── guides/               # 可执行步骤，按场景合并维护
├── runbooks/             # 生产故障、恢复和应急处置
├── standards/            # 必须遵守的工程规则
├── adr/                  # 长期技术决策及其历史
├── iterations/           # 当前迭代；封板后移入 archived/
├── templates/            # 迭代、需求和验收模板
├── backlog.md            # 尚未形成完整范围的事项
└── known-limitations.md  # 当前限制和容量边界
```

## 维护规则

- 设计文档只描述当前实现或已确认的目标边界，不写临时计划。
- 指南只保留能直接执行的步骤；配置和命令以 Makefile、Compose、Helm 为准。
- Runbook 只描述故障处置，不重复部署指南。
- ADR 记录为什么作出决定；若决定改变，新增或更新 ADR，并同步当前设计文档。
- 迭代文档记录范围和验收证据，不作为日常使用手册。
- 已封板迭代只读；新范围创建新迭代或进入 `backlog.md`。
- 移动、合并或删除文档后，必须更新全仓库链接并运行链接检查。
