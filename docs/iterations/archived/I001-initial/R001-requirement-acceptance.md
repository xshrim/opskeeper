# I001-R001 需求验收报告：平台首个迭代交付

**迭代：** I001-initial
**需求：** R001 平台首个迭代交付
**验收结论：** 通过
**任务范围：** T01-T15

## 1. 需求级验收结论

本需求覆盖 OpsKeeper 首个迭代的工程基线、组织模型、身份认证、权限管理、资源目录、管理控制台、Kubernetes 导入、Connector、LLM/Skill、AI 诊断、内置诊断、自动巡检、MCP/操作审批及生产化加固。T01-T15 均已完成验收，用户已确认本需求通过并封板。

## 2. 任务验收汇总

| 任务 | 名称 | 结果 |
|---|---|---|
| T01 | 工程初始化与质量基线 | 通过 |
| T02 | Scope 与组织模型 | 通过 |
| T03 | 本地身份与会话 | 通过 |
| T04 | 三级 RBAC 与数据隔离 | 通过 |
| T05 | 身份权限管理与安全审计 | 通过 |
| T06 | 资源目录与关系模型 | 通过 |
| T07 | 管理控制台基础功能 | 通过 |
| T08 | Kubernetes 发现与导入 | 通过 |
| T09 | Connector 与监控平台 | 通过 |
| T10 | LLM、Skill 与 Runner | 通过 |
| T11 | AI 诊断与证据链 | 通过 |
| T12 | 内置诊断 Skill | 通过 |
| T13 | 自动巡检与通知 | 通过 |
| T14 | MCP、沙箱与操作审批 | 通过 |
| T15 | 生产化与端到端验收 | 通过 |

## 3. 各任务验收报告

以下小节保留原任务验收记录的完整内容，并统一归入本需求验收报告。每个任务的验收结论、验证步骤、证据和边界均以对应小节为准。

---

### T01 工程初始化与质量基线验收记录

#### 1. 验收结论

- 验收日期：2026-08-15
- T01 实现提交：`931222b43b7900603646cac97368fd1be8bd5227`
- 验证基线提交：`165cf0ba120df497594b920680b8c5c01db257ac`
- 验收结论：**通过**
- 阻塞问题：无

T01 建立的 Go API、Worker、Scheduler、Svelte 前端、本地 PostgreSQL/Redis 依赖、配置规范、健康检查、HTTP 中间件、测试及统一质量门禁均符合验收标准。组织模型、身份认证、RBAC、资源管理和实际业务页面不属于 T01 验收范围。

#### 2. 验收范围与环境

本次验收以 T01 实现提交为功能范围，并在当前保留该工程基础的主分支基线上执行运行和质量验证。

| 项目         | 实际环境                          |
| ------------ | --------------------------------- |
| Go           | `go1.26.5-X:nodwarf5 linux/amd64` |
| Node.js      | `v26.5.1`                         |
| npm          | `12.0.2`                          |
| PostgreSQL   | `postgres:16-alpine`              |
| Redis        | `redis:7-alpine`                  |
| 前端验收地址 | `http://127.0.0.1:55173/`         |
| API 健康检查 | `/health/live`、`/health/ready`   |

T01 实现提交共创建 42 个文件，包含 5,333 行新增内容，覆盖后端三个进程入口、前端工程、本地中间件编排、配置、健康检查、测试、Makefile 和工程文档。

#### 3. 逐项验收结果

##### 步骤 1：工程结构与进程入口检查

检查 T01 实现提交及当前工程目录，确认存在以下入口和基础设施：

```text
backend/cmd/api/main.go
backend/cmd/worker/main.go
backend/cmd/scheduler/main.go
backend/internal/config/
backend/internal/health/
backend/internal/httpapi/
frontend/src/
deploy/compose/docker-compose.yml
Makefile
.env.example
.editorconfig
.gitignore
```

实际结果：

- API、Worker、Scheduler 为三个独立 Go 进程入口，共享后端内部包。
- 前端采用 Svelte 5、TypeScript、Vite。
- PostgreSQL、Redis 本地依赖编排和健康检查已纳入仓库。
- 根目录 Makefile 提供开发、测试、检查和构建的统一入口。

结论：**通过**。

##### 步骤 2：配置机制检查

检查 `backend/internal/config/config.go`、配置测试及环境变量样例。

实际结果：

- 后端配置统一使用 `OPSK_` 环境变量前缀。
- 数据库、Redis、HTTP 地址、超时等运行参数由环境变量加载。
- 未配置时使用适合本地开发的默认值。
- 非法时长等错误配置会在启动阶段被拒绝。
- `TestLoadDefaults` 和 `TestLoadRejectsInvalidDuration` 覆盖默认配置和非法配置场景。

结论：**通过**。

##### 步骤 3：API 基础能力与中间件检查

检查 API 入口、路由和 HTTP 中间件实现。

实际结果：

- API 使用 JSON 结构化日志。
- 已接入 Request ID、Real IP、panic recovery 和请求日志中间件。
- 错误响应统一包含 `code`、`message`、`request_id`。
- 未匹配路由和不允许的 HTTP 方法分别映射为 404 和 405。
- API 监听 `SIGINT`、`SIGTERM`，支持有序关闭 HTTP Server、PostgreSQL 连接池和 Redis 客户端。

结论：**通过**。

##### 步骤 4：PostgreSQL 与 Redis 依赖检查

检查 `deploy/compose/docker-compose.yml`、数据库初始化脚本、健康检查脚本和应用连接配置。

实际结果：

- 本地环境定义 PostgreSQL 16 和 Redis 7 服务。
- 两个服务均配置健康检查。
- PostgreSQL 管理员与业务用户分离，业务进程通过 `opskeeper` 用户连接 `opskeeper` 数据库。
- API 创建 PostgreSQL 连接池和 Redis 客户端，并将实际连通性纳入就绪检查。

本机没有安装 Docker Compose 插件，执行 `make infra-up` 时按预期返回：

```text
Docker Compose is not installed
```

验收使用与编排定义等价的 PostgreSQL 16 和 Redis 7 容器验证实际连接；Compose YAML 由格式工具成功解析，初始化及健康检查 Shell 脚本通过语法检查。

结论：**通过，存在非阻塞环境说明**。

##### 步骤 5：存活与就绪检查

启动 API 并访问健康检查端点，实际结果：

| 请求                | HTTP 状态 | 结构化结果                        |
| ------------------- | --------: | --------------------------------- |
| `GET /health/live`  |       200 | `status=alive`                    |
| `GET /health/ready` |       200 | PostgreSQL 为 `up`，Redis 为 `up` |

测试同时覆盖依赖失败路径：`TestReadinessReportsFailedDependency` 和 `TestReadinessFailure` 验证依赖异常会反映在就绪结果中，而 `TestLiveness` 验证存活端点不依赖外部服务状态。

结论：**通过**。

##### 步骤 6：Worker 与 Scheduler 启动

分别启动 Worker 和 Scheduler，两个进程均进入运行状态并输出结构化日志。

Worker 实际输出：

```json
{
  "time": "2026-08-15T14:47:25.778499924+08:00",
  "level": "INFO",
  "msg": "worker started",
  "version": "dev"
}
```

Scheduler 实际输出：

```json
{
  "time": "2026-08-15T14:47:25.778440975+08:00",
  "level": "INFO",
  "msg": "scheduler started",
  "version": "dev"
}
```

两个进程均能接收中断信号并有序退出。

结论：**通过**。

##### 步骤 7：Svelte 开发服务启动

启动前端开发服务，实际输出：

```text
VITE v7.3.6 ready in 281 ms
Local: http://127.0.0.1:55173/
```

随后发起 HTTP 请求，实际结果：

```text
HTTP_STATUS:200
SIZE:401
CONTENT_TYPE:text/html
```

返回页面包含应用挂载节点和 Vite 模块入口：

```html
<div id="app"></div>
<script type="module" src="/src/main.ts"></script>
```

前端状态页每 10 秒轮询 `/health/ready`，展示 API、PostgreSQL、Redis 状态，并实现请求异常处理和组件销毁时的请求取消。

结论：**通过**。

##### 步骤 8：完整质量门禁

执行：

```bash
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make quality
```

实际结果：

- Prettier 格式检查通过。
- `go vet ./...` 通过。
- Svelte Check 为 `0 errors and 0 warnings`。
- Shell 脚本语法检查通过。
- Go 单元测试全部通过。
- Vitest：`1 passed`。
- Go API、Worker、Scheduler 全量构建通过。
- Vite 生产构建通过，共转换 `110 modules`。

结论：**通过**。

##### 步骤 9：凭据与仓库忽略规则检查

检查 `.env.example`、`.gitignore`、`.editorconfig` 和版本库内容。

实际结果：

- 仓库提交的是配置样例，不包含本地 `.env`。
- `.env`、`node_modules`、`dist` 和后端构建产物均被忽略。
- 配置使用环境变量，不包含与验收机器绑定的绝对运行路径。
- `.editorconfig` 统一 LF、文件末尾换行、尾随空白及 Go/Make Tab 规则。
- 未发现提交到仓库的真实凭据。

结论：**通过**。

##### 步骤 10：验收环境清理

完成运行验证后停止前端、Worker 和 Scheduler 进程；中间件验收容器已清理。未保留验收专用后台进程或临时容器。

结论：**通过**。

#### 4. 验收标准映射

| T01 验收标准                                         | 验证依据                                             | 结果 |
| ---------------------------------------------------- | ---------------------------------------------------- | ---- |
| Go API、Worker、Scheduler 和 Svelte 开发服务均能启动 | 健康端点、两个后台进程启动日志、Vite HTTP 200        | 通过 |
| API 存活及就绪检查返回结构化结果                     | `/health/live` 与 `/health/ready` 实际响应及单元测试 | 通过 |
| PostgreSQL 和 Redis 连通性能够进入就绪检查           | 就绪结果中 PostgreSQL、Redis 均为 `up`               | 通过 |
| 后端测试、前端测试、格式化和静态检查全部通过         | `make quality` 完整执行结果                          | 通过 |
| 仓库中不存在真实凭据或与机器绑定的绝对配置           | 配置样例、忽略规则和仓库内容检查                     | 通过 |

#### 5. 非阻塞环境说明

1. 本机未安装 Docker Compose，因此没有直接执行 `docker compose up`；等价容器运行、配置解析、脚本语法、数据库角色权限及应用连通性均已分别验证。
2. 沙箱中的默认 Go 缓存目录只读，质量门禁将 `GOMODCACHE` 和 `GOCACHE` 指向 `/tmp` 下的验收缓存目录。该调整不改变构建和测试内容。
3. 前端首次在受限沙箱中绑定本地端口时返回 `EPERM`，允许本地监听后使用相同启动命令成功运行并返回 HTTP 200。

上述事项均为验收环境限制，不构成产品缺陷，不影响 T01 验收结论。

---

### T02 Scope 与组织模型验收记录

#### 1. 验收结论

- 验收日期：2026-08-15
- 验收分支：`refactor/go-feature-conventions`
- 基线提交：`3f626ad77d785100a78f7973b7902ae44d848617`
- 验收结论：**通过**
- 阻塞问题：无

本次按当前阶段 2 工作树重新执行 T02 全量验收。根目录 `README.md` 和 `docs/standards/version-control.md` 的最新修改已保留在阶段 2 分支工作树中，并随当前文档与代码一起通过质量检查。

数据库初始化、管理员与业务角色分离、迁移幂等与内容校验、三级 Scope/组织模型、事务化业务操作、HTTP API、前端嵌入、可信代理处理、生产二进制和最终镜像均符合当前验收要求。身份认证、三级 RBAC 和组织管理页面不属于 T02 范围。

#### 2. 验收环境

| 项目 | 实际环境 |
|---|---|
| Go | `go1.26.5-X:nodwarf5 linux/amd64` |
| Node.js | `v26.5.1` |
| npm | `12.0.2` |
| Docker Compose | `v5.4.0` |
| PostgreSQL | `postgres:16-alpine` |
| Redis | `redis:7-alpine` |
| PostgreSQL 验收地址 | `127.0.0.1:55432` |
| Redis 验收地址 | `127.0.0.1:56379` |
| 开发运行 API | `127.0.0.1:58080/opskeeper` |
| 生产二进制 API | `127.0.0.1:58081/opskeeper` |

