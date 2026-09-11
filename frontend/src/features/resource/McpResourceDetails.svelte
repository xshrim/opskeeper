<script lang="ts">
  import type { MCPSnapshot, Resource } from '../../lib/api';
  import { resourceEndpointFor, resourceLabelsText } from './resourceCatalog';
  export let resource: Resource;
  export let snapshots: MCPSnapshot[] = [];
  export let formatDate: (value: string) => string;

  $: config = resource.config ?? {};
  $: snapshot = snapshots[0];
  $: connectionStatus = snapshot ? snapshot.status === 'succeeded' ? `正常·${snapshot.latency_ms ?? 0}ms` : '异常' : '尚未测试';
  $: enabledStatus = resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知';
</script>

<div class="provider-resource-details mcp-resource-details">
  <div class="provider-resource-meta">
    <div><span>传输方式</span><strong>{String(config.transport ?? 'streamable_http') === 'sse' ? 'SSE' : 'Streamable HTTP'}</strong></div>
    <div class="provider-resource-address"><span>Server 地址</span><strong>{resourceEndpointFor(resource)}</strong></div>
    <div><span>工具数量</span><strong>{snapshot?.tools?.length ?? 0} 个</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>超时时间</span><strong>{Number(config.timeout_seconds ?? 120)} 秒</strong></div>
    <div><span>响应体大小限制</span><strong>{Math.round(Number(config.max_response_bytes ?? 4 * 1024 * 1024) / 1024 / 1024)} MiB</strong></div>
    <div><span>自定义 Header</span><strong>{resource.credential_id ? '已配置' : '未配置'}</strong></div>
    <div><span>工具白名单</span><strong>{Array.isArray(config.tool_allowlist) && config.tool_allowlist.length ? `${config.tool_allowlist.length} 条规则` : '不限制'}</strong></div>
    <div><span>连接凭据</span><strong>{resource.credential_id ? '已配置 Token / Header' : '未配置'}</strong></div>
    <div><span>传输凭据</span><strong>{resource.credential_id ? '随连接凭据保存' : '未配置'}</strong></div>
    <div class="provider-resource-labels"><span>标签</span><strong>{resourceLabelsText(resource) || '未设置标签'}</strong></div>
    <div><span>状态</span><strong>{connectionStatus} ({enabledStatus})</strong></div>
  </div>
  <div class="provider-resource-models mcp-resource-tools">
    <div class="provider-resource-models-heading"><strong>工具列表</strong><span>{snapshot?.tools?.length ?? 0} 个</span></div>
    {#each snapshot?.tools ?? [] as tool}<div class="provider-resource-model-row mcp-resource-tool-row"><div><span>工具名称</span><strong>{tool.name}</strong></div><div class="mcp-tool-description"><span>工具说明</span><strong>{tool.description || '未提供说明'}</strong></div></div>
    {:else}<div class="empty-state">尚未发现工具，请先执行连接测试。</div>{/each}
  </div>
</div>
