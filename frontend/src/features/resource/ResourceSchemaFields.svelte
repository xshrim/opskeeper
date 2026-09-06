<script lang="ts">
  import type { ResourceSchema } from '../../lib/api';

  export let schema: ResourceSchema | null = null;
  export let values: Record<string, string> = {};
  export let sensitiveValues: Record<string, string> = {};
  export let rawConfig = '{}';
  export let editMode = false;
  export let credentialConfigured = false;
  export let isRequired: (key: string) => boolean = () => false;
</script>

{#if schema?.schema.properties}
  <div class="schema-inputs">
    <p class="eyebrow">SCHEMA FIELDS</p>
    {#each Object.entries(schema.schema.properties) as [key, field]}
      <label>
        <span>{#if editMode && isRequired(key)}<i>*</i>{/if}{field.title || key}</span>
        {#if field.sensitive}
          <input type="password" bind:value={sensitiveValues[key]} placeholder={editMode && credentialConfigured ? '已有关联凭据，留空保持不变' : '敏感信息将加密保存'} autocomplete="new-password" />
        {:else if field.enum}
          <select bind:value={values[key]}><option value="">未设置</option>{#each field.enum as option}<option value={option}>{option}</option>{/each}</select>
        {:else if field.type === 'array'}
          <textarea bind:value={values[key]} rows="4" placeholder={'JSON 数组，例如 [{"name":"model","context_window":8192}]'} spellcheck="false"></textarea>
        {:else}
          <input bind:value={values[key]} type={field.type === 'number' || field.type === 'integer' ? 'number' : field.type === 'url' || field.format === 'uri' ? 'url' : 'text'} placeholder={editMode ? undefined : field.description || key} autocomplete="off" />
        {/if}
      </label>
    {/each}
  </div>
{:else}
  <label>配置 JSON<textarea bind:value={rawConfig} rows="4" spellcheck="false"></textarea></label>
{/if}
