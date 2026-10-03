# I006-R001 需求验收报告：统一通知机制

**迭代：** I006-notification-mechanism
**需求：** R001 统一通知机制
**验收结论：** 待验收

## 1. 需求级验收结论

需求整体仍在实施中。T01-T06 已按任务分支完成并通过各自的验收；T07 页面与菜单、T08 集成验收与运维文档仍未完成，因此不能据此认定 R001 或 I006 已完成。

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
| T04 | 规则匹配与巡检关联 | 已完成 | `go test ./...`、`go vet ./...`、迁移与巡检 PostgreSQL 集成测试、规则匹配与日历窗口单测 |
| T05 | PostgreSQL 队列与可靠投递 | 已完成 | `go test ./...`、`go vet ./...`、迁移与巡检 PostgreSQL 集成测试、TLS provider/聚合/租约/并发/重试/死信测试 |
| T06 | HTTP API、权限与审计 | 已完成 | `go test ./...`、`make backend-lint`、迁移与巡检 PostgreSQL 集成测试、通知 API 权限测试，见下文 |
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

**结果：** 已完成
**验收目标：** 事件路由按事件类型、最低级别和白名单资源/Finding 条件匹配；策略规则绑定原子校验 Scope；冷却、聚合等待和静默窗口在入队事务中生效，恢复和重新打开绕过冷却。
**验证步骤和结果：**

1. `cd backend && go test ./...`、`cd backend && go vet ./...`：通过。
2. `OPSK_TEST_DATABASE_URL="$OPSK_DATABASE_URL" go test -tags=integration ./migrations ./inspection -count=1`：通过；验证迁移 up/down/replay、规则过滤、策略绑定去重与失败原子性、冷却窗口抑制、事件状态转换及入队事务。
3. `backend/notification/rules_test.go`：通过；覆盖事件/严重级别/资源/Finding 条件匹配及跨午夜、时区、星期静默窗口计算。

**遗留问题：** 多事件聚合投递的领取和渲染由 T05 Worker 实现；T04 已将聚合时间和最大批次写入不可变投递快照，并延后队列可领取时间。子 Scope 共享仍受当前同 Scope 外键约束，必须在 T06 做授权/API 范围设计时决定是否扩展数据关系，当前为显式拒绝跨 Scope 绑定。

### T05 PostgreSQL 队列与可靠投递

**结果：** 已完成
**验收目标：** Worker 通过 PostgreSQL `SKIP LOCKED` 领取版本化队列消息和聚合批次；网络调用不持有数据库锁；租约超时可恢复；渠道凭据按配置版本解密；发送结果追加尝试记录，支持退避、Retry-After、有限重试和死信。
**验证步骤和结果：**

1. `cd backend && go test ./...`、`cd backend && go vet ./...`、仓库 `make backend-lint`：通过。
2. `OPSK_TEST_DATABASE_URL="$OPSK_DATABASE_URL" go test -tags=integration ./migrations ./inspection -count=1`：通过；包含迁移、临时 Schema 清理、通知队列流程验证。
3. `TestFindingNotificationEventsAreTransactional`：通过真实本地 TLS webhook 验证签名与模板输出、相同路由聚合、尝试追加、租约过期后接管、HTTP 429/Retry-After 延迟重试、死信、队列指标和两个并发 worker 不重复领取。
4. `backend/config/config_test.go`：默认 `postgres` 与非法后端拒绝验证通过；示例配置增加队列后端及轮询间隔。

**遗留问题：** 当前只有 HTTPS Webhook provider 可投递；其他 provider 在 T02 支持矩阵中明确标记不支持。队列后端只支持 PostgreSQL，按需求不引入外部消息队列。发送超时后仍存在外部服务已接收但本地未确认的至少一次投递窗口，接收端应使用投递 ID/事件幂等键去重。

### T06 HTTP API、权限与审计

**结果：** 已完成
**验收目标：** 为渠道、模板、规则、巡检策略关联和投递记录提供 HTTP API；通过通知专属权限和 Scope 校验保护读取、管理、测试与重试操作；渠道密钥只以脱敏配置返回，并对变更、测试和重试记录审计事件。父 Scope 对象只有显式共享后才可供后代读取和绑定，子 Scope 不能编辑父对象。

**验证步骤和结果：**

1. `cd backend && go test ./...`：通过；覆盖新增 API 请求处理、错误响应、模板响应及权限拒绝用例。
2. `make backend-lint`：通过。
3. `go test -p 1 -tags=integration ./migrations ./inspection -count=1`，连接本地 compose PostgreSQL：通过；覆盖 0075-0078 迁移应用/回滚/重放、通知专属角色权限、跨 Scope 父级共享与只读访问、共享规则绑定多个项目、私有路由替换拒绝、共享撤销保护、投递 Scope 隔离及 T01-T05 队列流程回归。
4. HTTP 路由权限用例验证渠道/模板/规则/投递读取、管理、测试及重试分别使用 `notification:read`、`notification:manage`、`notification:test` 和 `notification:retry`；策略关联仍要求 `inspection:manage`。
5. `git diff --check`：通过。0077 已应用迁移保持原始校验和；新增路由共享保护放在 0078，且其 Down 会恢复 0077 的函数定义。

**遗留问题：** 无 T06 范围内未解决问题。通知页面和菜单由 T07 实施，迭代级端到端验收及运维文档由 T08 实施。

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
