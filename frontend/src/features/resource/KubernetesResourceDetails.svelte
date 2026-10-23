<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import TagList from '../../components/TagList.svelte';
  import { labelTags } from '../../lib/labels';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;

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
  $: tags = labelTags(resource.labels);
</script>

<div class="provider-resource-details docker-resource-details">
  <div class="provider-resource-meta">
    <div><span>接入方式</span><strong class:agent={accessMode === 'agent'}>{accessModeDisplay}</strong></div>
    <div><span>连接端点</span><strong>{endpoint}</strong></div>
    <div><span>工具数量</span><strong>19 个</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>连接凭据</span><strong>{connectionCredential}</strong></div>
    <div><span>传输凭据</span><strong>{transportCredential}</strong></div>
    <div class="provider-resource-labels"><span>标签</span><TagList tags={tags} emptyText="未设置标签" /></div>
    <div class="provider-resource-connection"><span>状态</span><strong>{connectionStatus}</strong>{#if resourceCheck?.status === 'failed'}<small>{resourceCheck.message}</small>{/if}</div>
  </div>
</div>
