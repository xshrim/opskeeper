# 本地开发环境

## 前置条件

- Go 1.26 或兼容版本
- Node.js 22 或更高版本
- npm 11 或兼容版本
- Docker 和 Docker Compose v2

本机既可以使用 `docker compose` 插件，也可以使用独立的 `docker-compose` 命令，Makefile 会自动选择可用实现。

## 初始化

```bash
cp .env.example .env
make deps
make dev-services-up
```

## 启动

分别在不同终端运行：

```bash
make run-api
make run-worker
make run-scheduler
make run-frontend
```

默认地址：

- 前端：`http://localhost:5173`
- API 存活检查：`http://localhost:8080/health/live`
- API 就绪检查：`http://localhost:8080/health/ready`

## 质量检查

```bash
make quality
```

该命令依次检查格式、静态分析、单元测试和构建结果。

## 配置规范

- 后端配置统一使用 `OPSK_` 前缀环境变量。
- `.env` 仅用于本地开发，不能提交到版本库。
- `.env.example` 只能包含无敏感性的开发默认值。
- 生产环境必须通过 Secret 管理系统注入数据库、Redis 和后续外部资源凭据。
