<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor } from './resourceCatalog';
  export let resource: Resource;
  export let selectedResourceId = '';
  export let connectionCheck: ConnectionCheck | null = null;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;

  $: isAgent = String(resource.subtype ?? '').toLowerCase() === 'agent';
  $: enabledStatus = resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知';
</script>

<div class="resource-row-details">
  <div><span>资源地址</span><strong>{isAgent ? mcpServerEndpoint || '关联 MCPServer' : resourceEndpointFor(resource)}</strong></div>
  <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
  <div><span>接入方式</span><strong>{isAgent ? 'Agent · MCP 代理' : 'Direct'}</strong></div>
  <div><span>管理范围</span><strong>{resourceCanManage(resource, 'resource:update') ? '当前 Scope 可管理' : '继承资源，仅限查看'}</strong></div>
  <div><span>服务凭据</span><strong>{resource.credential_id ? '已配置' : '未配置'}</strong></div>
  <div><span>传输凭据</span><strong>未配置</strong></div>
  <div><span>标签</span><strong>{Object.entries(resource.labels ?? {}).map(([key, value]) => value ? `${key}=${value}` : key).join(', ') || '未设置标签'}</strong></div>
  <div>
    <span>状态</span>
    <strong>{selectedResourceId === resource.id && connectionCheck ? connectionCheck.status === 'succeeded' ? `正常·${connectionCheck.latency_ms}ms` : '异常' : '尚未测试'} ({enabledStatus})</strong>
    {#if selectedResourceId === resource.id && connectionCheck?.status === 'failed'}<small>{connectionCheck.message}</small>{/if}
  </div>
</div>
