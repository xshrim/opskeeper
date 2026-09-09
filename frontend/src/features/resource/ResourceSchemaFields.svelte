<script lang="ts">
  import type { ResourceSchema } from '../../lib/api';

  export let schema: ResourceSchema | null = null;
  export let values: Record<string, string> = {};
  export let sensitiveValues: Record<string, string> = {};
  export let rawConfig = '{}';
  export let editMode = false;
  export let credentialConfigured = false;
  export let showTimeout = false;
  export let timeoutSeconds = 60;
  export let isRequired: (key: string) => boolean = () => false;
  export let configurationAttempted = false;
</script>

{#if schema?.schema.properties && Object.keys(schema.schema.properties).length > 0}
  <div class="schema-inputs">
    <p class="eyebrow">SCHEMA FIELDS</p>
    {#each Object.entries(schema.schema.properties) as [key, field]}
      <label class:invalid={configurationAttempted && isRequired(key) && !(field.sensitive ? (sensitiveValues[key] ?? '') : (values[key] ?? '')).trim()}>
        <span>{#if isRequired(key)}<i>*</i>{/if}{field.title || key}</span>
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
    {#if showTimeout && !schema.schema.properties.timeout_seconds}
      <label>
        <span>超时时间（秒）</span>
        <input bind:value={timeoutSeconds} type="number" min="1" max="600" />
      </label>
    {/if}
  </div>
{:else}
  {#if showTimeout}
    <div class="schema-inputs">
      <p class="eyebrow">SCHEMA FIELDS</p>
      <label><span>超时时间（秒）</span><input bind:value={timeoutSeconds} type="number" min="1" max="600" /></label>
    </div>
  {/if}
  <label>配置 JSON<textarea bind:value={rawConfig} rows="4" spellcheck="false"></textarea></label>
{/if}
