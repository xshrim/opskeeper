# 本地开发

本文是本地初始化、运行、迁移和检查的操作手册。所有命令均在仓库根目录执行。

## 1. 快速开始

前置软件：Go 1.26.5 或更高版本、Node.js 22、npm 11、Docker、Helm 3.18，以及 Docker Compose v2 或独立的 `docker-compose`。ADK Go v2.2.0 的模块要求为 Go 1.26.5。

前端 npm 依赖安装统一使用 Makefile 变量 `NPM_REGISTRY` 指定的镜像，默认值为国内镜像 `https://registry.npmmirror.com`。执行 `make deps` 或 `make image` 时会自动把该变量传给 npm；需要切换镜像时直接覆盖变量：

```bash
make deps NPM_REGISTRY=https://registry.npmmirror.com
make image NPM_REGISTRY=https://registry.npmmirror.com
```

如需单独安装或更新包，也必须使用同一个变量值：`npm install <package> --registry="${NPM_REGISTRY}"`。不要在 `package.json` 或业务代码中写入临时 registry 配置。

首次初始化：

```bash
make start
```

`make start` 自动创建缺失的 `.env` 和 `deploy/compose/.env`，安装前后端依赖，启动并等待 PostgreSQL/Redis，执行迁移，在用户表为空时创建默认管理员，最后调用 `make run-front-api`。首次创建管理员时会打印随机密码；后续执行会保留已有管理员和数据。最终只运行一个 Go 进程，由 API 同时提供前端页面、静态资源和业务接口。按 `Ctrl+C` 停止 API 后，中间件仍会运行，可通过 `make infra-down` 停止。

默认地址：

| 用途         | 地址                                             |
| ------------ | ------------------------------------------------ |
| 前端         | `http://localhost:8080/opskeeper/`             |
| API 存活检查 | `http://localhost:8080/opskeeper/health/live`  |
| API 就绪检查 | `http://localhost:8080/opskeeper/health/ready` |

## 2. 常用命令

| 命令                              | 用途                                                                     |
| --------------------------------- | ------------------------------------------------------------------------ |
| `make help`                     | 查看全部 Make 入口                                                       |
| `make start`                    | 一键准备并启动完整本地开发环境                                           |
| `make deps`                     | 安装 Go 和前端依赖                                                       |
| `make infra-up`                 | 启动 PostgreSQL 和 Redis                                                 |
| `make infra-logs`               | 持续查看中间件日志                                                       |
| `make infra-down`               | 停止中间件，保留数据卷                                                   |
| `make infra-clean`             | 删除中间件容器、网络和全部数据卷                                         |
| `make migrate`                  | 应用待执行迁移                                                           |
| `make migrate-down`             | 回滚最近一条迁移，仅用于开发和测试                                       |
| `make admin-create`             | 通过受控流程创建首个管理员，只允许成功一次                               |
| `make run-api`                  | 临时运行 API                                                             |
| `make run-worker`               | 临时运行 Worker                                                          |
| `make run-scheduler`            | 临时运行 Scheduler                                                       |
| `make run-frontend`             | 临时运行 Vite 前端                                                       |
| `make run-front-api`            | 构建并嵌入前端，然后通过一个 API 进程提供完整应用                        |
| `make test`                     | 运行前后端单元测试                                                       |
| `make backend-integration-test` | 运行数据库集成测试                                                       |
| `make llm-provider-test`        | 使用`.env` 中的外部 Provider 配置，经 ADK Runner 验证非流式和 SSE 调用 |
| `make quality`                  | 执行完整本地质量门禁                                                     |
| `make build`                    | 构建生产二进制制品                                                       |
| `make image`                    | 构建最终应用镜像                                                         |

应用临时运行、二进制构建和最终镜像打包的标准入口只能定义在根目录 `Makefile` 中，不得再增加 `scripts/*.sh` 等包装脚本。底层 Go、npm 和 Docker 命令属于 Make recipe 的实现细节；日常开发和流水线统一调用 `make run-*`、`make build` 和 `make image`。

## 3. 开发方式

前后端同仓但依赖独立：`backend/go.mod` 定义 Go Module `opskeeper/backend`，`frontend/package.json` 管理 Svelte 前端依赖。

运行前后端合并后的完整应用：

