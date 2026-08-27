# 自动化发布

本文定义应用制品、镜像、数据库迁移和自动化发布的统一流程。本地命令参见[本地开发](development.md)，Git 门禁参见[Git 与远端仓库](../standards/version-control.md)。

## 1. 发布原则

1. 应用临时运行、二进制构建和最终镜像打包只通过根目录 `Makefile` 暴露和编排，不使用独立包装脚本。
2. 前后端源码保持独立；生产构建将 Vite 制品嵌入 Go API。
3. API、Worker、Scheduler、Migration 和 Admin 使用同一个不可变镜像及同一个 digest。
4. 长期运行的应用进程永不自动执行数据库迁移。
5. 每次部署先运行一个独立 Migration Job；成功后才能滚动发布应用。
6. 迁移失败立即阻断发布；应用回滚不自动执行数据库 `down`。
7. Schema 变更遵循 Expand/Contract，保证滚动发布期间新旧版本兼容。
8. 构建阶段不接触数据库凭据；部署阶段按进程最小权限注入。

## 2. 标准入口与制品

流水线只调用以下 Make 入口：

```bash
make quality
make build VERSION=<version> COMMIT=<commit> BUILD_TIME=<UTC timestamp>
make image IMAGE_REPOSITORY=<registry>/opskeeper IMAGE_TAG=<version> VERSION=<version> COMMIT=<commit> BUILD_TIME=<UTC timestamp>
```

| 入口 | 结果 |
|---|---|
| `make quality` | 格式、静态检查、测试、嵌入式前端验证和生产构建全部通过 |
| `make build` | 在 `backend/bin/` 生成五个本地二进制 |
| `make image` | 使用 `deploy/Dockerfile` 生成最终不可变镜像 |

`deploy/Dockerfile` 是由 `make image` 调用的镜像构建描述，不作为开发者或流水线的独立操作入口。Node Builder 调用 Make 的前端构建目标，Go Builder 接收前端制品后调用 Make 的后端构建目标；Dockerfile 不重复维护 npm 构建命令、Go Build Tag、版本注入或二进制清单。不得增加脚本来包装 Go、npm、Make 或 Docker 命令。

本地 `make front-api-run` 使用相同的嵌入链路：先将 Vite 制品从 `frontend/dist` 复制到 `backend/webui/dist`，再通过 `embed_webui` 标签和 `go:embed` 将其编译进临时 API，最终只启动一个同时提供前后端服务的 Go 进程。

最终镜像包含：

```text
/app/opskeeper-api
/app/opskeeper-worker
/app/opskeeper-scheduler
/app/opskeeper-migrate
/app/opskeeper-admin
```

`opskeeper-api` 内嵌经过构建和检查的 Vite 制品。最终镜像不包含 Node.js，不依赖独立静态文件服务器，也不读取构建机器的 `.env`。

## 3. 构建与运行配置边界

| 配置 | 阶段 | 作用 |
|---|---|---|
| `IMAGE_REPOSITORY` | `make image` | 设置镜像仓库，默认 `opskeeper` |
| `IMAGE_TAG` | `make image` | 设置镜像标签，默认 `local` |
| `GOPROXY` | 构建 | 设置 Go 依赖代理链，不属于应用运行配置；默认在 `goproxy.cn`、`proxy.golang.org` 不可用时自动故障转移，可通过 `make GOPROXY=...` 覆盖 |
| `ALPINE_MIRROR` | 镜像构建 | 设置 Builder 的 Alpine 软件源，默认 `https://mirrors.aliyun.com/alpine` |
| `NPM_REGISTRY` | 镜像构建 | 设置 Builder 的 npm Registry，默认 `https://registry.npmmirror.com` |
| `VERSION` | 构建 | 应用版本；默认使用当前 Git 描述，流水线应显式传入发布版本 |
| `COMMIT` | 构建 | 完整 Git Commit；默认读取当前工作树 HEAD |
| `BUILD_TIME` | 构建 | UTC 构建时间；流水线应传入 RFC 3339 时间 |
| `OPSK_BASE_PATH` | 运行 | 设置 API 的 HTTP 路径前缀，默认 `/opskeeper`，根路径使用 `/` |
| `OPSK_LOG_FORMAT` | 运行 | 设置全部 Go 应用日志为 `json`、`text` 或 `raw`，默认 `raw`；详见[后端日志规范](../standards/backend-logging.md) |
| `OPSK_LOG_HEALTH_IGNORE` | API 运行 | 是否忽略 `/health/live`、`/health/ready` 的访问日志，默认 `true` |
| `OPSK_TRUSTED_PROXIES` | API 运行 | 允许提供客户端转发头的直接代理 IP/CIDR；默认空，不信任任何代理头 |
| `OPSK_CREDENTIAL_KEY` | API/Worker 运行 | 资源凭据密文加密密钥；生产环境必须设置为 32 字节原值或 Base64 编码值 |

