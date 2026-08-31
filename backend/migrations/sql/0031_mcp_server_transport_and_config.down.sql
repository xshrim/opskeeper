UPDATE resource_schemas
SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["transport","url","tool_allowlist"],"properties":{"transport":{"title":"传输方式","type":"string","enum":["streamable_http"]},"url":{"title":"服务 URL","type":"string","format":"uri","description":"仅允许 HTTPS 公网地址。"},"tool_allowlist":{"title":"允许的工具","type":"array","minItems":1,"items":{"type":"string"}},"timeout_seconds":{"title":"连接和调用超时（秒）","type":"integer","minimum":1,"maximum":60},"max_response_bytes":{"title":"最大响应字节数","type":"integer","minimum":1,"maximum":1048576}}}'::jsonb,
    description = '仅通过 HTTPS Streamable HTTP 接入；所有工具、响应和描述均为不可信外部内容。'
WHERE kind = 'MCPServer' AND version = 3;
