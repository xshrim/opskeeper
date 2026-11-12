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

`make start` 自动创建缺失的 `.env`，同步 Compose 环境文件，先单独构建 API、Worker、Scheduler 镜像，再启动这三个服务和 PostgreSQL。Compose 文件只引用镜像，不包含 `build`。API 启动时执行迁移；首次访问登录页会弹出管理员初始化窗口。默认缓存和 Bundle 存储也共用该 PostgreSQL。

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
| `make infra-up`                 | 只启动 PostgreSQL，供本机二进制服务连接                                   |
| `make infra-logs`               | 持续查看 PostgreSQL 日志                                                 |
| `make infra-down`               | 只停止 PostgreSQL，保留数据卷                                            |
| `make infra-clean`             | 删除 Compose 容器、网络和全部数据卷                                      |
| `make obs-up`                   | 启动 Jaeger、OTel Collector、Prometheus、Loki 和 Grafana                 |
| `make obs-logs`                 | 持续查看本地监控组件日志                                                  |
| `make obs-down`                 | 停止本地监控组件                                                          |
| `make api-stop`                 | 按 `.env` 中的 `OPSK_LISTEN_PORT` 端口强制停止已监听的 API 进程       |
| `make api-run`                  | 停止旧 API 后临时运行 API                                                |
| `make worker-run`               | 临时运行 Worker                                                          |
| `make scheduler-run`            | 临时运行 Scheduler                                                       |
| `make frontend-run`             | 临时运行 Vite 前端                                                       |
| `make front-api-run`            | 停止旧 API、构建并嵌入前端，然后通过一个 API 进程提供完整应用            |
| `make run`                      | 构建并嵌入前端，然后通过 `go run` 启动 API；迁移由 API 自动执行           |
| `make sample-seed`              | 向本地开发库写入标记过的 sample 数据                                     |
| `make sample-clean`             | 仅清理 sample 工具写入的标记数据                                         |
| `make test`                     | 运行前后端单元测试                                                       |
| `make backend-integration-test` | 运行数据库集成测试                                                       |
| `make llm-provider-test`        | 使用`.env` 中的外部 Provider 配置，经 ADK Runner 验证非流式和 SSE 调用 |
| `make quality`                  | 执行完整本地质量门禁                                                     |
| `make build`                    | 构建生产二进制制品                                                       |
| `make image`                    | 单独构建 API、Worker 和 Scheduler 镜像                                   |

应用临时运行、二进制构建和最终镜像打包的标准入口只能定义在根目录 `Makefile` 中，不得再增加 `scripts/*.sh` 等包装脚本。底层 Go、npm 和 Docker 命令属于 Make recipe 的实现细节；日常开发和流水线统一调用 `make *-run`、`make build` 和 `make image`。

## 3. 开发方式

前后端同仓但依赖独立：`backend/go.mod` 定义 Go Module `opskeeper/backend`，`frontend/package.json` 管理 Svelte 前端依赖。

本项目支持“容器 PostgreSQL + 本机二进制服务”和“容器 PostgreSQL + 容器服务”两种方式。`make infra-up` 只启动数据库，不会连带启动 Compose 中的应用容器。

### 容器 PostgreSQL + 本机二进制服务

首次使用时创建本地环境文件，然后启动数据库并构建服务二进制：

```bash
test -f .env || cp .env.example .env
make infra-up
make build
```

在三个独立终端中分别加载根目录 `.env` 并运行 API、Worker、Scheduler：

```bash
set -a; source .env; set +a; backend/bin/opskeeper-api
```

```bash
set -a; source .env; set +a; backend/bin/opskeeper-worker
```

```bash
set -a; source .env; set +a; backend/bin/opskeeper-scheduler
```

也可以在开发时用 Make 目标运行 Go 源码，而不预先生成持久二进制：

```text
终端 1: make front-api-run
终端 2: make worker-run
终端 3: make scheduler-run
```

`make front-api-run` 会构建并嵌入当前前端，再通过一个 API 进程提供完整应用；前端源代码变化后需要重新运行。若要使用 Vite 热更新，则改用 `make api-run` 和 `make frontend-run`。这些命令都不会启动 PostgreSQL。

