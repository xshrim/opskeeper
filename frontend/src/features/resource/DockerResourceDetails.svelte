<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { dockerAccessModeLabel } from './resourceWorkflow';
  import { resourceEndpointFor } from './resourceCatalog';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;

  $: accessMode = String(resource.subtype ?? 'direct').toLowerCase() === 'agent' ? 'agent' : 'direct';

  const tools = [
    ['docker_info', '读取 Docker Engine 信息'],
    ['docker_images', '读取镜像列表'],
    ['docker_containers', '读取容器列表'],
    ['docker_container_logs', '读取容器日志'],
    ['docker_container_inspect', '读取容器详情'],
    ['docker_container_stats', '读取容器统计']
  ];
</script>

<div class="provider-resource-details docker-resource-details">
  <div class="provider-resource-meta">
    <div><span>接入方式</span><strong class:agent={accessMode === 'agent'}>{dockerAccessModeLabel(accessMode)}</strong></div>
    <div><span>连接端点</span><strong>{accessMode === 'direct' ? resourceEndpointFor(resource) : (mcpServerEndpoint || '关联 MCPServer')}</strong></div>
    <div><span>工具数量</span><strong>6 个</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>传输凭据</span><strong>{accessMode === 'agent' ? '由 MCPServer 管理' : resource.credential_id ? 'TLS 凭据已关联' : '未配置 TLS 凭据'}</strong></div>
    <div class="provider-resource-labels"><span>标签</span><strong>{Object.entries(resource.labels ?? {}).map(([key, value]) => value ? `${key}=${value}` : key).join(', ') || '未设置标签'}</strong></div>
    <div class="provider-resource-connection"><span>连接状态</span><strong>{resourceCheck ? resourceCheck.status === 'succeeded' ? `正常 · ${resourceCheck.latency_ms} ms` : '连接异常' : accessMode === 'agent' ? '由 MCPServer 负责' : '尚未测试'}</strong>{#if resourceCheck?.status === 'failed'}<small>{resourceCheck.message}</small>{/if}</div>
    <div><span>启用状态</span><strong>{resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知'}</strong></div>
  </div>
  <div class="provider-resource-models mcp-resource-tools">
    <div class="provider-resource-models-heading"><strong>工具列表</strong><span>6 个</span></div>
    {#each tools as [name, description]}
      <div class="provider-resource-model-row mcp-resource-tool-row"><div><span>工具名称</span><strong>{name}</strong></div><div class="mcp-tool-description"><span>工具说明</span><strong>{description}</strong></div></div>
    {/each}
  </div>
</div>