镜像内文件名和五个应用的服务名称固定为：

```text
opskeeper-api
opskeeper-worker
opskeeper-scheduler
opskeeper-migrate
opskeeper-admin
```

`VERSION`、`COMMIT` 和 `BUILD_TIME` 通过 Go `ldflags` 注入五个二进制，API 健康响应可以返回这三个字段。普通日志遵循后端日志规范，不把这些发布元数据放入固定日志头；需要关联发布版本时使用部署标签、指标或日志平台元数据。相同源码需要字节级可复现构建时，流水线必须复用固定的 `BUILD_TIME`。

`OPSK_BASE_PATH` 只改变 API 页面、静态资源、健康检查和业务接口的路径。Ingress Path、健康探针路径和 API 容器中的 `OPSK_BASE_PATH` 必须来自同一个发布配置。生产环境通常设置 `OPSK_LOG_FORMAT=json` 供日志平台解析；不设置时输出默认的 `raw` 日志。`OPSK_LOG_HEALTH_IGNORE` 默认 `true`，用于忽略 API 健康检查访问日志；设为 `false` 可在排障时恢复记录，接口本身和健康探针不受影响。

## 4. 流水线顺序

```text
检出确定提交
    -> make quality
    -> make image
    -> 推送镜像并记录 digest
    -> 使用同一 digest 创建 Migration Job
    -> 等待 Migration Job 成功
    -> 滚动发布 API、Worker、Scheduler
    -> 等待 readiness
    -> 执行关键业务冒烟检查
    -> 记录发布结果
```

Migration Job 和应用必须引用同一镜像 digest，确保迁移 SQL 与应用代码完全一致。不得使用 `latest`，也不得从工作区临时挂载 SQL。

| 结果 | 流水线行为 |
|---|---|
| Migration Job 成功 | 继续发布应用 |
| Migration Job 失败或超时 | 停止发布，保留旧应用并输出 Job 日志 |
| 迁移成功但应用发布失败 | 回滚应用镜像，不执行 `down` |
| 冒烟检查失败 | 停止扩大发布，回滚应用或前滚修复 |

## 5. Migration Job

每次测试、预发布和生产部署固定执行一次 `opskeeper-migrate up`。即使没有待处理迁移也照常运行，因为迁移器读取 `schema_migrations` 并跳过已应用版本，外部流水线无需解析 SQL 判断是否需要迁移。当前发布基线为单一的 `0001_initial`；历史迁移仅存档，不会被新迁移器自动读取，已有历史数据库必须在发布前完成受控重建或专项数据迁移。

迁移器使用 PostgreSQL session advisory lock 保护整个迁移过程：

1. 获取一条固定连接并等待项目迁移锁。
2. 创建或复用 `schema_migrations`，校验已执行迁移的名称和前滚 SQL SHA-256。
3. 按版本升序跳过已执行且校验一致的迁移。
4. 在独立事务中执行每条待处理 SQL 和版本记录。
5. 失败时回滚当前事务并返回非零状态。
6. 完成后在同一连接释放锁；释放失败时销毁该连接。

advisory lock 是并发误配置的最后防线，不代替发布编排。每个环境和版本仍只应创建一个逻辑 Migration Job。Job 必须监听取消信号并设置总超时、有限重试和可审计日志。

