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
</script>

<div class="resource-row-details">
  <div><span>资源地址</span><strong>{isAgent ? mcpServerEndpoint || '关联 MCPServer' : resourceEndpointFor(resource)}</strong></div>
  <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
  <div>
    <span>连接状态</span>
    <strong>{selectedResourceId === resource.id && connectionCheck ? connectionCheck.status === 'succeeded' ? `连接正常 · ${connectionCheck.latency_ms} ms` : '连接异常' : '尚未测试'}</strong>
    {#if selectedResourceId === resource.id && connectionCheck?.status === 'failed'}<small>{connectionCheck.message}</small>{/if}
  </div>
  <div><span>管理范围</span><strong>{resourceCanManage(resource, 'resource:update') ? '当前 Scope 可管理' : '继承资源，仅限查看'}</strong></div>
</div>