为避免影响默认开发服务和已有数据，本次使用 `opskeeper-t02-postgres`、`opskeeper-t02-redis` 两个无持久化卷的临时容器，并使用隔离端口。Docker Compose v5.4.0 可用，`deploy/compose/docker-compose.yml` 已通过 `docker compose config --quiet` 解析；临时容器使用与 Compose 相同的镜像、环境变量和 PostgreSQL 初始化目录。

#### 3. 逐项验收结果

##### 步骤 1：分支、文档与工作树确认

执行：

```bash
git branch --show-current
git rev-parse HEAD
git status --short --branch
git diff -- README.md docs/standards/version-control.md
```

实际结果：

- 当前分支为 `refactor/go-feature-conventions`。
- 当前阶段 2 内容提交为 `3f626ad77d785100a78f7973b7902ae44d848617`。
- 用户更新的根 README 和版本控制规范均存在于当前阶段 2 工作树，没有被覆盖或回退。
- `main` 未在本次验收中合并或修改。

结论：**通过**。

##### 步骤 2：启动隔离中间件

以 Compose 等价参数启动临时 PostgreSQL 和 Redis，随后检查服务状态：

```text
/var/run/postgresql:5432 - accepting connections
PONG
opskeeper-t02-postgres|Up|127.0.0.1:55432->5432/tcp
opskeeper-t02-redis|Up|127.0.0.1:56379->6379/tcp
```

PostgreSQL 初始化参数为：

```text
POSTGRES_DB=postgres
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
OPSK_DB_NAME=opskeeper
OPSK_DB_USER=opskeeper
OPSK_DB_PASSWORD=opskeeper
```

结论：**通过**。验收环境与默认开发环境隔离，PostgreSQL 和 Redis 均正常就绪。

##### 步骤 3：PostgreSQL 管理员与业务角色分离

查询角色属性，字段依次为 `rolsuper`、`rolcreatedb`、`rolcreaterole`、`rolreplication`、`rolbypassrls`：

```text
opskeeper|false|false|false|false|false
postgres|true|true|true|true|true
```

查询数据库所有权：

```text
opskeeper|opskeeper
postgres|postgres
```

以 `opskeeper` 连接业务数据库，在事务中创建测试表并回滚：

```text
BEGIN
CREATE TABLE
opskeeper|opskeeper
ROLLBACK
```

以同一业务用户尝试创建集群角色，PostgreSQL 返回：

```text
ERROR: permission denied to create role
DETAIL: Only roles with the CREATEROLE attribute may create roles.
```

结论：**通过**。`postgres` 保持集群超级用户；`opskeeper` 拥有业务数据库并可执行库内 DDL，但不具备集群级管理权限。

##### 步骤 4：迁移首次执行、幂等和状态模型

使用业务连接连续执行两次：

```bash
cd backend
OPSK_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:55432/opskeeper?sslmode=disable' \
  go run ./cmd/migrate
```

两次均返回：

```text
migration command completed direction=up
```

`schema_migrations` 最终只有两个版本，每项均记录 64 个十六进制字符的 SHA-256 校验和：

```text
1|scope_organization|544e8c9a9f10ce860997bc15c02b1295e9a568e3053f46e17a9d45a862165c80
2|scope_status|a0e416908ab818ba8dffb4d3e4c72b0023ad479e096844a807e75ab934620941
```

预期表全部存在：

```text
platforms
projects
schema_migrations
scopes
teams
```

默认平台根节点为：

```text
default|active|platform|parent_is_null=true
```

`platforms`、`teams`、`projects` 均不存在 `status` 列，组织状态只持久化在 `scopes.status`。

结论：**通过**。空库迁移成功，重复前滚无副作用，迁移校验和完整，默认平台和状态单一数据源符合设计。

##### 步骤 5：迁移与组织真实数据库集成测试

执行：

```bash
cd backend
OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:55432/opskeeper?sslmode=disable' \
  go test -count=1 -v -tags=integration ./migrations ./organization
```

实际通过的数据库测试：

```text
TestApplyWaitsForAdvisoryLock
TestConcurrentApplyIsIdempotent
TestApplyRejectsChecksumMismatch
TestLoadOrdersEmbeddedMigrations
TestOrganizationLifecycle
TestConcurrentTeamCodeIsUnique
TestDatabaseRejectsIllegalScopeHierarchy
TestMigrationRollback
```

组织参数规范化、编码校验、分页默认值、项目来源默认值和空更新校验等非数据库测试也全部通过。迁移包耗时 `1.196s`，组织包耗时 `1.435s`。

结论：**通过**。迁移锁、并发幂等、内容篡改阻断、回滚、数据库层级约束和编码唯一性均有真实 PostgreSQL 证据。

##### 步骤 6：构建并嵌入前端后启动 API

执行与开发文档一致的 Make 入口，并仅覆盖本次验收连接参数：

```bash
OPSK_HTTP_ADDRESS=127.0.0.1:58080 \
OPSK_BASE_PATH=/opskeeper \
OPSK_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:55432/opskeeper?sslmode=disable' \
OPSK_REDIS_URL='redis://127.0.0.1:56379/0' \
OPSK_LOG_FORMAT=json \
make APP_ENV_FILE=/dev/null run-front-api
```

实际过程：

1. Vite 构建成功，共转换 `110 modules`。
2. `frontend/dist` 被复制到 `backend/webui/dist`。
3. API 使用 `-tags=embed_webui` 启动。
4. 日志显示 `service=opskeeper-api`、`address=127.0.0.1:58080`、`base_path=/opskeeper`。

结论：**通过**。`run-front-api` 确实先复制前端制品，再运行包含嵌入资源的 Go API。

##### 步骤 7：存活、就绪、默认平台和嵌入前端

| 请求 | HTTP 状态 | 关键结果 |
|---|---:|---|
| `GET /opskeeper/health/live` | 200 | `status=alive`，包含版本字段 |
| `GET /opskeeper/health/ready` | 200 | PostgreSQL 与 Redis 均为 `up` |
| `GET /opskeeper/api/v1/platform` | 200 | `code=default`、Platform Scope 无父节点 |
| `GET /opskeeper/` | 200 | 返回嵌入的 `index.html` |
| `GET /opskeeper/assets/index-B0zh8Ocx.js` | 200 | 返回嵌入 JS，长期不可变缓存头正确 |

页面 HTML 中的运行时基路径为：

```html
<base href="/opskeeper/" data-opsk-runtime-base />
```

平台 ID 为 `8172ddf7-720c-4777-a35f-901d6df13992`，平台 Scope ID 为 `fcdade33-c6fb-4ef9-bdf1-bda42be020cc`。

结论：**通过**。单个 Go 进程同时提供健康检查、业务 API、页面和静态资源。

##### 步骤 8：创建团队和项目并核对三级 Scope

创建团队返回 `201 Created`：

```text
team.id=922c1399-4388-4160-89aa-98f634fda2bc
team.code=t02-acceptance-team-20260815
team.scope.id=792f5bc9-59a0-47a2-9755-f1b15d85fec6
team.scope.type=team
team.scope.parent_id=fcdade33-c6fb-4ef9-bdf1-bda42be020cc
team.status=active
```

创建项目返回 `201 Created`：

```text
project.id=8681f0c1-cb98-4805-ab3c-ba7ba2f70ca0
project.code=t02-acceptance-project-20260815
project.team_id=922c1399-4388-4160-89aa-98f634fda2bc
project.scope.id=3ad5064a-6694-4c17-8bbe-c9bbab66d030
project.scope.type=project
project.scope.parent_id=792f5bc9-59a0-47a2-9755-f1b15d85fec6
project.source=manual
project.status=active
```

结论：**通过**。Scope 路径为 Platform → Team → Project，两个父 Scope ID 均准确匹配，项目默认来源为 `manual`。

##### 步骤 9：详情、列表、分页和更新

| 请求 | HTTP 状态 | 关键结果 |
|---|---:|---|
| `GET /api/v1/teams/{teamId}` | 200 | 返回已创建团队及 Scope |
| `GET /api/v1/projects/{projectId}` | 200 | 返回已创建项目及 Scope |
| `GET /api/v1/teams?page=1&page_size=1` | 200 | `page=1`、`page_size=1`、`total=1` |
| `GET /api/v1/teams/{teamId}/projects?page=1&page_size=1` | 200 | `page=1`、`page_size=1`、`total=1` |
| `PATCH /api/v1/teams/{teamId}` | 200 | 名称和标签更新，Scope 关系不变 |
| `PATCH /api/v1/projects/{projectId}` | 200 | 名称和标签更新，Scope 关系不变 |

团队和项目的 `updated_at` 均发生变化，编码和父子关系保持不变。

结论：**通过**。

##### 步骤 10：输入校验、唯一性和错误映射

| 场景 | HTTP 状态 | 错误码 |
|---|---:|---|
| 非法团队 UUID `not-a-uuid` | 400 | `invalid_request` |
| 创建团队包含未知 JSON 字段 | 400 | `invalid_json` |
| 查询不存在的合法 UUID | 404 | `not_found` |
| 重复团队编码 | 409 | `conflict` |

所有错误响应均包含结构化 `error.code`、`error.message` 和 `request_id`。集成测试另行覆盖非法 Scope 父子层级、并发重复编码和空更新。

结论：**通过**。

##### 步骤 11：停用父级并保留历史数据

将团队状态更新为 `disabled` 后返回 `200`：

```text
team.scope.status=disabled
team.status=disabled
```

随后在该团队下创建项目，返回：

```text
HTTP 409
error.code=parent_inactive
error.message=Parent organization is inactive
```

再次查询停用前创建的项目仍返回 `200`。数据库终态：

```text
team|t02-acceptance-team-20260815|team|disabled|fcdade33-c6fb-4ef9-bdf1-bda42be020cc
project|t02-acceptance-project-20260815|project|active|792f5bc9-59a0-47a2-9755-f1b15d85fec6
```

结论：**通过**。停用上级组织阻止新增下级，但不删除或隐藏已有项目；API 状态与 `scopes.status` 一致。

##### 步骤 12：可信代理与请求日志

API 未配置 `OPSK_TRUSTED_PROXIES`。从本机直接请求时伪造：

```text
X-Forwarded-For: 203.0.113.77
X-Real-IP: 198.51.100.8
```

对应 JSON 请求日志仍记录：

```text
path=/opskeeper/health/live client_ip=127.0.0.1 status=200
```

执行竞态测试：

```bash
go test -race -count=1 ./config ./httpapi
```

结果：两个包均为 `ok`。

结论：**通过**。未受信直连客户端不能通过转发头伪造源 IP，请求日志包含正确的 `client_ip`。

##### 步骤 13：完整质量门禁

执行：

```bash
GOMODCACHE=/tmp/opskeeper-gomodcache \
GOCACHE=/tmp/opskeeper-gocache \
make quality \
  VERSION=t02-acceptance \
  COMMIT=3f626ad77d785100a78f7973b7902ae44d848617 \
  BUILD_TIME=2026-08-15T13:41:00Z
```

实际结果：

- Prettier 格式检查通过。
- `go vet ./...` 通过。
- Svelte Check：`0 errors and 0 warnings`。
- PostgreSQL 初始化和健康检查 Shell 脚本通过 `sh -n`。
- Go 单元测试全部通过。
- Vitest：`1` 个测试文件、`3` 个测试全部通过。
- 嵌入前端的 Go 测试通过。
- Vite 生产构建成功，共转换 `110 modules`。
- `opskeeper-api`、`opskeeper-worker`、`opskeeper-scheduler`、`opskeeper-migrate` 四个生产二进制构建成功。
- `go mod tidy -diff` 无输出，依赖文件无需整理。
- `git diff --check` 无输出，没有空白错误。
- Docker Compose 配置解析通过。

结论：**通过**。

##### 步骤 14：生产二进制和最终镜像

执行最终镜像构建：

```bash
make image \
  IMAGE_TAG=t02-acceptance \
  VERSION=t02-acceptance \
  COMMIT=3f626ad77d785100a78f7973b7902ae44d848617 \
  BUILD_TIME=2026-08-15T13:41:00Z
```

构建成功，镜像信息：

```text
image=opskeeper:t02-acceptance
id=sha256:d0f2f4e6acf76053786371461d26507ea93b5e59f8cc4a3d3a473957943a919d
version=t02-acceptance
revision=3f626ad77d785100a78f7973b7902ae44d848617
created=2026-08-15T13:41:00Z
entrypoint=["/app/opskeeper-api"]
```

Docker 构建使用：

```text
GOPROXY=https://goproxy.cn
ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
NPM_REGISTRY=https://registry.npmmirror.com
```

