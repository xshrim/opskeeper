# 本地开发

本文是本地初始化、运行、迁移和检查的操作手册。所有命令均在仓库根目录执行。

## 1. 快速开始

前置软件：Go 1.26、Node.js 22、npm 11、Docker，以及 Docker Compose v2 或独立的 `docker-compose`。

首次初始化：

```bash
cp .env.example .env
cp deploy/compose/.env.example deploy/compose/.env
make deps
make dev-services-up
make migrate
make run-front-api
```

`make run-front-api` 先构建 Vite 前端，将制品同步到后端嵌入目录，再使用 `embed_webui` 构建标签启动 API。最终只运行一个 Go 进程，由 API 同时提供前端页面、静态资源和业务接口。它不会启动 PostgreSQL、Redis、Worker、Scheduler，也不会自动迁移数据库。

默认地址：

| 用途 | 地址 |
|---|---|
| 前端 | `http://localhost:8080/opskeeper/` |
| API 存活检查 | `http://localhost:8080/opskeeper/health/live` |
| API 就绪检查 | `http://localhost:8080/opskeeper/health/ready` |

## 2. 常用命令

| 命令 | 用途 |
|---|---|
| `make help` | 查看全部 Make 入口 |
| `make deps` | 安装 Go 和前端依赖 |
| `make dev-services-up` | 启动 PostgreSQL 和 Redis |
| `make dev-services-logs` | 持续查看中间件日志 |
| `make dev-services-down` | 停止中间件，保留数据卷 |
| `make migrate` | 应用待执行迁移 |
| `make migrate-down` | 回滚最近一条迁移，仅用于开发和测试 |
| `make run-api` | 临时运行 API |
| `make run-worker` | 临时运行 Worker |
| `make run-scheduler` | 临时运行 Scheduler |
| `make run-frontend` | 临时运行 Vite 前端 |
| `make run-front-api` | 构建并嵌入前端，然后通过一个 API 进程提供完整应用 |
| `make test` | 运行前后端单元测试 |
| `make backend-integration-test` | 运行数据库集成测试 |
| `make quality` | 执行完整本地质量门禁 |
| `make build` | 构建生产二进制制品 |
| `make image` | 构建最终应用镜像 |

应用临时运行、二进制构建和最终镜像打包的标准入口只能定义在根目录 `Makefile` 中，不得再增加 `scripts/*.sh` 等包装脚本。底层 Go、npm 和 Docker 命令属于 Make recipe 的实现细节；日常开发和流水线统一调用 `make run-*`、`make build` 和 `make image`。

## 3. 开发方式

前后端同仓但依赖独立：`backend/go.mod` 定义 Go Module `opskeeper/backend`，`frontend/package.json` 管理 Svelte 前端依赖。

运行前后端合并后的完整应用：

```bash
make dev-services-up
make run-front-api
```

`run-front-api` 与生产环境采用相同的前端嵌入和 HTTP 服务方式，但使用 `go run` 临时运行。前端源代码变化后需要重新执行该命令，不提供 Vite 热更新。

执行链路为：

```text
frontend-build
    -> frontend/dist
    -> webui-assets 复制到 backend/webui/dist
    -> go run -tags=embed_webui ./cmd/api
    -> assets_embed.go 使用 go:embed 将 backend/webui/dist 编译进 API
```

仅开发后端时运行 API：

```bash
make dev-services-up
make run-api
```

需要前端热更新时，在不同终端分开运行 API 和 Vite：

```bash
make run-api
make run-frontend
```

Vite 默认通过 `http://localhost:5173/<prefix>/` 提供页面和热更新，并把相同前缀下的 `/api` 和 `/health` 请求代理到本地 API。只执行 `make run-frontend` 时仍可打开页面，但 API 数据不可用。

Worker 和 Scheduler 按需在其他终端启动：

```bash
make run-worker
make run-scheduler
```

`make run-front-api` 不生成持久二进制；`make build` 才生成包含相同嵌入前端的生产 `opskeeper-api` 制品。

