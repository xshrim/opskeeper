# 部署指南

## 1. 制品

使用根目录 Makefile 构建，Compose 不负责 build：

```bash
make quality
make image IMAGE_REPOSITORY=registry.example.com/opskeeper IMAGE_TAG=<version>
```

生成五个同版本镜像：

```text
opskeeper-api        # 非嵌入前端的 API
opskeeper-worker     # 巡检任务执行器
opskeeper-scheduler  # 巡检调度器
opskeeper-frontend   # Nginx 静态前端
opskeeper-app        # 嵌入前端的单进程服务
```

发布应记录每个镜像 digest，不使用 `latest`。构建阶段不读取运行时 Secret。

## 2. Compose

准备根目录 `.env`，然后启动默认单进程模式：

```bash
make env
make app-image
docker compose -f deploy/compose/docker-compose.yml --env-file .env \
  up -d --wait --wait-timeout 180 postgres app
```

拆分服务模式：

```bash
make api-up
make worker-up
make scheduler-up
make frontend-up
```

`api`、`worker`、`scheduler`、`frontend` 属于 `svc` profile；`app` 为默认服务。服务不使用 `condition`，程序自身负责依赖重试。Compose 文件只引用镜像，不包含 `build`。

停止和清理：

```bash
make down       # 停止 PostgreSQL 和 App
make clean      # 删除 Compose 数据卷
```

## 3. Helm

生产部署使用 `deploy/helm/opskeeper`。Chart 的 API 工作负载使用嵌入前端的 `opskeeper-app` 镜像，以便 Ingress 直接提供页面；Worker 和 Scheduler 使用独立镜像。

创建运行时 Secret（示例值不要提交）：

```bash
kubectl -n opskeeper create secret generic opskeeper-runtime \
  --from-literal=OPSK_DB_HOST='<database-host:5432>' \
  --from-literal=OPSK_DB_NAME='opskeeper' \
  --from-literal=OPSK_DB_USER='<application-user>' \
  --from-literal=OPSK_DB_PASSWORD='<application-password>' \
  --from-literal=OPSK_CREDENTIAL_KEY='<32-byte-or-base64-key>'
```

准备 values：

```yaml
image:
  repository: registry.example.com/opskeeper
  digest: sha256:<digest>
basePath: /opskeeper
existingSecret: opskeeper-runtime
ingress:
  enabled: true
  host: ops.example.com
  tlsSecretName: opskeeper-tls
```

部署和验证：

```bash
helm lint deploy/helm/opskeeper
helm upgrade --install opskeeper deploy/helm/opskeeper \
  --namespace opskeeper --create-namespace --atomic --timeout 15m \
  -f production-values.yaml
kubectl -n opskeeper wait --for=condition=available \
  deployment/opskeeper-opskeeper-api --timeout=5m
curl --fail --silent https://ops.example.com/opskeeper/health/ready
```

## 4. 迁移与权限

API/App 在监听端口前自动迁移，并通过 PostgreSQL advisory lock 防止并发执行。运行账号只需要本平台业务数据库的迁移权限，不需要 PostgreSQL 实例级超级用户权限，也不需要独立 Migration Job。

迁移采用前向兼容的 Expand/Contract 方式。发布顺序为：

1. 发布 API/App，等待迁移和 readiness 成功。
2. 发布 Worker 和 Scheduler。
3. 验证登录、只读资源、诊断、巡检和通知。

迁移失败时停止发布；不要在生产环境执行反向迁移。

## 5. 回滚

1. 保留当前和目标镜像 digest、Chart 版本及数据库备份校验和。
2. 应用失败时回滚到与当前 Schema 兼容的旧镜像。
3. Schema 不兼容时前滚修复，不执行 `down` migration。
4. 回滚后检查 readiness、任务租约、通知幂等和错误率。

完整故障处置见 [runbooks/](../runbooks/)。
