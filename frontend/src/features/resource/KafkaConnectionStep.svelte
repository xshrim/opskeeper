<script lang="ts">
  import type { Resource } from '../../lib/api';
  export let accessMode: 'direct' | 'agent' = 'direct'; export let brokers = ''; export let tls = false; export let tlsServerName = ''; export let username = ''; export let password = ''; export let timeoutSeconds = 10; export let mcpServerResourceId = ''; export let mcpServers: Resource[] = []; export let configurationAttempted = false; export let onConfigurationChange: () => void = () => {};
</script>
{#if accessMode === 'agent'}
<label class:invalid={configurationAttempted && !mcpServerResourceId}><span><i>*</i>关联 MCPServer</span><select bind:value={mcpServerResourceId} on:change={onConfigurationChange}><option value="">请选择活动的 MCPServer</option>{#each mcpServers as server}<option value={server.id}>{server.name}</option>{/each}</select></label>
{:else}
<div class="docker-form-grid"><label class:invalid={configurationAttempted && !brokers.trim()}><span><i>*</i>Broker 地址</span><textarea bind:value={brokers} rows="3" placeholder="每行一个 broker，例如 kafka-1:9092" on:input={onConfigurationChange}></textarea></label><label><span>启用 TLS</span><input type="checkbox" bind:checked={tls} on:change={onConfigurationChange} /></label><label><span>TLS Server Name</span><input bind:value={tlsServerName} on:input={onConfigurationChange} /></label><label><span>用户名</span><input bind:value={username} on:input={onConfigurationChange} /></label><label><span>密码</span><input type="password" bind:value={password} on:input={onConfigurationChange} /></label><label><span>超时时间（秒）</span><input type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></label></div>
{/if}