直接运行 `backend/bin/opskeeper-api` 后，健康响应为 `200`，并返回：

```text
service=opskeeper-api
version=t02-acceptance
commit=3f626ad77d785100a78f7973b7902ae44d848617
build_time=2026-08-15T13:41:00Z
```

同一生产二进制的 `/opskeeper/` 返回嵌入前端首页 `200`。

结论：**通过**。本地二进制和最终镜像使用同一 Makefile 构建逻辑，版本元数据一致，最终制品由单个 Go API 提供前后端服务。

##### 步骤 15：资源清理

验收结束后停止两个 API 进程，并删除本次创建的两个临时容器：

```text
opskeeper-t02-postgres
opskeeper-t02-redis
```

随后使用 `docker ps` 确认不再存在本次验收容器。没有删除默认 Compose 卷、开发数据库、仓库文件或用户已有容器。

临时容器内的验收数据会随容器删除且不可恢复；该数据仅用于本次 T02 验收。

结论：**通过**。

#### 4. 验收标准映射

| T02 验收标准 | 验收证据 | 结果 |
|---|---|---|
| 创建团队和项目并得到正确三层 Scope 路径 | 步骤 7、8 | 通过 |
| 非法父子层级和孤立组织被拒绝 | `TestDatabaseRejectsIllegalScopeHierarchy`、数据库触发器测试 | 通过 |
| 重复编码被拒绝 | 并发集成测试、HTTP `409 conflict` | 通过 |
| 停用上级后阻止新增下级并保留历史 | 步骤 11 | 通过 |
| 迁移在空库可重复执行 | 步骤 4 | 通过 |
| 迁移支持回滚、串行执行和内容校验 | 步骤 5 | 通过 |
| 组织模块测试通过 | 步骤 5、13 | 通过 |
| 管理员与业务数据库角色分离 | 步骤 3 | 通过 |
| 开发时可构建并嵌入前端运行单个 API | 步骤 6、7 | 通过 |
| 生产二进制和镜像可追踪 | 步骤 14 | 通过 |

#### 5. 非阻塞说明

1. 本次运行验收使用无持久化卷的隔离临时容器，而不是启动默认 Compose 项目，目的是不触碰已有开发数据；Compose 文件本身已经由 Docker Compose v5.4.0 实际解析通过。
2. Docker 当前使用 legacy builder，构建时提示未来将移除。镜像仍构建成功；后续可在开发环境安装 buildx，但这不影响当前 Dockerfile 和制品正确性。
3. 沙箱默认 Go 缓存目录只读，验收统一使用 `/tmp/opskeeper-gocache` 和 `/tmp/opskeeper-gomodcache`；该环境调整不改变程序行为。
4. T02 按任务范围不包含身份认证、三级 RBAC 和组织管理页面，这些能力由后续阶段独立实施和验收。

---

### T03 本地身份与会话验收记录

#### 1. 验收结论

- 验收日期：2026-08-15
- 验收分支：`feat/t03-identity-session`
- 验收基线：`0e6c783 docs(identity): document T03 authentication baseline`
- 技术验收结论：**通过**
- 阻塞问题：无
- 任务状态：**已完成**，用户已于 2026-08-16 确认验收

本次验收覆盖 T03 规定的用户、凭据、管理员 bootstrap、登录、刷新、注销、会话失效和认证中间件。密码与 Token 摘要存储、并发 bootstrap、并发刷新、重放、过期、用户停用和 HTTP Cookie/Bearer 认证均已通过验证。

T03 不包含角色、Scope 继承、RBAC、用户管理 API、OIDC/LDAP 和完整安全审计；这些能力分别属于后续任务。

#### 2. 验收环境

| 项目 | 实际环境 |
|---|---|
| Go | `go1.26.5-X:nodwarf5 linux/amd64` |
| Node.js | `v26.5.1` |
| npm | `12.0.2` |
| Docker Compose | `v5.4.0` |
| PostgreSQL | `postgres:16-alpine` |
| Redis | `redis:7-alpine` |
| PostgreSQL 验收地址 | `127.0.0.1:5432` |
| Redis 验收地址 | `127.0.0.1:6379` |
| API 验收地址 | `127.0.0.1:58082/opskeeper` |

使用 `COMPOSE_PROJECT_NAME=opskeeper-t03 make infra-up` 创建隔离 Compose 项目。验收结束后通过 `make infra-down` 停止并删除容器和网络，并删除本次专用数据卷；未使用默认开发数据。

#### 3. 验收步骤与结果

##### 步骤 1：分支、提交和工作树

执行：

```bash
git status --short --branch
git log --oneline --decorate -4
```

结果：

```text
## feat/t03-identity-session...origin/feat/t03-identity-session
0e6c783 docs(identity): document T03 authentication baseline
5bedfcd feat(identity): add local authentication and sessions
fe13836 feat(runtime): finalize stage two delivery model
```

当前实现分支与远端同步，未修改 `main`。

结论：**通过**。

##### 步骤 2：质量门禁和竞态测试

执行：

```bash
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make quality

cd backend
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  go test -race -count=1 ./config ./httpapi ./identity
```

结果：

- `make quality` 通过：Prettier、Go vet、Svelte Check、Go 全量单测、前端测试、前端构建、嵌入前端测试和五个后端二进制构建全部成功。
- 竞态测试通过：`config`、`httpapi`、`identity` 三个包均无 race 报告。

结论：**通过**。

##### 步骤 3：启动隔离 PostgreSQL 和 Redis

执行：

```bash
COMPOSE_PROJECT_NAME=opskeeper-t03 make infra-up
docker ps --format '{{.Names}}|{{.Ports}}|{{.Status}}'
```

结果：

```text
opskeeper-t03-postgres-1|127.0.0.1:5432->5432/tcp|Up (healthy)
opskeeper-t03-redis-1|127.0.0.1:6379->6379/tcp|Up (healthy)
```

结论：**通过**。两个依赖均通过健康检查。

##### 步骤 4：迁移首次执行、重复执行和存储结构

执行两次：

```bash
env OPSK_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make APP_ENV_FILE=/dev/null migrate
```

两次均返回：

```text
direction=up
```

迁移和身份存储只读核对结果：

```text
1|scope_organization|64
2|scope_status|64
3|identity_session|64
admin@example.com|active|$argon2id$
32|32
```

说明：迁移校验和均为 64 位十六进制 SHA-256；密码使用 Argon2id；访问和刷新 Token 在数据库中均为 32 字节摘要，而不是明文 Token。

结论：**通过**。迁移可重复执行，T03 三张表和校验记录已创建。

##### 步骤 5：真实 PostgreSQL 集成测试

执行：

```bash
env OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make APP_ENV_FILE=/dev/null backend-integration-test
```

结果：

```text
ok  opskeeper/backend/migrations
ok  opskeeper/backend/organization
ok  opskeeper/backend/identity
```

覆盖内容：迁移锁和并发幂等、迁移 checksum、身份迁移回滚、首个管理员并发创建只成功一次、用户名/邮箱/手机号登录、邮箱与手机号唯一绑定、错误密码、密码哈希不落明文、Token 摘要不落明文、访问和刷新过期、刷新轮换、旧 Token 重放、并发刷新单赢家、注销、全部会话失效，以及 disabled/locked 用户会话立即失效。

结论：**通过**。

##### 步骤 6：受控创建首个管理员

通过临时密码文件执行：

```bash
env OPSK_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  OPSK_REDIS_URL='redis://127.0.0.1:6379/0' \
  OPSK_BOOTSTRAP_USERNAME=admin \
  OPSK_BOOTSTRAP_EMAIL=admin@example.com \
  OPSK_BOOTSTRAP_PASSWORD_FILE=/tmp/opskeeper-t03-password \
  make APP_ENV_FILE=/dev/null admin-create
```

首次执行结果：

```text
bootstrap administrator created
```

使用第二个用户名再次执行，命令返回非零状态：

```text
create bootstrap administrator: bootstrap administrator already exists
```

结论：**通过**。管理员只能通过受控命令创建一次，没有公开 HTTP bootstrap 接口，也没有默认生产密码。

##### 步骤 7：嵌入前端并启动 API

执行：

```bash
env OPSK_HTTP_ADDRESS=127.0.0.1:58082 \
  OPSK_BASE_PATH=/opskeeper \
  OPSK_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  OPSK_REDIS_URL='redis://127.0.0.1:6379/0' \
  make APP_ENV_FILE=/dev/null run-front-api
```

结果：

- Vite 成功构建前端，转换 `110 modules`。
- `frontend/dist` 被复制到 `backend/webui/dist`。
- API 使用 `-tags=embed_webui` 启动并监听 `127.0.0.1:58082`。
- `GET /opskeeper/` 返回嵌入的 `index.html`，其中包含 `<base href="/opskeeper/">`。
- `GET /opskeeper/health/live` 返回 `200` 和 `status=alive`。

结论：**通过**。前端制品确实被嵌入 Go API，单进程同时提供页面、静态资源和 API。

##### 步骤 8：HTTP 认证和会话生命周期

| 验证场景 | 结果 |
|---|---|
| 错误密码登录 | `401 invalid_credentials`，响应不包含密码或内部错误 |
| `/auth/bootstrap` 公开接口 | `404 not_found` |
| 未认证访问组织 API | `401 invalid_session` |
| 正确密码登录 | `200`，JSON 仅返回用户资料 |
| 登录 Cookie | `HttpOnly`、`SameSite=Lax`、`Path=/opskeeper`、有效过期时间 |
| Cookie 访问 `/auth/me` | `200` |
| Bearer Token 访问 `/auth/me` | `200` |
| Cookie 访问组织平台 API | `200` |
| 刷新会话 | `200`，访问和刷新 Cookie 均轮换 |
| 重放旧刷新 Token | `401 invalid_session`，同时清理 Cookie |
| 刷新前旧访问 Token | `401 invalid_session` |
| `logout-all` | `204`，清理 Cookie |
| `logout-all` 后继续访问 | `401 invalid_session` |
| 数据库将用户设为 `disabled` 后访问 | `401 invalid_session` |

结论：**通过**。认证中间件只确认身份和用户状态，不提前引入角色或 Scope 授权判断。

##### 步骤 9：清理验收资源

执行：

```bash
COMPOSE_PROJECT_NAME=opskeeper-t03 make infra-down
docker volume rm opskeeper-t03_postgres-data opskeeper-t03_redis-data
```

结果：

- `opskeeper-t03-postgres-1` 和 `opskeeper-t03-redis-1` 已停止并删除。
- `opskeeper-t03_default` 网络和两个临时数据卷已删除。
- 临时密码文件和 Cookie 文件已删除。
- 未留下验收专用容器或卷。

结论：**通过**。

#### 4. 遗留范围

T03 已完成本地身份和会话安全基线，但以下内容明确留给后续任务：

- T04：三级 RBAC、Scope 继承和数据隔离。
- T05：用户/角色管理、权限缓存失效和安全审计。
- 后续任务：OIDC/LDAP、企业单点登录和其他外部身份同步。

用户已确认验收通过，T03 可按版本控制规范合并到 `main`。

---

### T04 三级 RBAC 与数据隔离验收记录

#### 1. 验收结论

- 验收日期：2026-08-16
- 验收分支：`feat/t04-rbac-data-isolation`
- 实施基线：`ef068ee docs(authorization): record T04 RBAC baseline`
- 技术验收结论：**通过**
- 阻塞问题：无
- 任务状态：**已完成**，用户已于 2026-08-16 确认验收

本次验收覆盖九个内置角色、角色权限种子、平台/团队/项目 Scope 继承、用户状态和 Scope 状态约束、组织查询层过滤、按 ID 访问隔离、分页与计数隔离、管理员 bootstrap 绑定，以及 HTTP 授权中间件。T04 不包含角色管理 API、用户组、自定义角色、权限缓存和安全审计，这些能力留给 T05。

#### 2. 验收环境

| 项目 | 实际环境 |
|---|---|
| Go | `go1.26.5-X:nodwarf5 linux/amd64` |
| Node.js | `v26.5.1` |
| npm | `12.0.2` |
| Docker Compose | `v5.4.0` |
| PostgreSQL | `postgres:16-alpine` |
| Redis | `redis:7-alpine` |
| PostgreSQL 验收地址 | `127.0.0.1:5432` |
| Redis 验收地址 | `127.0.0.1:6379` |

验收期间使用 `COMPOSE_PROJECT_NAME=opskeeper-t04 make infra-up` 创建隔离的 PostgreSQL、Redis、网络和数据卷。验收结束后已执行 `make infra-down` 并删除两个临时数据卷。

