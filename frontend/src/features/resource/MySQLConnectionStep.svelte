<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import { databaseCredentialComplete } from './resourceWorkflow';
  export let accessMode: 'direct' | 'agent' = 'direct';
  export let host = ''; export let port = 3306; export let database = ''; export let username = ''; export let password = ''; export let timeoutSeconds = 10; export let mcpServerResourceId = ''; export let mcpServers: Resource[] = []; export let configurationAttempted = false; export let credentialConfigured = false; export let editing = false; export let onConfigurationChange: () => void = () => {};
  $: credentialInvalid = configurationAttempted && !databaseCredentialComplete(username, password, credentialConfigured, editing);
  $: mcpServerOptions = mcpServers.map((server) => ({ value: server.id, label: server.name }));
</script>
{#if accessMode === 'agent'}
  <FormField label="关联 MCPServer" required asLabel={false} invalid={configurationAttempted && !mcpServerResourceId}><DropdownSelect bind:value={mcpServerResourceId} options={mcpServerOptions} placeholder="请选择活动的 MCPServer" ariaLabel="关联 MCPServer" invalid={configurationAttempted && !mcpServerResourceId} on:change={onConfigurationChange} /></FormField>
  <p class="docker-field-help">MySQL Agent 通过关联 MCPServer 提供统一只读工具。</p>
{:else}
  <div class="docker-form-grid">
    <FormField label="数据库主机" required invalid={configurationAttempted && !host.trim()}><TextInput bind:value={host} required invalid={configurationAttempted && !host.trim()} on:input={onConfigurationChange} placeholder="例如 db.example.com" /></FormField>
    <FormField label="端口"><TextInput type="number" min="1" max="65535" bind:value={port} on:input={onConfigurationChange} /></FormField>
    <FormField label="数据库名" required invalid={configurationAttempted && !database.trim()}><TextInput bind:value={database} required invalid={configurationAttempted && !database.trim()} on:input={onConfigurationChange} /></FormField>
    <FormField label="用户名" required invalid={credentialInvalid}><TextInput bind:value={username} required={!editing || !credentialConfigured} invalid={credentialInvalid} placeholder={editing && credentialConfigured ? '留空以保留已配置凭据' : ''} on:input={onConfigurationChange} /></FormField>
    <FormField label="密码" required invalid={credentialInvalid}><PasswordInput bind:value={password} required={!editing || !credentialConfigured} placeholder={editing && credentialConfigured ? '留空以保留已配置凭据' : ''} on:input={onConfigurationChange} ariaLabel="密码" /></FormField>
    <FormField label="超时时间（秒）"><TextInput type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></FormField>
  </div>
{/if}