API 每次启动时会在监听 HTTP 端口前检查并自动应用待执行迁移。无需也不存在单独的 `make migrate` 操作命令。首次访问登录页时初始化管理员密码，无需 `make admin-create`。

### 容器 PostgreSQL + 容器服务

一键构建并启动完整环境：

```bash
make start
```

`make start` 会在缺少 `.env` 时从 `.env.example` 创建，然后同步 Compose 环境文件、执行独立的 `make image` 镜像构建，最后运行 `docker compose up`。若要分开执行构建和启动：

```bash
test -f .env || cp .env.example .env # 仅首次创建；保留已有配置
make compose-env
make image
docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --wait --wait-timeout 180
```

Compose 不会构建镜像。发布新镜像时先执行 `make image`，再强制重建应用服务容器以切换到新镜像。需要查看或停止完整容器环境时：

```bash
docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --force-recreate api worker scheduler
docker compose -f deploy/compose/docker-compose.yml --env-file .env logs -f
docker compose -f deploy/compose/docker-compose.yml --env-file .env down
```

`docker compose down` 保留数据库数据卷；`make infra-clean` 会删除 Compose 数据卷，只能用于确认可丢弃的本地环境。

两种方式中迁移和管理员初始化流程相同：API 启动自动迁移，管理员在首次登录页初始化，不需要额外迁移或管理员创建命令。监控组件可在任一模式下额外用 `make obs-up` 启动。

### 写入和清理 sample 数据

在 API 已成功启动、数据库 schema 已建立后，可写入开发演示数据：

```bash
make sample-seed
```

`sample-seed` 从根目录 `.env` 读取配置，要求设置 `OPSK_TEST_USER_PASWORD` 作为样例用户密码；Host 和 LLM 样例资源根据对应 `OPSK_TEST_*` 配置选择性创建。命令只允许连接 `development` 环境下本机 `opskeeper` 数据库，不会用于远程或生产数据库。

清理 sample 工具创建的数据：

```bash
make sample-clean
```

`sample-clean` 同样只允许连接本机开发数据库，并按 sample 标记删除团队、项目、样例用户、Docker/Host 资源和样例 Provider，不会清空整个数据库，也不需要用户密码配置。

Vite 默认通过 `http://localhost:5173/<prefix>/` 提供页面和热更新，并把相同前缀下的 `/api` 和 `/health` 请求代理到本地 API。只执行 `make frontend-run` 时仍可打开页面，但 API 数据不可用。`make front-api-run` 通过 `go run` 临时运行 API，不生成持久二进制；`make build` 才生成包含嵌入式前端的服务二进制。

## 4. 应用配置

根目录 `.env` 是 API、Worker、Scheduler、Compose 和 Vite 的配置源；`make start` 将其复制到 `deploy/compose/.env`。API 运行账号需要具备迁移权限，因为迁移在 API 启动时执行。

常用环境变量按用途分组如下；根目录 `.env.example` 提供按用途组织的完整示例配置。

#### HTTP 与身份安全

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `OPSK_BASE_PATH` | `/opskeeper` | 页面、静态资源、健康检查和业务 API 的路径前缀 |
| `OPSK_ENVIRONMENT` | `development` | 运行环境 |
| `OPSK_LOG_FORMAT` | `raw` | 应用日志格式：`json`、`text` 或 `raw`；详见[后端日志规范](../standards/backend-logging.md) |
| `OPSK_LISTEN_PORT` | `8080` | API 监听端口；Compose 将相同端口发布到宿主机 |
| `OPSK_TRUSTED_PROXIES` | 空 | 可提供客户端转发头的代理 IP/CIDR，逗号分隔 |
| `OPSK_ALLOWED_ORIGINS` | 空 | 额外允许的精确 HTTP(S) Origin，默认仅同源 |
| `OPSK_COOKIE_SECURE` | 开发环境 `false`，生产环境 `true` | 会话 Cookie 是否仅允许 HTTPS；生产环境必须开启 |
| `OPSK_CREDENTIAL_KEY` | 开发环境使用内置密钥 | 资源凭据加密密钥；生产环境必须设置为 32 字节原值或 Base64 值 |

