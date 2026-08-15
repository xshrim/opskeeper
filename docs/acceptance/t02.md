# T02 Scope 与组织模型验收记录

## 1. 验收结论

- 验收日期：2026-08-15
- 验收分支：`feat/t02-organization`
- 基线提交：`1e3a4bd27966b17cebd25703977e8ab65aa77d8e`
- 验收结论：**通过**
- 阻塞问题：无

T02 的数据库初始化、权限分离、迁移、三级 Scope/组织模型、事务化业务操作、HTTP API、异常映射和测试均符合验收标准。T03 身份认证与三级 RBAC 不属于本次验收范围。

## 2. 验收环境

| 项目 | 实际环境 |
|---|---|
| Go | `go1.26.5-X:nodwarf5 linux/amd64` |
| Node.js | `v26.5.1` |
| npm | `12.0.2` |
| PostgreSQL | `postgres:16-alpine` |
| Redis | `redis:7-alpine` |
| API 验收地址 | `127.0.0.1:58080` |
| PostgreSQL 验收地址 | `127.0.0.1:5432` |
| Redis 验收地址 | `127.0.0.1:6379` |

本机没有安装 Docker Compose 插件，`make dev-services-up` 正确返回 `Docker Compose is not installed`。因此本次使用开发文档中与 Compose 等价的临时 `docker run` 容器进行运行验收；Compose YAML 由 Prettier 成功解析，初始化和健康检查 Shell 脚本通过 `sh -n`，容器中的实际角色、数据库和权限由 SQL 单独核验。该环境限制不影响 T02 业务验收结论，但本次未实际执行 `docker compose up`。

## 3. 逐项验收结果

### 步骤 1：分支与工作区确认

执行：

```bash
git branch --show-current
git status --short
```

结果：

- 当前分支为 `feat/t02-organization`。
- T02 修改均位于任务分支工作区。
- `main` 与 `origin/main` 均保持在基线提交，没有提前合并。

结论：**通过**。

### 步骤 2：配置与中间件启动

使用 `.env.example` 和 `deploy/compose/.env.example` 的默认开发值生成本地忽略配置，随后尝试：

```bash
make dev-services-up
```

实际结果：本机缺少 Docker Compose，Makefile 返回明确错误。改用等价临时容器启动 PostgreSQL 16 和 Redis 7，端口分别为 `5432` 和 `6379`。`pg_isready` 返回：

```text
/var/run/postgresql:5432 - accepting connections
```

`docker ps` 显示两个容器均处于 `Up` 状态。API 后续就绪检查确认 PostgreSQL 和 Redis 均为 `up`。

结论：**通过，存在非阻塞环境说明**。

### 步骤 3：PostgreSQL 管理员与业务角色分离

查询角色属性，实际结果：

```text
opskeeper|false|false|false|false|false
postgres|true|true|true|true|true
```

字段依次为 `rolsuper`、`rolcreatedb`、`rolcreaterole`、`rolreplication`、`rolbypassrls`。

查询数据库所有权，实际结果：

```text
opskeeper|opskeeper
postgres|postgres
```

使用 `opskeeper` 通过 TCP 连接业务数据库后，在事务中成功创建表并回滚：

```text
BEGIN
CREATE TABLE
ROLLBACK
opskeeper|opskeeper
```

使用同一业务用户在事务中尝试创建角色，PostgreSQL 返回：

```text
ERROR: permission denied to create role
DETAIL: Only roles with the CREATEROLE attribute may create roles.
```

结论：**通过**。`postgres` 保持 Cluster 超级用户，`opskeeper` 能管理自身业务库但不具备集群级角色管理权限。

### 步骤 4：首次迁移与幂等执行

以 `opskeeper` 业务连接连续执行两次迁移：

```bash
go run ./cmd/migrate
go run ./cmd/migrate
```

两次均返回：

```text
migration command completed direction=up
```

数据库检查结果：

