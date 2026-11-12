# 开发指南

所有命令从仓库根目录执行。项目支持两种开发方式：容器 PostgreSQL + 本机进程，或容器 PostgreSQL + Compose 服务。

## 1. 初始化环境

```bash
make env
```

`make env` 会创建根目录 `.env`（不存在时复制 `.env.example`），并同步到 `deploy/compose/.env` 和 `deploy/compose/.env.example`。只修改根目录 `.env`；它是本机进程和 Compose 的配置源。

安装依赖：

```bash
make deps
```

## 2. 运行方式

### 容器 PostgreSQL + 本机进程

```bash
make db-up
make build                 # 生成 API、App、Worker、Scheduler 二进制
```

分别在终端运行：

```bash
make api-run
make worker-run
make scheduler-run
make frontend-run          # Vite 热更新页面
```

只运行一个嵌入前端的进程：

```bash
make app-run
```

这些 `*-run` 命令直接运行开发源码，不负责启动 PostgreSQL；`make run` 是组合入口，会先执行 `env`、启动数据库，再运行 `app-run`。

### 容器 PostgreSQL + 容器服务

默认运行嵌入式 App：

```bash
make up
```

`make up` 等价于 `env` + `app-image` + Compose 启动 `postgres app`。Compose 从不 build 镜像。

分别运行服务：

```bash
make api-up
make worker-up
make scheduler-up
make frontend-up
```

这些服务都在 `svc` profile 中；`app` 不在 profile 中。对应的 `*-up` 命令都会先构建自己的镜像。

停止和清理：

```bash
make down       # 停止 PostgreSQL 和 App，保留数据卷
make clean      # 删除全部 Compose 容器、网络和数据卷
make db-down    # 只停止 PostgreSQL
make db-clean   # 删除 PostgreSQL 容器和数据卷
```

## 3. 命令速查

| 类别 | 命令 |
| --- | --- |
| 环境 | `env`、`deps` |
| 本机运行 | `api-run`、`worker-run`、`scheduler-run`、`frontend-run`、`app-run` |
| 单独构建 | `api-image`、`worker-image`、`scheduler-image`、`frontend-image`、`app-image` |
| 全部构建 | `image` |
| 容器运行 | `api-up`、`worker-up`、`scheduler-up`、`frontend-up`、`app-up` |
| 数据库 | `db-up`、`db-down`、`db-clean`、`db-logs` |
| 可观察性 | `obs-up`、`obs-down`、`obs-clean`、`obs-logs` |
| 数据库 + 可观察性 | `infra-up`、`infra-down`、`infra-clean`、`infra-logs` |
| 质量 | `test`、`lint`、`build`、`quality` |
| Sample 数据 | `sample-seed`、`sample-clean` |

## 4. 配置要点

`.env.example` 按用途分组。通常只需修改：

```dotenv
IMAGE_REPOSITORY=opskeeper
IMAGE_TAG=local
OPSK_DB_HOST=localhost:5432
OPSK_DB_NAME=opskeeper
OPSK_DB_USER=opskeeper
OPSK_DB_PASSWORD=opskeeper
OPSK_LISTEN_PORT=8080
OPSK_CREDENTIAL_KEY=
OPSK_OTEL_ENDPOINT=http://127.0.0.1:4318
```

Compose 会把数据库主机改为 `postgres:5432`，把 OTLP 地址改为 `http://otel-collector:4318`。`OPSK_DB_SSLMODE` 默认关闭，不需要写入 `.env`。缓存和 Bundle 存储默认使用 PostgreSQL；只有选择 Redis、Local 或 S3 时才填写对应配置。

## 5. 数据、管理员和迁移

API/App 启动时先连接 PostgreSQL、获取迁移锁并应用待执行迁移；迁移失败则不监听 HTTP 端口。Worker 和 Scheduler 不执行迁移，只在数据库可用后持续重试。

首次打开登录页时初始化管理员密码。不存在独立的 `migrate` 或 `admin` 命令。

在 API/App 已启动并完成迁移后写入或清理本地演示数据：

```bash
make sample-seed
make sample-clean
```

命令只允许连接本地 development 数据库，并只删除带有 sample 标记的数据。

## 6. 可观察性

```bash
make obs-up
make api-run             # 本机进程使用 .env 中的 127.0.0.1:4318
make obs-logs
make obs-down
```

应用通过 OTLP/HTTP 把 Trace 和 Metrics 发给 Collector；Collector 将 Trace 转给 Jaeger、Metrics 写入 Prometheus，Grafana 提供预置展示。应用日志仍写标准输出，可用 `docker compose logs` 查看，当前未接入 Loki 日志采集。

## 7. 检查与构建

```bash
LC_ALL=C make test
make lint
make build
make image
```

生产镜像分别为 `opskeeper-api`、`opskeeper-worker`、`opskeeper-scheduler`、`opskeeper-frontend` 和 `opskeeper-app`。构建时通过 `IMAGE_REPOSITORY`、`IMAGE_TAG`、`VERSION`、`COMMIT` 和 `BUILD_TIME` 固定制品身份。

更多设计细节见 [design/](../design/)，发布流程见[部署指南](deployment.md)。
