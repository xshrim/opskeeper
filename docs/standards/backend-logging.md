# 后端日志规范

本文是 OpsKeeper 后端日志的强制性规范，适用于 API、Worker、Scheduler、Migration 以及其他 Go 后端进程。它定义日志字段、上下文传递、输出格式和敏感信息边界；具体组件可以增加 `kind` 的消息约定，但不得改变公共日志头。

## 1. 目标与原则

- 日志必须单行、可检索、可关联、可解析。
- 普通日志使用固定字段顺序，避免同一字段在不同服务中含义漂移。
- 业务消息只记录排障所需的最小信息，不记录凭据、完整请求正文或第三方原始响应。
- 日志格式由 `OPSK_LOG_FORMAT` 统一控制；默认使用便于终端和容器日志直接阅读的 `raw`。
- 日志格式不能改变字段语义。切换格式后，日志平台仍应能按相同字段检索和聚合。

## 2. 公共日志头

所有普通日志的逻辑字段顺序固定为：

```text
time level service kind traceid spanid func file:line msg
```

| 字段 | 规则 |
|---|---|
| `time` | 格式为 `yyyy-MM-ddTHH:mm:ss.ms`，Go 布局为 `2006-01-02T15:04:05.000`。不包含时区后缀；部署环境必须统一时区。 |
| `level` | `DEBUG`、`INFO`、`WARN` 或 `ERROR`，使用大写。 |
| `service` | 稳定服务名，例如 `opskeeper-api`、`opskeeper-worker`、`opskeeper-scheduler`、`opskeeper-migrate`。 |
| `kind` | 小写短横线命名的日志类别，例如 `service-start`、`http-request`、`job`、`audit`、`error`。 |
| `traceid` | OpenTelemetry Trace ID，使用小写十六进制字符串；没有有效 Trace 时为 `-`。不得使用 request ID 代替。 |
| `spanid` | OpenTelemetry Span ID，使用小写十六进制字符串；没有有效 Span 时为 `-`。不加入 `parentspanid` 固定字段。 |
| `func` | 直接发起日志调用的函数名，例如 `httpapi.requestLogger`。不记录完整调用栈。 |
| `file:line` | 直接日志调用所在的源文件和行号，例如 `middleware.go:138`。 |
| `msg` | 按 `kind` 约定排列的消息字段。必须单行；HTTP 请求的 `reqid` 放在消息开头。 |

普通日志不增加 `version`、`commit`、`build_time` 等发布元数据。需要发布关联时，应通过部署标签、指标或日志平台字段补充，而不是破坏公共日志头。

### 2.1 Trace、Span 与 ReqID

- `traceid` 和 `spanid` 只来自 OpenTelemetry 上下文。没有上下文时填 `-`。
- `reqid` 是 HTTP 请求级关联标识，用于响应头、错误响应、审计记录和请求消息；它不是 Trace ID，也不放入公共日志头。
- 请求处理必须把 `reqid` 放在 `http-request` 的 `msg` 第一位；没有请求上下文的后台日志不强行生成 `reqid`。
- Trace/Span 不可用时，仍应输出日志，不得因为缺少链路上下文而丢失业务事件。

### 2.2 调用位置与错误堆栈

`func` 和 `file:line` 记录直接日志调用位置，便于快速定位；不要求也不默认输出完整调用栈。`panic`、不可恢复错误或需要深入诊断时，可以在错误详情中按需附加经过脱敏的 stack 信息，但不得把 stack 变成普通日志的固定字段。

## 3. 消息约定

`msg` 的字段顺序由 `kind` 定义。新增 `kind` 时，必须在所属包的文档或代码注释中写明字段顺序、取值和脱敏规则。

### 3.1 `http-request`

固定顺序为：

```text
reqid clientip method path status duration
```

其中 `clientip` 按可信代理规则解析；`path` 不包含敏感查询参数；`status` 为 HTTP 状态码；`duration` 使用可读的耗时值，例如 `12ms`。

### 3.2 其他类别

`service-start`、`job`、`audit`、`error` 等类别应优先使用稳定、低基数的标识和结果。例如：

```text
service-start listen 127.0.0.1:8080 url http://127.0.0.1:8080/opskeeper/
job inspection-run run-123 succeeded 842ms
audit user.login user-123 succeeded
error database timeout
```

不得把任意 JSON、完整 SQL、请求正文或第三方响应直接拼入 `msg`。

## 4. 输出格式

`OPSK_LOG_FORMAT` 支持以下值：

| 值 | 名称 | 规则 |
|---|---|---|
| `raw` | 纯值文本 | 默认格式。按公共日志头顺序输出值，使用单个空格分隔，不输出字段名。 |
| `text` | 键值文本 | 输出 `key=value`，键名与公共字段一致；值包含空格、引号或反斜杠时按统一转义规则处理。 |
| `json` | JSON | 输出单行 JSON 对象，字段使用公共字段名；适合日志平台直接解析。 |

其他值必须在应用启动时被拒绝。三种格式都必须保持相同的字段语义、`kind` 和消息顺序。

### 4.1 `raw` 格式

`raw` 的前 8 个字段固定解析，剩余内容全部属于 `msg`。由于时间使用 `T` 分隔，时间值不需要引号。`msg` 中禁止换行；需要表达空格时直接保留空格，解析器按前 8 个字段切分后将余下内容视为消息。

