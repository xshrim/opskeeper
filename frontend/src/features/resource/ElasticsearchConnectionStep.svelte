<script lang="ts">
 import type { Resource } from '../../lib/api';
 import PasswordInput from '../../components/PasswordInput.svelte';
  export let accessMode: 'direct'|'agent'='direct'; export let url=''; export let username=''; export let password=''; export let tlsInsecure=false; export let timeoutSeconds=10; export let mcpServerResourceId=''; export let mcpServers:Resource[]=[]; export let configurationAttempted=false; export let onConfigurationChange:()=>void=()=>{};
</script>
<div class="provider-fields">
  {#if accessMode === 'direct'}
    <label class:invalid={configurationAttempted&&!url.trim()}><span><i>*</i>服务 URL</span><input bind:value={url} placeholder="http://elasticsearch:9200" on:input={onConfigurationChange} /></label>
    <label><span>用户名</span><input bind:value={username} on:input={onConfigurationChange} /></label><label><span>密码</span><PasswordInput bind:value={password} on:input={onConfigurationChange} ariaLabel="密码" /></label>
    <label class="checkbox"><input type="checkbox" bind:checked={tlsInsecure} on:change={onConfigurationChange} /> 跳过 TLS 证书校验</label>
    <label><span>超时时间（秒）</span><input type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></label>
  {:else}
    <label class:invalid={configurationAttempted&&!mcpServerResourceId}><span><i>*</i>关联 MCPServer</span><select bind:value={mcpServerResourceId} on:change={onConfigurationChange}><option value="">请选择活动的 MCPServer</option>{#each mcpServers as s}<option value={s.id}>{s.name}</option>{/each}</select></label>
  {/if}
</div>
