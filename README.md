# OpsKeeper

OpsKeeper 是面向 Kubernetes 业务应用和各类中间件的 AI 运维值守平台。系统以平台、团队、项目三级 Scope 组织用户、资源和权限，目标是统一资源发现、证据驱动诊断、自动巡检、受控操作和审计流程。

本 README 是项目文档的统一入口。设计事实、工程规则、操作步骤、迭代需求和验收证据分别维护在对应文档中，不在此重复描述。文档组织规则见[文档规范](docs/standards/documentation.md)。

## 快速开始

前置软件：Go 1.26.5 或更高版本、Node.js 22、npm 11、Docker，以及 Docker Compose v2 或独立 `docker-compose`。

### 分步启动

```bash
# 1. 生成应用本地配置
cp .env.example .env

# 2. 生成 PostgreSQL 和 Redis 本地配置
cp deploy/compose/.env.example deploy/compose/.env

# 3. 安装 Go 和前端依赖
make deps

# 4. 启动并等待 PostgreSQL 和 Redis
make infra-up

# 5. 执行数据库迁移
make migrate

# 6. 创建首个管理员
make admin-create

# 7. 构建并嵌入前端，然后运行 API
make run-front-api
```

`make admin-create` 在未提供密码时生成随机密码并打印一次，请立即保存。

### 一键启动

```bash
make start
```

该命令依次完成上述步骤，并只在系统尚无用户时创建默认用户名为 `admin` 的管理员。默认访问地址：`http://localhost:8080/opskeeper/`；按 `Ctrl+C` 停止 API，使用 `make infra-down` 停止中间件。完整的环境要求、分离开发方式、配置项和常用命令见[本地开发](docs/guides/development.md)。

## 文档分类

| 目录                 | 内容                                     | 使用场景                         |
| -------------------- | ---------------------------------------- | -------------------------------- |
| `docs/design/`     | 项目级当前设计事实和架构边界             | 理解系统现在是什么               |
| `docs/standards/`  | 必须遵守的编码、文档、Git 和工程约束     | 开发、重构和代码评审             |
| `docs/guides/`     | 本地开发、使用、测试和交付步骤           | 执行具体工程操作                 |
| `docs/runbooks/`   | 生产故障处置、恢复和应急步骤             | 处理运行事件                     |
| `docs/iterations/` | 当前迭代、需求、任务和验收；封板后归档   | 管理版本范围和交付证据           |
| `docs/adr/`        | 重要技术决策的背景、选项和结论           | 追溯长期架构决策                 |
| `docs/backlog.md`  | 跨迭代待规划事项和技术债                 | 规划后续需求                     |

## 设计

- [项目概览](docs/design/overview.md)
- [总体架构设计](docs/design/architecture.md)
- [三级权限与安全设计](docs/design/authorization.md)
- [组织、资源与拓扑模型](docs/design/resource-model.md)
- [Kubernetes 导入、AI 诊断与自动巡检](docs/design/operations-workflows.md)

## 工程规范

- [文档组织、模板与归档规范](docs/standards/documentation.md)
- [Go 编码与工程组织通用规范](docs/standards/go-coding-conventions.md)
- [Git 版本控制与开发模式](docs/standards/version-control.md)

## 操作指南

- [本地开发](docs/guides/development.md)
- [自动化发布](docs/guides/delivery.md)
- [内置诊断 Skill 与最小权限](docs/guides/builtin-skills.md)
- [自动巡检、健康评分和通知](docs/guides/inspection.md)
- [MCP、受控操作与自定义 Skill 沙箱](docs/guides/mcp-operations.md)
- [生产环境 Helm 部署](docs/guides/production-deployment.md)
- [管理员手册](docs/guides/administration.md)
- [用户手册](docs/guides/user-guide.md)
- [已知限制与容量边界](docs/known-limitations.md)

## 计划与验收

- [跨迭代 backlog](docs/backlog.md)
- [I001 迭代封板说明](docs/iterations/archived/I001-initial/iteration.md)
- [I001-R001 需求文档](docs/iterations/archived/I001-initial/R001-requirement.md)
- [I001-R001 需求验收报告](docs/iterations/archived/I001-initial/R001-requirement-acceptance.md)
- 新迭代直接创建在 `docs/iterations/Ixxx-name/`，封板后移动到 `docs/iterations/archived/`

## 架构决策与运行手册

- [架构决策记录索引](docs/adr/README.md)
- [数据库恢复](docs/runbooks/database-recovery.md)
- [任务恢复](docs/runbooks/task-recovery.md)
- [升级与回滚](docs/runbooks/upgrade-rollback.md)

## 文档维护原则

1. 当前项目设计事实写入 `docs/design/`；强制性工程和文档规则写入 `docs/standards/`。
2. 可执行步骤写入 `docs/guides/`；生产事件处置步骤写入 `docs/runbooks/`。
3. 迭代范围、需求和任务写入 `docs/iterations/`；需求验收报告只保存已经执行的步骤和结果。
4. 影响长期技术方向的重要决定写入编号 ADR；ADR 记录决策历史，不替代当前设计文档。
5. 跨迭代待规划事项和技术债写入 `docs/backlog.md`；已知产品限制写入 `docs/known-limitations.md`。
6. 每项信息只在一个权威位置维护；其他文档通过链接引用，不复制完整规则。
7. 迭代封板后整体移动到 `docs/iterations/archived/`，历史文档原则上只读。
8. 移动或重命名文档时，必须同步更新本索引和仓库内的全部引用。
