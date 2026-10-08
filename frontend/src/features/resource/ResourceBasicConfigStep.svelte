<script lang="ts">
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FieldLabel from '../../components/FieldLabel.svelte';
  import Switch from '../../components/Switch.svelte';
  import TextInput from '../../components/TextInput.svelte';
  export let category = '';
  export let subtype = '';
  export let name = '';
  export let status = 'active';
  export let labels = '';
  export let categoryOptions: Record<string, string[]> = {};
  export let subtypeOptions: string[] = [];
  export let typeSelectionAttempted = false;
  export let basicConfigurationAttempted = false;
  export let editing = false;
  export let scopeSummary = '';
  export let onSelectCategory: (category: string) => void = () => {};
  export let onSelectSubtype: (subtype: string) => void = () => {};

  $: categoryDropdownOptions = Object.keys(categoryOptions).filter((option) => option !== '全部').map((option) => ({ value: option, label: option }));
  $: subtypeDropdownOptions = subtypeOptions.map((option) => ({ value: option, label: option }));

  function selectCategory(value: string) {
    typeSelectionAttempted = false;
    onSelectCategory(value);
  }

  function selectSubtype(value: string) {
    typeSelectionAttempted = false;
    onSelectSubtype(value);
  }
</script>

<div class="resource-type-selection">
  <div class="resource-basic-type-row">
    <label>
      <FieldLabel text="资源类型" required invalid={typeSelectionAttempted && !category} />
      <DropdownSelect bind:value={category} options={categoryDropdownOptions} disabled={editing} placeholder="请选择资源类型" ariaLabel="资源类型" invalid={typeSelectionAttempted && !category} invalidMode="bubble" invalidMessage="请选择资源类型" on:change={(event) => selectCategory(String(event.detail))} />
    </label>
    <label>
      <FieldLabel text="资源子类型" required invalid={typeSelectionAttempted && !subtype} />
      <DropdownSelect bind:value={subtype} options={subtypeDropdownOptions} disabled={!category || editing} placeholder="请选择资源子类型" ariaLabel="资源子类型" invalid={typeSelectionAttempted && !subtype} invalidMode="bubble" invalidMessage="请选择资源子类型" on:change={(event) => selectSubtype(String(event.detail))} />
    </label>
  </div>
  <div class="resource-basic-identity-row">
    <label class="resource-basic-name">
      <FieldLabel text="资源名称" required invalid={basicConfigurationAttempted && !name.trim()} /><TextInput bind:value={name} required invalid={basicConfigurationAttempted && !name.trim()} invalidMode="bubble" invalidMessage="请填写资源名称" placeholder="例如 production-resource" autocomplete="off" />
    </label>
    <label class="resource-basic-level"><FieldLabel text="资源级别" required invalid={basicConfigurationAttempted && !scopeSummary} /><TextInput value={scopeSummary} readonly invalid={basicConfigurationAttempted && !scopeSummary} invalidMode="bubble" invalidMessage="请选择资源范围" ariaLabel="资源级别" /></label>
    <label class="resource-basic-enabled">
      <FieldLabel text="是否启用" /><Switch checked={status === 'active'} ariaLabel="是否启用资源" on:change={(event) => (status = event.detail ? 'active' : 'disabled')} />
    </label>
  </div>
  <label class="resource-basic-labels"><FieldLabel text="资源标签" /><TextInput bind:value={labels} placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform" autocomplete="off" /></label>
</div>
