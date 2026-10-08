<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import type { Resource } from '../../lib/api';

  export let value = '';
  export let resources: Resource[] = [];
  export let invalid = false;
  export let allowAdd = true;
  export let ariaLabel = '关联 MCPServer';
  const dispatch = createEventDispatcher<{ change: string; value: string }>();

  $: options = [
    { value: '', label: '请选择活动的 MCPServer' },
    ...resources.map((resource) => ({
      value: resource.id,
      label: `${resource.name} · ${String(resource.config?.url ?? '未设置地址')}`
    })),
    ...(allowAdd ? [{ value: '__add_mcp_server__', label: '添加 MCPServer' }] : [])
  ];
</script>

<FormField label="关联 MCPServer" required invalid={invalid}>
  <DropdownSelect bind:value options={options} ariaLabel={ariaLabel} invalid={invalid} on:change={(event) => { value = String(event.detail); dispatch('value', value); dispatch('change', value); }} />
</FormField>
