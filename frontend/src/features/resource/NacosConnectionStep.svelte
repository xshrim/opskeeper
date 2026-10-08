<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';

  export let accessMode: 'direct' | 'agent' = 'direct';
  export let host = '';
  export let port = 8848;
  export let scheme = 'http';
  export let contextPath = '/nacos';
  export let username = '';
  export let password = '';
  export let accessToken = '';
  export let timeoutSeconds = 10;
  export let mcpServerResourceId = '';
  export let mcpServers: Resource[] = [];
  export let configurationAttempted = false;
  export let onConfigurationChange: () => void = () => {};

  $: mcpServerOptions = mcpServers.map((server) => ({ value: server.id, label: server.name }));
  const schemeOptions = [{ value: 'http', label: 'HTTP' }, { value: 'https', label: 'HTTPS' }];
</script>

{#if accessMode === 'agent'}
  <FormField label="关联 MCPServer" required invalid={configurationAttempted && !mcpServerResourceId}><DropdownSelect bind:value={mcpServerResourceId} options={mcpServerOptions} placeholder="请选择活动的 MCPServer" ariaLabel="关联 MCPServer" invalid={configurationAttempted && !mcpServerResourceId} on:change={onConfigurationChange} /></FormField>
  <p class="docker-field-help">Nacos Agent 通过关联 MCPServer 提供统一只读 API 工具。</p>
{:else}
  <div class="docker-form-grid">
    <FormField label="Nacos 主机" required invalid={configurationAttempted && !host.trim()}><TextInput bind:value={host} required invalid={configurationAttempted && !host.trim()} on:input={onConfigurationChange} placeholder="例如 nacos.example.com" /></FormField>
    <FormField label="协议"><DropdownSelect bind:value={scheme} options={schemeOptions} ariaLabel="协议" on:change={onConfigurationChange} /></FormField>
    <FormField label="端口"><TextInput type="number" min="1" max="65535" bind:value={port} on:input={onConfigurationChange} /></FormField>
    <FormField label="上下文路径"><TextInput bind:value={contextPath} on:input={onConfigurationChange} placeholder="/nacos" /></FormField>
    <FormField label="用户名（可选）"><TextInput bind:value={username} on:input={onConfigurationChange} /></FormField>
    <FormField label="密码（可选）"><PasswordInput bind:value={password} on:input={onConfigurationChange} ariaLabel="密码" /></FormField>
    <FormField label="访问 Token（可选）"><PasswordInput bind:value={accessToken} on:input={onConfigurationChange} ariaLabel="访问 Token" /></FormField>
    <FormField label="超时时间（秒）"><TextInput type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange} /></FormField>
  </div>
{/if}
