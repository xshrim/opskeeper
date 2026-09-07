<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { dockerAccessModeLabel } from './resourceWorkflow';
  import { resourceEndpointFor } from './resourceCatalog';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerName = '';
  export let formatDate: (value: string) => string;
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;

  $: accessMode = String(resource.access_mode ?? resource.subtype ?? 'direct').toLowerCase() === 'agent' ? 'agent' : 'direct';
  $: tlsConfigured = Boolean(resource.credential_id);
</script>

<div class="docker-resource-details">
  <div class="docker-resource-meta">
    <div><span>接入方式</span><strong class:agent={accessMode === 'agent'}>{dockerAccessModeLabel(accessMode)}</strong></div>
    <div><span>连接端点</span><strong>{accessMode === 'direct' ? resourceEndpointFor(resource) : (mcpServerName || '关联 MCPServer')}</strong></div>
    <div><span>传输凭据</span><strong>{accessMode === 'direct' ? tlsConfigured ? 'TLS 凭据已关联' : '未配置 TLS 凭据' : '由 MCPServer 管理'}</strong></div>
    <div><span>连接状态</span><strong>{resourceCheck ? resourceCheck.status === 'succeeded' ? `正常 · ${resourceCheck.latency_ms} ms` : '连接失败' : accessMode === 'agent' ? '由 MCPServer 负责' : '尚未测试'}</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>管理范围</span><strong>{resourceCanManage(resource, 'resource:update') ? '当前 Scope 可管理' : '继承资源，仅限查看'}</strong></div>
  </div>
  <div class="docker-tool-list">
    <span>只读工具集</span>
    <div>
      {#each ['docker_info', 'docker_images', 'docker_containers', 'docker_container_logs', 'docker_container_inspect', 'docker_container_stats'] as tool}
        <code>{tool}</code>
      {/each}
    </div>
  </div>
</div>
