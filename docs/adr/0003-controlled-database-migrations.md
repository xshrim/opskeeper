# ADR-0003：由 API/App 启动阶段执行数据库迁移

- 状态：已接受
- 日期：2026-08-15

## 背景

数据库 Schema 必须与应用版本保持一致，同时不能让多个 API 副本在启动时并发修改生产数据库。发布失败时还需要明确区分应用回滚与数据结构回滚，避免自动执行不安全的 Down Migration。

## 候选方案

1. API 启动时自动执行迁移。
2. 流水线直接执行仓库中的 SQL 文件。
3. 提供与应用同提交构建的专用迁移二进制，由发布系统在应用滚动升级前运行单实例 Migration Job。
4. 引入外部迁移框架和独立迁移镜像。

## 决定

采用方案 1。API/App 在开始监听 HTTP 端口前自动执行迁移；不再构建、部署或调用独立的 migrate/Migration 程序。初始 Schema 由唯一的 `0001_initial.sql` 建立；新安装无需顺序重放历史增量脚本。历史 SQL 仅保存在 `sql/archive/`，不被迁移器嵌入。迁移器记录版本、名称和前滚 SQL SHA-256，并使用 PostgreSQL session advisory lock 串行执行。每条迁移在独立事务中完成 SQL 和历史记录写入。

API/App 启动迁移失败时不监听 HTTP 端口并退出；Worker 和 Scheduler 不执行迁移，只在数据库可访问后运行。Schema 演进采用 Expand/Contract，生产应用回滚不自动执行 `down`，数据问题优先通过新的前滚迁移修复。

## 影响

- 发布编排必须能够等待 API/App readiness；迁移失败会阻断应用启动。
- API/App 镜像与迁移 SQL 来自同一 Commit，避免 SQL 与应用版本漂移。
- advisory lock 防止误配置并发，但不代替发布系统的单任务编排。
- 自定义迁移器需要自行维护校验和、兼容升级、回滚策略和集成测试。
- `0001_initial` 仅用于全新数据库；已有历史数据库不兼容此基线校验，必须按受控流程重建或另行迁移，不能伪造 `schema_migrations` 记录。
- 完整机制见[部署指南](../guides/deployment.md)。
