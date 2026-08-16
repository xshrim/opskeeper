# OpsKeeper

OpsKeeper 是面向 Kubernetes 业务应用和各类中间件的 AI 运维值守平台。系统以平台、团队、项目三级 Scope 组织用户、资源和权限，目标是统一资源发现、证据驱动诊断、自动巡检、受控操作和审计流程。

本 README 是项目文档的统一入口。设计事实、工程规则、操作步骤、实施计划和验收证据分别维护在对应文档中，不在此重复描述。当前进度和后续范围见[分阶段实施任务书](docs/planning/implementation-tasks.md)。

## 快速开始

前置软件：Go 1.26、Node.js 22、npm 11、Docker，以及 Docker Compose v2 或独立 `docker-compose`。

```bash
# 1. 生成应用本地配置
cp .env.example .env

# 2. 生成 PostgreSQL 和 Redis 本地配置
cp deploy/compose/.env.example deploy/compose/.env

# 3. 安装 Go 和前端依赖
make deps

# 4. 启动 PostgreSQL 和 Redis
make infra-up

# 5. 执行数据库迁移
make migrate

# 6. 通过交互式受控流程创建首个管理员
make admin-create

# 7. 构建并嵌入前端，然后运行 API
make run-front-api
```

默认访问地址：`http://localhost:8080/opskeeper/`。完整的环境要求、分离开发方式、配置项和常用命令见[本地开发](docs/guides/development.md)。

## 文档分类

| 目录 | 内容 | 使用场景 |
|---|---|---|
| `docs/design/` | 已实现事实、目标设计、领域模型和架构边界 | 理解系统现在是什么以及将如何演进 |
| `docs/standards/` | 必须遵守的编码、Git 和工程约束 | 开发、重构和代码评审 |
| `docs/guides/` | 本地开发、测试、迁移和自动化发布步骤 | 执行具体工程操作 |
| `docs/planning/` | 阶段规划、任务拆分、未决事项和审批状态 | 确认当前进度和后续范围 |
| `docs/acceptance/` | 已完成阶段的验收步骤、结果和证据 | 复核已交付能力 |
| `docs/adr/` | 重要技术决策的背景、选项和结论 | 追溯为什么采用当前方案 |
| `docs/runbooks/` | 生产故障处置、恢复和巡检操作 | 处理运行事件 |

## 设计

- [总体架构设计](docs/design/architecture.md)
- [三级权限与安全设计](docs/design/authorization.md)
- [组织、资源与拓扑模型](docs/design/resource-model.md)
- [Kubernetes 导入、AI 诊断与自动巡检](docs/design/operations-workflows.md)

## 工程规范

- [Go 编码与工程组织通用规范](docs/standards/go-coding-conventions.md)
- [Git 版本控制与开发模式](docs/standards/version-control.md)

## 操作指南

- [本地开发](docs/guides/development.md)
- [自动化发布](docs/guides/delivery.md)

## 计划与验收

- [分阶段实施任务书](docs/planning/implementation-tasks.md)
- [待确认事项与未解决问题](docs/planning/open-items.md)
- [T01 工程初始化与质量基线验收记录](docs/acceptance/t01.md)
- [T02 Scope 与组织模型验收记录](docs/acceptance/t02.md)
- [T03 本地身份与会话验收记录](docs/acceptance/t03.md)
- [T04 三级 RBAC 与数据隔离验收记录](docs/acceptance/t04.md)
- [T05 身份权限管理与安全审计验收记录](docs/acceptance/t05.md)
- [T06 统一资源目录、凭据和关系模型验收记录](docs/acceptance/t06.md)
- [T07 Svelte 管理控制台基础功能验收记录](docs/acceptance/t07.md)
- [T08 Kubernetes 集群发现与项目导入验收记录](docs/acceptance/t08.md)
- [T09 Connector 框架与外部监控平台验收记录](docs/acceptance/t09.md)

## 架构决策与运行手册

- [架构决策记录索引](docs/adr/README.md)
- [生产运行手册索引](docs/runbooks/README.md)

## 文档维护原则

1. 已实现事实和目标设计写入 `docs/design/`，目标内容必须标注计划任务；强制性工程规则写入 `docs/standards/`。
2. 可执行步骤写入 `docs/guides/`；已成形的阶段任务和零散跨阶段事项分别写入 `implementation-tasks.md` 与 `open-items.md`。
3. 验收记录只保存已经执行的步骤和结果，不用于描述后续计划。
4. 影响长期技术方向的重要决定写入编号 ADR；ADR 记录决策历史，不替代当前设计文档。
5. 生产事件的诊断和恢复步骤写入 `docs/runbooks/`，不与自动化发布流程混写。
6. 每项信息只在一个权威位置维护；其他文档通过链接引用，不复制完整规则。
7. 移动或重命名文档时，必须同步更新本索引和仓库内的全部引用。
