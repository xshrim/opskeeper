export type ResourceTool = {
  name: string;
  description: string;
};

const tool = (name: string, description: string): ResourceTool => ({
  name,
  description
});

const toolsByKind: Record<string, ResourceTool[]> = {
  Docker: [
    tool('docker_info', '读取 Docker Engine 信息'),
    tool('docker_images', '读取镜像列表'),
    tool('docker_containers', '读取容器列表'),
    tool('docker_container_logs', '读取容器日志'),
    tool('docker_container_inspect', '读取容器详情'),
    tool('docker_container_stats', '读取容器统计')
  ],
  Kubernetes: [
    tool('kubernetes_cluster_info', '读取 Kubernetes 版本和连接信息'),
    tool('kubernetes_api_resources', '列出当前集群支持的 API 资源'),
    tool('kubernetes_namespaces', '列出 Kubernetes 命名空间'),
    tool('kubernetes_nodes', '列出 Kubernetes 节点'),
    tool('kubernetes_pods', '列出 Kubernetes Pod'),
    tool('kubernetes_services', '列出 Kubernetes Service'),
    tool('kubernetes_configmaps', '列出 Kubernetes ConfigMap'),
    tool('kubernetes_ingresses', '列出 Kubernetes Ingress'),
    tool('kubernetes_endpoint_slices', '列出 Kubernetes EndpointSlice'),
    tool('kubernetes_events', '列出 Kubernetes 事件'),
    tool('kubernetes_workloads', '列出 Kubernetes 工作负载'),
    tool('kubernetes_resource_search', '按关键字搜索 Kubernetes 资源名称'),
    tool('kubernetes_workload_pods', '解析指定工作负载正在运行的 Pod'),
    tool(
      'kubernetes_pod_stat',
      '读取 Kubernetes Metrics API 中的 Pod CPU 和内存使用情况'
    ),
    tool(
      'kubernetes_node_stat',
      '读取 Kubernetes Metrics API 中的节点 CPU 和内存使用情况'
    ),
    tool('kubernetes_pod_logs', '读取限定大小且不持续跟随的 Pod 日志'),
    tool('kubernetes_pod_file', '读取 Kubernetes Pod 中的限定大小普通文件'),
    tool('kubernetes_resource_get', '按名称获取允许访问的 Kubernetes 资源'),
    tool('kubernetes_health', '检查 Kubernetes API 健康状态')
  ],
  Host: [
    tool('host_info', '读取主机信息'),
    tool('host_metrics', '读取主机指标'),
    tool('host_processes', '读取主机进程'),
    tool('host_file_logs', '读取主机文件日志'),
    tool('host_health', '检查主机健康')
  ],
  PostgreSQL: [
    tool('postgresql_health', '检查 PostgreSQL 连接和服务器健康状态'),
    tool('postgresql_sessions', '读取 PostgreSQL 活跃会话数量'),
    tool(
      'postgresql_long_running_queries',
      '读取运行超过五秒的 PostgreSQL 活跃查询'
    ),
    tool('postgresql_locks', '读取等待授予的 PostgreSQL 锁'),
    tool('postgresql_replication', '读取已连接的 PostgreSQL 副本数量'),
    tool('postgresql_capacity', '读取当前数据库大小'),
    tool('postgresql_tables', '列出用户表及其大小、行数和读写统计'),
    tool('postgresql_table_columns', '读取指定用户表的列、类型、默认值和注释'),
    tool('postgresql_performance', '读取 PostgreSQL 性能计数器和配置参数'),
    tool('postgresql_vacuum', '读取 PostgreSQL VACUUM 和自动清理配置'),
    tool('postgresql_extensions', '列出已安装及可用的 PostgreSQL 扩展和版本'),
    tool('postgresql_database_info', '读取 PostgreSQL 数据库身份和概要信息')
  ],
  MySQL: [
    tool('mysql_health', '检查 MySQL 连接和服务器健康状态'),
    tool('mysql_status', '读取 MySQL 服务器状态计数器'),
    tool('mysql_performance', '读取 MySQL 性能计数器和配置参数'),
    tool('mysql_tables', '列出用户表及其行数、大小和元数据'),
    tool('mysql_table_columns', '读取指定用户表的列、类型、默认值和注释'),
    tool('mysql_table_structure', '读取指定用户表的建表定义和索引'),
    tool('mysql_database_info', '读取 MySQL 数据库身份和概要信息')
  ],
  Oracle: [
    tool('oracle_health', '检查 Oracle 连接和实例健康状态'),
    tool('oracle_status', '读取 Oracle 实例和会话状态'),
    tool('oracle_performance', '读取 Oracle 性能计数器和配置参数'),
    tool('oracle_database_info', '读取 Oracle 数据库身份、角色、模式和字符集'),
    tool('oracle_tables', '列出用户表及其统计信息、表空间和段大小'),
    tool('oracle_table_columns', '读取指定用户表的列、类型、默认值和注释'),
    tool('oracle_table_structure', '读取指定用户表的约束和索引')
  ],
  OceanBase: [
    tool('oceanbase_health', '检查 OceanBase 连接和服务器健康状态'),
    tool('oceanbase_status', '读取 OceanBase 服务器状态计数器'),
    tool('oceanbase_performance', '读取 OceanBase 性能计数器和配置'),
    tool('oceanbase_database_info', '读取 OceanBase 数据库身份和配置'),
    tool('oceanbase_tables', '列出 OceanBase 用户表及其行数和大小'),
    tool('oceanbase_table_columns', '读取指定表的列、类型、默认值和注释'),
    tool('oceanbase_table_structure', '读取指定表的索引和约束')
  ],
  RabbitMQ: [
    tool('rabbitmq_health', '检查 RabbitMQ Management API 健康状态和概要'),
    tool('rabbitmq_overview', '读取 RabbitMQ 集群概要和消息总量'),
    tool('rabbitmq_nodes', '列出 RabbitMQ 节点和资源使用情况'),
    tool('rabbitmq_queues', '列出 RabbitMQ 队列及消息速率'),
    tool('rabbitmq_exchanges', '列出 RabbitMQ 交换机'),
    tool('rabbitmq_connections', '列出 RabbitMQ 客户端连接'),
    tool('rabbitmq_channels', '列出 RabbitMQ 通道和消费者数量'),
    tool('rabbitmq_consumers', '列出 RabbitMQ 消费者'),
    tool('rabbitmq_vhosts', '列出 RabbitMQ 虚拟主机')
  ],
  MinIO: [
    tool('minio_health', '检查 MinIO 连接并返回存储桶概要'),
    tool('minio_buckets', '列出当前凭证可见的 MinIO 存储桶'),
    tool('minio_bucket_objects', '列出 MinIO 存储桶中的部分对象'),
    tool('minio_object_stat', '读取指定 MinIO 对象的元数据'),
    tool('minio_bucket_versioning', '读取指定存储桶的版本配置'),
    tool('minio_bucket_lifecycle', '读取指定存储桶的生命周期配置')
  ],
  Redis: [
    tool('redis_health', '检查 Redis 连接和服务器健康状态'),
    tool('redis_memory', '读取 Redis 内存统计'),
    tool('redis_clients', '读取 Redis 客户端统计'),
    tool('redis_replication', '读取 Redis 复制状态'),
    tool('redis_slowlog', '读取最近的 Redis 慢日志条目'),
    tool('redis_database_info', '读取 Redis 键空间和指定数据库信息')
  ],
  Nacos: [
    tool('nacos_server_state', '读取 Nacos 服务器状态和指标'),
    tool('nacos_namespaces', '列出 Nacos 命名空间'),
    tool('nacos_services', '列出已注册的 Nacos 服务'),
    tool('nacos_service_instances', '列出指定 Nacos 服务的实例'),
    tool('nacos_configs', '列出 Nacos 配置元数据'),
    tool('nacos_config_detail', '读取指定 Nacos 配置内容')
  ],
  Kafka: [
    tool('kafka_health', '检查 Kafka 集群健康状态并返回概要'),
    tool('kafka_brokers', '列出 Kafka Broker'),
    tool('kafka_topics', '列出 Kafka Topic 指标'),
    tool('kafka_consumer_groups', '列出 Kafka 消费组'),
    tool('kafka_consumer_lag', '读取 Kafka 消费积压'),
    tool('kafka_cluster_info', '读取 Kafka 控制器和 Broker 元数据'),
    tool('kafka_topic_partitions', '读取指定 Kafka Topic 的分区')
  ],
  Elasticsearch: [
    tool('elasticsearch_health', '读取 Elasticsearch 集群健康状态'),
    tool('elasticsearch_cluster_info', '读取 Elasticsearch 集群元数据'),
    tool('elasticsearch_nodes', '列出 Elasticsearch 节点和角色'),
    tool('elasticsearch_indices', '列出用户索引及存储、文档指标'),
    tool('elasticsearch_shards', '读取 Elasticsearch 分片分配摘要'),
    tool('elasticsearch_settings', '读取选定的 Elasticsearch 集群配置'),
    tool('elasticsearch_index_mapping', '读取指定索引的映射')
  ],
  ElasticSearch: [
    tool('elasticsearch_health', '读取 Elasticsearch 集群健康状态'),
    tool('elasticsearch_cluster_info', '读取 Elasticsearch 集群元数据'),
    tool('elasticsearch_nodes', '列出 Elasticsearch 节点和角色'),
    tool('elasticsearch_indices', '列出用户索引及存储、文档指标'),
    tool('elasticsearch_shards', '读取 Elasticsearch 分片分配摘要'),
    tool('elasticsearch_settings', '读取选定的 Elasticsearch 集群配置'),
    tool('elasticsearch_index_mapping', '读取指定索引的映射')
  ]
};

export function resourceToolsFor(kind: string): ResourceTool[] {
  return toolsByKind[kind] ?? [];
}