#### 数据库、缓存与对象存储

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `OPSK_DB_HOST` | `localhost:5432` | PostgreSQL 主机与端口 |
| `OPSK_DB_NAME` / `OPSK_DB_USER` / `OPSK_DB_PASSWORD` | `opskeeper` | 业务数据库及受限应用账号 |
| `OPSK_DB_SSLMODE` | `disable` | PostgreSQL SSL 模式 |
| `OPSK_CACHE_BACKEND` | `postgres` | 缓存后端：`memory`、`postgres` 或 `redis` |
| `OPSK_REDIS_URL` | 空 | `OPSK_CACHE_BACKEND=redis` 时必需；也用于连接 Redis 的集成测试 |
| `OPSK_STORAGE_BACKEND` | `postgres` | 通用对象存储后端：`local`、`postgres` 或 `s3` |
| `OPSK_REPOSITORY_LOCAL_ROOT` | `/var/lib/opskeeper/repositories` | Repository 本地存储目录 |
| `OPSK_STORAGE_S3_PROVIDER` | `minio` | S3-compatible 服务提供方标识 |
| `OPSK_STORAGE_S3_ENDPOINT` / `OPSK_STORAGE_S3_BUCKET` | 空 | S3 后端端点和 Bucket；使用 S3 时必需 |
| `OPSK_STORAGE_S3_PREFIX` | `repositories` | S3 对象前缀 |
| `OPSK_STORAGE_S3_ACCESS_KEY` / `OPSK_STORAGE_S3_SECRET_KEY` | 空 | S3 凭据 |
| `OPSK_STORAGE_S3_USE_SSL` | `false` | S3 连接是否使用 TLS |
| `OPSK_REPOSITORY_MAX_BUNDLE_BYTES` | `536870912` | Repository Bundle 上传上限，范围 1 MiB–4 GiB |

#### 执行、诊断与巡检

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `OPSK_SHUTDOWN_TIMEOUT` | `10s` | 优雅退出期限 |
| `OPSK_DEPENDENCY_TIMEOUT` | `2s` | 单次依赖探测超时；服务在代码中持续重试依赖连接 |
| `OPSK_CONNECTOR_TIMEOUT` | `10s` | 单次 Connector 执行超时 |
| `OPSK_CONNECTOR_MAX_CONCURRENCY` | `8` | 单进程 Connector 并发数，范围 1–128 |
| `OPSK_CONNECTOR_MAX_RESPONSE_BYTES` | `4194304` | Connector 响应上限，范围 1 KiB–64 MiB |
| `OPSK_DIAGNOSIS_COLLABORATION` | `2` | 诊断协同度，范围 1–5 |
| `OPSK_DIAGNOSIS_MAX_SUBAGENTS` | `24` | 单轮 Subagent 任务数上限，范围 1–128 |
| `OPSK_DIAGNOSIS_SUBAGENT_CONCURRENCY` | `4` | 单轮 Subagent 并发数，范围 1–16 |
| `OPSK_INSPECTION_SCHEDULE_INTERVAL` | `15s` | Scheduler 策略轮询间隔 |
| `OPSK_INSPECTION_WORKER_POLL_INTERVAL` | `2s` | Worker 空闲轮询间隔 |
| `OPSK_INSPECTION_LEASE_DURATION` | `45s` | 巡检任务租约时长 |

#### 监控

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `OPSK_OTEL_ENDPOINT` | 空 | OTLP/HTTP Collector 地址；空值禁用导出 |
| `OPSK_LOG_HEALTH_IGNORE` | `true` | 忽略健康探针访问日志 |

#### 测试配置

`OPSK_TEST_*` 仅用于集成测试、`make llm-provider-test` 和 `make sample-seed`；真实凭据只放在 Git 忽略的 `.env` 中。

| 配置组 | 变量 |
| --- | --- |
| 数据库与中间件集成测试 | `OPSK_TEST_DATABASE_URL`、`OPSK_TEST_KAFKA_BROKERS` |
| Host 测试与样例资源 | `OPSK_TEST_HOST_ADDRESS`、`OPSK_TEST_HOST_PORT`、`OPSK_TEST_HOST_USERNAME`、`OPSK_TEST_HOST_PASSWORD`、`OPSK_TEST_HOST_AUTH_METHOD`、`OPSK_TEST_DOCKER_HOST_URL` |
| 外部 LLM 测试与样例资源 | `OPSK_TEST_LLM_BASE_URL`、`OPSK_TEST_LLM_MODEL`、`OPSK_TEST_LLM_SCENARIO`、`OPSK_TEST_LLM_API_KEY` |
| 样例用户 | `OPSK_TEST_USER_PASWORD` |

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

