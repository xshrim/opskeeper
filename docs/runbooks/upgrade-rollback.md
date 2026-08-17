# 升级与回滚

1. 记录当前和目标镜像 digest、Chart 版本及数据库备份校验和。
2. 执行 `helm diff` 或保存 `helm template` 差异，重点复核 Secret 引用、NetworkPolicy、探针和资源限制。
3. 使用 `helm upgrade --atomic --timeout 15m`。Migration Hook 失败时停止，不手工跳过。
4. 迁移成功后验证 readiness、登录、只读资源、诊断和巡检流程。
5. 应用失败时回滚到上一个与新 Schema 兼容的 digest：`helm rollback <release> <revision> --wait --timeout 10m`。

不在生产回滚流程中执行 `migrate down`。若新 Schema 与旧应用不兼容，必须前滚修复；这类不兼容迁移本就不应通过 Expand/Contract 评审。
