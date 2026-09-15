# 已知限制与容量边界

- 首版为单地域模块化单体，不提供跨地域容灾、商业多租户和计费。
- API 单请求正文默认上限 2 MiB，JSON 业务对象仍使用更严格的 1 MiB 解码上限。单客户端默认限制为每分钟 600 请求。
- 诊断会话最多 20 个目标；Connector 默认单进程并发 8、超时 10 秒、响应 4 MiB。增大限制前必须执行负载验证。
- Scheduler 建议单副本；Worker 可水平扩展，但数据库连接和外部系统配额是共享瓶颈。
- 默认 PostgreSQL 同时承担数据库、授权缓存和 Repository Bundle 存储；高峰期缓存回源或 Bundle 读写会增加数据库负载。可按部署规模切换 Memory 或 Redis 缓存、Local 或 S3 Bundle 存储。
- `OPSK_CREDENTIAL_KEY` 尚无在线双密钥重加密工具，不得直接替换。
- 内置 Skill 仍是简短 Instruction 和受控 Tool 清单；Markdown 编辑、不可变发布流程和细粒度 Tool Catalog 由 OI-006 跟踪，不属于 T15。
- 不支持无人值守高风险处置、任意 Shell/SQL 和任意生产脚本。
