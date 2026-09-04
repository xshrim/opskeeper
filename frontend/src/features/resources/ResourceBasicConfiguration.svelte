<script lang="ts">
  export let resourceCategoryOptions: Record<string, string[]> = {};
  export let resourceAddSubtypeOptions: string[] = [];
  export let resourceTypeSelectionAttempted = false;
  export let resourceBasicConfigurationAttempted = false;
  export let resourceAddCategory = '';
  export let resourceAddSubtype = '';
  export let resourceName = '';
  export let resourceStatus = 'active';
  export let resourceLabels = '';
  export let editingProviderResourceId = '';
  export let editingResourceId = '';
  export let activeScopeSummary: () => string;
  export let onSelectCategory: (category: string) => void;
  export let onSelectSubtype: (subtype: string) => void;
</script>

<p class="resource-add-description">
  配置资源的基础身份、归属和标签；资源类型、子类型与名称为必填项。
</p>
<div class="resource-type-selection">
  <div class="resource-basic-type-row">
    <label class:invalid={resourceTypeSelectionAttempted && !resourceAddCategory}>
      <span><i>*</i>资源类型</span>
      <select
        bind:value={resourceAddCategory}
        disabled={Boolean(editingProviderResourceId || editingResourceId)}
        on:change={(event) => onSelectCategory((event.currentTarget as HTMLSelectElement).value)}
      >
        <option value="">请选择资源类型</option>
        {#each Object.keys(resourceCategoryOptions).filter((category) => category !== '全部') as category}
          <option value={category}>{category}</option>
        {/each}
      </select>
    </label>
    <label class:invalid={resourceTypeSelectionAttempted && !resourceAddSubtype}>
      <span><i>*</i>资源子类型</span>
      <select
        bind:value={resourceAddSubtype}
        disabled={!resourceAddCategory || Boolean(editingProviderResourceId || editingResourceId)}
        on:change={(event) => onSelectSubtype((event.currentTarget as HTMLSelectElement).value)}
      >
        <option value="">请选择资源子类型</option>
        {#each resourceAddSubtypeOptions as subtype}
          <option value={subtype}>{subtype}</option>
        {/each}
      </select>
    </label>
  </div>
  <div class="resource-basic-identity-row">
    <label class="resource-basic-name" class:invalid={resourceBasicConfigurationAttempted && !resourceName.trim()}>
      <span><i>*</i>资源名称</span>
      <input bind:value={resourceName} required placeholder="例如 production-resource" autocomplete="off" />
    </label>
    <label class="resource-basic-level">
      <span>资源级别</span><input value={activeScopeSummary()} readonly aria-readonly="true" />
    </label>
    <label class="resource-basic-enabled">
      <span>是否启用</span>
      <span class="provider-toggle-control"><input type="checkbox" checked={resourceStatus === 'active'} on:change={(event) => (resourceStatus = (event.currentTarget as HTMLInputElement).checked ? 'active' : 'disabled')} aria-label="是否启用资源" /><i aria-hidden="true"></i></span>
    </label>
  </div>
  <label class="resource-basic-labels">
    <span>资源标签</span><input bind:value={resourceLabels} placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform" autocomplete="off" />
  </label>
</div>
