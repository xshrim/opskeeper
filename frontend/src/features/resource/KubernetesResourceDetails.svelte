<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;

  const tools: Array<[string, string]> = [
    ['kubernetes_cluster_info', '读取 Kubernetes 版本和连接信息'],
    ['kubernetes_api_resources', '列出当前集群支持的 API 资源'],
    ['kubernetes_namespaces', '列出 Kubernetes 命名空间'],
    ['kubernetes_nodes', '列出 Kubernetes 节点'],
    ['kubernetes_pods', '列出 Kubernetes Pod'],
    ['kubernetes_services', '列出 Kubernetes Service'],
    ['kubernetes_configmaps', '列出 Kubernetes ConfigMap'],
    ['kubernetes_ingresses', '列出 Kubernetes Ingress'],
    ['kubernetes_endpoint_slices', '列出 Kubernetes EndpointSlice'],
    ['kubernetes_events', '列出 Kubernetes 事件'],
    ['kubernetes_workloads', '列出 Kubernetes 工作负载'],
    ['kubernetes_resource_search', '按关键字搜索 Kubernetes 资源名称'],
    ['kubernetes_workload_pods', '解析指定工作负载正在运行的 Pod'],
    ['kubernetes_pod_stat', '读取 Kubernetes Metrics API 中的 Pod CPU 和内存使用情况'],
    ['kubernetes_node_stat', '读取 Kubernetes Metrics API 中的节点 CPU 和内存使用情况'],
    ['kubernetes_pod_logs', '读取限定大小且不持续跟随的 Pod 日志'],
    ['kubernetes_pod_file', '读取 Kubernetes Pod 中的限定大小普通文件'],
    ['kubernetes_resource_get', '按名称获取允许访问的 Kubernetes 资源'],
    ['kubernetes_health', '检查 Kubernetes API 健康状态']
  ];

  $: accessMode = String(resource.subtype ?? '').toLowerCase() === 'agent' ? 'agent' : 'direct';
  $: connectionModeKey = String(resource.config?.connection_mode ?? (resource.config?.server ? 'endpoint' : 'kubeconfig')).toLowerCase() === 'endpoint' ? 'endpoint' : 'kubeconfig';
  $: connectionMode = connectionModeKey === 'endpoint' ? 'Endpoint' : 'Kubeconfig';
  $: hasCustomAgentConnection = accessMode === 'agent' && Boolean(resource.config?.connection_mode || resource.config?.server);
  $: accessModeDisplay = accessMode === 'agent' && !hasCustomAgentConnection
    ? 'Agent'
    : `${accessMode === 'agent' ? 'Agent' : 'Direct'}·${connectionMode}`;
  $: transportCredential = accessMode === 'agent' && !resource.credential_configured
    ? '由 MCPServer 管理'
    : resource.credential_configured
      ? connectionModeKey === 'kubeconfig' ? '包含在 kubeconfig 中' : 'TLS/Token 已配置'
      : '未配置凭据';
  $: connectionCredential = accessMode === 'agent' && !resource.credential_configured
    ? '由 MCPServer 管理'
    : resource.credential_configured
      ? connectionModeKey === 'kubeconfig' ? 'Kubeconfig 已配置' : 'Endpoint 凭据已配置'
      : '未配置凭据';
  $: endpoint = accessMode === 'agent'
    ? mcpServerEndpoint || '关联 MCPServer'
    : connectionModeKey === 'kubeconfig'
      ? String(resourceCheck?.endpoint ?? '').trim() || '未获取 Kubernetes API Server'
      : String(resource.config?.server ?? '未设置 Kubernetes API Server');
  $: connectionStatus = resourceCheck
    ? resourceCheck.status === 'succeeded' ? `正常·${resourceCheck.latency_ms}ms` : '异常'
    : '尚未测试';
  $: enabledStatus = resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知';
</script>

<div class="provider-resource-details docker-resource-details">
  <div class="provider-resource-meta">
    <div><span>接入方式</span><strong class:agent={accessMode === 'agent'}>{accessModeDisplay}</strong></div>
    <div><span>连接端点</span><strong>{endpoint}</strong></div>
    <div><span>工具数量</span><strong>{tools.length} 个</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>连接凭据</span><strong>{connectionCredential}</strong></div>
    <div><span>传输凭据</span><strong>{transportCredential}</strong></div>
    <div class="provider-resource-labels"><span>标签</span><strong>{Object.entries(resource.labels ?? {}).map(([key, value]) => value ? `${key}=${value}` : key).join(', ') || '未设置标签'}</strong></div>
    <div class="provider-resource-connection"><span>状态</span><strong>{connectionStatus} ({enabledStatus})</strong>{#if resourceCheck?.status === 'failed'}<small>{resourceCheck.message}</small>{/if}</div>
  </div>
  <div class="provider-resource-models mcp-resource-tools">
    <div class="provider-resource-models-heading"><strong>工具列表</strong><span>{tools.length} 个</span></div>
    {#each tools as [name, description]}
      <div class="provider-resource-model-row mcp-resource-tool-row"><div><span>工具名称</span><strong>{name}</strong></div><div class="mcp-tool-description"><span>工具说明</span><strong>{description}</strong></div></div>
    {/each}
  </div>
</div>
