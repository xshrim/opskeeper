<script lang="ts">
 import type { Resource } from '../../lib/api';
 import DropdownSelect from '../../components/DropdownSelect.svelte';
 import FormField from '../../components/FormField.svelte';
 import PasswordInput from '../../components/PasswordInput.svelte';
 import TextInput from '../../components/TextInput.svelte';
 import Switch from '../../components/Switch.svelte';
  export let accessMode: 'direct'|'agent'='direct'; export let url=''; export let username=''; export let password=''; export let tlsInsecure=false; export let timeoutSeconds=10; export let mcpServerResourceId=''; export let mcpServers:Resource[]=[]; export let configurationAttempted=false; export let onConfigurationChange:()=>void=()=>{};
  $: mcpServerOptions = mcpServers.map((server) => ({ value: server.id, label: server.name }));
</script>
<div class="provider-fields">
  {#if accessMode === 'direct'}
    <FormField label="服务 URL" required invalid={configurationAttempted&&!url.trim()}><TextInput bind:value={url} placeholder="http://elasticsearch:9200" invalid={configurationAttempted&&!url.trim()} on:input={onConfigurationChange} /></FormField>
    <FormField label="用户名"><TextInput bind:value={username} on:input={onConfigurationChange} /></FormField><FormField label="密码"><PasswordInput bind:value={password} on:input={onConfigurationChange} ariaLabel="密码" /></FormField>
    <div class="checkbox-field"><span>跳过 TLS 证书校验</span><Switch bind:checked={tlsInsecure} ariaLabel="跳过 TLS 证书校验" on:change={onConfigurationChange} /></div>
    <FormField label="超时时间（秒）"><TextInput type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></FormField>
  {:else}
    <FormField label="关联 MCPServer" required invalid={configurationAttempted&&!mcpServerResourceId}><DropdownSelect bind:value={mcpServerResourceId} options={mcpServerOptions} placeholder="请选择活动的 MCPServer" ariaLabel="关联 MCPServer" invalid={configurationAttempted&&!mcpServerResourceId} on:change={onConfigurationChange} /></FormField>
  {/if}
</div>
