<script lang="ts">
  import { dockerAccessModeLabel, type DockerAccessMode } from './resourceWorkflow';

  export let resourceName = '';
  export let resourceStatus = 'active';
  export let accessMode: DockerAccessMode = 'direct';
  export let connectionOverride = false;
  export let host = '';
  export let serverName = '';
  export let skipTLSVerify = false;
  export let mcpServerName = '';
  export let credentialConfigured = false;
  export let scopeSummary = '';
  export let labelsConfigured = false;
  export let onTest: () => void = () => {};
  export let testBusy = false;
  export let testStatus = '';
  export let testMessage = '';
  export let testError = '';
  export let testLatency: number | undefined;
  export let testToolCount = 0;
  export let onSubmit: () => void = () => {};

  const tools = ['docker_info', 'docker_images', 'docker_containers', 'docker_container_logs', 'docker_container_inspect', 'docker_container_stats'];
</script>

<p class="resource-add-description">确认接入路径和连接边界后保存资源。Docker 工具集只提供只读信息、日志和统计能力。</p>
<form id="docker-review-form" class="provider-summary docker-summary" on:submit|preventDefault={onSubmit}>
  <div>
    <span>Docker 资源</span>
    <strong>{resourceName}</strong>
    <small>{resourceStatus === 'active' ? '已启用' : '已停用'} · {scopeSummary}</small>
  </div>
  <div>
    <span>接入方式</span>
    <strong>{dockerAccessModeLabel(accessMode)}</strong>
    {#if accessMode === 'direct'}
      <small>{host || '本机默认 Unix Socket'}{serverName ? ` · Server Name ${serverName}` : ''}</small>
    {:else}
      <small>{mcpServerName || '未选择 MCPServer'}{connectionOverride ? ' · 自定义 Docker 连接' : ''}</small>
    {/if}
  </div>
  <div>
    <span>连接凭据</span>
    <strong>{accessMode === 'direct' || connectionOverride ? credentialConfigured ? 'TLS Base64 已加密保存' : '未配置 TLS 凭据' : '由 MCPServer 管理'}</strong>
    {#if (accessMode === 'direct' || connectionOverride) && skipTLSVerify}<small class="warning">已启用跳过 TLS 校验</small>{:else}<small>不会暴露给模型</small>{/if}
  </div>
  <div>
    <span>资源属性</span>
    <strong>{labelsConfigured ? '已配置标签' : '未配置标签'}</strong>
    <small>{scopeSummary}</small>
  </div>
  <div class="docker-tool-summary">
    <span>可用只读工具</span>
    <div>{#each tools as tool}<code>{tool}</code>{/each}</div>
  </div>
  <div class="provider-test-summary">
    <span>连接核验</span>
    {#if testBusy}
      <strong>正在测试连接...</strong>
      <small>请等待 Docker 或关联 MCPServer 返回结果。</small>
    {:else if testStatus === 'succeeded'}
      <strong class="success">连接正常 · {testLatency ?? 0} ms</strong>
      <small>{testMessage}{testToolCount ? ` · 发现 ${testToolCount} 个工具` : ''}</small>
    {:else if testError}
      <strong class="failed">连接失败</strong>
      <small>{testError}</small>
    {:else}
      <strong>尚未核验</strong>
      <small>创建或保存前必须完成连接测试。</small>
    {/if}
    <button class="secondary provider-test-button" type="button" on:click={onTest} disabled={testBusy}>
      {testBusy ? '测试中' : '连接测试'}
    </button>
  </div>
</form>
