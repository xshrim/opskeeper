<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';

  export let accessMode: 'direct' | 'agent' = 'direct';
  export let endpoint = '';
  export let accessKey = '';
  export let secretKey = '';
  export let sessionToken = '';
  export let region = '';
  export let secure = false;
  export let timeoutSeconds = 10;
  export let mcpServerResourceId = '';
  export let mcpServers: Resource[] = [];
  export let configurationAttempted = false;
  export let onConfigurationChange: () => void = () => {};

  $: mcpServerOptions = mcpServers.map((server) => ({ value: server.id, label: server.name }));
</script>

{#if accessMode === 'agent'}
  <FormField label="关联 MCPServer" required invalid={configurationAttempted && !mcpServerResourceId}><DropdownSelect bind:value={mcpServerResourceId} options={mcpServerOptions} placeholder="请选择活动的 MCPServer" ariaLabel="关联 MCPServer" invalid={configurationAttempted && !mcpServerResourceId} on:change={onConfigurationChange} /></FormField>
  <p class="docker-field-help">MinIO Agent 通过关联 MCPServer 提供统一只读工具。</p>
{:else}
  <div class="docker-form-grid">
    <FormField label="Endpoint" required invalid={configurationAttempted && !endpoint.trim()}><TextInput bind:value={endpoint} required invalid={configurationAttempted && !endpoint.trim()} on:input={onConfigurationChange} placeholder="例如 minio.example.com:9000" /></FormField>
    <FormField label="Access Key"><TextInput bind:value={accessKey} on:input={onConfigurationChange} /></FormField>
    <FormField label="Secret Key"><PasswordInput bind:value={secretKey} on:input={onConfigurationChange} ariaLabel="Secret Key" /></FormField>
    <FormField label="Session Token"><PasswordInput bind:value={sessionToken} on:input={onConfigurationChange} ariaLabel="Session Token" /></FormField>
    <FormField label="Region"><TextInput bind:value={region} on:input={onConfigurationChange} /></FormField>
    <FormField label="超时时间（秒）"><TextInput type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></FormField>
    <label><span>启用 HTTPS</span><input type="checkbox" bind:checked={secure} on:change={onConfigurationChange} /></label>
  </div>
{/if}
