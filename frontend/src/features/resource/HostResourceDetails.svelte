<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor } from './resourceCatalog';
  import { hostAccessSummary } from './resourceWorkflow';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;

  $: config = resource.config ?? {};
  $: subtype = String(resource.subtype ?? 'Direct');
  $: accessMode = subtype.toLowerCase() === 'agent' ? 'agent' : 'direct';
  $: accessSummary = hostAccessSummary(subtype, config);
  $: connectionEndpoint =
    accessMode === 'agent'
      ? mcpServerEndpoint || '关联 MCPServer'
      : resourceEndpointFor(resource);
  $: connectionStatus = resourceCheck
    ? resourceCheck.status === 'succeeded'
      ? `正常·${resourceCheck.latency_ms}ms`
      : '异常'
    : '尚未测试';
</script>

<div class="provider-resource-details host-resource-details">
  <div class="provider-resource-meta">
    <div>
      <span>接入方式</span><strong>{accessSummary}</strong>
    </div>
    <div><span>连接端点</span><strong>{connectionEndpoint}</strong></div>
    <div><span>工具数量</span><strong>5 个</strong></div>
    <div>
      <span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong>
    </div>
    <div>
      <span>连接凭据</span><strong
        >{resource.credential_configured ? '已配置' : '未配置'}</strong
      >
    </div>
    <div>
      <span>超时时间</span><strong
        >{Number(config.timeout_seconds ?? 30)} 秒</strong
      >
    </div>
    <div>
      <span>标签</span><strong
        >{Object.entries(resource.labels ?? {})
          .map(([key, value]) => (value ? `${key}=${value}` : key))
          .join(', ') || '未设置标签'}</strong
      >
    </div>
    <div class="provider-resource-connection">
      <span>状态</span><strong>{connectionStatus}</strong
      >{#if resourceCheck?.status === 'failed'}<small
          >{resourceCheck.message}</small
        >{/if}
    </div>
  </div>
</div>
