<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor } from './resourceCatalog';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';

  $: agent = String(resource.subtype ?? '').toLowerCase() === 'agent';
  $: status = resourceCheck?.status === 'succeeded'
    ? `正常·${resourceCheck.latency_ms}ms`
    : resourceCheck?.status === 'failed' ? '异常' : '尚未测试';
</script>

<div class="provider-resource-details postgres-resource-details">
  <div class="provider-resource-meta">
    <div><span>接入方式</span><strong class:agent>{agent ? 'Agent · MCP 代理' : 'Direct'}</strong></div>
    <div><span>连接端点</span><strong>{agent ? mcpServerEndpoint || '关联 MCPServer' : resourceEndpointFor(resource)}</strong></div>
    <div><span>状态</span><strong>{status}</strong></div>
  </div>
</div>