```bash
make infra-up
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
make infra-up
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

| 变量                                     | 默认值                                     | 作用                                                                     |
| ---------------------------------------- | ------------------------------------------ | ------------------------------------------------------------------------ |
| `OPSK_BASE_PATH`                       | `/opskeeper`                             | 页面、静态资源、健康检查和业务 API 的路径前缀                            |
| `OPSK_ENVIRONMENT`                     | `development`                            | 标识运行环境                                                             |
| `OPSK_LOG_FORMAT`                      | `raw`                                    | Go 应用日志格式，可选 `json`、`text` 或 `raw`；详见[后端日志规范](../standards/backend-logging.md) |
| `OPSK_LOG_HEALTH_IGNORE`               | `true`                                   | 是否忽略`/health/live`与`/health/ready`的 API 访问日志；设为`false`可恢复记录 |
| `OPSK_COOKIE_SECURE`                   | 开发环境`false`，生产环境 `true`       | 会话 Cookie 是否只允许 HTTPS，生产环境不能关闭                           |
| `OPSK_SESSION_ACCESS_TTL`              | `15m`                                    | 短期访问会话有效期                                                       |
| `OPSK_SESSION_REFRESH_TTL`             | `168h`                                   | 刷新会话有效期，必须长于访问会话                                         |
| `OPSK_HTTP_ADDRESS`                    | `:8080`                                  | API 监听地址                                                             |
| `OPSK_TRUSTED_PROXIES`                 | 空                                         | 允许提供客户端转发头的反向代理 IP 或 CIDR，逗号分隔                      |
| `OPSK_ALLOWED_ORIGINS`                 | 空                                         | 额外允许的精确 HTTP(S) Origin，默认仅同源                                |
| `OPSK_HTTP_MAX_BODY_BYTES`             | `2097152`                                | 全局请求正文上限，范围 1 KiB-64 MiB                                      |
| `OPSK_HTTP_RATE_LIMIT_PER_MINUTE`      | `600`                                    | 每客户端 IP 每分钟请求速率，范围 1-100000                                |
| `OPSK_CREDENTIAL_KEY`                  | 开发环境使用内置本地密钥；生产环境必须设置 | 资源凭据密文加密密钥，支持 32 字节原值或 Base64 编码值                   |
| `OPSK_DATABASE_URL`                    | 本地`opskeeper` 连接串                   | 业务数据库连接                                                           |
| `OPSK_REDIS_URL`                       | `redis://localhost:6379/0`               | Redis 连接                                                               |
| `OTEL_EXPORTER_OTLP_ENDPOINT`          | 空                                         | OTLP/HTTP Collector 地址；空值禁用导出                                   |
| `OPSK_SHUTDOWN_TIMEOUT`                | `10s`                                    | 优雅退出期限                                                             |
| `OPSK_DEPENDENCY_TIMEOUT`              | `2s`                                     | 健康检查依赖超时                                                         |
| `OPSK_CONNECTOR_TIMEOUT`               | `10s`                                    | 单次 Connector 执行总超时，必须为正数                                    |
| `OPSK_CONNECTOR_MAX_CONCURRENCY`       | `8`                                      | 单个进程允许同时执行的 Connector 数，范围 1-128                          |
| `OPSK_CONNECTOR_MAX_RESPONSE_BYTES`    | `4194304`                                | 单次 Connector 响应上限，范围 1 KiB-64 MiB                               |
| `OPSK_INSPECTION_SCHEDULE_INTERVAL`    | `15s`                                    | Scheduler 检查到期策略的轮询间隔                                         |
| `OPSK_INSPECTION_WORKER_POLL_INTERVAL` | `2s`                                     | Worker 在无任务时的轮询间隔                                              |
| `OPSK_INSPECTION_LEASE_DURATION`       | `45s`                                    | 巡检任务租约与心跳续约期限                                               |
| `OPSK_OPERATION_SUBMITTER_ENABLED`     | `false`                                  | 是否启用受控操作提交器；启用前必须配置集群权限                           |
| `OPSK_OPERATION_RUNNER_IMAGE`          | `opskeeper:local`                        | 受控操作提交器使用的 Runner 镜像                                         |
| `OPSK_BOOTSTRAP_USERNAME`              | `admin`                                  | 首次创建管理员的用户名                                                       |
| `OPSK_BOOTSTRAP_EMAIL`                 | 空                                       | 可选邮箱；非空值只能绑定一个用户名                                           |
| `OPSK_BOOTSTRAP_PHONE`                 | 空                                       | 可选手机号；非空值只能绑定一个用户名                                         |
| `OPSK_BOOTSTRAP_DISPLAY_NAME`          | `admin`                                  | 首次创建管理员的显示名称                                                     |
| `OPSK_BOOTSTRAP_PASSWORD_FILE`         | 空                                       | 首次创建管理员的密码文件路径；未指定时自动生成随机密码                       |
| `OPSK_TEST_LLM_BASE_URL`               | 空                                         | 仅供`make llm-provider-test` 使用的 OpenAI-compatible `/v1` 地址     |
| `OPSK_TEST_LLM_MODEL`                  | 空                                         | 仅供外部 Provider 验证使用的模型名                                       |
| `OPSK_TEST_LLM_API_KEY`                | 空                                         | 仅保存在 Git 忽略的`.env` 中的测试 Token；不得写入样例、日志或验收文档 |

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