## 4. 应用配置

根目录 `.env` 供 API、Worker、Scheduler、迁移命令和 Vite 使用。`deploy/compose/.env` 只供 Docker Compose 初始化中间件使用。应用不会读取 Compose 配置，也不会获得 PostgreSQL 超级用户密码。

常用环境变量：

| 变量 | 默认值 | 作用 |
|---|---|---|
| `OPSK_BASE_PATH` | `/opskeeper` | 页面、静态资源、健康检查和业务 API 的路径前缀 |
| `OPSK_ENVIRONMENT` | `development` | 标识运行环境 |
| `OPSK_LOG_FORMAT` | `text` | Go 应用日志格式，可选 `text` 或 `json` |
| `OPSK_HTTP_ADDRESS` | `:8080` | API 监听地址 |
| `OPSK_DATABASE_URL` | 本地 `opskeeper` 连接串 | 业务数据库连接 |
| `OPSK_REDIS_URL` | `redis://localhost:6379/0` | Redis 连接 |
| `OPSK_SHUTDOWN_TIMEOUT` | `10s` | 优雅退出期限 |
| `OPSK_DEPENDENCY_TIMEOUT` | `2s` | 健康检查依赖超时 |

### 4.1 HTTP Base Path

`OPSK_BASE_PATH` 只控制 API 提供的 HTTP 路径，不影响应用名称或二进制文件名。默认值为：

```dotenv
OPSK_BASE_PATH=/opskeeper
```

根路径部署使用：

```dotenv
OPSK_BASE_PATH=/
```

还可以使用 `/platform/opskeeper` 这样的多段路径。除根路径外，每一段只能包含小写字母、数字或内部连字符，路径必须以 `/` 开头且不能以 `/` 结尾，总长度不能超过 128 个字符。

| `OPSK_BASE_PATH` | 页面 | API | 存活检查 |
|---|---|---|---|
| `/opskeeper` | `/opskeeper/` | `/opskeeper/api/v1/*` | `/opskeeper/health/live` |
| `/` | `/` | `/api/v1/*` | `/health/live` |

四个应用服务名称固定为 `opskeeper-api`、`opskeeper-worker`、`opskeeper-scheduler` 和 `opskeeper-migrate`。

### 4.2 日志格式

所有 Go 应用使用相同的 `OPSK_LOG_FORMAT`：

```bash
OPSK_LOG_FORMAT=text make run-api
OPSK_LOG_FORMAT=json make run-api
```

- `text`：默认值，适合本地终端阅读。
- `json`：适合容器平台和日志采集系统解析。
- 其他值会在应用启动时被拒绝。

格式切换不改变日志字段。API、Worker、Scheduler 和 Migration 分别写入固定的 `service` 字段。

## 5. PostgreSQL 与 Redis

启动、查看和停止中间件：

```bash
make dev-services-up
make dev-services-logs
make dev-services-down
```

`dev-services-down` 不删除持久化数据卷，再次启动会复用已有数据。PostgreSQL 初始化变量和初始化脚本仅在数据目录为空时生效；修改环境变量不会更新已有数据卷中的用户、密码或数据库所有权。

### 5.1 管理员与业务角色

默认开发凭据：

| 用途 | 数据库 | 用户 | 密码 | 权限 |
|---|---|---|---|---|
| Cluster 管理 | `postgres` | `postgres` | `postgres` | PostgreSQL 超级用户 |
| 应用与迁移 | `opskeeper` | `opskeeper` | `opskeeper` | 仅拥有 `opskeeper` 数据库 |

API、Worker、Scheduler 和 Migration 只使用 `opskeeper` 业务凭据。`opskeeper` 是 `NOSUPERUSER`、`NOCREATEDB`、`NOCREATEROLE`、`NOREPLICATION`、`NOBYPASSRLS` 角色。

### 5.2 `POSTGRES_*` 的关系

