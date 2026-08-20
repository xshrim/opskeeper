# ADR-0002：前后端源码分离，生产时嵌入 Web 并使用单镜像

- 状态：已接受
- 日期：2026-08-15

## 背景

前端需要 Vite 热更新和独立的 TypeScript 依赖，后端需要保持 Go 工程边界；生产部署又希望减少静态文件服务、镜像和版本配对的复杂度。API、Worker、Scheduler 和 Migration 还需要由同一提交生成，避免应用与迁移版本不一致。

## 候选方案

1. 前端和后端使用不同仓库、镜像和发布流程。
2. 同仓库但生产仍使用独立 Web 镜像和反向代理。
3. 前后端源码和依赖保持独立，生产构建把 Vite 制品嵌入 Go API，四个 Go 进程放入同一个最终镜像。

## 决定

采用方案 3。本地开发可以分别运行 Vite 和 Go API，也可以通过 `make front-api-run` 构建并嵌入前端。生产构建通过 `go:embed` 把 Web 制品编译进 `opskeeper-api`。

最终镜像同时包含 API、Worker、Scheduler 和 Migration 四个固定二进制。部署编排按工作负载选择不同入口，但所有入口来自同一个镜像 Digest。构建入口只由根 Makefile 暴露，Dockerfile 的 Builder 调用 Make 内部构建目标。

## 影响

- 页面、API 和迁移器可以精确关联到同一版本和 Commit。
- 生产环境不需要 Node.js 或独立静态文件服务。
- 任何前端变化都会重新构建 API 二进制和镜像。
- 无法独立扩缩 Web 静态服务；在当前管理控制台负载下这是可接受的。
- 具体构建和发布流程见[自动化发布](../guides/delivery.md)。
