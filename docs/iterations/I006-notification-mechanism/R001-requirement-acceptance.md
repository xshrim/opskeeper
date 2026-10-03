# I006-R001 需求验收报告：统一通知机制

**迭代：** I006-notification-mechanism
**需求：** R001 统一通知机制
**验收结论：** 待验收

## 1. 需求级验收结论

需求整体仍在实施中。当前分支完成 T01 的通知领域 Schema、状态事件和事务入队实现，T01 待用户验收；T02-T08 尚未实施，因此不能据此认定 R001 或 I006 已完成。

## 2. 验收环境和范围

本次 T01 验证范围：

- PostgreSQL 集成测试使用 `OPSK_TEST_DATABASE_URL` 指向的连接，并在专用临时 Schema 内迁移、测试、清理；未将连接凭据写入报告；
- 迁移 `0071_notification_domain` 的完整应用、回滚和重放；
- Finding 状态转换、事务事件/投递写入、跨 Scope 约束、不可变路由快照及回滚；
- 基线提交：`399b7bf`；当前实现仍在任务分支，尚无 T01 验收提交。

## 3. 任务验收汇总

| 任务 | 名称 | 结果 | 证据 |
|---|---|---|---|
| T01 | 通知领域与事务事件 | 已完成 | `go test ./...`；`go vet ./...`；迁移与 inspection 集成测试，见下文 |
| T02 | Provider 注册与通知渠道 | 已完成 | `go test ./...`；`go vet ./...`；PostgreSQL inspection channel 集成测试，provider catalog 单测 |
| T03 | 模板版本与安全渲染 | 已完成 | `go test ./...`、`go vet ./...`、`make backend-lint`、迁移 PostgreSQL 集成测试、模板安全渲染单测 |
| T04 | 规则匹配与巡检关联 | 待批准 | 实施后记录匹配、冷却、聚合、静默和策略关联测试 |
| T05 | PostgreSQL 队列与可靠投递 | 待批准 | 实施后记录 PostgreSQL、租约、重试、死信和非法配置测试 |
| T06 | HTTP API、权限与审计 | 待批准 | 实施后记录 API、Scope/RBAC、脱敏和审计测试 |
| T07 | 通知页面与菜单 | 待批准 | 用户确认预览后记录页面检查、前端测试和 Playwright 证据 |
| T08 | 集成验收与运维文档 | 待批准 | 实施后记录 `make quality`、迁移、配置和恢复验证 |

## 4. 任务验收报告

每项任务完成后在对应小节追加实际证据，不提前填写未来结果。每个小节必须记录验收目标、验证步骤和结果、遗留问题；不得只写“已验证”，也不得把规划中的测试写成已通过。

### T01 通知领域与事务事件

**结果：** 已完成
**验收目标：** 让巡检 Finding 的打开、重开、严重级别变化和恢复，与通知事件以及匹配规则生成的 PostgreSQL 投递行在同一事务提交；为最终巡检失败和智能解释降级建立事件；保证同 Scope 外键、幂等键、快照哈希及不可变事件/投递尝试约束。

**验证步骤和结果：**

1. `cd backend && go test ./...`、`cd backend && go vet ./...`：通过。
2. `OPSK_TEST_DATABASE_URL="$OPSK_DATABASE_URL" go test -tags=integration ./migrations -count=1`：通过；验证迁移完整应用、已有 `dead_letter` 投递回滚映射为 `failed`、最近迁移重放。迁移集成测试使用临时 Schema。
3. `OPSK_TEST_DATABASE_URL="$OPSK_DATABASE_URL" go test -tags=integration ./inspection -run '^TestFindingNotificationEventsAreTransactional$' -count=1`：通过；验证 opened、severity_changed、resolved、reopened 转换，同次重复保存不重复发事件，路由生成投递、快照不可变、事件不可更新/删除、尝试只追加、跨 Scope 关联拒绝，以及中途资源约束失败时 Finding/事件整体回滚。测试使用独立临时 Schema。

**遗留问题：** 无 T01 范围内未解决问题。规则/渠道/模板管理 API、Scope 子级共享策略、完整通知过滤与聚合属于 T02-T06；全仓 `make backend-integration-test` 曾受既有路径/构建问题及 Redis 未启动阻塞，本次以定向 PostgreSQL 集成用例和完整后端单测作为 T01 验收证据。

