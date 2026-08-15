# 本地开发环境

本文档前半部分说明如何搭建和运行本地环境，后半部分说明配置边界、PostgreSQL 初始化和数据库迁移机制。

本仓库采用前后端同仓的 Monorepo 结构。`backend/go.mod` 定义独立 Go Module `opskeeper/backend`，后端包直接位于 `backend/<package>` 并使用 `opskeeper/backend/<package>` 导入；`frontend/package.json` 独立管理 Svelte 前端依赖。两个工程共享版本库和发布流程，但依赖、构建和包解析相互独立。

`opskeeper/backend` 是仓库内应用代码的本地导入命名空间，不作为可由外部项目通过 `go get` 获取的公共 Module 地址。未来如果提供 Go SDK，应为 SDK 单独建立带代码托管域名的 Module 路径和兼容性边界。

后端采用按业务特性组织的模块化单体。特性包拥有自身的业务规则和持久化实现，共享基础设施只在出现真实的跨特性需求时抽取。新增或重构 Go 代码前请阅读 [Go 编码与工程组织通用规范](../standards/go-coding-conventions.md)。

## 1. 快速开始

### 1.1 前置条件

- Go 1.26 或兼容版本
- Node.js 22 或更高版本
- npm 11 或兼容版本
- Docker 和 Docker Compose v2

本机既可以使用 `docker compose` 插件，也可以使用独立的 `docker-compose` 命令，Makefile 会自动选择可用实现。以下命令均在仓库根目录执行。

### 1.2 首次初始化

```bash
cp .env.example .env
cp deploy/compose/.env.example deploy/compose/.env
make deps
make dev-services-up
```

以上命令依次创建本地配置、安装依赖并启动 PostgreSQL 和 Redis。首次初始化可能需要等待 PostgreSQL 容器完成用户和数据库创建；容器健康后再执行：

```bash
make migrate
```

该命令会将尚未执行的数据库迁移应用到业务数据库。

### 1.3 日常启动

启动中间件并确保其正在运行：

```bash
make dev-services-up
```

然后分别在不同终端启动需要的应用进程：

```bash
make run-api
make run-worker
make run-scheduler
make run-frontend
```

本地默认访问地址：

| 服务 | 地址 |
|---|---|
| 前端 | `http://localhost:5173` |
| API 存活检查 | `http://localhost:8080/health/live` |
| API 就绪检查 | `http://localhost:8080/health/ready` |

### 1.4 中间件日志与停止

查看 PostgreSQL 和 Redis 日志：

```bash
make dev-services-logs
```

停止中间件容器：

```bash
make dev-services-down
```

该命令不会主动删除持久化数据卷。再次执行 `make dev-services-up` 时会继续使用已有数据。

## 2. 数据库变更操作

应用所有尚未执行的迁移：

```bash
make migrate
```

回滚最近一条迁移：

```bash
make migrate-down
```

API Server 不会自动修改数据库 Schema。首次初始化以及拉取包含新迁移的代码后，应在启动新版本 API 前显式执行 `make migrate`；没有新迁移时，日常重启应用不需要重复执行。自动化环境在每次部署中固定运行一次独立 Migration Job，再滚动发布应用，完整流程见[数据库与应用自动化发布](delivery.md)。

`make migrate-down` 主要用于本地开发和迁移测试。生产环境回滚应用时不得自动执行数据库 `down`。

## 3. 质量检查与测试

执行完整质量检查：

```bash
make quality
```

该命令依次检查格式、静态分析、单元测试和构建结果。

PostgreSQL 集成测试需要显式提供测试连接：

```bash
OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@localhost:5432/opskeeper?sslmode=disable' make backend-integration-test
```

集成测试会在目标数据库中创建临时 Schema，并在结束后删除，不会清空默认 Schema。

## 4. 本地配置

### 4.1 配置文件边界

| 文件 | 使用方 | 内容 |
|---|---|---|
| 根目录 `.env` | API、Worker、Scheduler 和迁移命令 | 应用配置、业务数据库连接串和 Redis 连接串 |
| `deploy/compose/.env` | Docker Compose | PostgreSQL/Redis 容器配置、PostgreSQL 管理员凭据和业务库初始化参数 |

应用进程只加载根目录 `.env`，不会加载 `deploy/compose/.env`，因此不会获得 `postgres` 管理员密码。

默认数据库连接信息：

| 用途 | 数据库 | 用户 | 密码 |
|---|---|---|---|
| PostgreSQL 管理 | `postgres` | `postgres` | `postgres` |
| OpsKeeper 业务连接 | `opskeeper` | `opskeeper` | `opskeeper` |

