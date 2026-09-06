<script lang="ts">
  import type { Resource, ResourceSchema } from '../../lib/api';
  export let resource: Resource;
  export let schema: ResourceSchema | null = null;
</script>

{#if schema?.schema.properties}
  <div class="schema-fields">
    <p class="eyebrow">SCHEMA FIELDS</p>
    {#each Object.entries(schema.schema.properties) as [key, field]}
      <div><span>{field.title || key}</span><code>{field.sensitive ? resource.credential_id ? '已由加密凭据保存' : '未设置' : String(resource.config[key] ?? '未设置')}</code></div>
    {/each}
  </div>
{:else}
  <pre class="config-preview">{JSON.stringify(resource.config, null, 2)}</pre>
{/if}
