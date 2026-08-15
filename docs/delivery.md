# 数据库与应用自动化发布

## 1. 文档目的

本文定义 OpsKeeper 数据库迁移在本地开发、持续集成和自动化发布中的统一原则，并说明应用发布顺序、失败处理和权限边界。迁移文件格式与本地命令参见[本地开发环境](development.md)，Git 分支和合并门禁参见[Git 与远端仓库](version-control.md)。

## 2. 最终原则

数据库迁移必须遵守以下原则：

1. API、Worker 和 Scheduler 等长期运行的应用进程永不自动执行迁移。
2. 每次部署由一个独立、一次性的 Migration Job 执行 `up`；即使当前版本没有待处理迁移，也允许幂等执行。
3. Migration Job 成功后才能滚动发布应用；迁移失败立即阻断发布，旧应用继续运行。
4. 迁移器使用 PostgreSQL advisory lock 对整个 `up` 或 `down` 过程加数据库级互斥，流水线仍应只创建一个逻辑迁移任务。
5. 应用回滚不自动执行 `down`；生产数据库问题优先通过新的前滚迁移修复。
6. 滚动发布中的 Schema 变更遵循 Expand/Contract，保证新旧应用版本并存时兼容。
7. 构建阶段不接触数据库凭据；迁移凭据只在部署环境注入 Migration Job。

这意味着“每次发布执行一次迁移步骤”，而不是“每次启动一个应用实例都执行一次迁移”。

## 3. 什么时候运行迁移

| 场景 | 是否运行 | 执行者 |
|---|---|---|
| 本地首次初始化 | 必须 | 开发者执行 `make migrate` |
| 拉取包含新迁移的代码 | 必须 | 开发者在启动新代码前执行 |
| 日常重启且代码无新迁移 | 不必 | 无 |
| 持续集成 | 必须 | 临时 PostgreSQL 上执行迁移测试 |
| 测试、预发布和生产部署 | 每次部署固定执行 | 单实例 Migration Job |
| API Pod 重启或扩容 | 禁止触发 | 应用只启动自身进程 |

流水线不需要在外部分析 SQL 文件来判断是否存在新迁移。迁移器读取 `schema_migrations`，跳过已应用版本，因此每次部署固定调用一次更加可靠。

## 4. 当前迁移器机制

迁移入口是 `backend/cmd/migrate`，迁移实现位于 `backend/migrations`，SQL 使用 `go:embed` 编译进迁移二进制。

执行 `up` 时：

1. 从连接池获取一条固定 PostgreSQL 连接。
2. 在该会话上等待并持有项目固定的 session advisory lock。
3. 创建或复用 `schema_migrations(version, name, applied_at)`。
4. 按版本升序检查迁移，已经记录的版本直接跳过。
5. 每条待执行迁移使用独立事务执行 SQL 并写入版本记录。
6. 任一步骤失败时回滚当前迁移并返回非零退出状态。
7. 全部完成后在同一连接上释放 advisory lock，再将连接归还连接池。

执行 `down` 时使用同一把 advisory lock，一次只回滚最新版本。释放锁失败时迁移器会销毁持锁连接，不会把可能仍持有 session lock 的连接放回池中。

迁移命令监听 `SIGINT` 和 `SIGTERM`。流水线取消 Job 或达到执行期限时，等待锁、SQL 和事务会收到 Context 取消信号。Migration Job 必须设置外部总超时，避免数据库锁竞争或长时间 DDL 造成无限等待。

advisory lock 是并发误配置的最后防线，不替代发布编排。正常情况下同一环境、同一版本仍然只创建一个 Migration Job。

## 5. 持续集成流程

持续集成只访问临时数据库，不持有测试、预发布或生产凭据。涉及迁移的 Pull Request 至少验证：

```text
创建空 PostgreSQL 16 环境
    -> up
    -> 再次 up，验证幂等
    -> down
    -> up
    -> 并发运行两个 up，验证互斥和最终版本
    -> 运行数据库集成测试
```

当前本地入口为：

```bash
OPSK_TEST_DATABASE_URL='postgres://<user>:<password>@<host>:<port>/<database>?sslmode=disable' \
  make backend-integration-test
```

测试必须使用可丢弃的数据库、Schema 或容器，结束后清理测试对象。

## 6. 自动化发布流程

推荐的持续部署顺序为：

```text
质量检查
    -> 构建不可变镜像
    -> 推送镜像并确定 digest
    -> 在目标环境创建 Migration Job
    -> 等待 Job 成功
    -> 滚动发布 API、Worker、Scheduler
    -> 检查 readiness 和关键业务冒烟用例
    -> 标记发布成功
```

Migration Job 必须使用与待发布应用相同提交构建的镜像和确定的镜像 digest，确保二进制中嵌入的迁移与应用代码完全一致。不能使用 `latest` 或从工作区临时挂载 SQL。

