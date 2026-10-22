UPDATE resource_schemas
SET schema = jsonb_set(schema, '{properties,url,description}', '"MCP 服务地址。开发环境可使用本地 HTTP；生产环境建议启用增强安全策略。"'::jsonb, true),
    description = '通过 Streamable HTTP 接入 MCP 服务；是否启用 HTTPS、公网地址和受限网络校验由 OPSK_MCP_ENHANCED_SECURITY 控制。'
WHERE kind = 'MCPServer' AND version = 3;