| `OPSK_BASE_PATH` | 页面            | API                     | 存活检查                   |
| ------------------ | --------------- | ----------------------- | -------------------------- |
| `/opskeeper`     | `/opskeeper/` | `/opskeeper/api/v1/*` | `/opskeeper/health/live` |
| `/`              | `/`           | `/api/v1/*`           | `/health/live`           |

四个应用服务名称固定为 `opskeeper-api`、`opskeeper-worker`、`opskeeper-scheduler` 和 `opskeeper-migrate`。

### 4.2 可信代理与客户端 IP

默认 `OPSK_TRUSTED_PROXIES` 为空，API 不信任任何 `X-Forwarded-For` 或 `X-Real-IP`。直接访问 API 的客户端即使自行提供这些请求头，也不能改变日志和后续审计使用的来源 IP。

通过 Ingress、Nginx 或其他反向代理部署时，只配置会直接连接 API 的代理地址或网段：

```dotenv
OPSK_TRUSTED_PROXIES=10.42.0.0/16,192.0.2.10,2001:db8:42::/64
```

API 只在 TCP 直连来源属于该列表时解析转发头。对于 `X-Forwarded-For`，从右向左跳过可信代理，选择第一个非可信地址作为客户端 IP；头部包含非法地址时整条链失效并保留直连来源。可信范围只能包含受控代理节点，不能为方便而配置全部内网或客户端网段。

### 4.3 日志格式

所有 Go 应用使用相同的 `OPSK_LOG_FORMAT`，字段和消息约定见[后端日志规范](../standards/backend-logging.md)：

```bash
OPSK_LOG_FORMAT=json make run-api
OPSK_LOG_FORMAT=text make run-api
OPSK_LOG_FORMAT=raw make run-api
```

- `raw`：默认值，按固定字段顺序输出纯值文本，适合本地终端和简单采集器。
- `text`：键值文本，适合需要保留字段名的终端工具。
- `json`：单行 JSON，适合容器平台和日志采集系统解析。
- 其他值会在应用启动时被拒绝。

格式切换不改变日志字段。API、Worker、Scheduler 和 Migration 分别写入固定的 `service` 字段；API 请求日志按规范在消息中写入 `reqid`、经过可信代理规则解析的 `clientip` 及请求结果。

默认 `OPSK_LOG_HEALTH_IGNORE=true`，API 不记录 `GET /health/live` 和 `GET /health/ready` 访问日志，避免 Kubernetes 探针和前端状态刷新淹没业务日志。接口仍会正常执行；需要排查健康检查访问时设为 `false` 后重启 API。

### 4.4 外部 LLM Provider 验证

在本地 `.env` 配置测试 Provider：

```dotenv
OPSK_TEST_LLM_BASE_URL=https://provider.example/v1
OPSK_TEST_LLM_MODEL=provider/model-name
OPSK_TEST_LLM_API_KEY=<local-secret>
```

执行：

```bash
make llm-provider-test
```

