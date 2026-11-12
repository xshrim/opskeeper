# ADR-0002：前后端源码分离与嵌入式 App 镜像

- 状态：已接受
- 日期：2026-08-15

## 背景

前端需要 Vite 热更新和独立的 TypeScript 依赖，后端需要保持 Go 工程边界；生产部署需要同时支持独立前后端扩缩和单进程嵌入式 App。API、Worker、Scheduler 和迁移逻辑还需要由同一提交生成，避免应用与迁移版本不一致。

## 候选方案

1. 前端和后端使用不同仓库、镜像和发布流程。
2. 同仓库但生产仍使用独立 Web 镜像和反向代理。
3. 前后端源码和依赖保持独立，生产构建同时生成非嵌入 API、独立 Frontend 镜像，以及嵌入 Vite 制品的 App 镜像；Worker 和 Scheduler 使用各自镜像。

## 决定

采用方案 3 的拆分实现。本地开发可以分别运行 Vite 和 Go API，也可以通过 `make app-run` 构建并嵌入前端。生产构建通过 `go:embed` 把 Web 制品编译进 `opskeeper-app`，同时保留非嵌入的 `opskeeper-api` 和独立 `opskeeper-frontend`。

每个镜像只提供一个运行入口；App 镜像与 API、Worker、Scheduler 镜像由同一 Dockerfile 构建并使用同一版本标识。部署编排按工作负载选择独立服务或单进程 App。构建入口只由根 Makefile 暴露，Dockerfile 的 Builder 调用 Make 内部构建目标。

## 影响

- 页面、API 和迁移逻辑可以精确关联到同一版本和 Commit。
- 最终运行镜像不需要 Node.js；独立模式由 Frontend 镜像提供静态文件，嵌入模式由 App 提供页面。
- 任何前端变化都会重新构建 Frontend 和 App 镜像。
- 独立模式可以分别扩缩 API、Worker、Scheduler 和 Frontend；App 模式适合本地或简单部署。
- 具体构建和发布流程见[部署指南](../guides/deployment.md)。
