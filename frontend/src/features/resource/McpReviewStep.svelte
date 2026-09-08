<script lang="ts">
  export let transport = 'streamable_http';
  export let url = '';
  export let tokenConfigured = false;
  export let headerCount = 0;
  export let toolAllowlist = '';
  export let timeoutSeconds = 120;
  export let maxResponseBytes = 4 * 1024 * 1024;
  export let testStatus = '';
  export let testError = '';
  export let toolCount = 0;
  export let latency: number | undefined;
</script>

<div class="provider-summary mcp-summary">
  <div><span>传输方式</span><strong>{transport === 'sse' ? 'SSE' : 'Streamable HTTP'}</strong></div>
  <div><span>Server 地址</span><strong>{url || '未设置'}</strong></div>
  <div><span>Token / Header</span><strong>{tokenConfigured ? 'Token 已配置' : 'Token 未配置'}</strong><small>{headerCount} 个请求 Header</small></div>
  <div><span>工具白名单</span><strong>{toolAllowlist.trim() || '不限制'}</strong><small>支持通配符，空白表示允许全部工具</small></div>
  <div><span>超时时间</span><strong>{timeoutSeconds} 秒</strong></div>
  <div><span>响应体大小限制</span><strong>{Math.round(Number(maxResponseBytes) / 1024 / 1024)} MiB</strong></div>
  <div class="provider-test-summary">
    <span>连接核验</span>
    {#if testStatus === 'succeeded'}<strong class="success">连接正常 · 发现 {toolCount} 个工具{latency ? ` · ${latency} ms` : ''}</strong><small>Server 初始化和工具发现已完成</small>
    {:else if testError}<strong class="failed">{testError}</strong><small>请修正配置后重新测试</small>
    {:else}<strong>尚未核验</strong><small>进入此步骤后自动执行连接测试</small>{/if}
  </div>
</div>