```text
1|scope_organization
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
default|active|platform|NULL
```

结论：**通过**。首次迁移成功，重复执行没有重复记录或对象，默认 Platform Scope 无父节点。

### 步骤 5：完整质量检查

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
- PostgreSQL 初始化和健康检查脚本通过 `sh -n`。
- Go 单元测试全部通过。
- Vitest：`1 passed`。
- Go 全量构建通过。
- Vite 生产构建通过，共转换 `110 modules`。

结论：**通过**。

### 步骤 6：真实 PostgreSQL 集成测试

执行：

```bash
OPSK_TEST_DATABASE_URL='postgres://opskeeper:opskeeper@127.0.0.1:5432/opskeeper?sslmode=disable' \
  go test -count=1 -v -tags=integration ./internal/organization
```

实际通过的测试：

```text
TestCreateTeamNormalizesInput
TestCreateTeamRejectsInvalidCode
TestListTeamsAppliesPaginationDefaults
TestCreateProjectDefaultsSource
TestUpdateTeamRequiresAtLeastOneField
TestOrganizationLifecycle
TestConcurrentTeamCodeIsUnique
TestDatabaseRejectsIllegalScopeHierarchy
TestMigrationRollback
```

最终结果：`PASS`，耗时 `0.919s`。每项数据库测试使用独立临时 Schema，结束后自动删除。

结论：**通过**。

### 步骤 7：API 存活、就绪与默认平台

启动 API 后检查三个端点，实际结果：

| 请求 | HTTP 状态 | 关键结果 |
|---|---:|---|
| `GET /health/live` | 200 | `status=alive` |
| `GET /health/ready` | 200 | PostgreSQL 与 Redis 均为 `up` |
| `GET /api/v1/platform` | 200 | `code=default`、`scope.type=platform`、无父 Scope |

平台 ID 为 `9df14408-d920-40dd-8b43-2d995c68e45d`，平台 Scope ID 为 `d8eb61ec-91c5-4551-8c24-4763fb990bb5`。

结论：**通过**。

### 步骤 8：创建团队及 Scope 关系

执行 `POST /api/v1/teams`，请求包含名称、编码和标签。

实际结果：`201 Created`。创建结果：

```text
team.id=44de8441-64db-4100-ada9-411bba8b11a8
team.code=t02-acceptance-team-20260815
team.scope.id=15adf985-cecc-4052-a427-22ca28ca5b41
team.scope.type=team
team.scope.parent_id=d8eb61ec-91c5-4551-8c24-4763fb990bb5
team.status=active
```

团队 Scope 的 `parent_id` 与平台 Scope ID 一致。

结论：**通过**。

### 步骤 9：创建项目及 Scope 关系

执行 `POST /api/v1/teams/{teamId}/projects`。

实际结果：`201 Created`。创建结果：

```text
project.id=46f45d69-3f54-4aa6-b8bc-0847f2bbcc54
project.code=t02-acceptance-project-20260815
project.team_id=44de8441-64db-4100-ada9-411bba8b11a8
project.scope.id=947d99f6-0696-46b2-a78f-ce8d8543e0e8
project.scope.type=project
project.scope.parent_id=15adf985-cecc-4052-a427-22ca28ca5b41
project.source=manual
project.status=active
```

项目 Scope 的 `parent_id` 与团队 Scope ID 一致，默认来源为 `manual`。

结论：**通过**。

### 步骤 10：详情、列表与分页

实际结果：

| 请求 | HTTP 状态 | 关键结果 |
|---|---:|---|
| `GET /api/v1/teams/{teamId}` | 200 | 返回已创建团队 |
| `GET /api/v1/projects/{projectId}` | 200 | 返回已创建项目 |
| `GET /api/v1/teams?page=1&page_size=1` | 200 | `page=1`、`page_size=1`、`total=1` |
| `GET /api/v1/teams/{teamId}/projects?page=1&page_size=1` | 200 | `page=1`、`page_size=1`、`total=1` |

