# Docker MCP Server

`docker-mcp` 是一个独立的只读 MCP Server 进程，用官方 [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) 提供 Docker Engine 信息查询能力。它位于 `backend/mcpserver/docker/`，入口是 `backend/cmd/docker-mcp/`；它与 API、Worker、Scheduler 平行运行，不是 API 内部的 Docker Connector。

## 快速启动

```bash
# 1. 生成本地配置（不存在时）
test -f .env || cp .env.example .env

# 2. 启动 docker-mcp
make docker-mcp-run
```

默认监听 `0.0.0.0:8811`，同时提供 Streamable HTTP 和 SSE 两种 MCP 传输。构建独立二进制使用 `make docker-mcp-build`，运行测试使用 `make docker-mcp-test`。

启动后，Streamable HTTP MCP 端点固定为 `/mcp`，SSE 端点固定为 `/sse`。服务监听在所有网卡的 `0.0.0.0:8811`；MCP 客户端应使用服务器实际可访问的主机名或 IP，例如 `http://127.0.0.1:8811/mcp` 或 `http://<server-ip>:8811/mcp`，不要把 `0.0.0.0` 作为客户端连接目标。它不是浏览器页面，直接用浏览器打开不能完成 MCP 初始化。使用 SSE 时直接连接 `http://<server-ip>:8811/sse`，不需要切换服务端环境变量。

## 服务日志

服务启动、请求和响应日志都使用毫秒级 ISO-8601 本地时间，格式固定为：

```text
2006-01-02T15:04:05.000
```

实际日志示例：

```text
2026-08-20T23:34:08.222 [REQUEST] Session: 33SOPJPQOU6ME2NQ3UXKNUAXBX | Method: tools/list
2026-08-20T23:34:08.222 [RESPONSE] Session: 33SOPJPQOU6ME2NQ3UXKNUAXBX | Method: tools/list | Status: OK | Duration: 25.588µs
```

日志不会记录 MCP 请求正文、Bearer Token、Docker 客户端证书或私钥内容。部署环境应统一时区；日志时间不附加时区后缀，以保持与后端日志规范一致。

## 传输与访问认证

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DOCKER_MCP_HTTP_ADDRESS` | `0.0.0.0:8811` | HTTP 监听地址 |
| `DOCKER_MCP_BEARER_TOKEN` | 空 | 非空时启用 Bearer Token；为空时不启用；服务不会打印该值 |
| `DOCKER_MCP_CORS_ENABLED` | `false` | 是否允许浏览器跨来源访问 MCP HTTP/SSE 端点；仅为 `true` 时启用 |

例如：

```bash
DOCKER_MCP_BEARER_TOKEN='change-me' \
make docker-mcp-run
```

Bearer Token 是 MCP HTTP 访问认证，与 Docker daemon 的客户端证书无关。只要 `DOCKER_MCP_BEARER_TOKEN` 非空就会启用认证；变量为空或未设置时不启用认证。客户端启用认证时，应在每个 MCP HTTP 请求中发送 `Authorization: Bearer <token>`。

默认不允许跨来源请求。需要从浏览器页面直接连接时，可以显式开启：

```bash
DOCKER_MCP_CORS_ENABLED=true make docker-mcp-run
```

开启后服务返回 `Access-Control-Allow-Origin: *`，允许 MCP 所需的 `Authorization`、`Mcp-Session-Id` 和 `Last-Event-ID` 请求/响应头；不启用 Cookie 凭据。该设置只处理 CORS，不能放宽浏览器或宿主应用的 Content Security Policy。如果客户端仍报告 `unsafe-eval` 被 CSP 禁止，应调整客户端页面或宿主应用的 CSP/实现，而不是继续修改 MCP Server 的 CORS。

### 严格 CSP 客户端的 `unsafe-eval`

如果错误正文包含：

```text
Evaluating a string as JavaScript violates ... 'unsafe-eval'
```

这是客户端依赖的 Zod 4 在严格 CSP 页面中探测 JIT `new Function()` 产生的报告，Docker MCP 服务端无法通过 CORS、响应头或监听地址消除它。使用 WebMind 或类似的浏览器客户端时，应在创建 MCP `Client` 之前关闭 Zod JIT：

```ts
import { config as configureZod } from "zod/v4";

configureZod({ jitless: true });
```

同时，客户端如果在 `tools/call` 阶段使用 MCP SDK 默认的 AJV JSON Schema 校验器，还需要为该客户端配置不使用动态代码生成的 JSON Schema 校验器，或在客户端侧完成安全的静态校验。不要为了连接本 MCP Server 而给整个浏览器扩展 CSP 增加 `'unsafe-eval'`，这会扩大页面脚本执行权限。

例如，Streamable HTTP 客户端的连接信息可以表达为：

```text
URL:   http://127.0.0.1:8811/mcp
Header: Authorization: Bearer change-me   # 仅启用 Token 时需要
```

## Docker daemon 连接

六个工具都接受相同的连接字段：`docker_host`、`docker_ca`、`docker_cert`、`docker_key`、`docker_server_name` 和 `docker_skip_tls_verify`。其中 `docker_skip_tls_verify` 是布尔值，默认 `false`；只有明确设置为 `true` 才会跳过 TLS 证书校验。连接优先级固定为：

```text
工具参数 > DOCKER_MCP_DOCKER_* 环境变量 > Docker 默认 Unix socket
```

工具参数可以使用：

- `http://host:2375`：明文 TCP；
- `tcp://host:2375`：明文 TCP；当同时配置 TLS 文件或跳过校验时按 TLS 使用；
- `https://host:2376`：TLS TCP；
- `unix:///var/run/docker.sock`：Unix socket。