| 变量 | PostgreSQL 官方镜像语义 | 本地值 |
|---|---|---|
| `POSTGRES_USER` | 首次 `initdb` 创建的初始超级用户 | `postgres` |
| `POSTGRES_PASSWORD` | 初始超级用户密码 | `postgres` |
| `POSTGRES_DB` | 首次启动时确保存在并由初始用户拥有的连接数据库 | `postgres` |

`initdb` 会创建 `postgres`、`template0` 和 `template1`。`postgres` 数据库是管理连接使用的维护数据库，`postgres` 角色是数据库用户，两者是不同对象。

`POSTGRES_DB=postgres` 在当前配置下与默认结果一致，但保留显式声明，确保初始化脚本和健康检查始终连接明确的管理数据库。项目初始化脚本再读取 `OPSK_DB_USER`、`OPSK_DB_PASSWORD` 和 `OPSK_DB_NAME`，创建受限的 `opskeeper` 角色及其拥有的业务数据库。

Compose 健康检查会验证管理员为超级用户、业务角色不是特权用户、业务数据库归业务角色所有。旧数据卷不符合该模型时会显示为 `unhealthy`，但健康检查不会修改或删除数据。

## 6. 数据库迁移

首次初始化或拉取到新迁移后执行：

```bash
make migrate
```

没有新迁移时，日常重启应用无需重复执行。长期运行的应用进程永不自动迁移数据库。

迁移文件位于 `backend/migrations/sql/`：

```text
NNNN_description.sql
NNNN_description.down.sql
```

迁移器通过 `go:embed` 将 SQL 编译进 `opskeeper-migrate`。执行 `up` 时，它获取固定数据库连接和 PostgreSQL session advisory lock，创建或复用 `schema_migrations`，按版本顺序跳过已执行项，并在独立事务中执行每条待处理迁移和版本记录。失败会回滚当前迁移并返回非零状态。

执行 `make migrate-down` 时，迁移器使用同一把锁，在一个事务中回滚最新版本并删除其版本记录。生产发布回滚应用时不得自动执行 `down`；自动化发布流程见[自动化发布](delivery.md)。

## 7. 检查与制品

提交前执行：

```bash
make quality
```

完整门禁包含格式检查、静态检查、单元测试、嵌入式前端测试和生产构建。单独构建二进制或镜像：

```bash
make build
make image
```

默认二进制：

```text
backend/bin/opskeeper-api
backend/bin/opskeeper-worker
backend/bin/opskeeper-scheduler
backend/bin/opskeeper-migrate
```

本地二进制名称可用 `make build BINARY_PREFIX=acme-ops` 覆盖；最终镜像内始终使用固定的 `opskeeper-*` 文件名。镜像名可通过 `make image IMAGE_REPOSITORY=registry.example.com/opskeeper IMAGE_TAG=<version>` 指定。

数据库集成测试必须显式提供可丢弃数据库：

```bash
OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@localhost:5432/opskeeper?sslmode=disable' \
  make backend-integration-test
```

测试会创建临时 Schema 并在结束后删除，不会清空默认 Schema。

## 8. 当前组织 API

以下路径假设默认 `OPSK_BASE_PATH=/opskeeper`，当前阶段尚未接入认证；使用根路径时去掉开头的 `/opskeeper`。

```text
GET   /opskeeper/api/v1/platform
GET   /opskeeper/api/v1/teams?page=1&page_size=20
POST  /opskeeper/api/v1/teams
GET   /opskeeper/api/v1/teams/{teamId}
PATCH /opskeeper/api/v1/teams/{teamId}
GET   /opskeeper/api/v1/teams/{teamId}/projects?page=1&page_size=20
POST  /opskeeper/api/v1/teams/{teamId}/projects
GET   /opskeeper/api/v1/projects/{projectId}
PATCH /opskeeper/api/v1/projects/{projectId}
```

创建请求使用 `name`、`code` 和可选 `labels`。更新请求支持 `name`、`labels` 和 `status`；`status` 可选 `active` 或 `disabled`。