该入口通过项目内 OpenAI-compatible Adapter、ADK `llmagent` 和 ADK Runner 分别执行非流式与 SSE 请求，并验证响应非空及 Token usage。它会访问外部服务并可能产生费用，因此不并入默认 `make test` 或 `make quality`。测试只输出模式、字符数和 Token 数，不输出 Token 或模型正文。

应用中登记 `LLMProvider` 时，Base URL、Provider 类型和模型能力保存在资源配置；API Token 必须通过资源凭据加密保存。Model 是 Provider 内的版本化配置，不登记成独立资源。

## 5. PostgreSQL 与 Redis

启动、查看和停止中间件：

```bash
make infra-up
make infra-logs
make infra-down
```

`make infra-down` 不删除持久化数据卷，再次启动会复用已有数据。PostgreSQL 初始化变量和初始化脚本仅在数据目录为空时生效；修改环境变量不会更新已有数据卷中的用户、密码或数据库所有权。

需要彻底清理当前 Compose 项目的本地中间件环境时，使用 `make infra-clean`。该命令会删除 PostgreSQL、Redis 及其他 Compose 中间件容器、网络和数据卷，数据不可恢复，仅适用于明确确认的本地环境。

### 5.1 管理员与业务角色

默认开发凭据：

| 用途         | 数据库        | 用户          | 密码          | 权限                       |
| ------------ | ------------- | ------------- | ------------- | -------------------------- |
| Cluster 管理 | `postgres`  | `postgres`  | `postgres`  | PostgreSQL 超级用户        |
| 应用与迁移   | `opskeeper` | `opskeeper` | `opskeeper` | 仅拥有`opskeeper` 数据库 |

API、Worker、Scheduler 和 Migration 只使用 `opskeeper` 业务凭据。`opskeeper` 是 `NOSUPERUSER`、`NOCREATEDB`、`NOCREATEROLE`、`NOREPLICATION`、`NOBYPASSRLS` 角色。

### 5.2 `POSTGRES_*` 的关系

| 变量                  | PostgreSQL 官方镜像语义                        | 本地值       |
| --------------------- | ---------------------------------------------- | ------------ |
| `POSTGRES_USER`     | 首次`initdb` 创建的初始超级用户              | `postgres` |
| `POSTGRES_PASSWORD` | 初始超级用户密码                               | `postgres` |
| `POSTGRES_DB`       | 首次启动时确保存在并由初始用户拥有的连接数据库 | `postgres` |

`initdb` 会创建 `postgres`、`template0` 和 `template1`。`postgres` 数据库是管理连接使用的维护数据库，`postgres` 角色是数据库用户，两者是不同对象。

`POSTGRES_DB=postgres` 在当前配置下与默认结果一致，但保留显式声明，确保初始化脚本和健康检查始终连接明确的管理数据库。项目初始化脚本再读取 `OPSK_DB_USER`、`OPSK_DB_PASSWORD` 和 `OPSK_DB_NAME`，创建受限的 `opskeeper` 角色及其拥有的业务数据库。

Compose 健康检查会验证管理员为超级用户、业务角色不是特权用户、业务数据库归业务角色所有。旧数据卷不符合该模型时会显示为 `unhealthy`，但健康检查不会修改或删除数据。

## 6. 数据库迁移

首次初始化或拉取到新迁移后执行：

```bash
make migrate
```

没有新迁移时，日常重启应用无需重复执行。长期运行的应用进程永不自动迁移数据库。

当前迁移基线位于 `backend/migrations/sql/0001_initial.sql`，对应回滚脚本为 `0001_initial.down.sql`。历史版本 SQL 保存在 `backend/migrations/sql/archive/`，仅用于追溯和生成新的基线，不会被迁移器加载。

```text
0001_initial.sql
0001_initial.down.sql
archive/                 # 历史迁移，仅供追溯
```

当前基线文件作为已审核的迁移输入直接维护；如需重新整合归档 SQL，必须在变更中重新生成并审查顶层 `0001_initial.sql` 和 `0001_initial.down.sql`，不能依赖运行时脚本。

