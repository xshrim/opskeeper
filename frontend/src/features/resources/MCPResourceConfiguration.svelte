<script lang="ts">
  export let mcpURL = '';
  export let mcpToken = '';
  export let mcpTimeoutSeconds = 120;
  export let mcpMaxResponseBytes = 4 * 1024 * 1024;
  export let mcpRequestHeaders = '';
  export let mcpToolAllowlist = '';
  export let onSubmit: () => void;
</script>

<p class="resource-add-description">
  配置 MCP Server 的连接参数。增强安全模式下服务地址必须使用 HTTPS。
</p>
<form id="resource-create-form" class="stack-form resource-create-form" on:submit|preventDefault={onSubmit}>
  <div class="mcp-resource-form">
    <label class="mcp-url-field"><span><i>*</i>Server 地址</span><input bind:value={mcpURL} type="url" placeholder="https://mcp.example.com/mcp" autocomplete="off" /></label>
    <label><span>Token</span><input bind:value={mcpToken} type="password" placeholder="保存于加密凭据" autocomplete="new-password" /></label>
    <div class="mcp-number-grid"><label><span>超时时间（秒）</span><input bind:value={mcpTimeoutSeconds} type="number" min="1" max="600" /></label><label><span>响应体大小限制（字节）</span><input bind:value={mcpMaxResponseBytes} type="number" min="1" max="16777216" step="1024" /></label></div>
    <label><span>请求 Header</span><textarea bind:value={mcpRequestHeaders} rows="3" placeholder="每行一个 Header，例如 X-Tenant: production"></textarea></label>
    <label class="mcp-tools-field"><span>工具白名单</span><textarea bind:value={mcpToolAllowlist} rows="4" placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具" spellcheck="false"></textarea></label>
  </div>
</form>
