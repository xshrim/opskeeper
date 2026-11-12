<script lang="ts">
  import ConnectionTest from '../../components/ConnectionTest.svelte';
  import { hostAccessModeLabel, type HostAccessMode } from './resourceWorkflow';
  export let resourceName = '';
  export let accessMode: HostAccessMode = 'direct';
  export let connectionOverride = false;
  export let host = '';
  export let authMethod: 'password' | 'key' = 'password';
  export let credentialConfigured = false;
  export let mcpServerName = '';
  export let scopeSummary = '';
  export let labelsConfigured = false;
  export let testBusy = false;
  export let testStatus = '';
  export let testMessage = '';
  export let testError = '';
  export let testLatency: number | undefined;
  export let onSubmit: () => void = () => {};
  const tools = [
    'host_info',
    'host_metrics',
    'host_processes',
    'host_file_logs',
    'host_health'
  ];
</script>

<form
  id="host-review-form"
  class="provider-summary docker-summary"
  on:submit|preventDefault={onSubmit}
>
  <div>
    <span>Host 资源</span><strong>{resourceName}</strong><small
      >{scopeSummary}</small
    >
  </div>
  <div>
    <span>接入方式</span><strong>{hostAccessModeLabel(accessMode)}</strong
    ><small
      >{accessMode === 'direct'
        ? host || '本机 Linux'
        : `${mcpServerName || '未选择 MCPServer'}${connectionOverride ? ' · 自定义 SSH 连接' : ''}`}</small
    >
  </div>
  <div>
    <span>连接凭据</span><strong
      >{accessMode === 'direct' || connectionOverride
        ? credentialConfigured
          ? `${authMethod === 'key' ? 'SSH 私钥' : 'SSH 密码'} 已加密保存`
          : host
            ? '未配置 SSH 凭据'
            : '本机无需 SSH 凭据'
        : '由 MCPServer 环境管理'}</strong
    ><small>敏感值不会暴露给模型</small>
  </div>
  <div>
    <span>资源属性</span><strong
      >{labelsConfigured ? '已配置标签' : '未配置标签'}</strong
    ><small>{scopeSummary}</small>
  </div>
  <div class="docker-tool-summary">
    <span>可用只读工具</span>
    <div>
      {#each tools as tool}<code>{tool}</code>{/each}
    </div>
  </div>
  <div class="provider-test-summary">
    <span>连接核验</span><ConnectionTest
      status={testError || testStatus === 'failed'
        ? 'failed'
        : testStatus === 'succeeded'
          ? 'succeeded'
          : ''}
      latencyMs={testLatency ?? 0}
      message={testError || testMessage}
      busy={testBusy}
      disabled
      showTime={false}
      showTooltip={false}
    />
  </div>
</form>
