<script lang="ts">
  export let url = '';
  export let token = '';
  export let requestHeaders = '';
  export let toolAllowlist = '';
  export let timeoutSeconds = 120;
  export let maxResponseBytes = 4 * 1024 * 1024;
  export let tokenPlaceholder = '保存于加密凭据';
</script>

<div class="mcp-resource-form">
  <label class="mcp-url-field"><span><i>*</i>Server 地址</span><input bind:value={url} type="url" placeholder="https://mcp.example.com/mcp" autocomplete="off" /></label>
  <label><span>Token</span><input bind:value={token} type="password" placeholder={tokenPlaceholder} autocomplete="new-password" /></label>
  <div class="mcp-number-grid">
    <label><span>超时时间（秒）</span><input bind:value={timeoutSeconds} type="number" min="1" max="600" /></label>
    <label><span>响应体大小限制（字节）</span><input bind:value={maxResponseBytes} type="number" min="1" max="16777216" step="1024" /></label>
  </div>
  <label><span>请求 Header</span><textarea bind:value={requestHeaders} rows="3" placeholder="每行一个 Header，例如 X-Tenant: production"></textarea></label>
  <label class="mcp-tools-field"><span>工具白名单</span><textarea bind:value={toolAllowlist} rows="4" placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具" spellcheck="false"></textarea></label>
</div>