应用和迁移使用的默认业务连接串为：

```text
OPSK_DATABASE_URL=postgres://opskeeper:opskeeper@localhost:5432/opskeeper?sslmode=disable
```

### 4.2 配置规范

- 后端配置统一使用 `OPSK_` 前缀环境变量。
- 根目录 `.env` 仅包含应用配置和业务连接凭据，不能提交到版本库。
- `deploy/compose/.env` 包含基础设施初始化凭据和 PostgreSQL 管理员密码，不能提交到版本库。
- `.env.example` 只能包含无敏感性的开发默认值。
- 生产环境必须通过 Secret 管理系统注入数据库、Redis 和后续外部资源凭据。

## 5. PostgreSQL 初始化与权限机制

### 5.1 权限分离原则

数据库集群管理员与业务数据库所有者必须使用不同角色：

```text
postgres 管理员角色
├── SUPERUSER
├── CREATEDB
├── CREATEROLE
└── 管理 postgres 维护数据库和整个 Cluster

opskeeper 业务角色
├── LOGIN
├── NOSUPERUSER
├── NOCREATEDB
├── NOCREATEROLE
├── NOREPLICATION
├── NOBYPASSRLS
└── 仅作为 opskeeper 数据库所有者
```

API、Worker、Scheduler 和迁移命令都只获得 `opskeeper` 凭据，不获得超级用户凭据。

### 5.2 官方镜像初始化参数

PostgreSQL 官方镜像在空数据目录首次启动时，先使用 `POSTGRES_*` 调用 `initdb` 初始化数据库集群并确保初始连接数据库存在。

| 参数 | 官方镜像语义 | 未指定时的行为 | 本项目开发值 |
|---|---|---|---|
| `POSTGRES_USER` | `initdb` 创建的初始数据库超级用户，也是初始数据库的所有者 | `postgres` | `postgres` |
| `POSTGRES_PASSWORD` | 为 `POSTGRES_USER` 设置的密码 | 无可用默认密码，通常必须显式提供 | `postgres` |
| `POSTGRES_DB` | entrypoint 要确保存在的初始连接数据库；新建时归 `POSTGRES_USER` 所有 | 与 `POSTGRES_USER` 相同 | `postgres` |

`initdb` 本身会创建 `postgres`、`template0` 和 `template1` 三个数据库。`postgres` 数据库是供管理员和工具连接使用的维护数据库，`postgres` 角色是数据库用户；两者只是默认同名，不是同一个对象。

由于本项目使用 `POSTGRES_USER=postgres`，`POSTGRES_DB` 的默认值本来就是 `postgres`，因此显式设置 `POSTGRES_DB=postgres` 不会改变创建结果。Compose 仍保留该配置，用于完整声明管理员角色、管理员密码和管理连接数据库，并让初始化脚本与健康检查始终通过 `$POSTGRES_DB` 连接明确的管理数据库。

### 5.3 业务角色和数据库初始化

官方镜像完成 Cluster 初始化后，会按文件名顺序执行 `/docker-entrypoint-initdb.d/` 中的脚本。本项目的 `001-create-opskeeper.sh` 使用以下变量创建业务角色和数据库：

| 参数 | 用途 | 本项目开发值 |
|---|---|---|
| `OPSK_DB_USER` | 非超级用户的业务数据库所有者 | `opskeeper` |
| `OPSK_DB_PASSWORD` | 业务角色密码 | `opskeeper` |
| `OPSK_DB_NAME` | 由业务角色拥有的应用数据库 | `opskeeper` |

脚本将业务角色设置为 `NOSUPERUSER`、`NOCREATEDB`、`NOCREATEROLE`、`NOREPLICATION` 和 `NOBYPASSRLS`，并撤销 `PUBLIC` 对业务数据库的默认权限。

旧配置将 `POSTGRES_USER` 和 `POSTGRES_DB` 都设置为 `opskeeper`。由于 `POSTGRES_USER` 定义的是 `initdb` 创建的初始超级用户，这会让看似业务用户的 `opskeeper` 实际拥有整个 Cluster 的超级用户权限。当前配置将 `POSTGRES_*` 专用于基础设施管理，将 `OPSK_DB_*` 专用于受限业务账号初始化。

### 5.4 首次初始化约束与健康检查

`POSTGRES_*` 和 `/docker-entrypoint-initdb.d` 脚本只在 `/var/lib/postgresql/data` 为空时参与首次初始化。容器重启或使用已有数据卷时，更改这些环境变量不会自动重建数据库、重建用户或修改密码。