三个应用服务名称固定为 `opskeeper-api`、`opskeeper-worker` 和 `opskeeper-scheduler`。

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
OPSK_LOG_FORMAT=json make api-run
OPSK_LOG_FORMAT=text make api-run
OPSK_LOG_FORMAT=raw make api-run
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

应用中登记 Provider 时，连接地址、连接密文和模型目录保存在独立的 Provider 对象；模型条目只保留上游名称、参数和能力。业务调用方通过 Engine 选择 Provider/Model，未显式选择时按 Scope 场景标签解析并固定最终模型。API Token 必须加密保存在 `providers` 表中，不能出现在资源配置或审计响应中。Provider 不划分单模态或多模态子类；`tool_calling`、`vision`、音频、结构化输出和长上下文等属于可叠加能力字段。

## 5. PostgreSQL、缓存与 Bundle 存储

启动、查看和停止中间件：

```bash
make infra-up
make infra-logs
make infra-down
```

默认 Compose 使用 `pgvector/pgvector:pg16`。PostgreSQL 初始化脚本由数据库管理员在首次创建业务数据库时安装 `vector` 扩展；业务迁移不创建实例级扩展。删除本地数据卷后重新执行 `make infra-up` 即会重新运行初始化脚本。授权缓存写入 `cache_entries`（`UNLOGGED`，可安全重建），Repository Bundle 以原始 Git Bundle `bytea` 写入 `repository_bundles`，不使用 FlatBuffers；数据库、缓存和存储共用同一个 PostgreSQL 实例。`memory` 适合单进程开发或测试。平台内部中间件选择遵循 [PostgreSQL First 规约](../standards/postgresql-first.md)。

本地 PostgreSQL 容器名固定为 `opskeeper-postgres`，可使用 `docker logs opskeeper-postgres` 或 `docker exec -it opskeeper-postgres psql -U postgres` 排查。

`obs` profile 的容器名固定为 `opskeeper-jaeger`、`opskeeper-otel-collector`、`opskeeper-prometheus`、`opskeeper-loki` 和 `opskeeper-grafana`，可直接用对应名称执行 `docker logs` 或 `docker exec`。

`make infra-down` 不删除持久化数据卷，再次启动会复用已有数据。PostgreSQL 初始化变量和初始化脚本仅在数据目录为空时生效；修改环境变量不会更新已有数据卷中的用户、密码或数据库所有权。

### 5.1 本地链路追踪

Jaeger、OpenTelemetry Collector、Prometheus、Loki 和 Grafana 属于独立的 `obs` profile，`make infra-up` 不会启动它们。容器应用使用 Compose 网络地址向 Collector 导出；本机二进制使用 `.env` 中的 `http://127.0.0.1:4318`：

```bash
make obs-up
# 在 .env 中设置：OPSK_OTEL_ENDPOINT=http://127.0.0.1:4318
make api-run
```

启动对应的 API、Worker、Scheduler 后，Trace 会经 Collector 转发至 Jaeger，Metrics 会写入 Prometheus 并在 Grafana 预置 Dashboard 中展示。当前应用日志写到标准输出，还没有接入 Loki 的采集输入；可用 `docker compose logs` 查看容器日志。完成后使用 `make obs-down` 停止监控服务。

需要彻底清理当前 Compose 项目的本地环境时，使用 `make infra-clean`。该命令会删除 PostgreSQL、可观测容器、网络和数据卷，数据不可恢复，仅适用于明确确认的本地环境。

### 5.2 管理员与业务角色

默认开发凭据：

| 用途         | 数据库        | 用户          | 密码          | 权限                       |
| ------------ | ------------- | ------------- | ------------- | -------------------------- |
| Cluster 管理 | `postgres`  | `postgres`  | `postgres`  | PostgreSQL 超级用户        |
| 应用与迁移   | `opskeeper` | `opskeeper` | `opskeeper` | 仅拥有`opskeeper` 数据库 |