### 4.2 `text` 格式

键名必须稳定且使用小写短横线或下划线中的项目既有约定；本规范默认使用公共字段名，`file:line` 在键值格式中仍使用 `file:line` 作为完整键名。值的转义规则必须由日志 Handler 统一实现，业务代码不得自行拼接引号。

### 4.3 `json` 格式

JSON 必须是单行对象，不使用多行 pretty-print。公共字段使用字符串或数字等稳定类型；消息中可扩展的字段应保持明确的嵌套结构，不把任意文本当作 JSON 片段拼接。

## 5. 字段示例

以下示例使用同一事件在三种格式下的表达。示例中的 Trace/Span、请求 ID 和业务 ID 均为虚构值。

### 5.1 `service-start`

```text
# raw
2026-08-19T08:30:12.123 INFO opskeeper-api service-start - - api.run main.go:181 listen 127.0.0.1:8080 url=http://127.0.0.1:8080/opskeeper/

# text
time=2026-08-19T08:30:12.123 level=INFO service=opskeeper-api kind=service-start traceid=- spanid=- func=api.run file:line=main.go:181 msg="listen 127.0.0.1:8080 url=http://127.0.0.1:8080/opskeeper/"

# json
{"time":"2026-08-19T08:30:12.123","level":"INFO","service":"opskeeper-api","kind":"service-start","traceid":"-","spanid":"-","func":"api.run","file:line":"main.go:181","msg":"listen 127.0.0.1:8080 url=http://127.0.0.1:8080/opskeeper/"}
```

### 5.2 `http-request`

```text
# raw
2026-08-19T08:30:13.456 INFO opskeeper-api http-request 4bf92f3577b34da6a3ce929d0e0e4736 00f067aa0ba902b7 httpapi.requestLogger middleware.go:138 req-7f3a 192.0.2.10 GET /opskeeper/api/v1/teams 200 12ms

# text
time=2026-08-19T08:30:13.456 level=INFO service=opskeeper-api kind=http-request traceid=4bf92f3577b34da6a3ce929d0e0e4736 spanid=00f067aa0ba902b7 func=httpapi.requestLogger file:line=middleware.go:138 msg="reqid=req-7f3a clientip=192.0.2.10 method=GET path=/opskeeper/api/v1/teams status=200 duration=12ms"

# json
{"time":"2026-08-19T08:30:13.456","level":"INFO","service":"opskeeper-api","kind":"http-request","traceid":"4bf92f3577b34da6a3ce929d0e0e4736","spanid":"00f067aa0ba902b7","func":"httpapi.requestLogger","file:line":"middleware.go:138","msg":"reqid req-7f3a clientip 192.0.2.10 method GET path /opskeeper/api/v1/teams status 200 duration 12ms"}
```

在 `raw` 中，`http-request` 的消息严格按 `reqid clientip method path status duration` 排列；`text` 和 `json` 可以同时提供结构化的消息字段，但不得改变这些字段的语义或顺序约定。

### 5.3 `job`

```text
# raw
2026-08-19T08:30:14.789 INFO opskeeper-worker job - - worker.run jobs.go:81 inspection-run run-123 succeeded 842ms

# text
time=2026-08-19T08:30:14.789 level=INFO service=opskeeper-worker kind=job traceid=- spanid=- func=worker.run file:line=jobs.go:81 msg="inspection-run run-123 succeeded 842ms"

# json
{"time":"2026-08-19T08:30:14.789","level":"INFO","service":"opskeeper-worker","kind":"job","traceid":"-","spanid":"-","func":"worker.run","file:line":"jobs.go:81","msg":"inspection-run run-123 succeeded 842ms"}
```

## 6. 安全、隐私与可运维性

- 禁止记录密码、Cookie、Token、Authorization Header、数据库/Redis 连接串、密钥、个人敏感信息和凭据明文。
- 请求 URL 只记录路径；查询参数、请求正文和响应正文默认不记录，确需诊断时必须先脱敏并限制长度。
- 外部系统返回内容、用户输入和模型输出均视为不可信文本，必须限制长度并转义控制字符。
- 日志必须保持单行；换行、制表符和不可见控制字符应被转义或替换，避免伪造日志行。
- `kind`、服务名和状态值应保持低基数；不要把用户输入、资源名称或完整错误文本直接作为标签字段。
- 错误日志应包含可执行的错误分类和必要上下文；详细诊断信息放在受控的错误详情或追踪系统中。

## 7. 配置与验收

运行时通过环境变量切换格式：

```bash
OPSK_LOG_FORMAT=raw   # 默认
OPSK_LOG_FORMAT=text
OPSK_LOG_FORMAT=json
```

实现或修改日志时，至少验证：

- 三种格式均为单行，时间符合 `yyyy-MM-ddTHH:mm:ss.ms`；
- `raw` 的公共字段顺序和 `http-request` 消息顺序可稳定解析；
- 无 Trace/Span 时输出 `-`，不会把 `reqid` 冒充为 `traceid`；
- `func`、`file:line` 指向直接日志调用位置；
- 敏感值、控制字符和多行外部内容经过过滤；
- 默认未设置 `OPSK_LOG_FORMAT` 时输出 `raw`。

当前代码的日志 Handler 尚未完成三种格式的统一接入时，应在对应迭代任务中明确记录实现差距；在接入完成前，不得宣称运行时已经满足本规范。
