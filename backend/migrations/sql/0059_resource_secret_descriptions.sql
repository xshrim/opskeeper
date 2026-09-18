UPDATE resource_schemas
   SET description = '通过 Streamable HTTP 或 SSE 接入 MCP 服务。Token 加密后保存在资源本身；工具白名单支持通配符，空白表示不限制。'
 WHERE kind = 'MCPServer'
   AND version = 3;

UPDATE resource_schemas
   SET description = '模型服务连接、资源连接密文和模型目录。'
 WHERE kind = 'AIProvider';