环境变量：

| 环境变量 | 说明 |
| --- | --- |
| `DOCKER_MCP_DOCKER_HOST` | 组件默认 Docker URL；也兼容 `DOCKER_HOST` |
| `DOCKER_MCP_DOCKER_CA` | CA PEM 文件路径 |
| `DOCKER_MCP_DOCKER_CERT` | 客户端证书 PEM 文件路径 |
| `DOCKER_MCP_DOCKER_KEY` | 客户端私钥 PEM 文件路径 |
| `DOCKER_MCP_DOCKER_SERVER_NAME` | TLS ServerName 覆盖值 |
| `DOCKER_MCP_DOCKER_TLS_SKIP_VERIFY` | `true` 时跳过 TLS 证书校验；只应在明确受信任的开发环境使用 |

也可以使用 Docker CLI 兼容的 `DOCKER_CERT_PATH`（目录内包含 `ca.pem`、`cert.pem`、`key.pem`）和 `DOCKER_TLS_VERIFY`。CA、客户端证书、私钥必须成套配置。若工具调用显式传入的 host、证书或 TLS 参数无法解析、文件不存在、TLS 握手失败或 host 无法访问，服务会自动改用 Docker 默认 Unix socket（不会再次使用 `DOCKER_HOST`、`DOCKER_MCP_DOCKER_HOST` 或其他 TLS 环境参数）执行一次；成功时结果会带有 `connection_fallback.custom_connection_error` 和 `connection_fallback.fallback`，便于诊断但不会返回证书内容。普通 Docker 业务错误（例如容器不存在）不会触发回退。环境变量本身配置错误不会重复回退。

## 工具清单

| Tool | 能力 | 限制 |
| --- | --- | --- |
| `docker_info` | Docker Engine 信息 | 只读 |
| `docker_images` | 镜像列表 | 支持 `all` 和过滤器 |
| `docker_containers` | 容器列表 | 支持 `all`、过滤器，最多 500 条 |
| `docker_container_logs` | 容器日志 | 永不 follow，默认请求最近 1000 行，支持时间范围和 keyword 上下文筛选，最终最多返回 200 行、256 KiB |
| `docker_container_inspect` | 容器详情 | 只读，敏感环境变量和凭据字段脱敏 |
| `docker_container_stats` | 容器资源快照 | 使用一次性 stats，不建立持续流 |

所有工具仍返回 JSON 对象形式的结构化内容，但服务端不发布由复杂 DTO 自动生成的 `outputSchema`。这样可以避免部分严格 CSP 的浏览器 MCP 客户端为了编译嵌套 JSON Schema 而调用动态 JavaScript。MCP 适配层会优先传递 `structuredContent`，不会把同一份 JSON 再嵌套进 `content[].text` 字符串，因此下游不会看到大量 `\\` 转义符。列表工具使用对象字段承载数组：`docker_images` 返回 `{ "images": [...] }`，`docker_containers` 返回 `{ "containers": [...] }`；`docker_info` 返回 `{ "info": {...} }`。日志、inspect 和 stats 分别返回对象结构。

工具的 `tools/list` 响应会为每个参数提供 `description`，包括连接参数、容器标识、过滤器和日志范围。模型通常会据此生成符合格式的参数；服务端仍会校验参数，错误信息应视为工具反馈而不是模型指令。

`docker_images` 和 `docker_containers` 的 `filters` 参数是简化字符串，格式为 `key:value,key:value`，例如 `label:com.example.env=prod,status:running`。服务端会按每项的第一个冒号拆分键和值，因此 `reference:nginx:latest` 也是有效的过滤器。空项、缺少冒号或空键/值会返回输入错误。

`docker_container_logs`、`docker_container_inspect` 和 `docker_container_stats` 支持 `container_id` 或 `container_name`，至少提供一个；如果同时提供则优先使用 `container_id`。

`docker_container_logs` 的日志范围参数按以下顺序生效：`since` 和 `until` 先确定时间范围（均为空时取最新日志；只填写一个时按该边界过滤；两者都填写时取交集），然后按 `tail` 取结果末尾行数；`tail` 为空时默认为 1000。接着使用 `keyword` 对这批日志进行不区分大小写的匹配：每个命中行会连同前后最多 3 行一起保留，多个命中的上下文范围会合并，不会重复显示重叠行。最后对筛选结果取最近 200 行作为响应行数上限；`keyword` 为空时直接对 tail 结果取最近 200 行。日志读取仍不跟随（`follow=false`），总响应超过 256 KiB 时会截断并设置 `truncated=true`。

所有调用都由 Docker 官方 Go 客户端发起；Server 不接受任意 SQL、Shell 或 Docker 命令，也不提供容器启动、停止、删除、执行等写操作。

## 验证

```bash
# 编译与测试
make docker-mcp-test
make docker-mcp-build

# 查看服务日志时不会出现 Bearer Token、私钥或证书正文
DOCKER_MCP_BEARER_TOKEN='change-me' \
make docker-mcp-run
```
