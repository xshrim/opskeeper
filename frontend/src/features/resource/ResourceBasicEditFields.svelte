<script lang="ts">
  import type { Resource } from '../../lib/api';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FieldLabel from '../../components/FieldLabel.svelte';
  import FormField from '../../components/FormField.svelte';
  import Switch from '../../components/Switch.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import { resourceCategoryFor, resourceCategoryOptions, resourceSubtypeFor, resourceSubtypeOptionsFor } from './resourceCatalog';

  export let resource: Resource;
  export let name = '';
  export let status = 'active';
  export let labels = '';
  export let scopeName: (id: string) => string;
  $: categoryValue = resourceCategoryFor(resource);
  $: subtypeValue = resourceSubtypeFor(resource);
  $: categoryOptions = Object.keys(resourceCategoryOptions).filter((category) => category !== '全部').map((category) => ({ value: category, label: category }));
  $: subtypeOptions = resourceSubtypeOptionsFor(resource).map((subtype) => ({ value: subtype, label: subtype }));
</script>

<h3 class="editor-section-title">基础配置</h3>
<p class="editor-section-description">资源类型和子类型只读，资源名称、启用状态与资源标签可在此调整。</p>
<div class="resource-basic-edit-grid">
  <div class="resource-basic-type-row">
    <FormField label="资源类型"><DropdownSelect value={categoryValue} options={categoryOptions} disabled ariaLabel="资源类型" /></FormField>
    <FormField label="资源子类型"><DropdownSelect value={subtypeValue} options={subtypeOptions} disabled ariaLabel="资源子类型" /></FormField>
  </div>
  <div class="resource-basic-identity-row">
    <FormField label="资源名称" required><TextInput bind:value={name} required /></FormField>
    <FormField label="资源级别"><TextInput value={scopeName(resource.scope_id)} readonly /></FormField>
    <div class="resource-basic-enabled"><FieldLabel text="是否启用" /><Switch checked={status === 'active'} ariaLabel="是否启用资源" on:change={(event) => (status = event.detail ? 'active' : 'disabled')} /></div>
  </div>
  <FormField label="资源标签" className="resource-basic-labels"><TextInput bind:value={labels} placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform" autocomplete="off" /></FormField>
</div>
