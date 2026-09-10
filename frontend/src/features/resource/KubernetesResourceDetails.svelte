<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { kubernetesAccessModeLabel } from './resourceWorkflow';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;

  const tools = [
    'kubernetes_cluster_info',
    'kubernetes_api_resources',
    'kubernetes_namespaces',
    'kubernetes_nodes',
    'kubernetes_pods',
    'kubernetes_workloads',
    'kubernetes_pod_stat',
    'kubernetes_node_stat',
    'kubernetes_services',
    'kubernetes_configmaps',
    'kubernetes_ingresses',
    'kubernetes_endpoint_slices',
    'kubernetes_events',
    'kubernetes_pod_logs',
    'kubernetes_resource_get',
    'kubernetes_health'
  ];

  $: accessMode = String(resource.subtype ?? '').toLowerCase() === 'agent' ? 'agent' : 'direct';
  $: transportCredential = accessMode === 'agent' && !resource.credential_id
    ? '由 MCPServer 管理'
    : resource.credential_id
      ? 'Kubernetes 凭据已关联'
      : '未配置凭据';
  $: connectionStatus = resourceCheck
    ? resourceCheck.status === 'succeeded' ? `正常·${resourceCheck.latency_ms}ms` : '异常'
    : '尚未测试';
  $: enabledStatus = resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知';
</script>

<div class="provider-resource-details docker-resource-details">
  <div class="provider-resource-meta">
    <div><span>接入方式</span><strong class:agent={accessMode === 'agent'}>{kubernetesAccessModeLabel(accessMode)}</strong></div>
    <div><span>连接端点</span><strong>{accessMode === 'agent' ? mcpServerEndpoint || '关联 MCPServer' : String(resource.config?.server ?? '默认 kubeconfig')}</strong></div>
    <div><span>工具数量</span><strong>{tools.length} 个</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>服务凭据</span><strong>未配置</strong></div>
    <div><span>传输凭据</span><strong>{transportCredential}</strong></div>
    <div class="provider-resource-labels"><span>标签</span><strong>{Object.entries(resource.labels ?? {}).map(([key, value]) => value ? `${key}=${value}` : key).join(', ') || '未设置标签'}</strong></div>
    <div class="provider-resource-connection"><span>状态</span><strong>{connectionStatus} ({enabledStatus})</strong>{#if resourceCheck?.status === 'failed'}<small>{resourceCheck.message}</small>{/if}</div>
  </div>
  <div class="provider-resource-models mcp-resource-tools">
    <div class="provider-resource-models-heading"><strong>工具列表</strong><span>{tools.length} 个</span></div>
    {#each tools as tool}
      <div class="provider-resource-model-row mcp-resource-tool-row"><div><strong>{tool}</strong></div></div>
    {/each}
  </div>
</div>
