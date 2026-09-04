<script lang="ts">
  import type { MCPSnapshot } from '../../lib/api';

  type DraftTest = { result?: MCPSnapshot; error?: string } | null;

  export let mcpTransport = 'streamable_http';
  export let mcpURL = '';
  export let mcpToken = '';
  export let mcpToolAllowlist = '';
  export let mcpTimeoutSeconds = 120;
  export let mcpMaxResponseBytes = 4 * 1024 * 1024;
  export let mcpDraftTest: DraftTest = null;
  export let mcpDraftTestBusy = false;
  export let mcpHeaderCount: () => number;
  export let onTestConnection: () => void;
</script>

<div class="provider-summary mcp-summary">
  <div><span>传输方式</span><strong>{mcpTransport === 'sse' ? 'SSE' : 'Streamable HTTP'}</strong></div>
  <div><span>Server 地址</span><strong>{mcpURL || '未设置'}</strong></div>
  <div><span>Token / Header</span><strong>{mcpToken.trim() ? 'Token 已配置' : 'Token 未配置'}</strong><small>{mcpHeaderCount()} 个请求 Header</small></div>
  <div><span>工具白名单</span><strong>{mcpToolAllowlist.trim() || '不限制'}</strong><small>支持通配符，空白表示允许全部工具</small></div>
  <div><span>超时时间</span><strong>{mcpTimeoutSeconds} 秒</strong></div>
  <div><span>响应体大小限制</span><strong>{Math.round(Number(mcpMaxResponseBytes) / 1024 / 1024)} MiB</strong></div>
  <div class="provider-test-summary">
    <span>连接核验</span>
    {#if mcpDraftTest?.result?.status === 'succeeded'}
      <strong class="success">连接正常 · 发现 {mcpDraftTest.result.tools.length} 个工具{mcpDraftTest.result.latency_ms ? ` · ${mcpDraftTest.result.latency_ms} ms` : ''}</strong><small>Server 初始化和工具发现已完成</small>
    {:else if mcpDraftTest?.error}
      <strong class="failed">{mcpDraftTest.error}</strong><small>请修正配置后重新测试</small>
    {:else}
      <strong>尚未核验</strong><small>创建前必须完成连接测试</small>
    {/if}
    <button class="secondary provider-test-button" type="button" on:click={onTestConnection} disabled={mcpDraftTestBusy}>{mcpDraftTestBusy ? '连接中…' : '连接测试'}</button>
  </div>
</div>
