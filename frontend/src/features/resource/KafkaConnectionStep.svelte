<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import Switch from '../../components/Switch.svelte';

  export let accessMode: 'direct' | 'agent' = 'direct';
  export let brokers = '';
  export let tls = false;
  export let tlsServerName = '';
  export let username = '';
  export let password = '';
  export let timeoutSeconds = 10;
  export let mcpServerResourceId = '';
  export let mcpServers: Resource[] = [];
  export let configurationAttempted = false;
  export let onConfigurationChange: () => void = () => {};

  $: mcpServerOptions = mcpServers.map((server) => ({ value: server.id, label: server.name }));
</script>

{#if accessMode === 'agent'}
  <FormField label="关联 MCPServer" required invalid={configurationAttempted && !mcpServerResourceId}><DropdownSelect bind:value={mcpServerResourceId} options={mcpServerOptions} placeholder="请选择活动的 MCPServer" ariaLabel="关联 MCPServer" invalid={configurationAttempted && !mcpServerResourceId} on:change={onConfigurationChange} /></FormField>
{:else}
  <div class="docker-form-grid">
    <FormField label="Broker 地址" required invalid={configurationAttempted && !brokers.trim()}><TextArea bind:value={brokers} rows={3} placeholder="每行一个 broker，例如 kafka-1:9092" on:input={onConfigurationChange} /></FormField>
    <div class="checkbox-field"><span>启用 TLS</span><Switch bind:checked={tls} ariaLabel="启用 TLS" on:change={onConfigurationChange} /></div>
    <FormField label="TLS Server Name"><TextInput bind:value={tlsServerName} on:input={onConfigurationChange} /></FormField>
    <FormField label="用户名"><TextInput bind:value={username} on:input={onConfigurationChange} /></FormField>
    <FormField label="密码"><PasswordInput bind:value={password} on:input={onConfigurationChange} ariaLabel="密码" /></FormField>
    <FormField label="超时时间（秒）"><TextInput type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></FormField>
  </div>
{/if}