迁移器通过 `go:embed` 将顶层 SQL 编译进 `opskeeper-migrate`，不会读取 `archive/` 子目录。执行 `up` 时，它获取固定数据库连接和 PostgreSQL session advisory lock，创建或复用 `schema_migrations`，校验已执行版本的名称和前滚 SQL SHA-256，按版本顺序跳过已执行项，并在独立事务中执行每条待处理迁移和版本记录。失败会回滚当前迁移并返回非零状态；数据库存在当前二进制未知版本或历史迁移名称时立即失败。当前基线只有 `0001_initial`，历史 `0001`-`0019` 数据库需要按环境执行重建或专门的数据迁移，不能依赖 `archive/` 自动升级。

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
backend/bin/opskeeper-admin
```

本地二进制和最终镜像内文件名固定为 `opskeeper-*`。`make build` 默认从 Git 生成版本、提交和 UTC 构建时间，也可由流水线显式传入 `VERSION`、`COMMIT`、`BUILD_TIME`。镜像名通过 `make image IMAGE_REPOSITORY=registry.example.com/opskeeper IMAGE_TAG=<version>` 指定；镜像 Builder 默认使用 `goproxy.cn`、阿里云 Alpine 镜像和 npmmirror，均可通过同名 Make 变量覆盖。

数据库集成测试必须显式提供可丢弃数据库：

```bash
OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@localhost:5432/opskeeper?sslmode=disable' \
  make backend-integration-test
```

测试会创建临时 Schema 并在结束后删除，不会清空默认 Schema。

## 8. 身份与会话

### 8.1 首次创建管理员

首次启动一个空数据库后，必须通过 Makefile 提供的受控流程创建管理员：

```bash
make admin-create
```

命令默认使用用户名和显示名称 `admin`，邮箱与手机号保持未绑定，并生成一次性随机密码；随机密码只在创建账号后打印一次。也可以通过命令行参数覆盖：

```bash
make admin-create ADMIN_CREATE_ARGS='--username admin --email admin@example.com --phone +8613800138000 --display-name "Platform Admin" --password-file /run/secrets/opskeeper-admin-password'
```

本地环境也可以直接传入明文密码：

```bash
make admin-create ADMIN_CREATE_ARGS="--username admin --password 'TemporaryPassword123!'"
```

支持 `--username`、`--email`、`--phone`、`--display-name`、`--password` 和 `--password-file`。参数优先于同名环境变量；`--password` 优先于密码文件。明文 `--password` 会暴露在 shell 历史和进程参数中，只建议用于本地临时初始化；生产环境应使用权限受限的密码文件。用户名与密码是唯一必填项，密码至少需要 12 个字符；display name 默认使用 username，email 和 phone 默认空且各自只能绑定一个 username。登录时可输入 username、email 或 phone，服务端自动识别。

管理员 bootstrap 只允许在没有任何用户的数据库中成功一次，不能通过公开 HTTP 接口抢占首个账号。T04 起，`make admin-create` 在创建用户后同时建立平台级 `PlatformAdmin` 绑定，使首个管理员可以访问组织 API 和后续授权能力。

### 8.2 会话使用方式

登录成功后 API 通过 HttpOnly、SameSite=Lax Cookie 设置访问和刷新 Token；JSON 响应只返回用户资料。非浏览器客户端可读取 `Set-Cookie`，也可将访问 Token 作为 `Authorization: Bearer <token>` 发送。刷新会轮换访问和刷新 Token，旧刷新 Token 立即失效；注销、全部会话失效、用户 disabled 或 locked 均会阻止后续认证。

## 9. 当前 API

以下路径假设默认 `OPSK_BASE_PATH=/opskeeper`。T03 起，组织业务 API 要求通过身份认证；T04 起，组织读写还需要匹配当前用户的角色和 Scope。登录、刷新、注销和健康检查保持公开。使用根路径时去掉开头的 `/opskeeper`。

```text
POST  /opskeeper/api/v1/auth/login
POST  /opskeeper/api/v1/auth/refresh
POST  /opskeeper/api/v1/auth/logout
GET   /opskeeper/api/v1/auth/me
POST  /opskeeper/api/v1/auth/logout-all
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

创建请求使用 `name`、`code` 和可选 `labels`。更新请求支持 `name`、`labels` 和 `status`；`status` 可选 `active` 或 `disabled`。T04 使用九个内置角色和向下 Scope 继承；组织查询、分页、计数和按 ID 访问均在服务端应用 Scope 过滤。T04 暂不提供角色绑定管理 API，管理能力留给 T05。