#### 3. 验收步骤与结果

##### 步骤 1：分支和变更范围

执行：

```bash
git status --short --branch
git log --oneline --decorate -8
git diff --stat main...HEAD
```

结果：当前分支为 `feat/t04-rbac-data-isolation`，远端同步；T04 实现提交为 `ef0214f`，授权设计文档提交为 `ef068ee`。本次验收又补充了祖先 Scope 状态校验及对应集成测试。

结论：**通过**。变更集中在授权、组织查询、HTTP 路由、迁移、测试和相关设计文档。

##### 步骤 2：质量门禁

执行：

```bash
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make quality
```

结果：**通过**。Prettier、Go vet、Svelte Check、Go 全量单元测试、前端测试、前端构建、嵌入式前端测试和五个后端二进制构建全部成功。

结论：**通过**。

##### 步骤 3：竞态测试

执行：

```bash
cd backend
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  go test -race -count=1 ./config ./httpapi ./authorization ./identity
```

结果：`config`、`httpapi`、`authorization`、`identity` 四个包均通过，无 race 报告。

结论：**通过**。

##### 步骤 4：启动隔离中间件

执行：

```bash
COMPOSE_PROJECT_NAME=opskeeper-t04 make infra-up
docker ps --format '{{.Names}}|{{.Ports}}|{{.Status}}'
```

结果：

```text
opskeeper-t04-postgres-1|127.0.0.1:5432->5432/tcp|Up (healthy)
opskeeper-t04-redis-1|127.0.0.1:6379->6379/tcp|Up (healthy)
```

结论：**通过**。两个依赖均通过健康检查。

##### 步骤 5：真实 PostgreSQL 集成测试

执行：

```bash
env OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make APP_ENV_FILE=/dev/null backend-integration-test
```

结果：

```text
ok  opskeeper/backend/migrations
ok  opskeeper/backend/organization
ok  opskeeper/backend/identity
ok  opskeeper/backend/authorization
```

覆盖内容：

- `0004_rbac` 前滚、重复执行和回滚；
- 九个内置角色及角色权限种子；
- 角色 Scope 类型不匹配时由数据库约束拒绝；
- 首个管理员获得幂等的 `PlatformAdmin` 平台绑定；
- TeamViewer 向下继承到所属项目，但不能访问其他团队或平台；
- 列表、分页、总数、按 ID 查询和更新锁定均使用同一 Scope 过滤；
- 停用绑定 Scope 后权限失效；
- 停用平台祖先 Scope 后，即使团队和项目仍标记为 active，后代权限也失效；
- 多角色权限采用并集，停用用户不能获得授权。

结论：**通过**。数据隔离在 SQL 查询边界执行，不依赖先加载全部数据再以内存过滤。

##### 步骤 6：HTTP 授权行为

已使用真实 PostgreSQL、Redis 和 API 进程验证：

| 场景 | 结果 |
|---|---|
| 未认证访问组织接口 | `401 invalid_session` |
| PlatformAdmin 登录 | `200` |
| PlatformAdmin 读取平台 | `200` |
| PlatformAdmin 创建团队和项目 | `201` |
| ProjectViewer 读取自身项目 | `200` |
| ProjectViewer 按 ID 访问父团队 | `404 not_found` |
| ProjectViewer 修改项目 | `403 forbidden` |
| 健康检查 | `200`，保持公开 |

授权中间件位于 T03 身份认证中间件之后；无任何目标 Scope 权限的用户在中间件处返回 `403`，已授权用户的对象查询仍由组织 Store 再执行 Scope 过滤。

结论：**通过**。认证和授权职责分离，未授权对象不会通过按 ID 接口暴露存在性。

##### 步骤 7：清理验收资源

执行：

```bash
COMPOSE_PROJECT_NAME=opskeeper-t04 make infra-down
docker volume rm opskeeper-t04_postgres-data opskeeper-t04_redis-data
```

结果：验收容器、网络和临时数据卷均已删除，未留下 T04 专用运行资源。

结论：**通过**。

#### 4. 实施边界

T04 采用 PostgreSQL 作为授权权威来源，暂不使用 Redis 权限缓存；暂不提供角色绑定管理 API，也不引入显式拒绝规则、PostgreSQL RLS、用户组、自定义角色和安全审计。以上能力进入 T05 或后续任务，不能在 T04 验收中默认视为已实现。

用户已确认本记录，T04 功能分支随后合并到 `main`，任务书中的 T04 状态已更新为“已完成”。

---

### T05 身份权限管理与安全审计验收记录

#### 1. 验收结论

- 验收日期：2026-08-16
- 验收分支：`feat/t05-access-management-audit`
- 技术验收结论：**通过**
- 阻塞问题：无
- 任务状态：**已完成**
- 用户确认日期：2026-08-16

本次验收覆盖用户创建与状态管理、Scope 用户组、组成员、角色绑定管理、防权限提升、权限 revision 缓存、审计记录与查询，以及认证和授权中间件组合。T05 不包含 OIDC/LDAP、自定义角色编辑器、显式拒绝策略和 PostgreSQL RLS。

#### 2. 验收环境

| 项目 | 实际环境 |
|---|---|
| Go | `go1.26.5-X:nodwarf5 linux/amd64` |
| Node.js | `v26.5.1` |
| Docker Compose | `v5.4.0` |
| PostgreSQL | `postgres:16-alpine` |
| Redis | `redis:7-alpine` |
| API 验收地址 | `127.0.0.1:58085/opskeeper` |

验收使用 `COMPOSE_PROJECT_NAME=opskeeper-t05` 创建隔离中间件。结束后已删除容器、网络、数据卷、临时管理员密码文件和 Cookie 文件。

#### 3. 验收步骤与结果

##### 步骤 1：分支和范围

执行：

```bash
git status --short --branch
git log --oneline --decorate -6
```

结果：当前分支为 `feat/t05-access-management-audit`，基于已验收的 T04 `main` 创建。变更集中在 `0005` 迁移、`authorization` 管理服务、`identity` 用户管理、`audit` 审计包、HTTP 管理路由、测试和设计文档。

结论：**通过**。

##### 步骤 2：迁移和现有集成回归

执行：

```bash
COMPOSE_PROJECT_NAME=opskeeper-t05 make infra-up
env OPSK_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make APP_ENV_FILE=/dev/null migrate
env OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make APP_ENV_FILE=/dev/null backend-integration-test
```

结果：PostgreSQL 和 Redis 健康检查通过；迁移 `0001` 至 `0005` 前滚成功；迁移、组织、身份和授权四个集成包全部通过。迁移回滚测试覆盖 `0005` 访问管理与审计表、`0004` RBAC、`0003` 身份、`0002` 状态和 `0001` 组织模型。

结论：**通过**。

##### 步骤 3：T05 领域集成测试

结果：`authorization` 集成测试通过，覆盖：

- 用户组创建、组成员添加和 TeamViewer 组角色绑定；
- 组成员向下继承团队及项目权限；
- 用户停用后组权限立即失效；
- TeamAdmin 尝试向平台 Scope 授予 PlatformAdmin 被拒绝；
- 角色权限不是授权人有效权限子集时被拒绝；
- 授权 revision 变化后缓存键切换，不复用旧授权结果；
- 组创建、组成员和角色绑定均产生审计事件；
- 数据库角色 Scope 类型约束仍然生效。

结论：**通过**。

##### 步骤 4：质量门禁和竞态测试

执行：

```bash
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make quality

cd backend
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  go test -race -count=1 ./config ./httpapi ./authorization ./identity ./audit
```

结果：格式检查、Go vet、Svelte Check、前后端测试、前端构建、嵌入前端测试和五个后端二进制构建全部通过；`config`、`httpapi`、`authorization`、`identity` 和 `audit` 竞态测试通过，无 race 报告。

结论：**通过**。

##### 步骤 5：真实 HTTP 管理流程

使用临时管理员启动 API 后验证：

| 场景 | 结果 |
|---|---|
| 管理员登录 | `200`，只返回用户资料，Cookie 为 HttpOnly |
| 创建用户 | `201`，响应不包含密码或密码摘要 |
| 创建团队 | `201`，T04 组织 API 回归通过 |
| 创建 Scope 用户组 | `201` |
| 读取内置角色 | `200`，包含 `member:grant` |
| 创建 TeamViewer 组角色绑定 | `201` |
| 添加组成员 | `201` |
| 组成员登录并读取所属团队 | `200` |
| TeamViewer 创建组 | `403 forbidden` |
| 查询审计日志 | `200`，包含登录、用户创建、组成员和角色绑定事件 |
| 审计路由未认证访问 | `401 invalid_session` |

验证中还发现并修复了审计路由中间件顺序问题，当前顺序为 T03 身份认证后再执行 `audit:read` 授权。

结论：**通过**。

##### 步骤 6：清理验收资源

执行：

```bash
COMPOSE_PROJECT_NAME=opskeeper-t05 make infra-down
docker volume rm opskeeper-t05_postgres-data opskeeper-t05_redis-data
```

结果：T05 验收容器、网络、数据卷和临时凭据均已清理。

结论：**通过**。

#### 4. 实施边界

T05 使用 PostgreSQL 作为用户、组、角色绑定和审计权威来源；Redis 只缓存包含 PostgreSQL `authorization_revision` 的 Scope 授权结果。缓存读取失败时回源 PostgreSQL，不能沿用旧缓存结果。

当前用户管理限定为 PlatformAdmin；团队和项目管理员通过自身 `member:grant` Scope 管理用户组成员和角色绑定。T05 暂不提供 OIDC/LDAP、自定义角色编辑器、显式拒绝规则和 PostgreSQL RLS。

用户确认本记录后，将把 `feat/t05-access-management-audit` 合并到 `main`，并将任务书中的 T05 状态更新为“已完成”。

---

### T06 统一资源目录、凭据和关系模型验收记录

#### 1. 验收结论

- 验收日期：2026-08-16
- 验收分支：`feat/t06-resource-catalog`
- 验收提交：`6948b38 feat(resource): add catalog and relation model`
- 技术验收结论：**通过**
- 阻塞问题：无
- 任务状态：**已完成**
- 用户确认日期：2026-08-16

本次验收覆盖统一资源目录、版本化 Schema、资源凭据密文、Scope 关系约束、关系环路、拓扑查询、默认资源解析、资源 Scope 移动保护、资源授权过滤和既有 T01-T05 回归。T06 不包含具体资源连接器、Kubernetes 自动发现和资源管理前端。

#### 2. 验收环境

| 项目 | 实际环境 |
|---|---|
| Go | `go1.26.5-X:nodwarf5 linux/amd64` |
| Node.js | `v26.5.1` |
| Docker Compose | `v5.4.0` |
| PostgreSQL | `postgres:16-alpine` |
| Redis | `redis:7-alpine` |
| 验收 PostgreSQL | `127.0.0.1:55437` |
| 验收 Redis | `127.0.0.1:56384` |
| Compose 项目 | `opskeeper-t06-acceptance` |

验收使用独立 Compose 项目、端口、网络和数据卷。结束后已删除验收容器、网络和数据卷。

#### 3. 验收步骤与结果

##### 步骤 1：分支、提交和范围

执行：

```bash
git status --short --branch
git log -4 --oneline --decorate
git diff --check origin/main...HEAD
```

结果：当前分支为 `feat/t06-resource-catalog`，工作树干净，基于已完成 T05 的 `main`；变更集中在 `0006` 迁移、`backend/resource/`、`backend/credential/`、资源 HTTP API、测试和设计文档。

结论：**通过**。

##### 步骤 2：质量门禁

执行：

```bash
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make quality
```

结果：以下项目全部通过：

- Prettier 格式检查；
- Go vet；
- Svelte Check；
- Shell 语法检查；
- Go 全量测试；
- 前端测试和生产构建；
- 嵌入式前端测试；
- API、Worker、Scheduler、Migration、Admin 五个二进制构建。

结论：**通过**。

##### 步骤 3：竞态测试

执行：

```bash
cd backend
env GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  go test -race -count=1 \
    ./config ./httpapi ./authorization ./identity ./audit ./credential ./resource
```

结果：全部通过，无 race detector 报告。

结论：**通过**。

##### 步骤 4：迁移和数据库集成回归

使用 PostgreSQL `127.0.0.1:55437` 启动隔离数据库后执行：

