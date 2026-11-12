<script lang="ts">
  import ConnectionTest from '../../components/ConnectionTest.svelte';
  import {
    dockerAccessModeLabel,
    type DockerAccessMode
  } from './resourceWorkflow';

  export let accessMode: DockerAccessMode = 'direct';
  export let connectionOverride = false;
  export let host = '';
  export let skipTLSVerify = false;
  export let mcpServerName = '';
  export let credentialConfigured = false;
  export let scopeSummary = '';
  export let labelsConfigured = false;
  export let testBusy = false;
  export let testStatus = '';
  export let testMessage = '';
  export let testError = '';
  export let testLatency: number | undefined;
  export let testToolCount = 0;
  export let onSubmit: () => void = () => {};

  const tools = [
    'docker_info',
    'docker_images',
    'docker_containers',
    'docker_container_logs',
    'docker_container_inspect',
    'docker_container_stats'
  ];
</script>

<form
  id="docker-review-form"
  class="provider-summary docker-summary"
  on:submit|preventDefault={onSubmit}
>
  <div>
    <span>接入方式</span>
    <strong>{dockerAccessModeLabel(accessMode)}</strong>
    {#if accessMode === 'direct'}
      <small>{host || '本机默认 Unix Socket'}</small>
    {:else}
      <small
        >{mcpServerName || '未选择 MCPServer'}{connectionOverride
          ? ' · 自定义 Docker 连接'
          : ''}</small
      >
    {/if}
  </div>
  <div>
    <span>连接凭据</span>
    <strong
      >{accessMode === 'direct' || connectionOverride
        ? credentialConfigured
          ? 'TLS Base64 已加密保存'
          : '未配置 TLS 凭据'
        : '由 MCPServer 管理'}</strong
    >
    {#if (accessMode === 'direct' || connectionOverride) && skipTLSVerify}<small
        class="warning">已启用跳过 TLS 校验</small
      >{:else}<small>不会暴露给模型</small>{/if}
  </div>
  <div>
    <span>资源属性</span>
    <strong>{labelsConfigured ? '已配置标签' : '未配置标签'}</strong>
    <small>{scopeSummary}</small>
  </div>
  <div class="provider-test-summary">
    <span>连接核验</span>
    <ConnectionTest
      status={testError || testStatus === 'failed'
        ? 'failed'
        : testStatus === 'succeeded'
          ? 'succeeded'
          : ''}
      latencyMs={testLatency ?? 0}
      message={testError ||
        `${testMessage}${testToolCount ? ` · 发现 ${testToolCount} 个工具` : ''}`}
      busy={testBusy}
      disabled
      showTime={false}
      showTooltip={false}
    />
  </div>
  <div class="docker-tool-summary">
    <span>可用只读工具</span>
    <div>
      {#each tools as tool}<code>{tool}</code>{/each}
    </div>
  </div>
</form>