切换权限模型时必须使用新的空卷，或者由数据库管理员在现有实例中受控地迁移角色、密码和数据库所有权。删除数据卷会永久删除其中的数据，不应在包含有效数据时执行。

Compose 健康检查不仅调用 `pg_isready`，还会读取 PostgreSQL 系统目录，验证管理员的超级用户属性、业务角色的非特权属性和业务数据库所有权。旧数据卷如果仍是单用户模型，PostgreSQL 容器会显示为 `unhealthy`；健康检查不会自动修改或删除旧数据。

## 6. 数据库迁移机制

迁移入口是 `backend/cmd/migrate`，实现位于 `backend/migrations`。迁移命令使用 `OPSK_DATABASE_URL` 中的 `opskeeper` 业务数据库所有者执行，不使用 `postgres` 超级用户。

迁移文件位于 `backend/migrations/sql/`，命名规则为：

```text
NNNN_description.sql
NNNN_description.down.sql
```

每个前滚 SQL 必须有同版本的回滚 SQL。迁移器通过 `go:embed` 将这些文件编译进迁移命令，启动时解析版本号、按版本升序排序，并拒绝重复版本、空文件或缺少回滚文件的迁移。

执行 `make migrate` 时：

1. 从连接池获取固定连接，并在该 PostgreSQL 会话上持有迁移 advisory lock。
2. 创建或复用 `schema_migrations(version, name, applied_at)` 版本表。
3. 按版本顺序检查每条迁移是否已经记录。
4. 对每条尚未执行的迁移开启独立 PostgreSQL 事务。
5. 在同一事务中执行前滚 SQL 并写入版本记录。
6. SQL 或版本记录写入失败时回滚整条迁移，不留下半完成 Schema。
7. 全部完成后释放 advisory lock；释放失败时销毁持锁连接。

执行 `make migrate-down` 时：

1. 读取 `schema_migrations` 中版本号最大的记录。
2. 校验该版本和名称仍存在于当前二进制的嵌入迁移中。
3. 在一个事务中执行对应的 `.down.sql` 并删除版本记录。
4. 一次只回滚一个版本；需要回滚多个版本时重复执行命令。

当前 `0001_scope_organization` 迁移创建三级 Scope 和 Platform、Team、Project 表、约束、触发器及默认平台根节点；回滚会删除这些组织对象，但保留 `schema_migrations` 表和共享的 `pgcrypto` 扩展。`up` 和 `down` 使用同一把数据库级锁；发布系统仍应只创建一个逻辑迁移任务，不由 API 副本或每个 Pod 的 Init Container 执行。

自动化发布顺序、Kubernetes Job 约束、Expand/Contract 和生产回滚原则见[数据库与应用自动化发布](delivery.md)。

## 7. 组织 API 参考

当前提供以下无认证开发接口；认证和 Scope 授权将在后续阶段接入：

```text
GET   /api/v1/platform
GET   /api/v1/teams?page=1&page_size=20
POST  /api/v1/teams
GET   /api/v1/teams/{teamId}
PATCH /api/v1/teams/{teamId}
GET   /api/v1/teams/{teamId}/projects?page=1&page_size=20
POST  /api/v1/teams/{teamId}/projects
GET   /api/v1/projects/{projectId}
PATCH /api/v1/projects/{projectId}
```

创建请求使用 `name`、`code` 和可选 `labels`。更新请求支持 `name`、`labels` 和 `status`；`status` 取值为 `active` 或 `disabled`。

## 附录：不使用 Compose 启动中间件

本机没有 Docker Compose 时，可以直接启动等价容器：

```bash
docker run --name opskeeper-postgres --detach \
  --publish 127.0.0.1:5432:5432 \
  --env POSTGRES_DB=postgres \
  --env POSTGRES_USER=postgres \
  --env POSTGRES_PASSWORD=postgres \
  --env OPSK_DB_NAME=opskeeper \
  --env OPSK_DB_USER=opskeeper \
  --env OPSK_DB_PASSWORD=opskeeper \
  --volume opskeeper-postgres-data:/var/lib/postgresql/data \
  --volume "$PWD/deploy/compose/postgres/init:/docker-entrypoint-initdb.d:ro" \
  postgres:16-alpine

docker run --name opskeeper-redis --detach \
  --publish 127.0.0.1:6379:6379 \
  --volume opskeeper-redis-data:/data \
  redis:7-alpine redis-server --appendonly yes
```

后续可以停止或重新启动这两个容器：

```bash
docker stop opskeeper-postgres opskeeper-redis
docker start opskeeper-postgres opskeeper-redis
```