```bash
env OPSK_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:55437/opskeeper?sslmode=disable' \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make APP_ENV_FILE=/dev/null migrate

env OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:55437/opskeeper?sslmode=disable' \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make APP_ENV_FILE=/dev/null backend-integration-test
```

结果：

```text
ok opskeeper/backend/migrations
ok opskeeper/backend/organization
ok opskeeper/backend/identity
ok opskeeper/backend/authorization
ok opskeeper/backend/resource
```

迁移测试覆盖 `0001` 至 `0006` 前滚、回滚、checksum 和并发迁移；既有组织、身份和授权集成回归均通过。

结论：**通过**。

##### 步骤 5：资源目录和 Scope 规则

资源集成测试验证：

- 同一种资源可创建在平台、团队和项目 Scope；
- 资源保存实际使用的 `schema_version`；
- 标签和 JSON 配置可以持久化；
- 项目可以引用所属团队资源和平台资源；
- 项目不能引用其他团队或项目资源；
- 项目用户可见自身资源及所属团队、平台资源；
- 上级资源不因可见性而自动获得下级资源的修改权限；
- 资源 Scope 移动存在失效关系时会被拒绝；
- 软删除不会返回已删除资源。

结论：**通过**。

##### 步骤 6：关系、环路和拓扑

资源集成测试验证：

- 关系支持 `contains`、`depends_on`、`observed_by` 等注册类型；
- 数据库触发器拒绝向下引用和跨团队引用；
- 自引用和递归关系环路被拒绝；
- 关系查询支持资源的入边和出边；
- 拓扑查询使用递归 CTE；
- 最大深度限制为 8；
- 最大节点数量限制为 200；
- 拓扑结果继续执行资源可见范围过滤。

结论：**通过**。

##### 步骤 7：Schema、默认配置和凭据安全

验证结果：

- 首批资源类型 Schema 在 `resource_schemas` 中注册；
- Schema 校验覆盖对象类型、必填字段、属性类型和额外属性；
- 项目默认配置优先于团队默认配置，团队默认配置优先于平台默认配置；
- 默认资源必须位于请求 Scope 的祖先链上；
- `resource_credentials` 与登录用户 `credentials` 分离；
- API 响应只包含凭据 ID、名称、用途、Scope 和密钥版本；
- API 响应不包含 `secret`、`ciphertext` 或密码摘要；
- AES-GCM 使用随机 nonce；
- 数据库中的凭据字段不包含明文；
- 生产环境缺少 `OPSK_CREDENTIAL_KEY` 时 API 不启动，开发环境使用本地密钥实现。

HTTP 脱敏回归测试位于 `backend/httpapi/resource_test.go`，凭据密文集成测试位于 `backend/resource/integration_test.go`。

结论：**通过**。

##### 步骤 8：验收资源清理

执行：

```bash
docker compose -f deploy/compose/docker-compose.yml \
  --env-file deploy/compose/.env.example \
  -p opskeeper-t06-acceptance down -v
```

结果：验收专用 PostgreSQL、Redis、网络和数据卷均已清理，未修改默认开发环境资源。

结论：**通过**。

#### 4. 实施边界

T06 当前提供资源目录和关系模型的后端基础能力，包含资源 CRUD、Schema、凭据密文、默认配置、关系和拓扑 API。具体 Kubernetes 或中间件连接器、自动发现、导入流程和资源管理前端不在本任务范围内。

用户已确认本记录。T06 验收文档状态已更新为“已完成”，并将把 `feat/t06-resource-catalog` 以非快进方式合并到 `main`。

---

### T07 Svelte 管理控制台基础功能验收记录

#### 1. 基本信息

- 任务：T07 Svelte 管理控制台基础功能
- 实施分支：`feat/t07-admin-console`
- 当前状态：**已完成**
- 依赖任务：T06 统一资源目录、凭据和关系模型
- 验收提交：`026c07d`（实施提交，已完成页面和资源模型验收）
- 用户确认：2026-08-16 已确认验收通过

#### 2. 本次交付

T07 在现有 Svelte/Vite 前端上实现了可操作的管理控制台，使用已有后端认证、组织、授权、资源和关系 API：

1. 登录、会话恢复、Cookie 刷新、注销和失效会话提示。
2. 平台、团队、项目三级作用域选择器，列表中的组织和资源均显示作用域。
3. 平台总览、控制平面健康状态、团队列表、项目列表及创建表单。
4. 资源目录、类型筛选、资源创建、Schema 字段预览、软删除、关系编辑和拓扑节点展示。
5. 用户、成员组和角色绑定页面，提供成员组和角色绑定创建及删除操作。
6. 统一加载、空数据、错误、无权限、登录中和操作中状态，支持桌面和移动布局。
7. 前端 API 客户端统一处理 `OPSK_BASE_PATH`、JSON 错误、Cookie 会话和 401 刷新重试。
8. 团队、项目和资源类型图标；资源 Schema 中文名称、说明和类型化选择卡片。
9. PostgreSQL、Redis、Kafka、LLMProvider 和 KubernetesCluster 的差异化字段；密码、Token 和 kubeconfig 通过加密凭据保存。
10. 禁止将 Kubernetes Namespace、Node、Workload、Pod、Service、Ingress、LLM Model 和 Credential 登记为资源。

后端仍是最终权限边界。前端只根据接口结果展示或禁用操作，不能替代服务端授权判断。

#### 3. 验证步骤与结果

##### 3.1 前端格式与静态检查

```bash
cd frontend
npm run format:check
npm run check
```

结果：通过。Prettier 无格式差异，`svelte-check found 0 errors and 0 warnings`。

##### 3.2 前端单元测试

```bash
cd frontend
npm run test
```

结果：通过。2 个测试文件、5 个测试用例通过，覆盖健康状态映射、base path URL、会话刷新重试和结构化错误映射。

##### 3.3 前端生产构建

```bash
cd frontend
npm run build
```

结果：通过。Vite 成功生成 `frontend/dist` 制品。

##### 3.4 资源类型和凭据边界

使用管理员会话验证：

- `/api/v1/resources/schemas` 返回中文名称、说明和图标字段。
- `Pod` 等 Kubernetes 派生对象创建请求返回 HTTP `400`，错误为 `Resource kind or schema is not registered`。
- PostgreSQL 类型表单的 Host、Port、Database、Username 进入资源配置，Password 创建独立加密凭据并通过 `credential_id` 关联。
- 测试资源和凭据在验证后已删除。

结果：通过。

##### 3.5 页面验收路径

启动依赖和 API 后，访问 `http://localhost:8080/opskeeper/`，使用 `make admin-create` 创建的管理员登录，依次检查：

1. 刷新页面后会话保持；退出后回到登录页。
2. 在作用域选择器中切换平台、团队和项目，组织与资源列表显示对应范围。
3. 在“组织”页面创建团队和项目，并确认新对象出现在列表。
4. 在“资源”页面选择 Schema、创建资源，查看配置、关系和拓扑；尝试停用资源。
5. 在“成员与角色”页面查看用户、成员组和角色绑定；无权限账号显示清晰错误，不展示越权数据。
6. 在窄窗口检查导航、表格、表单和详情区域无横向遮挡或内容重叠。

#### 4. 范围边界

T07 不包含 Kubernetes 发现导入、AI 对话、巡检和 Connector 连接测试；这些能力分别属于 T08 及后续任务。资源配置表单对已注册 Schema 展示字段预览，未在本阶段引入任意 JSON Schema 编辑器。

#### 5. 补充端到端验收

```bash
cd frontend
npm run test:e2e
```

结果：通过，完整套件 5 个 Playwright 用例通过，其中 2 个覆盖 T07。路由夹具覆盖登录/会话恢复、平台到团队 Scope 切换、资源页移动视口及横向溢出检查；浏览器使用系统 Chrome，未使用生产凭据。

#### 6. 验收结论

用户已确认 T07 验收通过。桌面和移动视口、组织与资源管理、资源类型卡片、类型化配置字段、凭据边界及派生资源登记限制均符合验收要求。

---

### T08 Kubernetes 集群发现与项目导入验收记录

#### 1. 基本信息

- 任务：T08 Kubernetes 集群发现与项目导入
- 实施分支：`feat/t08-kubernetes-discovery`
- 当前状态：**已完成**
- 依赖任务：T07 Svelte 管理控制台基础功能
- 验收提交：`7189bc1`（Kubernetes 发现、项目与应用导入、具体资源授权）
- 验收日期：2026-08-16
- 用户确认：2026-08-16 已确认验收通过

#### 2. 本次交付

1. `Kubernetes` 作为平台、团队或项目 Scope 下可登记的集群资源，非敏感连接配置与加密 kubeconfig 分离保存。
2. 使用 Kubernetes API 发现 Namespace、Deployment、StatefulSet、DaemonSet、Job、CronJob、Pod、Service、Ingress 和 EndpointSlice。
3. Namespace 映射为已有或新建 Project，不登记为资源；平台、团队和项目级集群分别执行对应的项目映射边界。
4. 工作负载统一映射为项目级 `Application`，原始 Kubernetes 类型保存在 `config.kubernetes.workload_kind`，不增加重复的 `application_type` 字段。
5. Pod 副本聚合到 Application 的 `instances`；Service、Ingress 和 EndpointSlice 聚合为 Application 配置，均不登记为独立资源。
6. 以 `scope_id + kind + external_uid + source_resource_id` 作为有效资源幂等键；重复导入更新原 Application，失联 Application 标记为 `unknown`。
7. 增加发现运行、发现项、预览、确认导入和历史查询 API，以及前端“集群导入”操作页面。
8. 同一 Kubernetes 资源同时只允许一个 queued/running 扫描；单次 Kubernetes API 扫描最长运行 5 分钟。
9. 保持 platform、team、project 三级 Scope，不增加 application Scope；通过 `ProjectMember` 和通用资源角色实现项目内具体资源授权。
10. 增加 `Repository` 和 `Artifact` 资源类型，并保持非敏感配置和加密凭据分离。

#### 3. 自动化验证步骤与结果

##### 3.1 发现模块集成测试

```bash
cd backend
env \
  OPSK_TEST_DATABASE_URL=postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  go test -tags=integration ./discovery
```

结果：通过。验证发现运行与发现项写入、Project/Application 导入计数，以及发现记录遵循来源 Kubernetes 资源的可见性过滤。

##### 3.2 完整后端集成测试

```bash
cd backend
env \
  OPSK_TEST_DATABASE_URL=postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  go test -tags=integration \
  ./migrations ./organization ./identity ./authorization ./resource ./discovery
```

结果：通过。迁移、组织、身份、授权、资源和发现模块全部通过；其中覆盖：

1. `ProjectMember` 可查看平台、团队、项目导航链，但默认看不到项目资源。
2. 绑定 `ResourceViewer` 后只能读取指定资源，已知其他资源 ID 仍返回 Not Found。
3. 资源角色绑定的创建和删除会递增授权 revision，使缓存结果失效。
4. 没有所属 Scope 访问资格时不能创建资源角色绑定。
5. 同一 Kubernetes 工作负载重复导入只保留一个 Application，并更新其配置和名称。
6. 本轮未出现的已导入 Application 被标记为 `unknown`，不会自动删除。
7. 空的 `item_ids` 不会被解释成“导入全部 Application”，只有显式选中的 Application 会被导入。
8. 同一 Kubernetes 资源已有 queued/running 发现任务时，创建第二个任务返回冲突。

##### 3.3 项目质量门禁

```bash
env \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  make quality
```

结果：通过。包括 Prettier、`go vet`、`svelte-check`、Shell 语法检查、全部 Go 单元测试、前端 5 个测试、Vite 生产构建、嵌入式 Web UI 测试和 API/Worker/Scheduler/Migrate/Admin 五个 Go 二进制构建。

##### 3.4 数据库迁移结果

本地开发数据库已应用 `0009_kubernetes_discovery`。只读检查确认：

- `Application`、`Kubernetes`、`Repository`、`Artifact` 为可登记资源类型。
- `Namespace`、`Pod`、`Service`、`Ingress`、`Endpoint`、`CronApplication` 为停用类型。
- `ResourceViewer`、`ResourceOperator`、`ResourceAdmin` 三个通用资源角色存在。

##### 3.5 嵌入式页面与响应式检查

验收服务地址：`http://localhost:58080/opskeeper/`。

结果：登录页、嵌入式 CSS/JavaScript 和 `/health/ready` 正常返回；桌面视口无内容重叠，390 x 844 移动视口无横向溢出。由于没有可复用的管理员登录会话，登录后的集群导入与具体资源授权页面保留给人工验收。