结论：**通过**。

### 步骤 11：团队和项目更新

更新项目名称和标签，实际返回 `200`，结果为：

```text
name=T02 Acceptance Project Updated
labels={"verified":"true"}
```

更新团队名称和标签，实际返回 `200`，结果为：

```text
name=T02 Acceptance Team Updated
labels={"purpose":"t02-acceptance","verified":"true"}
```

两个对象的 `updated_at` 均发生变化，编码和 Scope 关系保持不变。

结论：**通过**。

### 步骤 12：输入校验与错误映射

| 场景 | 实际状态 | 实际错误码 | 预期 |
|---|---:|---|---|
| `page=0` | 400 | `invalid_request` | 400 |
| 空项目更新 `{}` | 400 | `invalid_request` | 400 |
| 创建团队包含未知字段 | 400 | `invalid_json` | 400 |
| 非法团队 ID `not-a-uuid` | 400 | `invalid_request` | 400 |
| 查询不存在的合法团队 UUID | 404 | `not_found` | 404 |
| 重复团队编码 | 409 | `conflict` | 409 |

所有错误响应均包含结构化 `error.code`、`error.message` 和 `request_id`。

结论：**通过**。

### 步骤 13：停用上级组织与历史数据保留

将验收团队更新为 `disabled`，API 返回 `200`，团队记录和团队 Scope 状态均变为 `disabled`。

随后在该团队下创建新项目，实际返回：

```text
HTTP 409
error.code=parent_inactive
error.message=Parent organization is inactive
```

再次查询停用前创建的项目，返回 `200`，项目和项目 Scope 仍为 `active`。数据库终态为：

```text
t02-acceptance-team-20260815|disabled|team|disabled|t02-acceptance-project-20260815|active|project|active
```

查询被拒绝的新项目记录数为 `0`。

结论：**通过**。上级停用能够阻止新增下级，同时保留历史项目。

### 步骤 14：API 日志与资源清理

API 共记录 20 个验收请求，状态码与上述结果一致，没有非预期 `5xx`。验收结束后：

- API 进程已停止。
- 临时 PostgreSQL 容器已删除。
- 临时 Redis 容器已删除。
- `docker ps` 不再显示本次验收容器。
- 本地 `.env`、`deploy/compose/.env`、前端依赖和构建产物均受 `.gitignore` 管理。

临时容器和其中的验收数据已删除，不可恢复；没有删除仓库文件或持久化开发数据卷。

结论：**通过**。

## 4. 验收标准映射

| T02 验收标准 | 验收证据 | 结果 |
|---|---|---|
| 创建团队和项目并得到正确三层 Scope 路径 | 步骤 7、8、9 | 通过 |
| 非法父子层级被数据库拒绝 | `TestDatabaseRejectsIllegalScopeHierarchy` | 通过 |
| 重复编码被拒绝 | 并发集成测试及 HTTP `409 conflict` | 通过 |
| 停用上级后阻止新增下级并保留历史 | 步骤 13 | 通过 |
| 迁移可在空数据库执行并支持回滚 | 步骤 4、`TestMigrationRollback` | 通过 |
| 组织模块测试通过 | 步骤 5、6 | 通过 |
| 管理员和业务角色权限分离 | 步骤 3 | 通过 |

## 5. 非阻塞说明

1. 本机缺少 Docker Compose 插件，因此本次未执行 `docker compose up`；使用等价容器完成实际运行验证，并对 Compose YAML 和 Shell 脚本进行了静态检查。
2. 沙箱中的默认 Go 构建缓存和 Module 缓存目录只读，验收命令改用 `/tmp/opskeeper-gocache` 和 `/tmp/opskeeper-gomodcache`；该调整不改变代码或运行行为。
3. T02 按任务范围只提供无认证组织 API。身份认证、Scope RBAC 和前端组织管理页面分别属于后续任务，不构成本次遗留缺陷。
