# ADR-0003：使用内嵌迁移器和独立 Migration Job

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

采用方案 3。SQL 通过 `go:embed` 编译进 `opskeeper-migrate`，迁移器记录版本、名称和前滚 SQL SHA-256，并使用 PostgreSQL session advisory lock 串行执行。每条迁移在独立事务中完成 SQL 和历史记录写入。

API、Worker 和 Scheduler 永不自动迁移。测试、预发布和生产部署先运行一个 Migration Job，成功后才发布长期运行进程。Schema 演进采用 Expand/Contract，生产应用回滚不自动执行 `down`，数据问题优先通过新的前滚迁移修复。

## 影响

- 发布编排必须能够等待 Migration Job 完成并在失败时阻断发布。
- 同一镜像同时包含迁移器和应用，避免 SQL 与应用 Commit 漂移。
- advisory lock 防止误配置并发，但不代替发布系统的单任务编排。
- 自定义迁移器需要自行维护校验和、兼容升级、回滚策略和集成测试。
- 完整机制见[自动化发布](../guides/delivery.md)。