#### 4. 人工验收步骤

1. 使用管理员账号登录 `http://localhost:58080/opskeeper/`。
2. 在平台、团队或项目 Scope 登记一个 `Kubernetes` 资源，并通过凭据表单保存 kubeconfig。
3. 进入“集群导入”，启动发现并确认页面能显示 Namespace、Application 候选、工作负载类型、实例、Service 和 Ingress 摘要。
4. 将 Namespace 选择为新建 Project、映射已有 Project 或忽略，确认未提交前不会创建 Project 或 Application。
5. 确认导入后检查资源目录：存在 `Application`，其 Kubernetes 类型显示自 `kubernetes.workload_kind`；不存在 Namespace、Pod、Service、Ingress 或 Endpoint 资源。
6. 对同一集群再次发现和导入，确认不会产生重复 Application。
7. 为仅具有 `ProjectMember` 的用户授予一个 Application 的 `ResourceViewer`，确认该用户能进入项目且只能查看被授权资源。
8. 分别验证无权限启动发现、无权限导入和无效 kubeconfig，页面应显示明确错误且不泄露凭据内容。

#### 5. 范围边界

T08 不包含周期调度、主动修改 Kubernetes 对象、日志和事件采集、Connector 查询、AI 分析及真实大规模集群的性能验收。发现实现使用 Kubernetes 客户端分页参数读取列表；大集群吞吐、限流和长任务恢复将在后续 Connector、任务引擎及生产验收阶段继续验证。

#### 6. 验收结论

自动化测试、数据库迁移、页面检查以及用户人工验收均已通过。用户已确认 T08 验收完成，Kubernetes 发现、Project/Namespace 映射、Application 选择导入和具体资源授权符合本阶段要求。

---

### T09 Connector 框架与外部监控平台验收记录

#### 1. 基本信息

- 任务：T09 Connector 框架与外部监控平台
- 实施分支：`feat/t09-connectors`
- 当前状态：**已完成**
- 依赖任务：T08 Kubernetes 集群发现与项目导入
- 验收提交：`615bb46`（Connector 运行框架）、`fd17d9d`（页面自动填充修复与自动验收记录）
- 验收日期：2026-08-16
- 用户确认：2026-08-16 已确认验收通过

#### 2. 本次交付

1. 新增 Connector 注册表和类型化能力接口，按资源类型及 Schema 版本解析 Kubernetes、Prometheus、Loki 适配器。
2. Prometheus 支持指标 Range Query、告警和连接测试；Loki 支持日志 Range Query、Tenant Header 和连接测试；Kubernetes 支持只读白名单对象并拒绝 Secret。
3. 统一执行超时、临时错误重试、全局并发、查询范围、步长、条数和响应大小限制；分页截断结果标记为 `partial`。
4. 统一错误分类和安全公开消息，不在 API、连接检查记录或审计详情中保存上游正文及凭据。
5. 新增连接检查 PostgreSQL 表，以及受 `resource:use`、`resource:read` 保护的测试与最近结果 API。
6. 资源详情页为 Kubernetes、Prometheus、Loki 显示连接状态、耗时、检查时间、能力和“测试连接”操作。
7. Prometheus、Loki 改用类型化资源字段，URL 和 Tenant ID 保存在配置中，Username、Password、Token 通过现有凭据加密边界保存。

#### 3. 自动化验证步骤与结果

##### 3.1 后端单元测试

```bash
cd backend
env GOMODCACHE=/tmp/opskeeper-gomodcache GOCACHE=/tmp/opskeeper-gocache \
  go test ./...
```

结果：通过。覆盖注册版本匹配与冲突、Bearer/Basic/Tenant Header、HTTP 状态和超时分类、响应大小、临时错误重试、并发限制、查询范围、Kubernetes Secret 拒绝与分页截断，以及连接测试 API 权限和安全错误映射。

##### 3.2 PostgreSQL 集成测试

```bash
cd backend
env \
  OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  GOMODCACHE=/tmp/opskeeper-gomodcache \
  GOCACHE=/tmp/opskeeper-gocache \
  go test -tags=integration \
  ./migrations ./organization ./identity ./authorization ./resource ./discovery ./connector
```

结果：通过。连接检查的操作者、能力数组、失败分类、时间和最新结果排序均通过真实 pgx/PostgreSQL 验证；既有模块及完整迁移链无回归。

##### 3.3 前端检查与构建

```bash
cd frontend
npm run check
npm run test
npm run build
```

结果：通过。`svelte-check` 无错误或警告，6 项前端测试通过，Vite 生产构建成功。

##### 3.4 数据库迁移

```bash
make migrate
make migrate-down
make migrate
```

结果：通过。升级后存在 `resource_connection_checks` 和类型化 Prometheus/Loki Schema；回滚后表被删除且 Schema 恢复；再次升级后迁移 0010 处于最终应用状态。

##### 3.5 项目质量门禁

```bash
env GOMODCACHE=/tmp/opskeeper-gomodcache GOCACHE=/tmp/opskeeper-gocache \
  make quality
```

结果：通过。Prettier、Go vet、`svelte-check`、Shell 语法检查、全部 Go 单元测试、前端 6 项测试、嵌入式 Web 测试、Vite 生产构建，以及 API、Worker、Scheduler、Migrate、Admin 五个 Go 二进制构建全部成功。

##### 3.6 页面自动验收

1. 通过 `make run-front-api` 启动嵌入式 Web UI，使用临时 `PlatformAdmin` 账号登录；PostgreSQL、Redis 和平台作用域正常加载。
2. 登记 Kubernetes、Prometheus、Loki 临时资源并连接本地模拟服务；三类资源分别显示“连接正常”、耗时和正确的能力列表。
3. 停止模拟服务后再次测试 Loki；页面显示“连接失败”和安全公开消息，不包含上游正文、Token、密码或 kubeconfig。
4. 刷新页面并重新选择 Loki；最近失败结果、耗时、时间和能力从 PostgreSQL 正常恢复。
5. 登记无 T09 Connector 的 Tempo 资源；详情页不显示连接状态或“测试连接”操作。
6. 使用临时 `PlatformViewer` 账号验证权限；读取最近结果返回 200，执行连接测试返回 403。
7. 在 1440 x 900 和 390 x 844 视口检查资源目录与连接状态区域；未发现文字重叠、控件溢出或不可操作区域，页面控制台无应用错误。
8. 验收中发现浏览器会把已保存的登录信息自动填入资源标签和连接字段；已为资源标签、普通配置和敏感配置设置明确的自动填充策略，并重新验证登记成功。
9. 验收结束后删除全部临时资源、连接检查、加密凭据、授权、会话、账号和模拟服务文件；数据库核对无 T09 临时数据残留。

结果：通过。用户已确认验收，按版本控制规范合入 `main`。

#### 4. 人工验收步骤

1. 运行 `make run-front-api`，登录后进入“资源目录”。
2. 登记或选择 Kubernetes、Prometheus、Loki 资源，确认详情中出现连接状态行和“测试连接”按钮。
3. 使用有效地址及凭据执行测试，确认页面显示“连接正常”、耗时、时间和对应能力。
4. 使用错误凭据再次测试，确认页面显示安全失败消息，不显示 Token、密码、kubeconfig 或上游响应正文。
5. 使用只有 `resource:read` 的账号确认可以查看最近结果但不能执行测试；增加 `resource:use` 后确认可以执行。
6. 选择 PostgreSQL、Redis 等尚无 T09 Connector 的资源，确认详情页不显示误导性的连接测试操作。

#### 5. 本阶段边界

- 指标、日志、告警和 Kubernetes 查询尚未作为公共 HTTP API 暴露，只提供给后续受控 Runner 的 Go 能力接口。
- T09 不实现 Tempo、Jaeger、Elastic、中间件 Connector、AI 诊断、自动巡检、按资源配额或熔断。
- T09 已完成验收并合入 `main`。

---

### T10 LLM Provider、Skill 注册表与受控 Runner 验收记录

#### 1. 基本信息

- 任务：T10 LLM Provider、Skill 注册表与受控 Runner
- 实施分支：`feat/t10-llm-skill-runner`
- 当前状态：**验收通过**
- 依赖任务：T09 Connector 框架与外部监控平台
- 验收日期：2026-08-16
- 用户确认：通过

#### 2. 本次交付

1. 固定 `google.golang.org/adk/v2 v2.2.0`，Agent、Runner 和 Function Tool 分别使用 ADK `llmagent`、`runner` 和 `functiontool`。
2. LLMProvider v2 Schema 保存 Provider 类型、Base URL、模型、上下文窗口、能力和价格，API Token 继续使用独立加密凭据；Model 不登记为资源。
3. OpenAI 原生模式使用 ADK Responses API；项目内 OpenAI-compatible Chat Completions Adapter 支持文本、SSE、usage 和 Tool Calling，不 import 或依赖 `achetronic/adk-utils-go`。
4. 新增不可变 SkillVersion、发布/停用、输入输出 Schema、Tool Schema、工具白名单、风险等级，以及项目 > 团队 > 平台默认解析。
5. Runner 在 ADK 外统一执行 Skill/目标授权、参数 Schema、适用资源、超时、Tool 次数、Token 和输出预算，并固定每次执行的 SkillVersion、Provider 和 Model。
6. Connector 查询只通过 ADK Function Tool 暴露给模型；公共 HTTP API 不开放任意指标、日志、告警或 Kubernetes 查询。
7. 新增模型与 Skill 页面，可测试 Provider、设置默认模型、管理 Skill 版本、执行 Skill 并查看 Token 和 Tool 计数。

#### 3. 已执行验证

##### 3.1 模拟 OpenAI-compatible 与 ADK Runner

```bash
cd backend
go test ./llm ./skill ./httpapi
```

结果：通过。覆盖普通响应、SSE 分片聚合、usage、Function Call 参数、多轮 Tool Calling、输出 Schema、版本和模型固定、完整校验后输出与脱敏存档预览、Tool 调用记录，以及未声明 Tool、非法参数、越权资源和 Tool 预算阻断。被拒绝的工具调用也以 `rejected` 状态、错误码和输入摘要落库；上游错误不会向执行记录写入响应正文。

##### 3.2 真实 SiliconFlow Provider

```bash
make llm-provider-test
```

结果：通过。使用本地 `.env` 中的 SiliconFlow OpenAI-compatible Provider 和 `deepseek-ai/DeepSeek-R1-0528-Qwen3-8B`，经项目 Adapter、ADK `llmagent` 和 ADK Runner 完成非流式与 SSE 请求。非流式累计 280 Token，SSE 累计 246 Token；测试未输出或保存 Token 和模型正文。

##### 3.3 数据库迁移

```bash
cd backend
OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  go test -tags=integration ./migrations
```

结果：通过。隔离 Schema 中验证 0011 `up -> down -> up`、重复 apply、并发迁移锁、五张 T10 表和三个 v2 资源 Schema；测试结束自动删除隔离 Schema。

##### 3.4 完整质量门禁

```bash
cd backend && go test ./...
cd frontend && npm run check && npm run test
```

```bash
make quality
```

结果：通过。包含格式检查、`go vet`、部署脚本语法检查、全部后端单元测试、`svelte-check`（0 错误、0 警告）、前端 6 项测试、生产前端构建、嵌入式 Web 测试，以及 API、Worker、Scheduler、Migrate 和 Admin 五个 Go 二进制构建。

##### 3.5 页面与响应式检查

启动 `make run-front-api` 后，在浏览器访问 `http://localhost:8080/opskeeper/`。

结果：通过。嵌入式页面、静态资源、`/health/ready` 均返回成功；桌面 1280px 和移动 390px 宽度下均无横向溢出，控制台无错误或警告。验收浏览器中没有已登录会话，因此 Provider、Skill 草稿、默认绑定和执行记录的登录后路径由 API、Runner 和集成测试覆盖；未猜测或尝试管理员凭据。

##### 3.6 Playwright 工作台验收

```bash
cd frontend
npm run test:e2e
```

结果：T10 相关用例通过。使用受控 API 夹具验证 Provider/Skill 选择器加载和 Skill 版本显示；T10 的真实 Provider/Runner 行为仍由 ADK、LLM 和 Skill 单元/集成测试验证，夹具不伪造外部模型成功。

#### 4. 用户验收步骤

