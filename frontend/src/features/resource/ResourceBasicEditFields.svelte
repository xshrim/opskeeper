<script lang="ts">
  import type { Resource } from '../../lib/api';
  import { resourceCategoryFor, resourceCategoryOptions, resourceSubtypeFor, resourceSubtypeOptionsFor } from './resourceCatalog';

  export let resource: Resource;
  export let name = '';
  export let status = 'active';
  export let labels = '';
  export let scopeName: (id: string) => string;
</script>

<h3 class="editor-section-title">基础配置</h3>
<p class="editor-section-description">资源类型和子类型只读，资源名称、启用状态与资源标签可在此调整。</p>
<div class="resource-basic-edit-grid">
  <div class="resource-basic-type-row">
    <label><span>资源类型</span><select value={resourceCategoryFor(resource)} disabled aria-label="资源类型">{#each Object.keys(resourceCategoryOptions).filter((category) => category !== '全部') as category}<option value={category}>{category}</option>{/each}</select></label>
    <label><span>资源子类型</span><select value={resourceSubtypeFor(resource)} disabled aria-label="资源子类型">{#each resourceSubtypeOptionsFor(resource) as subtype}<option value={subtype}>{subtype}</option>{/each}</select></label>
  </div>
  <div class="resource-basic-identity-row">
    <label><span><i>*</i>资源名称</span><input bind:value={name} required /></label>
    <label><span>资源级别</span><input value={scopeName(resource.scope_id)} readonly /></label>
    <label class="resource-basic-enabled"><span>是否启用</span><span class="provider-toggle-control"><input type="checkbox" checked={status === 'active'} on:change={(event) => (status = (event.currentTarget as HTMLInputElement).checked ? 'active' : 'disabled')} aria-label="是否启用资源" /><i aria-hidden="true"></i></span></label>
  </div>
  <label class="resource-basic-labels"><span>资源标签</span><input bind:value={labels} placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform" autocomplete="off" /></label>
</div>
