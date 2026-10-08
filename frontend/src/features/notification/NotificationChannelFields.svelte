<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import Switch from '../../components/Switch.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import type { NotificationProvider } from '../../lib/api';

  export let providers: NotificationProvider[] = [];
  export let channelName = '';
  export let channelKind = '';
  export let channelRateLimit = 30;
  export let channelShare = false;
  export let channelConfig: Record<string, string> = {};
  const dispatch = createEventDispatcher<{
    channelName: string;
    channelKind: string;
    channelRateLimit: number;
    channelShare: boolean;
    channelConfig: Record<string, string>;
  }>();

  $: selectedProvider = providers.find((provider) => provider.kind === channelKind);

  function selectProvider(kind: string) {
    channelKind = kind;
    channelConfig = {};
    dispatch('channelKind', channelKind);
    dispatch('channelConfig', channelConfig);
  }

  function setChannelConfig(name: string, value: string) {
    channelConfig = { ...channelConfig, [name]: value };
    dispatch('channelConfig', channelConfig);
  }

  function providerFieldType(type: string): 'text' | 'email' | 'url' | 'number' | 'password' {
    if (type === 'password' || type === 'email' || type === 'url' || type === 'number') return type;
    return 'text';
  }
</script>

<FormField label="渠道名称" required>
  <TextInput bind:value={channelName} required maxlength={120} placeholder="例如：生产值守群" ariaLabel="渠道名称" on:input={() => dispatch('channelName', channelName)} />
</FormField>
<FormField label="Provider" required>
  <DropdownSelect options={providers.filter((provider) => provider.supported).map((provider) => ({ value: provider.kind, label: provider.name }))} bind:value={channelKind} placeholder="选择通知 Provider" ariaLabel="通知 Provider" on:change={(event) => selectProvider(String(event.detail))} />
</FormField>
{#each selectedProvider?.fields ?? [] as field}
  <FormField label={field.label} required={field.required}>
    {#if field.type === 'textarea'}
      <TextArea rows={3} required={field.required} value={channelConfig[field.name] ?? ''} on:input={(event) => setChannelConfig(field.name, (event.currentTarget as HTMLTextAreaElement).value)} />
    {:else}
      <TextInput type={providerFieldType(field.type)} required={field.required} value={channelConfig[field.name] ?? ''} ariaLabel={field.label} on:input={(event) => setChannelConfig(field.name, (event.currentTarget as HTMLInputElement).value)} />
    {/if}
  </FormField>
{/each}
<FormField label="每分钟上限">
  <TextInput type="number" min="1" max="10000" bind:value={channelRateLimit} ariaLabel="每分钟上限" on:input={() => dispatch('channelRateLimit', Number(channelRateLimit))} />
</FormField>
<div class="notification-switch"><span>对子 Scope 可用</span><Switch bind:checked={channelShare} ariaLabel="对子 Scope 可用" on:change={(event) => dispatch('channelShare', Boolean(event.detail))} /></div>

<style>
  .notification-switch {
    display: flex;
    min-height: 36px;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    color: var(--theme-fg-muted);
    font-size: 12px;
  }
</style>