Kubernetes 约束：

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: opskeeper-migrate-<release-id>
spec:
  backoffLimit: 1
  activeDeadlineSeconds: 600
  ttlSecondsAfterFinished: 86400
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: <registry>/opskeeper@sha256:<digest>
          command: ["/app/opskeeper-migrate", "up"]
          env:
            - name: OPSK_LOG_FORMAT
              value: json
          envFrom:
            - secretRef:
                name: opskeeper-database-migration
```

发布系统必须等待 Job 的 `Complete` 条件，而不是只等待 Pod 启动。可以使用 Helm `pre-install,pre-upgrade` Hook 或 Argo CD `PreSync` Hook，但不得把迁移放入每个应用 Pod 的 Init Container。

## 6. CI 中的迁移验证

涉及数据库变更的 Pull Request 至少验证：

```text
创建空 PostgreSQL 16
    -> up
    -> 再次 up，验证幂等
    -> down
    -> up
    -> 并发执行两个 up，验证互斥和最终版本
    -> 数据库集成测试
```

本地等价入口：

```bash
OPSK_TEST_DATABASE_URL='postgres://<user>:<password>@<host>:<port>/<database>?sslmode=disable' \
  make backend-integration-test
```

测试必须使用可丢弃数据库、Schema 或容器，并清理测试对象。

## 7. Expand/Contract

滚动发布期间新旧应用会同时访问数据库，破坏性 Schema 变更必须拆分：

1. **Expand**：增加新表、新字段、兼容索引或宽松约束，不删除旧结构。
2. **Migrate**：发布兼容新旧结构的应用，执行双写、读切换或后台回填。
3. **Contract**：旧版本全部退出并经过观测期后，在后续发布删除旧结构。

具体要求：

- 新增必填字段时先允许空值或提供兼容默认值，回填后再加 `NOT NULL`。
- 字段重命名使用“新增、双写、切读、删除旧字段”，不直接改名。
- 表、字段、枚举值删除必须放到后续独立发布。
- 大表索引、约束和回填必须评估锁级别、执行时间及事务限制。
- 当前迁移器按单迁移事务执行；需要 `CREATE INDEX CONCURRENTLY` 时，应先扩展迁移格式并增加显式非事务标记。

## 8. 权限与凭据

| 角色 | 权限和使用方 |
|---|---|
| Cluster 管理员 | 管理实例、角色和数据库，不注入普通流水线和应用 |
| Migration 角色 | 拥有目标 Schema DDL 权限，只注入 Migration Job |
| Application 角色 | 只拥有运行所需查询和 DML 权限，注入 API、Worker、Scheduler |

当前阶段迁移器和应用使用受限的 `opskeeper` 业务数据库所有者，不使用 `postgres` 超级用户。生产环境应继续拆分 Migration 和 Application 角色。构建阶段不得获得任何数据库、Redis 或其他运行时 Secret。

## 9. 回滚

- 应用回滚只切换到上一个兼容镜像，不自动执行 `migrate down`。
- 迁移事务失败时，修复迁移后重新构建和发布。
- 迁移成功后发现问题，优先新增前滚修复迁移。
- `migrate down` 只用于本地开发、迁移测试和经过人工批准的恢复操作。
- 破坏性迁移执行前必须确认备份、PITR、影响范围和人工恢复步骤。

## 10. 发布验收

- [ ] 流水线只通过 Makefile 运行、构建和打包应用。
- [ ] `make quality` 通过。
- [ ] 镜像包含五个固定名称的二进制，API 已嵌入前端。
- [ ] 运行时不依赖 Node.js 或外部静态文件目录。
- [ ] `OPSK_BASE_PATH` 与 Ingress 和健康探针路径一致。
- [ ] `OPSK_LOG_FORMAT` 已按环境设为 `json`、`text` 或 `raw`，并符合[后端日志规范](../standards/backend-logging.md)。
- [ ] Migration Job 与应用使用同一镜像 digest。
- [ ] Migration Job 成功后才滚动发布应用，失败会阻断发布。
- [ ] 数据库迁移通过幂等、回滚、重放和并发互斥验证。
- [ ] Schema 在新旧版本并存期间兼容。
- [ ] 应用回滚不会触发数据库 `down`。
- [ ] 应用和流水线未获得 PostgreSQL 超级用户凭据。
