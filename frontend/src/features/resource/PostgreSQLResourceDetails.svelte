<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor } from './resourceCatalog';
  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;
	const tools = ['postgresql_health','postgresql_sessions','postgresql_long_running_queries','postgresql_locks','postgresql_replication','postgresql_capacity','postgresql_tables','postgresql_table_columns','postgresql_performance','postgresql_vacuum','postgresql_extensions','postgresql_database_info'];
  $: agent = String(resource.subtype ?? '').toLowerCase() === 'agent';
  $: endpoint = agent ? mcpServerEndpoint || '关联 MCPServer' : resourceEndpointFor(resource);
  $: status = resourceCheck ? resourceCheck.status === 'succeeded' ? `正常·${resourceCheck.latency_ms}ms` : '异常' : '尚未测试';
</script>
<div class="provider-resource-details postgres-resource-details">
  <div class="provider-resource-meta">
    <div><span>接入方式</span><strong class:agent>{agent ? 'Agent · MCP 代理' : 'Direct'}</strong></div>
    <div><span>连接端点</span><strong>{endpoint}</strong></div>
    <div><span>数据库</span><strong>{String(resource.config?.database ?? '未设置')}</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>连接凭据</span><strong>{agent ? '由 MCPServer 管理' : resource.credential_id ? '已配置' : '未配置'}</strong></div>
    <div><span>状态</span><strong>{status} ({resource.status === 'active' ? '已启用' : '已停用'})</strong>{#if resourceCheck?.status === 'failed'}<small>{resourceCheck.message}</small>{/if}</div>
  </div>
  <div class="provider-resource-models mcp-resource-tools"><div class="provider-resource-models-heading"><strong>工具列表</strong><span>{tools.length} 个</span></div>{#each tools as tool}<div class="provider-resource-model-row mcp-resource-tool-row"><div><strong>{tool}</strong></div></div>{/each}</div>
</div>
