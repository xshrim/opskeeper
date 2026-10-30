<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import Switch from '../../components/Switch.svelte';
  import { databaseCredentialComplete } from './resourceWorkflow';

  export let accessMode: 'direct' | 'agent' = 'direct';
  export let host = '';
  export let port = 1521;
  export let serviceName = '';
  export let sid = '';
  export let username = '';
  export let password = '';
  export let timeoutSeconds = 10;
  export let tls = false;
  export let mcpServerResourceId = '';
  export let mcpServers: Resource[] = [];
  export let configurationAttempted = false;
  export let credentialConfigured = false;
  export let editing = false;
  export let onConfigurationChange = () => {};
  $: credentialInvalid = configurationAttempted && !databaseCredentialComplete(username, password, credentialConfigured, editing);

  $: mcpServerOptions = mcpServers.map((server) => ({ value: server.id, label: server.name }));
</script>

{#if accessMode === 'agent'}
  <FormField label="关联 MCPServer" required invalid={configurationAttempted && !mcpServerResourceId}>
    <DropdownSelect bind:value={mcpServerResourceId} options={mcpServerOptions} placeholder="请选择活动的 MCPServer" ariaLabel="关联 MCPServer" invalid={configurationAttempted && !mcpServerResourceId} on:change={onConfigurationChange} />
  </FormField>
  <p class="docker-field-help">Oracle Agent 通过关联 MCPServer 提供统一只读工具。</p>
{:else}
  <div class="docker-form-grid">
    <FormField label="数据库主机" required invalid={configurationAttempted && !host.trim()}><TextInput bind:value={host} required invalid={configurationAttempted && !host.trim()} on:input={onConfigurationChange} placeholder="例如 oracle.example.com" /></FormField>
    <FormField label="端口"><TextInput type="number" min="1" max="65535" bind:value={port} on:input={onConfigurationChange} /></FormField>
    <FormField label="Service Name" required={!sid.trim()} invalid={configurationAttempted && !serviceName.trim() && !sid.trim()}><TextInput bind:value={serviceName} invalid={configurationAttempted && !serviceName.trim() && !sid.trim()} on:input={onConfigurationChange} placeholder="与 SID 二选一" /></FormField>
    <FormField label="SID" required={!serviceName.trim()} invalid={configurationAttempted && !serviceName.trim() && !sid.trim()}><TextInput bind:value={sid} invalid={configurationAttempted && !serviceName.trim() && !sid.trim()} on:input={onConfigurationChange} placeholder="与 Service Name 二选一" /></FormField>
    <FormField label="用户名" required invalid={credentialInvalid}><TextInput bind:value={username} required={!editing || !credentialConfigured} invalid={credentialInvalid} placeholder={editing && credentialConfigured ? '留空以保留已配置凭据' : ''} on:input={onConfigurationChange} /></FormField>
    <FormField label="密码" required invalid={credentialInvalid}><PasswordInput bind:value={password} required={!editing || !credentialConfigured} placeholder={editing && credentialConfigured ? '留空以保留已配置凭据' : ''} on:input={onConfigurationChange} ariaLabel="密码" /></FormField>
    <FormField label="超时时间（秒）"><TextInput type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></FormField>
    <div class="checkbox-field"><span>启用 TCPS</span><Switch bind:checked={tls} ariaLabel="启用 TCPS" on:change={onConfigurationChange} /></div>
  </div>
{/if}
