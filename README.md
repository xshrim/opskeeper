# OpsKeeper

OpsKeeper 是面向 Kubernetes 业务应用和各类中间件的 AI 运维值守平台。系统以平台、团队、项目三级 Scope 组织用户、资源和权限，目标是统一资源发现、证据驱动诊断、自动巡检、受控操作和审计流程。

> **产品命名备注**：当前产品名称仍为 **OpsKeeper**。经过品牌命名评估，后续可能调整为暂定名称 **OpSeek**；本备注只记录命名意向，本次不修改代码、界面、接口、环境变量或仓库标识。

本 README 是项目文档的统一入口。设计事实、工程规则、操作步骤、迭代需求和验收证据分别维护在对应文档中，不在此重复描述。文档组织规则见[文档规范](docs/standards/documentation.md)。

## 快速开始

前置软件：Go 1.26.5 或更高版本、Node.js 22、npm 11、Docker，以及 Docker Compose v2 或独立 `docker-compose`。

### 容器数据库 + 容器服务

```bash
# 一键构建镜像并启动 PostgreSQL、API、Worker 和 Scheduler
make start
```

`.env` 不存在时 `make start` 会从 `.env.example` 创建，并同步 Compose 环境文件。镜像由 `make image` 单独构建，随后再由 Compose 启动；`docker-compose.yml` 不包含 `build`。API 启动时自动迁移数据库。首次访问登录页时初始化 `admin` 密码，不需要单独运行 migrate 或 admin 命令。

### 容器数据库 + 本机二进制服务

```bash
# 仅首次创建；已有 .env 时保留现有配置
test -f .env || cp .env.example .env
make infra-up
make build
```

`make infra-up` 只启动 PostgreSQL。然后在三个终端分别加载 `.env` 并运行服务：

```bash
set -a; source .env; set +a; backend/bin/opskeeper-api
```

```bash
set -a; source .env; set +a; backend/bin/opskeeper-worker
```

```bash
set -a; source .env; set +a; backend/bin/opskeeper-scheduler
```

API 会自动迁移数据库；管理员仍在首次登录页面初始化。停止二进制时按 `Ctrl+C`，再运行 `make infra-down` 停止 PostgreSQL。两种方式的完整命令、开发模式入口和 sample 数据说明见[本地开发指南](docs/guides/development.md)。

本地查看链路数据时，单独启动可观测服务并配置 API、Worker 或 Scheduler：

```bash
make obs-up
# 在 .env 中设置：OPSK_OTEL_ENDPOINT=http://127.0.0.1:4318
make api-run
```

本地监控组件包括 Jaeger（链路）、Prometheus（指标）、Loki 和 Grafana（统一展示）以及 OTel Collector（OTLP 接收与转发）。Jaeger UI 地址为 `http://localhost:16686`，Grafana 地址为 `http://localhost:3000`（Grafana 初始账号密码默认为 `admin/admin`）。当前应用日志尚未接入 Loki，仍写在服务标准输出；不需要监控组件时执行 `make obs-down`。

已有 PostgreSQL 后，要在本机运行嵌入式 Web/API，可执行：

```bash
make run
```

`make run` 会构建并嵌入前端，最后通过 `go run` 启动 API；API 启动时自动应用待执行迁移。它不会自动启动 PostgreSQL，也不需要额外的 migrate/admin 命令。访问地址仍为 `http://localhost:8080/opskeeper/`。

## 文档分类

| 目录                 | 内容                                   | 使用场景               |
| -------------------- | -------------------------------------- | ---------------------- |
| `docs/design/`     | 项目级当前设计事实和架构边界           | 理解系统现在是什么     |
| `docs/standards/`  | 必须遵守的编码、文档、Git 和工程约束   | 开发、重构和代码评审   |
| `docs/guides/`     | 本地开发、使用、测试和交付步骤         | 执行具体工程操作       |
| `docs/runbooks/`   | 生产故障处置、恢复和应急步骤           | 处理运行事件           |
| `docs/iterations/` | 当前迭代、需求、任务和验收；封板后归档 | 管理版本范围和交付证据 |
| `docs/adr/`        | 重要技术决策的背景、选项和结论         | 追溯长期架构决策       |
| `docs/backlog.md`  | 跨迭代待规划事项和技术债               | 规划后续需求           |

## 设计

- [项目概览](docs/design/overview.md)
- [总体架构设计](docs/design/architecture.md)
- [三级权限与安全设计](docs/design/authorization.md)
- [组织、资源与拓扑模型](docs/design/resource-model.md)
- [大模型领域对象：Provider、Skill、Persona 与 Engine](docs/design/llm-domains.md)
- [统一资源接入设计](docs/design/resource-access.md)
- [图标系统设计](docs/design/icon-system.md)

## 工程规范

- [文档组织、模板与归档规范](docs/standards/documentation.md)
- [前端 UI 与交互规范](docs/standards/frontend-ui.md)
- [后端日志规范](docs/standards/backend-logging.md)
- [Go 编码与工程组织通用规范](docs/standards/go-coding-conventions.md)
- [PostgreSQL First 中间件规约](docs/standards/postgresql-first.md)
- [Git 版本控制与开发模式](docs/standards/version-control.md)

## 操作指南

- [本地开发](docs/guides/development.md)
- [自动化发布](docs/guides/delivery.md)
- [内置诊断 Skill 与最小权限](docs/guides/builtin-skills.md)
- [自动巡检、健康评分和通知](docs/guides/inspection.md)
- [Docker MCP Server](docs/guides/docker-mcp.md)
- [生产环境 Helm 部署](docs/guides/production-deployment.md)
- [管理员手册](docs/guides/administration.md)
- [用户手册](docs/guides/user-guide.md)
- [已知限制与容量边界](docs/known-limitations.md)

## 计划与验收

- [跨迭代 backlog](docs/backlog.md)
- [I002 UI 与功能优化迭代](docs/iterations/I002-ui-function-optimization/iteration.md)
- [I002-R001 需求文档](docs/iterations/I002-ui-function-optimization/R001-requirement.md)
- [I002-R001 需求验收报告](docs/iterations/I002-ui-function-optimization/R001-requirement-acceptance.md)
- [I005 Skill 与智能巡检迭代](docs/iterations/I005-skill-intelligent-inspection/iteration.md)
- [I005-R001 需求文档](docs/iterations/I005-skill-intelligent-inspection/R001-requirement.md)
- [I005-R001 需求验收报告](docs/iterations/I005-skill-intelligent-inspection/R001-requirement-acceptance.md)
- [I006 通知机制迭代](docs/iterations/I006-notification-mechanism/iteration.md)
- [I006-R001 需求文档](docs/iterations/I006-notification-mechanism/R001-requirement.md)
- [I006-R001 需求验收报告](docs/iterations/I006-notification-mechanism/R001-requirement-acceptance.md)
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
