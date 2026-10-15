export type ResourceTool = {
  name: string;
  description: string;
};

const tool = (name: string, description = '只读诊断工具'): ResourceTool => ({ name, description });

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
    tool('kubernetes_pod_stat', '读取 Kubernetes Metrics API 中的 Pod CPU 和内存使用情况'),
    tool('kubernetes_node_stat', '读取 Kubernetes Metrics API 中的节点 CPU 和内存使用情况'),
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
  PostgreSQL: ['postgresql_health', 'postgresql_sessions', 'postgresql_long_running_queries', 'postgresql_locks', 'postgresql_replication', 'postgresql_capacity', 'postgresql_tables', 'postgresql_table_columns', 'postgresql_performance', 'postgresql_vacuum', 'postgresql_extensions', 'postgresql_database_info'].map((name) => tool(name)),
  MySQL: ['mysql_health', 'mysql_status', 'mysql_performance', 'mysql_tables', 'mysql_table_columns', 'mysql_table_structure', 'mysql_database_info'].map((name) => tool(name)),
  Oracle: ['oracle_health', 'oracle_status', 'oracle_performance', 'oracle_database_info', 'oracle_tables', 'oracle_table_columns', 'oracle_table_structure'].map((name) => tool(name)),
  OceanBase: ['oceanbase_health', 'oceanbase_status', 'oceanbase_performance', 'oceanbase_database_info', 'oceanbase_tables', 'oceanbase_table_columns', 'oceanbase_table_structure'].map((name) => tool(name)),
  RabbitMQ: ['rabbitmq_health', 'rabbitmq_overview', 'rabbitmq_nodes', 'rabbitmq_queues', 'rabbitmq_exchanges', 'rabbitmq_connections', 'rabbitmq_channels', 'rabbitmq_consumers', 'rabbitmq_vhosts'].map((name) => tool(name)),
  MinIO: ['minio_health', 'minio_buckets', 'minio_bucket_objects', 'minio_object_stat', 'minio_bucket_versioning', 'minio_bucket_lifecycle'].map((name) => tool(name)),
  Redis: ['redis_health', 'redis_memory', 'redis_clients', 'redis_replication', 'redis_slowlog', 'redis_database_info'].map((name) => tool(name)),
  Nacos: ['nacos_server_state', 'nacos_namespaces', 'nacos_services', 'nacos_service_instances', 'nacos_configs', 'nacos_config_detail'].map((name) => tool(name)),
  Kafka: ['kafka_health', 'kafka_brokers', 'kafka_topics', 'kafka_consumer_groups', 'kafka_consumer_lag', 'kafka_cluster_info', 'kafka_topic_partitions'].map((name) => tool(name)),
  Elasticsearch: ['elasticsearch_health', 'elasticsearch_cluster_info', 'elasticsearch_nodes', 'elasticsearch_indices', 'elasticsearch_shards', 'elasticsearch_settings', 'elasticsearch_index_mapping'].map((name) => tool(name)),
  ElasticSearch: ['elasticsearch_health', 'elasticsearch_cluster_info', 'elasticsearch_nodes', 'elasticsearch_indices', 'elasticsearch_shards', 'elasticsearch_settings', 'elasticsearch_index_mapping'].map((name) => tool(name))
};

export function resourceToolsFor(kind: string): ResourceTool[] {
  return toolsByKind[kind] ?? [];
}
