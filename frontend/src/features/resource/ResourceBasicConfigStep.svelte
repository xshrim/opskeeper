<script lang="ts">
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
</script>

<div class="resource-type-selection">
  <div class="resource-basic-type-row">
    <label class:invalid={typeSelectionAttempted && !category}>
      <span><i>*</i>资源类型</span>
      <select bind:value={category} disabled={editing} required on:change={(event) => onSelectCategory((event.currentTarget as HTMLSelectElement).value)}>
        <option value="">请选择资源类型</option>
        {#each Object.keys(categoryOptions).filter((option) => option !== '全部') as option}<option value={option}>{option}</option>{/each}
      </select>
    </label>
    <label class:invalid={typeSelectionAttempted && !subtype}>
      <span><i>*</i>资源子类型</span>
      <select bind:value={subtype} disabled={!category || editing} required on:change={(event) => onSelectSubtype((event.currentTarget as HTMLSelectElement).value)}>
        <option value="">请选择资源子类型</option>
        {#each subtypeOptions as option}<option value={option}>{option}</option>{/each}
      </select>
    </label>
  </div>
  <div class="resource-basic-identity-row">
    <label class="resource-basic-name" class:invalid={basicConfigurationAttempted && !name.trim()}>
      <span><i>*</i>资源名称</span><input bind:value={name} required placeholder="例如 production-resource" autocomplete="off" />
    </label>
    <label class="resource-basic-level"><span><i>*</i>资源级别</span><input value={scopeSummary} readonly aria-readonly="true" required /></label>
    <label class="resource-basic-enabled">
      <span>是否启用</span><span class="provider-toggle-control"><input type="checkbox" checked={status === 'active'} on:change={(event) => (status = (event.currentTarget as HTMLInputElement).checked ? 'active' : 'disabled')} aria-label="是否启用资源" /><i aria-hidden="true"></i></span>
    </label>
  </div>
  <label class="resource-basic-labels"><span>资源标签</span><input bind:value={labels} placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform" autocomplete="off" /></label>
</div>