1. 执行 `make run-front-api`，访问 `http://localhost:8080/opskeeper/`，使用已有管理员登录；完成后以 `Ctrl-C` 停止该命令。
2. 在“资源”页创建或编辑 `LLMProvider`：Provider 类型选 `openai_compatible`，填写 Base URL、模型 JSON 数组，并在 `API Token` 敏感字段填写 Token。确认资源详情只显示“已由加密凭据保存”，不显示 Token。
3. 在“模型与 Skill”页选择 Provider 和模型，点击“测试连接”及“设为当前 Scope 默认”；确认连接成功且模型默认值保存。
4. 创建 `Skill` 资源后，在“创建 Skill 草稿”选择至少一个 Connector 工具，填写输入/输出 JSON Schema，创建并发布版本；再次发布新版后，确认原已发布版本仍可见且不会被隐式停用。
5. 将一个已发布版本设为 Scope 默认，运行一次只读 Skill；确认执行记录包含固定的模型、Token 数和 Tool 数，且无权目标、非法参数或超预算调用被拒绝并产生审计记录。
6. 验收结束后停止验收 API 和临时模拟服务，删除临时账号、资源、凭据、SkillVersion 和执行记录。

#### 5. 本阶段边界

T10 不包含完整诊断会话、证据链、内置中间件 Skill、MCP 调用、自定义代码执行和定期巡检。MCPServer 仅登记 v2 Schema；MCP 运行时按 ADR-0011 在 T14 使用官方 `modelcontextprotocol/go-sdk` 实施。

---

### T11 AI 诊断编排、证据链与对话工作台验收记录

#### 1. 基本信息

- 任务：T11 AI 诊断编排、证据链与对话工作台
- 实施分支：`feat/t11-ai-diagnosis`
- 当前状态：**验收通过**
- 依赖任务：T10 LLM Provider、Skill 注册表与受控 Runner
- 验收日期：2026-08-16
- 用户确认：通过

#### 2. 本次交付

1. 新增诊断会话、目标、消息、计划和阶段步骤、事件、Evidence、假设与报告九张持久化表；迁移支持回滚与重放。
2. 会话固定 Scope、操作者和 1 至 20 个已授权目标资源；创建、读取、追加目标、追问和事件读取均重新执行 Scope 与资源可见性校验。
3. Orchestrator 以原子领取机制将 `queued` 会话推进至 `planning`，避免重复触发并发执行；失败、超时和 Runner 异常都进入终态，不能留下永久运行会话。
4. 诊断过程明确分为 `plan`、`collect`、`verify`、`summarize`，并限制 Tool 调用数、Token、输出大小和总超时；Runner 仍是唯一可调用受控只读 Connector 的边界。
5. Connector 返回的 typed Evidence 被持久化为带来源、能力、采集时刻、查询窗口、SHA-256 内容哈希、部分结果与不可信标记的证据；报告和假设只能引用本会话 Evidence。
6. 有 Evidence 时生成可追溯结论；无 Evidence 时只生成 `needs_verification` 假设和 `warning` 报告，不伪造确定性结论。
7. 新增诊断 REST API 与 SSE，支持 `Last-Event-ID` / `after` 游标恢复，推送计划、阶段、工具完成、证据、报告和失败事件；使用 `http.ResponseController` 刷新和限制流式写入期限。
8. 新增“AI 诊断”工作台，包含图标化目标选择、会话历史、阶段时间线、对话追问、Evidence 抽屉和可跳转的 Evidence 引用；证据仅作为文本/JSON 显示，不执行外部内容。
9. 补齐诊断 API 的 snake_case JSON 契约，并以 HTTP 回归测试固定 `id`、`scope_id` 等前端依赖字段。

#### 3. 已执行验证

##### 3.1 数据库迁移与持久化行为

```bash
make migrate
make backend-integration-test
```

结果：通过。真实 PostgreSQL 隔离 Schema 验证迁移 `up -> down -> up`、诊断 Store 的会话创建与领取、Evidence 哈希持久化、报告引用以及跨会话 Evidence 引用拒绝。

##### 3.2 诊断编排与 HTTP/SSE 行为

```bash
cd backend
go test ./diagnosis ./httpapi ./skill
go test ./...
go vet ./...
```

结果：通过。覆盖目标 Scope/资源授权、已完成会话重新追问、有 Evidence 的可追溯报告、无 Evidence 的待验证假设、Runner 异常终态、重复执行领取拒绝、创建会话的操作者传递、JSON 契约、非法 SSE 游标和 `Last-Event-ID` 增量事件。

##### 3.3 前端与嵌入式制品

```bash
cd frontend
npm run format:check
npm run check
npm run test
npm run build

make backend-embedded-test
```

结果：通过。Svelte 检查为 0 errors / 0 warnings；前端 7 项测试通过；最终前端制品成功嵌入 Go API，并通过嵌入模式 Web UI 与 HTTP 测试。

##### 3.4 已登录浏览器验收

使用临时 PlatformAdmin 在浏览器访问 `http://localhost:8080/opskeeper/`。

结果：通过。

1. 成功登录并看到“AI 诊断”导航入口和工作台。
2. 创建临时 Application 目标后，目标以资源图标、中文类型和 Scope 信息显示在工作台中。
3. 成功创建诊断会话；会话历史、创建时间、失败状态、四阶段计划、原始提问、Evidence 空态和报告错误均正确显示。
4. 当前验收数据库没有默认 Skill，真实运行以 `Skill data not found` 进入 `failed` 终态，验证了缺少运行时前置条件时不会卡住或伪造证据。
5. 对失败会话发送追问后，消息被持久化并重新触发一轮诊断，验证继续追问路径。
6. 未认证调用创建诊断接口返回 `401 invalid_session`。

验收期间发现 Session 等诊断对象缺少显式 snake_case JSON 字段名，导致前端初次创建会话后无法读取 `session.id`。已立即修复并加入 HTTP 回归断言；重新登录浏览器后，会话标题、时间、状态、计划、对话和报告均正常显示。

##### 3.5 Playwright SSE 工作台验收

```bash
cd frontend
npm run test:e2e
```

结果：通过。用例恢复历史诊断会话、打开诊断详情并建立 SSE 事件流；断线后的游标恢复和真实 Connector/Evidence 持久化继续由后端 HTTP/SSE 与诊断集成测试覆盖。

#### 4. 验收环境清理

1. 临时 Application 目标已按资源停用机制软删除，从活动资源目录移除。
2. 临时验收账号已退出、删除 PlatformAdmin 绑定并设为 `disabled`；保留账号记录以保证审计可追溯，但不再具有访问权限。
3. 验收浏览器页面已关闭，验收 API 已停止，`8080` 无遗留监听。

#### 5. 本阶段边界

T11 不实现写操作、MCP、自定义 Skill 代码或周期巡检。缺少默认 Provider、Skill 或 Connector 能力时，诊断会明确失败或降级；T12 开始提供 Kubernetes、PostgreSQL、Redis 和 Kafka 的内置只读诊断 Skill。

---

### T12 Kubernetes 与中间件内置 Skill 验收记录

**验收日期：2026-08-17**
**验收结论：通过**
**验收范围：Kubernetes、PostgreSQL、Redis、Kafka 内置只读诊断 Skill；Connector、ADK Runner 工具约束、迁移、规则、降级、文档与质量门禁。**

#### 1. 验收环境

- 本地 Compose PostgreSQL、Redis；可选 `integration` profile 启动 Redpanda；业务数据库为 `opskeeper`。
- 内置前端制品由 Go API 提供，入口为 `http://localhost:8080/opskeeper/`。
- 验收使用本地 `.env` 配置；未在命令输出、文档或验收结果中记录任何密钥。

#### 2. 验收步骤与结果

| 步骤 | 操作 | 预期结果 | 实际结果 |
|---|---|---|---|
| 1 | 执行 `make migrate` | 内置 Skill 迁移可安全应用或重复执行 | 通过；`0013_builtin_skills` 与 `0014_builtin_skill_output_contract` 均已应用。 |
| 2 | 只读查询内置 Skill 版本 | 四个 Skill 均存在；历史版本不可作为默认执行版本 | 通过；Kubernetes、PostgreSQL、Redis、Kafka 均存在，v1 为 `disabled`，v2 为 `published`。 |
| 3 | 执行 `go test -count=1 -tags=integration ./migrations ./connector` | 验证迁移应用、回滚、再应用及真实中间件快照 | 通过；迁移回滚链和 PostgreSQL/Redis 只读快照均成功。 |
| 4 | 执行 PostgreSQL 固定只读快照 | 可获取连接、版本、会话、长时查询、锁、复制和容量；不执行写 SQL | 通过；真实本地 PostgreSQL 集成测试成功。 |
| 5 | 执行 Redis 固定只读快照 | 可获取连接、内存、客户端、复制、慢命令；热 Key 不可安全读取时明确降级 | 通过；真实本地 Redis 集成测试成功，`hot_keys` 采用显式安全降级。 |
| 6 | 运行 Kubernetes 故障夹具和规则测试 | 覆盖调度、Pod、探针、资源限制和发布状态的主要异常 | 通过；覆盖 Pending、不可调度、未就绪、重启、等待、资源限制、探针、Deployment、StatefulSet、DaemonSet、Job。 |
| 7 | `docker-compose --profile integration up -d redpanda`，执行 `OPSK_TEST_KAFKA_BROKERS=127.0.0.1:19092 go test -tags=integration ./connector -run TestBuiltinKafkaSnapshot` | 真实 Broker、Topic、分区、ISR 和能力降级可验证 | 通过；Redpanda 真实 Broker 快照测试通过，消费组/积压在 ACL、版本或协议限制时写入 `unavailable`。 |
| 8 | 运行 ADK Runner Tool 约束测试 | 模型只能调用 Skill 声明且 Tool Catalog 注册的只读工具 | 通过；PostgreSQL、Redis、Kafka 工具的白名单、参数 Schema、资源范围、调用预算与审计均受测试覆盖。 |
| 9 | 执行 `make quality` | 完整静态检查、单元测试、前端检查/测试、嵌入式前端测试与生产构建通过 | 通过；Go vet、Go 单元测试、Svelte 检查、Vitest、嵌入式前端测试和全部生产二进制构建成功。 |
| 10 | 执行 `make run-front-api` 并访问控制台入口 | 前端制品嵌入 Go API 且登录页可访问 | 通过；`/opskeeper/` 成功显示 OpsKeeper 登录控制台。 |
| 11 | 清理验收资源 | 验收页和临时 API 服务不应遗留 | 通过；临时浏览器页已关闭，未发现本次验收遗留的 API 进程。 |

#### 3. 已验证的实现边界

1. 所有诊断均从已授权资源解析 Connector，不能脱离 Resource、RBAC、Scope、凭据和审计边界直接调用。
2. Connector 只执行预定义的只读协议操作；模型不能构造任意 SQL、Redis 命令、Kafka 管理操作或 Kubernetes API 路径。
3. 确定性规则在模型解释前形成 Finding；模型仅负责基于 Evidence 给出解释、假设、置信度和不执行的建议。
4. 权限不足、统计视图缺失、旧版本 API、超时和协议能力缺失均作为 Connector 错误或 `unavailable` 返回，不误判为业务健康或业务故障。
5. 内置 Skill 已以不可变版本管理；为避免修改已发布版本内容，当前使用 v2，历史 v1 已禁用。

#### 4. 验收范围说明

本阶段不包含自动修复、自动重启、扩缩容、写 SQL、参数修改或任意命令执行。Kubernetes 真实集群不作为本机验收依赖，故障规则以 fake dynamic client 和黄金夹具验证。