API、Worker 和 Scheduler 只使用 `opskeeper` 业务凭据。API 启动时执行迁移，因此该角色还需要对业务 schema 执行迁移所需的权限。`opskeeper` 是 `NOSUPERUSER`、`NOCREATEDB`、`NOCREATEROLE`、`NOREPLICATION`、`NOBYPASSRLS` 角色。

### 5.3 `POSTGRES_*` 的关系

| 变量                  | PostgreSQL 官方镜像语义                        | 本地值       |
| --------------------- | ---------------------------------------------- | ------------ |
| `POSTGRES_USER`     | 首次`initdb` 创建的初始超级用户              | `postgres` |
| `POSTGRES_PASSWORD` | 初始超级用户密码                               | `postgres` |
| `POSTGRES_DB`       | 首次启动时确保存在并由初始用户拥有的连接数据库 | `postgres` |

`initdb` 会创建 `postgres`、`template0` 和 `template1`。`postgres` 数据库是管理连接使用的维护数据库，`postgres` 角色是数据库用户，两者是不同对象。

Compose 显式传入 `POSTGRES_*` 管理账号设置，并传入 `OPSK_DB_NAME`、`OPSK_DB_USER` 和 `OPSK_DB_PASSWORD` 供初始化脚本创建受限业务账号及其数据库。应用服务只接收自身所需的业务数据库连接变量。

Compose 健康检查会验证管理员为超级用户、业务角色不是特权用户、业务数据库归业务角色所有。旧数据卷不符合该模型时会显示为 `unhealthy`，但健康检查不会修改或删除数据。

## 6. 数据库迁移

API 在每次启动并监听 HTTP 端口前自动应用待执行迁移。没有待处理迁移时，迁移器会在 advisory lock 保护下校验历史后直接返回；Worker 和 Scheduler 会持续重试数据库连接。

当前迁移基线位于 `backend/migrations/sql/0001_initial.sql`。历史增量 SQL 保存在 `backend/migrations/sql/archive/`，仅供追溯，不会被迁移器加载。

```text
0001_initial.sql
archive/                 # 历史迁移，仅供追溯
```

当前基线是从完整迁移链应用后的 PostgreSQL schema 和内置种子数据导出的纯 SQL。更新数据库结构时，先在隔离数据库验证变更，再重建并审查该文件；不要把归档 SQL 加入运行时加载路径。

迁移器通过 `go:embed` 只嵌入顶层 SQL，不会读取 `archive/` 子目录。执行 `up` 时，它获取固定数据库连接和 PostgreSQL session advisory lock，创建或复用 `schema_migrations`，校验基线名称和前滚 SQL SHA-256，并在独立事务中执行基线和版本记录。基线为单一、不可逆初始化操作，不提供 `down` 命令；已有历史增量迁移数据库需要按环境执行受控重建或专项数据迁移，不能依赖 `archive/` 自动升级。

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
```

本地二进制和最终镜像内文件名固定为 `opskeeper-*`。`make build` 默认从 Git 生成版本、提交和 UTC 构建时间，也可由流水线显式传入 `VERSION`、`COMMIT`、`BUILD_TIME`。镜像名通过 `make image IMAGE_REPOSITORY=registry.example.com/opskeeper IMAGE_TAG=<version>` 指定；Go 依赖默认使用 `goproxy.cn`、`proxy.golang.org` 的故障转移代理链，镜像 Builder 还使用阿里云 Alpine 镜像和 npmmirror，均可通过同名 Make 变量覆盖。自定义代理请使用 `make GOPROXY=...` 传入，避免机器上的全局 Go 配置覆盖项目设置。

数据库集成测试必须显式提供可丢弃数据库：

```bash
OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@localhost:5432/opskeeper?sslmode=disable' \
  make backend-integration-test
```

测试会创建临时 Schema 并在结束后删除，不会清空默认 Schema。

## 8. 身份与会话

### 8.1 首次创建管理员

首次访问一个尚未有管理员的平台时，任意页面都会落到登录页，并显示初始化管理员窗口。窗口只接受 `admin` 的密码，密码至少 8 位；成功后它在同一数据库事务内创建账号及平台级 `PlatformAdmin` 绑定。管理员已存在时接口返回冲突，不能覆盖现有账号。

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
