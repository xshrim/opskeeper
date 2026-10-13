<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor } from './resourceCatalog';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;

  $: agent = String(resource.subtype ?? '').toLowerCase() === 'agent';
  $: endpoint = agent ? mcpServerEndpoint || '关联 MCPServer' : resourceEndpointFor(resource);
  $: status = resourceCheck ? resourceCheck.status === 'succeeded' ? `正常·${resourceCheck.latency_ms}ms` : '异常' : '尚未测试';
</script>

<div class="provider-resource-details postgres-resource-details">
  <div class="provider-resource-meta">
    <div><span>接入方式</span><strong class:agent>{agent ? 'Agent · MCP 代理' : 'Direct · S3 API'}</strong></div>
    <div><span>连接端点</span><strong>{endpoint}</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>连接凭据</span><strong>{agent ? '由 MCPServer 管理' : resource.credential_configured ? '已配置' : '未配置'}</strong></div>
    <div><span>状态</span><strong>{status}</strong>{#if resourceCheck?.status === 'failed'}<small>{resourceCheck.message}</small>{/if}</div>
  </div>
</div>
