# 生产环境部署

生产部署使用 `deploy/helm/opskeeper` Chart。API 内嵌 Web 制品；Worker 和 Scheduler 使用同一镜像；Migration 作为 Helm `pre-install,pre-upgrade` Hook 先行执行，失败会阻断发布。

## 1. 前置条件

- Kubernetes 1.29 或更高版本，支持 NetworkPolicy 的 CNI。
- Helm 3.18 或更高版本。
- 以 digest 标识的 OpsKeeper 镜像。
- PostgreSQL 16、Redis 7 和可接收 OTLP/HTTP 的 Collector。
- 应用与迁移凭据已分离，且 Ingress 直连地址范围已确认。

## 2. Secret 契约

Chart 不生成 Secret。先从受控密钥系统创建两个 Secret：

```bash
kubectl -n opskeeper create secret generic opskeeper-runtime \
  --from-literal=OPSK_DATABASE_URL='<application-role-url>' \
  --from-literal=OPSK_REDIS_URL='<redis-url>' \
  --from-literal=OPSK_CREDENTIAL_KEY='<32-byte-or-base64-key>'

kubectl -n opskeeper create secret generic opskeeper-migration \
  --from-literal=OPSK_DATABASE_URL='<migration-role-url>' \
  --from-literal=OPSK_REDIS_URL='<redis-url>' \
  --from-literal=OPSK_CREDENTIAL_KEY='<same-active-key>'
```

不要将 Secret 值写入 `values.yaml`、Helm 参数、终端历史或 CI 日志。上述命令中的占位符只说明必需键。

## 3. 生产 values

```yaml
image:
  repository: registry.example.com/opskeeper
  digest: sha256:<digest>
basePath: /opskeeper
existingSecret: opskeeper-runtime
migrationSecret: opskeeper-migration
trustedProxies: 10.42.0.0/16
otelExporterEndpoint: https://otel-collector.observability.svc:4318
ingress:
  enabled: true
  className: nginx
  host: ops.example.com
  tlsSecretName: opskeeper-tls
networkPolicy:
  databaseCIDR: 10.50.1.10/32
  redisCIDR: 10.50.1.11/32
```

`databaseCIDR`、`redisCIDR` 和 Ingress Namespace Selector 必须收紧到实际环境。若 Connector/LLM 不需访问公网 HTTPS，将 `allowExternalHTTPS` 设为 `false`。

## 4. 部署与验证

```bash
helm lint deploy/helm/opskeeper
helm template opskeeper deploy/helm/opskeeper -f production-values.yaml >/tmp/opskeeper-rendered.yaml
helm upgrade --install opskeeper deploy/helm/opskeeper \
  --namespace opskeeper --create-namespace --atomic --timeout 15m \
  -f production-values.yaml
kubectl -n opskeeper wait --for=condition=available deployment/opskeeper-opskeeper-api --timeout=5m
kubectl -n opskeeper get pods,jobs,networkpolicy,pdb
curl --fail --silent https://ops.example.com/opskeeper/health/ready
```

发布必须检查 Migration Job 日志、API readiness、Worker/Scheduler 错误率和 OTLP 指标。发布后再执行登录、资源读取、诊断和巡检冒烟流程。

## 5. 扩容边界

API 无状态，可通过 HPA 扩容。Worker 依赖 PostgreSQL 任务租约，可增加副本，但应先校验数据库连接上限和外部 Connector 配额。Scheduler 默认单副本。当请求、任务或 Connector P95 延迟连续超出 SLO 时，应先根据 OTLP 数据定位限制因素。
