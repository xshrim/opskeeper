UPDATE resource_schemas
   SET display_name = CASE kind
       WHEN 'Artifact' THEN '制品仓库'
       WHEN 'Repository' THEN '代码仓库'
       WHEN 'Nacos' THEN '注册中心和配置中心'
       WHEN 'Nginx' THEN '前端代理'
       WHEN 'TongHttpServer' THEN '前端代理'
       WHEN 'Kafka' THEN '消息队列'
       WHEN 'RabbitMQ' THEN '消息队列'
       WHEN 'Elasticsearch' THEN '文档检索'
       WHEN 'ElasticSearch' THEN '文档检索'
       WHEN 'MinIO' THEN '对象存储'
       WHEN 'MCPServer' THEN '工具接入'
       WHEN 'Monitor' THEN '可观察性'
       ELSE display_name
   END
 WHERE kind IN ('Artifact', 'Repository', 'Nacos', 'Nginx', 'TongHttpServer',
                'Kafka', 'RabbitMQ', 'Elasticsearch', 'ElasticSearch', 'MinIO',
                'MCPServer', 'Monitor');