流水线的行为必须是：

| 结果 | 流水线行为 |
|---|---|
| Migration Job 成功 | 继续滚动发布应用 |
| Migration Job 失败或超时 | 停止发布，保留旧应用，输出 Job 日志 |
| 迁移成功但应用发布失败 | 回滚应用镜像，不自动执行 `down` |
| 冒烟测试失败 | 停止扩大发布，按兼容性策略回滚应用或前滚修复 |

## 7. Kubernetes Migration Job 约束

T13 实现容器镜像和 Helm Chart 时，应提供一次性 Kubernetes Job。以下是结构模板，具体镜像路径、Secret 名和资源配置由部署环境确定：

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: opskeeper-migrate-<release-id>
  labels:
    app.kubernetes.io/name: opskeeper
    app.kubernetes.io/component: migration
    app.kubernetes.io/version: <version>
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
          command: ["/app/migrate", "up"]
          envFrom:
            - secretRef:
                name: opskeeper-database-migration
```

发布系统应等待 Job 的 `Complete` 条件，不能只等待 Pod 启动。Job 名包含发布 ID，日志和执行结果保留到发布审计中。

使用 Helm 时可以通过 `pre-install,pre-upgrade` Hook 实现；使用 Argo CD 时可以使用 `PreSync` Hook。但无论采用哪种工具，迁移都必须作为可观察的独立发布阶段，不能放到每个 Pod 的 Init Container 中。Helm 或应用自动回滚不会自动撤销已经提交的数据库迁移。

## 8. Expand/Contract 兼容发布

滚动发布期间旧应用和新应用会同时访问数据库，因此单次发布不得立即破坏旧版本使用的 Schema。

推荐三阶段：

1. **Expand**：先增加新表、新字段、兼容索引或宽松约束，不删除旧结构。
2. **Migrate**：发布同时兼容新旧结构的应用，完成双写、读切换或后台数据回填。
3. **Contract**：确认旧版本全部退出、数据回填完成并经过观测周期后，在后续独立迁移中删除旧字段、旧索引或旧约束。

典型规则：

- 新增必填字段时先允许空值或提供兼容默认值，回填后再增加 `NOT NULL`。
- 重命名字段使用“新增、双写、切读、删除旧字段”，不能直接改名后立即滚动发布。
- 删除表、字段和枚举值属于破坏性变更，必须在后续独立发布中执行。
- 大表索引、约束验证和数据回填需要评估锁级别、执行时间和事务限制。
- 当前迁移器按单迁移事务执行；未来需要 `CREATE INDEX CONCURRENTLY` 等不能位于事务中的语句时，必须先扩展迁移格式并增加显式的非事务标记，不能直接把特殊 SQL 塞入现有迁移。

## 9. 权限与凭据

当前环境已经将 PostgreSQL Cluster 超级用户与 `opskeeper` 业务数据库所有者分离。迁移命令只使用业务数据库凭据，不使用 `postgres` 超级用户。

生产加固时建议进一步拆分：

| 角色 | 权限和使用方 |
|---|---|
| Cluster 管理员 | 管理实例、角色和数据库；不注入普通流水线和应用 |
| Migration 角色 | 拥有目标 Schema 的 DDL 权限；只注入 Migration Job |
| Application 角色 | 只拥有运行所需的查询和 DML 权限；注入 API、Worker、Scheduler |

在独立 Migration/Application 角色正式实施前，流水线可以继续使用受限业务数据库所有者执行迁移，但应用和迁移都不得回退使用超级用户。

## 10. 回滚原则

- `migrate down` 主要用于本地开发、迁移测试和部署前验证。
- 生产发布不得因为应用回滚而自动执行 `down`。
- 已经写入新格式数据后，回滚 Schema 可能造成不可恢复的数据丢失。
- 生产迁移失败且事务已经回滚时，修复 SQL 后重新构建和发布。
- 迁移已经成功但发现设计问题时，优先新增前滚修复迁移。
- 破坏性迁移执行前必须确认备份、PITR、影响范围和人工恢复步骤。

## 11. 发布验收清单

- [ ] 应用入口不调用迁移器。
- [ ] 发布使用同一提交构建的不可变镜像 digest。
- [ ] Migration Job 是滚动发布前的独立阶段。
- [ ] Job 设置总超时、有限重试和结构化日志。
- [ ] Job 失败会阻断应用发布。
- [ ] 数据库 advisory lock 和并发迁移测试通过。
- [ ] 迁移在空数据库通过 `up -> down -> up`。
- [ ] 新旧应用版本在 Expand/Contract 窗口内兼容。
- [ ] 应用回滚不会自动触发数据库 `down`。
- [ ] 流水线和应用均未获得 PostgreSQL 超级用户凭据。
- [ ] 破坏性变更具有备份、PITR 和前滚修复方案。