### T02 Provider 注册与通知渠道

**结果：** 已完成
**验收目标：** 建立可审计 provider 描述和 HTTPS Webhook 适配；提供通知渠道创建、列表、更新、软删除、测试发送；凭据加密保存并按渠道限流。

**验证步骤和结果：**

1. `cd backend && go test ./...`、`cd backend && go vet ./...`：通过。
2. `OPSK_TEST_DATABASE_URL="$OPSK_DATABASE_URL" go test -tags=integration ./migrations ./inspection -count=1`：通过；验证 0072 迁移、两版本回滚/重放、通道配置加密、密钥不回传、版本递增、测试发送和渠道级限流。
3. `backend/notification/providers_test.go`：通过；验证 HTTPS URL、嵌入凭据和未知配置拒绝，秘密字段脱敏，v1.6.0 provider 清单及不支持状态可枚举。

**遗留问题：** Go-Notify v1.6.0 清单中除 HTTPS Webhook 外的 provider 在当前构建均显式标记不支持；不编译未使用的邮件、短信和平台 SDK。渠道测试发送的审计记录及细粒度 `notification:*` 权限由 T06 实施。渠道投递 Worker 对版本化密文的读取由 T05 实施。

### T03 模板版本与安全渲染

**结果：** 已完成
**验收目标：** 模板支持草稿版本、不可变发布、白名单变量和稳定哈希；预览与投递共用安全渲染器，并对 provider payload 和渲染大小进行限制。
**验证步骤和结果：**

1. `cd backend && go test ./...`：通过。
2. `cd backend && go vet ./...`、仓库 `make backend-lint`：通过。
3. `OPSK_TEST_DATABASE_URL="$OPSK_DATABASE_URL" go test -tags=integration ./migrations -count=1`：通过；验证 0073 迁移加载、应用、回滚/重放，已发布模板内容更新被数据库拒绝、停用允许。
4. `backend/notification/templates_test.go`：通过；覆盖白名单/必填变量、未声明变量、控制语句与命名子模板注入拒绝、JSON 字符串转义、输出上限、provider 转换/不支持 provider 拒绝及稳定内容哈希。
5. `make quality`：未通过前端 `npm run format:check`；报告仓库中 124 个既有文件格式不符，本任务没有格式化或改动这些无关文件。

**遗留问题：** 数据库模板版本增删改 API、通知权限和审计属于 T06；本任务提供的 Preview 与发送共用 `RenderTemplate`，未接入页面。全仓格式检查需后续独立清理既有前端格式差异。

### T04 规则匹配与巡检关联

**结果：** 待批准
**验收目标：** 待实施后填写。
**验证步骤和结果：** 待实施后记录匹配、冷却、聚合、静默和策略关联测试。
**遗留问题：** 待实施后填写。

### T05 PostgreSQL 队列与可靠投递

**结果：** 待批准
**验收目标：** 待实施后填写。
**验证步骤和结果：** 待实施后记录 PostgreSQL、租约、重试、死信和非法配置测试。
**遗留问题：** 待实施后填写。

### T06 HTTP API、权限与审计

**结果：** 待批准
**验收目标：** 待实施后填写。
**验证步骤和结果：** 待实施后记录 API、Scope/RBAC、脱敏和审计测试。
**遗留问题：** 待实施后填写。

### T07 通知页面与菜单

**结果：** 待批准
**验收目标：** 待实施后填写。
**验证步骤和结果：** 待用户确认预览后记录页面检查、前端测试和 Playwright 证据。
**遗留问题：** 待实施后填写。

### T08 集成验收与运维文档

**结果：** 待批准
**验收目标：** 待实施后填写。
**验证步骤和结果：** 待实施后记录 `make quality`、迁移、配置和恢复验证。
**遗留问题：** 待实施后填写。

## 5. 需求级遗留事项

待实施后填写。所有未完成能力必须注明责任任务、转移迭代或关闭条件。

## 6. 用户确认和最终结论

待用户验收。需求只有在所有任务有可复核证据、用户确认并完成迭代封板记录后，才能标记为已完成。
