<script lang="ts">
  import type { ResourceSchema } from '../../lib/api';

  export let resourceAddCategory = '';
  export let resourceAddSubtype = '';
  export let createSchema: ResourceSchema | null = null;
  export let resourceConfigValues: Record<string, string> = {};
  export let resourceSensitiveValues: Record<string, string> = {};
  export let resourceConfig = '';
  export let onSubmit: () => void;
</script>

<p class="resource-add-description">
  配置将按 {resourceAddCategory} · {resourceAddSubtype} 的资源契约保存；敏感字段会单独加密存储。
</p>
<form id="resource-create-form" class="stack-form resource-create-form" on:submit|preventDefault={onSubmit}>
  {#if createSchema?.schema.properties}
    <div class="schema-inputs">
      <p class="eyebrow">SCHEMA FIELDS</p>
      {#each Object.entries(createSchema.schema.properties) as [key, field]}
        <label>
          {field.title || key}
          {#if field.sensitive}
            <input type="password" bind:value={resourceSensitiveValues[key]} placeholder="敏感信息将加密保存" autocomplete="new-password" />
          {:else if field.enum}
            <select bind:value={resourceConfigValues[key]}><option value="">未设置</option>{#each field.enum as option}<option value={option}>{option}</option>{/each}</select>
          {:else if field.type === 'array'}
            <textarea bind:value={resourceConfigValues[key]} rows="4" placeholder={'JSON 数组，例如 [{"name":"model","context_window":8192}]'} spellcheck="false"></textarea>
          {:else}
            <input bind:value={resourceConfigValues[key]} type={field.type === 'number' || field.type === 'integer' ? 'number' : field.type === 'url' || field.format === 'uri' ? 'url' : 'text'} placeholder={field.description || key} autocomplete="off" />
          {/if}
        </label>
      {/each}
    </div>
  {:else}
    <label>配置 JSON<textarea bind:value={resourceConfig} rows="4" spellcheck="false"></textarea></label>
  {/if}
</form>
