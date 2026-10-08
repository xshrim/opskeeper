<script lang="ts">
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';
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
      {@const fieldInvalid = configurationAttempted && isRequired(key) && !(field.sensitive ? (sensitiveValues[key] ?? '') : (values[key] ?? '')).trim()}
        {#if field.sensitive}
          <FormField label={field.title || key} required={isRequired(key)} invalid={fieldInvalid}><PasswordInput bind:value={sensitiveValues[key]} required={isRequired(key) && !(editMode && credentialConfigured)} ariaInvalid={fieldInvalid} placeholder={editMode && credentialConfigured ? '已有资源密文，留空保持不变' : '敏感信息将加密保存'} autocomplete="new-password" secretLabel={field.title || key} /></FormField>
        {:else if field.enum}
          <FormField label={field.title || key} required={isRequired(key)} invalid={fieldInvalid}><DropdownSelect bind:value={values[key]} options={field.enum.map((option) => ({ value: option, label: option }))} placeholder="未设置" ariaLabel={field.title || key} invalid={fieldInvalid} /></FormField>
        {:else if field.type === 'array'}
          <FormField label={field.title || key} required={isRequired(key)} invalid={fieldInvalid}><textarea bind:value={values[key]} required={isRequired(key)} rows="4" placeholder={'JSON 数组，例如 [{"name":"model","context_window":8192}]'} spellcheck="false"></textarea></FormField>
        {:else}
          <FormField label={field.title || key} required={isRequired(key)} invalid={fieldInvalid}><TextInput bind:value={values[key]} required={isRequired(key)} invalid={fieldInvalid} type={field.type === 'number' || field.type === 'integer' ? 'number' : field.type === 'url' || field.format === 'uri' ? 'url' : 'text'} placeholder={editMode ? undefined : field.description || key} autocomplete="off" /></FormField>
        {/if}
    {/each}
    {#if showTimeout && !schema.schema.properties.timeout_seconds}
      <FormField label="超时时间（秒）"><TextInput bind:value={timeoutSeconds} type="number" min="1" max="600" /></FormField>
    {/if}
  </div>
{:else}
  {#if showTimeout}
    <div class="schema-inputs">
      <p class="eyebrow">SCHEMA FIELDS</p>
      <FormField label="超时时间（秒）"><TextInput bind:value={timeoutSeconds} type="number" min="1" max="600" /></FormField>
    </div>
  {/if}
  <FormField label="配置 JSON"><textarea bind:value={rawConfig} rows="4" spellcheck="false"></textarea></FormField>
{/if}
