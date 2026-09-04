<script lang="ts">
  export let editResourceName = '';
  export let editResourceStatus = 'active';
  export let editResourceLabels = '';
  export let mcpURL = '';
  export let mcpTimeoutSeconds = 120;
  export let mcpMaxResponseBytes = 4 * 1024 * 1024;
  export let mcpToken = '';
  export let mcpRequestHeaders = '';
  export let mcpToolAllowlist = '';
</script>

<div class="mcp-resource-form editor-mcp-form">
  <div class="form-row">
    <label><span>资源名称</span><input bind:value={editResourceName} required /></label>
    <label><span>状态</span><select bind:value={editResourceStatus}><option value="active">正常</option><option value="disabled">停用</option><option value="unknown">未知</option></select></label>
  </div>
  <label><span>标签</span><input bind:value={editResourceLabels} placeholder="env=prod, owner=platform" /></label>
  <label><span>Server 地址</span><input bind:value={mcpURL} type="url" placeholder="https://mcp.example.com/mcp" /></label>
  <div class="mcp-number-grid"><label><span>超时时间（秒）</span><input bind:value={mcpTimeoutSeconds} type="number" min="1" max="600" /></label><label><span>响应体大小限制（字节）</span><input bind:value={mcpMaxResponseBytes} type="number" min="1" max="16777216" step="1024" /></label></div>
  <label><span>Token</span><input bind:value={mcpToken} type="password" placeholder="留空保持原凭据" /></label>
  <label><span>请求 Header</span><textarea bind:value={mcpRequestHeaders} rows="3" placeholder="每行一个 Header，例如 X-Tenant: production"></textarea></label>
  <label><span>工具白名单</span><textarea bind:value={mcpToolAllowlist} rows="5" placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具"></textarea></label>
</div>
