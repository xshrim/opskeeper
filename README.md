# OpsKeeper

OpsKeeper 是面向 Kubernetes 和中间件的 AI 运维平台，提供资源目录、证据驱动诊断、自动巡检、受控操作和审计。权限以平台、团队、项目三级 Scope 为边界。

## 快速开始

前置：Go 1.26.5+、Node.js 22、npm 11、Docker 和 Docker Compose v2。

### 容器服务

```bash
make up
```

`make up` 会创建 `.env`、构建 `opskeeper-app` 镜像，并启动 PostgreSQL 与嵌入式 App。App 启动时自动迁移数据库；首次打开登录页时初始化管理员密码。不需要单独执行 `migrate` 或 `admin` 命令。

停止或清理：

```bash
make down     # 停止 PostgreSQL 和 App，保留数据
make clean    # 删除 Compose 容器、网络和数据卷
```

需要拆分运行 API、Worker、Scheduler 和 Frontend 时：

```bash
make api-up
make worker-up
make scheduler-up
make frontend-up
```

### 本机开发服务

```bash
make env
make db-up
make api-run       # 另开终端运行 Worker、Scheduler 或 Frontend
```

嵌入前端的单进程开发入口：

```bash
make run           # 启动 PostgreSQL 后运行 app-run
```

完整命令、配置和验证见[开发指南](docs/guides/development.md)。

## 文档入口

| 需要做什么 | 文档 |
| --- | --- |
| 了解文档边界和目录 | [文档总览](docs/README.md) |
| 本地运行、测试、配置、镜像 | [开发指南](docs/guides/development.md) |
| 生产构建、Compose/Helm 发布、迁移和回滚 | [部署指南](docs/guides/deployment.md) |
| 管理员、用户、巡检和故障处理 | [运维指南](docs/guides/operations.md) 与 [Runbook](docs/runbooks/) |
| MCP、内置 Skill 和数据库诊断 | [集成指南](docs/guides/integrations.md) 与 [诊断指南](docs/guides/diagnostics.md) |
| 当前架构和领域边界 | [设计文档](docs/design/) |
| 强制工程规则 | [工程规范](docs/standards/) |
| 长期技术决策 | [ADR](docs/adr/README.md) |
| 当前迭代和验收证据 | [迭代记录](docs/iterations/) |

## 常用命令

```bash
make help       # 查看全部入口
make test       # Go 与前端单元测试
make lint       # 静态检查和部署脚本检查
make build      # 构建本地二进制
make image      # 构建 API、Worker、Scheduler、Frontend、App 五个镜像
```

可观察性组件：`make obs-up`、`make obs-down`、`make obs-clean`、`make obs-logs`。数据库与可观察性一起管理使用 `make infra-up`、`make infra-down`、`make infra-clean`、`make infra-logs`。

Sample 数据：`make sample-seed` 写入标记数据，`make sample-clean` 只删除这些标记数据。两者都读取根目录 `.env`，只适用于本地开发库。

## 文档规则

当前事实只维护一份；其他文档用链接引用。设计描述“系统是什么”，指南描述“如何操作”，Runbook 描述“故障时怎么处理”，ADR 和迭代目录保留决策及历史证据。详见[文档规范](docs/standards/documentation.md)。
