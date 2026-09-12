<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor } from './resourceCatalog';
  import { hostAccessSummary } from './resourceWorkflow';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;

  const tools = [
    ['host_info', '读取主机信息'],
    ['host_metrics', '读取主机指标'],
    ['host_processes', '读取主机进程'],
    ['host_file_logs', '读取主机文件日志'],
    ['host_health', '检查主机健康']
  ];

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
  $: enabledStatus =
    resource.status === 'active'
      ? '已启用'
      : resource.status === 'disabled'
        ? '已停用'
        : '未知';
</script>

<div class="provider-resource-details host-resource-details">
  <div class="provider-resource-meta">
    <div>
      <span>接入方式</span><strong>{accessSummary}</strong>
    </div>
    <div><span>连接端点</span><strong>{connectionEndpoint}</strong></div>
    <div><span>工具数量</span><strong>{tools.length} 个</strong></div>
    <div>
      <span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong>
    </div>
    <div>
      <span>连接凭据</span><strong
        >{resource.credential_id ? '已配置' : '未配置'}</strong
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
      <span>状态</span><strong>{connectionStatus} ({enabledStatus})</strong
      >{#if resourceCheck?.status === 'failed'}<small
          >{resourceCheck.message}</small
        >{/if}
    </div>
  </div>
  <div class="provider-resource-models mcp-resource-tools">
    <div class="provider-resource-models-heading">
      <strong>工具列表</strong><span>{tools.length} 个</span>
    </div>
    {#each tools as [name, description]}
      <div class="provider-resource-model-row mcp-resource-tool-row">
        <div><span>工具名称</span><strong>{name}</strong></div>
        <div class="mcp-tool-description">
          <span>工具说明</span><strong>{description}</strong>
        </div>
      </div>
    {/each}
  </div>
</div>
