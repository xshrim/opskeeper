<script lang="ts">
  import type { Resource } from '../../lib/api';
  export let accessMode: 'direct' | 'agent' = 'direct';
  export let host = ''; export let port = 5432; export let database = ''; export let username = ''; export let password = ''; export let timeoutSeconds = 10; export let mcpServerResourceId = ''; export let mcpServers: Resource[] = []; export let configurationAttempted = false; export let onConfigurationChange: () => void = () => {};
</script>
{#if accessMode === 'agent'}
  <label class:invalid={configurationAttempted && !mcpServerResourceId}><span><i>*</i>关联 MCPServer</span><select bind:value={mcpServerResourceId} required on:change={onConfigurationChange}><option value="">请选择活动的 MCPServer</option>{#each mcpServers as server}<option value={server.id}>{server.name}</option>{/each}</select></label>
  <p class="docker-field-help">PostgreSQL Agent 通过关联 MCPServer 提供统一只读工具。</p>
{:else}
  <div class="docker-form-grid">
    <label class:invalid={configurationAttempted && !host.trim()}><span><i>*</i>数据库主机</span><input bind:value={host} required on:input={onConfigurationChange} placeholder="例如 db.example.com" /></label>
    <label><span>端口</span><input type="number" min="1" max="65535" bind:value={port} on:input={onConfigurationChange} /></label>
    <label class:invalid={configurationAttempted && !database.trim()}><span><i>*</i>数据库名</span><input bind:value={database} required on:input={onConfigurationChange} /></label>
    <label class:invalid={configurationAttempted && !username.trim()}><span><i>*</i>用户名</span><input bind:value={username} required on:input={onConfigurationChange} /></label>
    <label class:invalid={configurationAttempted && !password.trim()}><span><i>*</i>密码</span><input type="password" bind:value={password} required on:input={onConfigurationChange} /></label>
    <label><span>超时时间（秒）</span><input type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></label>
  </div>
{/if}
