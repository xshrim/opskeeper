<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import ConnectionTest from '../../components/ConnectionTest.svelte';
  import TagList from '../../components/TagList.svelte';
  import { labelTags } from '../../lib/labels';
  import { resourceEndpointFor } from './resourceCatalog';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let mcpServerEndpoint = '';
  export let formatDate: (value: string) => string;

  $: accessMode =
    String(resource.subtype ?? 'direct').toLowerCase() === 'agent'
      ? 'agent'
      : 'direct';
  $: tags = labelTags(resource.labels);
</script>

<div class="provider-resource-details docker-resource-details">
  <div class="provider-resource-meta">
    <div>
      <span>接入方式</span><strong class:agent={accessMode === 'agent'}
        >{accessMode === 'agent' ? 'Agent' : 'Direct'}</strong
      >
    </div>
    <div>
      <span>连接端点</span><strong
        >{accessMode === 'direct'
          ? resourceEndpointFor(resource)
          : mcpServerEndpoint || '关联 MCPServer'}</strong
      >
    </div>
    <div><span>工具数量</span><strong>6 个</strong></div>
    <div>
      <span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong>
    </div>
    <div><span>连接凭据</span><strong>未配置</strong></div>
    <div>
      <span>传输凭据</span><strong
        >{accessMode === 'agent'
          ? '由 MCPServer 管理'
          : resource.credential_configured
            ? 'TLS 凭据已配置'
            : '未配置 TLS 凭据'}</strong
      >
    </div>
    <div class="provider-resource-labels">
      <span>标签</span><TagList {tags} emptyText="未设置标签" />
    </div>
    <div class="provider-resource-connection">
      <span>状态</span><ConnectionTest
        status={resourceCheck?.status ?? ''}
        latencyMs={resourceCheck?.latency_ms ?? 0}
        message={resourceCheck?.message ?? ''}
        disabled
        showTime={false}
        showTooltip={false}
      />
    </div>
  </div>
</div>
