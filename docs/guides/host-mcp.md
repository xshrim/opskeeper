# Host MCP Agent

`host-mcp-agent` 是 Linux 主机的只读 MCP Agent，提供 Host 资源的统一本机和 SSH 接入。它与 Docker MCP Agent 使用相同的 Streamable HTTP、SSE、Bearer Token 和 CORS 运行方式，默认监听 `0.0.0.0:8813`。

```bash
make host-mcp-build
make host-mcp-run
```

## 目标主机解析

每个工具都接受 Host 连接字段。解析顺序固定为：

```text
工具参数 > HOST_MCP_* 环境变量 > 当前 Agent 本机
```

工具未传 `host`，且 `HOST_MCP_HOST`、`HOST_MCP_SSH_HOST` 均为空时，工具直接读取 Agent 所在主机的 `/proc`、`/sys` 和 `/etc/os-release`。填写 `host` 后，使用 SSH 密码或私钥登录目标 Linux 主机。`username`、`password`、`private_key`、`passphrase` 和 `known_hosts` 只用于连接，不会出现在结果、日志或审计正文中。

常用环境变量：

| 变量 | 说明 |
| --- | --- |
| `HOST_MCP_HTTP_ADDRESS` | HTTP 监听地址，默认 `0.0.0.0:8813` |
| `HOST_MCP_BEARER_TOKEN` | 可选 Bearer Token |
| `HOST_MCP_HOST` / `HOST_MCP_SSH_HOST` | SSH 目标主机；为空时回退本机 |
| `HOST_MCP_PORT` / `HOST_MCP_SSH_PORT` | SSH 端口，默认 22 |
| `HOST_MCP_USERNAME` / `HOST_MCP_SSH_USERNAME` | SSH 用户名 |
| `HOST_MCP_AUTH_METHOD` | `password` 或 `key` |
| `HOST_MCP_PASSWORD` | SSH 密码 |
| `HOST_MCP_PRIVATE_KEY` | PEM 或 Base64 私钥 |
| `HOST_MCP_PASSPHRASE` | 私钥口令 |
| `HOST_MCP_KNOWN_HOSTS` | known_hosts 文件内容；为空时读取 Agent 本机默认 `~/.ssh/known_hosts` |
| `HOST_MCP_TIMEOUT_SECONDS` | 连接超时，范围 1 到 300 秒 |

SSH 强制执行 known_hosts 校验，不提供不安全的自动接受主机密钥模式。`HOST_MCP_PRIVATE_KEY` 和 `HOST_MCP_KNOWN_HOSTS` 均接收文件内容，不接收文件路径。

## 工具

| 工具 | 能力 |
| --- | --- |
| `host_info` | 主机名、内核、架构、OS、启动时间、运行时长、CPU 和内存总量 |
| `host_metrics` | Node Exporter 风格结构化 JSON：CPU、负载、内存、Swap、文件系统、磁盘、网络、PSI 和进程汇总；`sample_seconds` 范围 0 到 10 |
| `host_processes` | 按 `pid` 或不区分大小写的字面 `keyword` 查询进程；返回 PID、父进程、状态、可执行文件、工作目录、用户、资源使用率和脱敏命令行 |
| `host_file_logs` | 读取绝对文件路径日志；支持 `tail`、`since`、`until`、`keyword` 和 `timestamps`，单次最多读取 2 MiB |
| `host_health` | 汇总 `host_info` 和 `host_metrics`，返回 `healthy` 或 `degraded` |

工具不执行模型提供的 Shell、命令、信号、写文件或进程控制操作。进程环境变量不读取，命令行会限长并脱敏。权限不足或进程退出会返回 `partial` / `unavailable`，不会伪造数据。

## 验证

```bash
make host-mcp-test
make host-mcp-build
```
