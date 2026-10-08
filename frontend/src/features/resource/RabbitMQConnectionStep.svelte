<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import Switch from '../../components/Switch.svelte';
  import TextInput from '../../components/TextInput.svelte';
  export let accessMode: 'direct' | 'agent' = 'direct';
  export let url = '';
  export let username = '';
  export let password = '';
  export let timeoutSeconds = 10;
  export let tlsInsecure = false;
  export let mcpServerResourceId = '';
  export let mcpServers: Resource[] = [];
  export let configurationAttempted = false;
  export let onConfigurationChange = () => {};
  $: mcpServerOptions = mcpServers.map((server) => ({
    value: server.id,
    label: server.name
  }));
</script>

{#if accessMode === 'agent'}<FormField
    label="关联 MCPServer"
    required
    invalid={configurationAttempted && !mcpServerResourceId}
    ><DropdownSelect
      bind:value={mcpServerResourceId}
      options={mcpServerOptions}
      placeholder="请选择活动的 MCPServer"
      ariaLabel="关联 MCPServer"
      invalid={configurationAttempted && !mcpServerResourceId}
      on:change={onConfigurationChange}
    /></FormField
  >
  <p class="docker-field-help">
    RabbitMQ Agent 通过关联 MCPServer 提供统一只读工具。
  </p>{:else}<div class="docker-form-grid">
    <FormField
      label="Management API URL"
      required
      invalid={configurationAttempted && !url.trim()}
      ><TextInput
        bind:value={url}
        required
        invalid={configurationAttempted && !url.trim()}
        on:input={onConfigurationChange}
        placeholder="例如 http://rabbitmq:15672"
      /></FormField
    ><FormField label="用户名"
      ><TextInput
        bind:value={username}
        on:input={onConfigurationChange}
      /></FormField
    ><FormField label="密码"
      ><PasswordInput
        bind:value={password}
        on:input={onConfigurationChange}
        ariaLabel="密码"
      /></FormField
    ><FormField label="超时时间（秒）"
      ><TextInput
        type="number"
        min="1"
        max="300"
        bind:value={timeoutSeconds}
        on:input={onConfigurationChange}
      /></FormField
    ><label class="docker-agent-override-toggle"
      ><span>跳过 TLS 校验</span><Switch
        bind:checked={tlsInsecure}
        ariaLabel="跳过 TLS 校验"
        on:change={onConfigurationChange}
      /></label
    >
  </div>{/if}