当前 Skill 为受控工具和数据库声明的安全基线。完整 Markdown Skill 文档、控制台查看/编辑、Draft 发布、版本差异和细粒度 Tool Catalog 已登记为 [OI-006](../../../backlog.md#oi-006markdown-skill-文档编辑体验与受控工具目录)，不属于 T12 验收范围。

---

### T13 自动巡检、健康评分和通知验收记录

**验收日期：2026-08-17**
**验收结论：通过**
**验收范围：巡检策略、可靠调度与任务租约、确定性健康评分、Finding 生命周期、Webhook 通知、控制台与运行文档。**

#### 1. 验收环境

- 本地 Compose PostgreSQL、Redis；业务数据库为 `opskeeper`。
- API、Scheduler、Worker 读取相同的根目录 `.env` 配置。
- 数据库验证使用临时 schema 或测试创建后清理的夹具；不记录凭据内容。

#### 2. 验收步骤与结果

| 步骤 | 操作 | 预期结果 | 实际结果 |
|---|---|---|---|
| 1 | 应用 `0015_inspection_notification` | 创建策略、运行、任务、Finding、健康快照、渠道和投递表 | 通过；本地 PostgreSQL 已应用迁移，巡检集成测试确认七张核心表存在。 |
| 2 | 执行 `go test -count=1 -tags=integration ./migrations ./inspection` | 迁移链可应用、回滚和重放；巡检持久化正常 | 通过；迁移测试与巡检测试均成功。 |
| 3 | 执行 Job 租约、重试和最终失败测试 | 任务具有租约、指数退避和最终失败状态 | 通过；Worker Store 使用 `SKIP LOCKED`、租约、心跳、重试和幂等键；失败状态经 PostgreSQL 验证。 |
| 4 | 执行 Finding 与健康快照测试 | 评分确定、异常可恢复、空原因保存为 JSON 数组 | 通过；critical/warning 分数为 30，规则消失后 Finding 状态为 `resolved`，空原因写入 `[]`。 |
| 5 | 运行 Webhook Sender 单元测试 | HTTPS、时间戳和 HMAC 签名受验证 | 通过；Webhook 测试覆盖 HTTPS 限制与 `X-OpsKeeper-Signature`。 |
| 6 | 执行涉及 T13 的 Go 单元检查 | API、配置、巡检、Scheduler、Worker 可编译 | 通过；`./inspection`、`./httpapi`、`./config` 均通过；三个命令包完成编译。 |
| 7 | 执行 `npm run check` 和 `npm run build` | 巡检页面类型正确，生产前端可构建 | 通过；`svelte-check found 0 errors and 0 warnings`，Vite 生产构建成功。 |
| 8 | 启动 `make run-front-api`，请求页面、就绪和巡检路由 | 前端制品嵌入 Go API，依赖就绪，巡检路由受认证保护 | 通过；`/opskeeper/` 返回嵌入页面，`/health/ready` 显示 PostgreSQL/Redis `up`，未认证的巡检策略请求返回 `401` 而非 `404`。验收 API 子进程已停止。 |
| 9 | `go test -count=1 -tags=integration ./inspection` | 标签策略只冻结匹配的 active 目标，Worker 按策略并发限制执行内置诊断 | 通过；PostgreSQL 集成测试排除不匹配和停用资源，Worker 按目标并发执行 PostgreSQL/Redis/Kafka 快照并将 Skill 解释结果写入 run steps。 |
| 10 | `cd frontend && npm run test:e2e` | 策略和 Webhook 表单可创建，移动布局无横向溢出 | 通过；Playwright 5 个用例全部通过，包含标签策略、Webhook、登录、Scope 和诊断工作台流程。 |

#### 3. 已验证边界

1. 健康评分只由确定性规则及权重计算；LLM 只解释异常，不能改变分数。
2. LLM 不可用时，确定性巡检仍完成并以 `llm_status=degraded` 明示降级。
3. 调度窗口和投递均有幂等约束；Webhook 使用持久投递记录、签名、限流和退避重试。
4. T13 不包含邮件/短信通知、容量预测和自动变更；Webhook 仅传递通知，不执行处置。
5. 目标快照在运行创建时冻结；策略选择器为显式目标与标签匹配 active 资源的并集，运行期间不会重新解析标签。

---

### T14 MCP、自定义 Skill 沙箱与高风险操作审批验收记录

**验收日期：2026-08-17**
**验收结论：通过**
**验收范围：官方 MCP SDK 接入、MCP 资源与 Scope 隔离、工具快照和不可信内容边界、操作审批状态机、dry-run 与幂等执行、Kubernetes Job 沙箱基线、控制台入口、迁移和审计。**

#### 1. 验收环境

- 本地 PostgreSQL 运行在 `127.0.0.1:5432`；迁移集成测试使用临时 schema，测试结束后清理。
- API 临时运行通过 `make run-front-api` 启动，前端构建制品嵌入 Go API。
- 运行检查结束后已停止 shell、`go run` 和 API 子进程，并确认 `127.0.0.1:8080` 不再监听。
- 验收过程不记录或输出 `.env` 中的凭据和 Token。

#### 2. 验收步骤与结果

| 步骤 | 操作 | 验收结果 |
|---|---|---|
| 1 | 检查依赖与实现边界 | 通过。`backend/go.mod` 直接依赖 `github.com/modelcontextprotocol/go-sdk v1.7.0`；生产代码未 import `achetronic/adk-utils-go`，未自行实现 JSON-RPC。 |
| 2 | `go test -count=1 ./mcp` | 通过。HTTPS/stdio/非法地址被拒绝；官方 SDK 内存传输完成初始化、`tools/list`、`tools/call`，外部恶意文本按原始数据返回。 |
| 3 | `go test -count=1 ./operation ./sandbox ./httpapi` | 通过。覆盖 Scope/Resource Filter、操作目录、Medium/High 审批要求、参数哈希、审批重放、自批、过期、dry-run 计划和沙箱安全默认值。 |
| 4 | `go test -count=1 -tags=integration ./migrations ./operation` | 通过。迁移链包含 0016/0017；隔离 schema 验证迁移应用、重复应用、回滚重放、并发互斥，以及五张 T14 核心表。 |
| 5 | 检查迁移及资源 Schema | 通过。`MCPServer` v3 强制 `streamable_http`、HTTPS URL、非空精确工具白名单、超时和响应大小；操作请求、审批、执行和快照均有外键、状态约束和索引。 |
| 6 | `make quality` | 通过。前端格式、`go vet`、Shell 静态检查、全部 Go 单元测试、前端测试、前端构建、嵌入式 WebUI 测试和五个 Go 制品构建均成功。 |
| 7 | `make run-front-api` 后请求 `/opskeeper/health/live` | 通过。返回 `200` 和 `status=alive`；页面从嵌入式制品提供。 |
| 8 | 请求未认证的 `/opskeeper/api/v1/operation-requests` | 通过。返回 `401`，没有绕过身份认证进入操作服务。 |
| 9 | 浏览器打开 `/opskeeper/` 并检查可访问 DOM | 通过。页面标题为 `OpsKeeper`，登录表单、控制台说明和邮箱/密码控件正常渲染，浏览器控制台无错误。 |
| 10 | 停止验收服务并再次请求健康端点 | 通过。临时服务进程已停止，健康端点连接失败，确认没有遗留验收 API。 |
| 11 | `go test -count=1 ./operation ./sandbox ./cmd/operation-runner` | 批处理 Job 实际生成固定 runner 参数，runner 可执行受控 restart/scale，Job 状态可回写执行记录 | 通过；fake Kubernetes client 验证 Job 创建、独立 ServiceAccount、固定参数、Deployment 重启注解和 reconciler 成功状态回写。 |
| 12 | `helm template opskeeper deploy/helm/opskeeper --set operation.enabled=true` | 启用操作执行时 API/Worker 获得 Job 最小权限，runner 获得工作负载最小权限 | 通过；条件化 ServiceAccount、Role、RoleBinding、环境变量和六个二进制镜像制品均成功渲染。 |

#### 3. 已验证安全边界

1. MCP 调用先检查当前用户的 Scope 和 Resource Filter，再进行网络连接；工具名必须同时满足资源白名单和调用白名单。
2. MCP 工具描述、Schema、响应和错误文本都带 `untrusted` 语义，不能改变权限、工具清单或审批策略。
3. MCP 出口拒绝明文、localhost、私网、回环、链路本地、组播和运行时解析到非公网地址的目标。
4. Medium/High 请求必须由不同用户批准准确参数哈希；参数变化、过期、拒绝、自批和未批准请求均不能执行。
5. 删除、写 SQL 和目录外操作在服务端拒绝；API、Worker 和 Scheduler 没有任意命令执行路径。
6. 自定义代码只生成受限 Kubernetes Job 规格：非 root、只读根文件系统、RuntimeDefault seccomp、禁特权提升、删除全部 capabilities、资源配额、无 Token、零权限 ServiceAccount 和默认拒绝出站网络。
7. 请求、审批、执行、MCP 发现和 Tool 调用均有持久化或审计记录；凭据不进入快照、响应或审计详情。
8. 操作提交通过 `OPSK_OPERATION_SUBMITTER_ENABLED=true` 或 Helm `operation.enabled=true` 显式启用，首版使用 API/Worker 所在集群及发布 namespace 的 in-cluster client；跨 namespace 或远程 kubeconfig Job 提交不在本阶段边界内。

#### 4. 验收结论

T14 验收通过。操作请求已从“仅生成声明式 Job”补全为固定 runner Job 提交、执行状态回写和审计闭环；自定义代码仍使用无 Token、默认拒绝出站的独立沙箱。

---

### T15 生产化、安全加固与端到端验收记录

**记录日期：2026-08-17**
**验收结论：通过**
**验收范围：生产 Helm Chart、OpenTelemetry、安全中间件、生产配置校验、审计保留、跨模块 PostgreSQL E2E、CI 安全门禁和生产运维文档。**

#### 1. 已完成实现

- Helm Chart 包含 API、Worker、Scheduler、Migration Hook、探针、资源限制、PDB、可选 HPA、NetworkPolicy 和非 root 安全上下文。
- API、Worker、Scheduler 和 Migration 支持 OTLP/HTTP；HTTP 链路以及任务、Connector、LLM Token 和错误指标使用受控低基数属性。
- HTTP 边界包含安全响应头、CORS、CSRF、请求体上限和按可信客户端 IP 限流；生产配置拒绝开发数据库/Redis 默认地址、不安全 Cookie 和非 HTTPS Origin。
- 审计敏感字段会被脱敏；审计事件和保留批次禁止普通 UPDATE/DELETE，保留清理要求先完成不可变导出并记录变更单。
- 新增跨模块 PostgreSQL E2E，覆盖团队/项目、Kubernetes 发现导入、资源关系与拓扑、诊断 Evidence/Report、巡检策略、租约和 Finding。
- 已提供生产部署、管理、用户、已知限制以及数据库恢复、凭据轮换、任务恢复、升级回滚文档。

#### 2. 自动化验证结果

| 检查                              | 结果                                                                                                                                        |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `make quality`                  | 通过。包含 Go vet、Go 单元测试、Svelte check、前端测试与生产构建、Shell 检查、Helm lint/render、嵌入式 WebUI 测试和五个 Go 生产二进制构建。 |
| `make backend-integration-test` | 通过。使用隔离 PostgreSQL/Redis 环境，覆盖迁移、审计保留和跨模块 E2E；测试容器、网络和数据卷已清理。                                        |
| `go mod verify`                 | 通过。                                                                                                                                      |
| `go mod tidy -diff`             | 通过，模块文件无额外差异。                                                                                                                  |
| Helm 静态验证                     | 通过。`helm lint` 和默认 values 渲染成功。                                                                                                |

#### 3. 后续生产环境验证事项

1. 尚未在干净 Kubernetes 集群执行 `helm upgrade --install`、Migration Hook、Pod 就绪、NetworkPolicy、PDB/HPA 和升级回滚验证。本机 kubeconfig 指向非本任务验收集群，实施过程未访问或修改该集群。
2. GitHub Actions 工作流已入库，但需要推送后确认托管运行结果，并由仓库管理员将质量与安全检查设为 `main` 分支必需检查。
3. 依赖与镜像联网扫描需要在 GitHub Actions 或具备 registry 网络访问的环境形成最终报告；本地 `npm audit` 因当前沙箱无法访问 npm registry，未作为验收证据。
4. 典型生产规模下的容量、延迟、故障恢复目标以及 PostgreSQL PITR 必须结合实际集群、数据库和外部服务完成演练，仓库内自动化测试不能替代环境验收。

#### 4. 验收结论

T15 的代码、静态部署验证、本地质量门禁和隔离数据库 E2E 已完成，用户已于 2026-08-17 确认验收通过，当前状态为 `已完成`。干净 Kubernetes 环境部署、托管 CI、安全扫描、容量与恢复演练继续作为生产上线前的环境验证事项跟踪，不影响本阶段验收结论。

## 4. 需求级遗留事项

- 生产 Kubernetes 部署、托管 CI、安全扫描、容量评估和恢复演练仍需在对应外部环境执行，详见 [项目 backlog](../../../backlog.md) 和 [已知限制](../../../known-limitations.md)。
- 后续迭代不得直接修改本报告；新增能力应建立新的迭代和需求文档。

## 5. 封板信息

- 需求状态：已完成
- 迭代状态：已封板
- 验收方式：任务级自动化验证与用户确认
- 归档位置：`docs/iterations/archived/I001-initial/`
