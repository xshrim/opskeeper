<script lang="ts">
  import { ChevronDown, ChevronRight } from 'lucide-svelte';

  type Provider = {
    provider_resource_id: string;
    name: string;
    models: Array<{ name: string; capabilities: string[] }>;
  };

  export let providers: Provider[] = [];
  export let selectedProviderId = '';
  export let modelName = '';
  export let open = false;
  export let menuProviderId = '';
  export let onToggle: () => void;
  export let onProvider: (providerID: string) => void;
  export let onModel: (modelName: string) => void;

  $: selectedProvider = providers.find((item) => item.provider_resource_id === selectedProviderId);
  $: menuProvider = providers.find((item) => item.provider_resource_id === (menuProviderId || selectedProviderId))
    ?? selectedProvider ?? providers[0];
</script>

<div class="diagnosis-model-picker">
  <button
    class="diagnosis-model-trigger"
    type="button"
    aria-label="选择模型服务商和模型"
    aria-haspopup="menu"
    aria-expanded={open}
    disabled={providers.length === 0}
    on:click={onToggle}
  >
    <span>{#if selectedProvider}{selectedProvider.name}{:else}暂无可用模型服务商{/if}{#if modelName} · {modelName}{/if}</span>
    <ChevronDown size={13} aria-hidden="true" />
  </button>
  {#if open}
    <div class="diagnosis-model-menu" role="menu" aria-label="模型服务商和模型">
      <div class="diagnosis-model-provider-list">
        <small class="diagnosis-model-menu-heading">模型服务商</small>
        {#each providers as provider}
          <button
            type="button"
            role="menuitem"
            class:active={provider.provider_resource_id === menuProvider?.provider_resource_id}
            aria-haspopup="menu"
            aria-expanded={provider.provider_resource_id === menuProvider?.provider_resource_id}
            on:click={() => onProvider(provider.provider_resource_id)}
          ><span>{provider.name}</span><ChevronRight size={12} aria-hidden="true" /></button>
        {/each}
      </div>
      <div class="diagnosis-model-option-list" role="menu" aria-label="模型">
        <small class="diagnosis-model-menu-heading">模型</small>
        {#if menuProvider}
          {#each menuProvider.models as model}
            {@const name = String(model.name ?? '')}
            <button
              type="button"
              role="menuitemradio"
              aria-checked={selectedProviderId === menuProvider.provider_resource_id && modelName === name}
              class:active={selectedProviderId === menuProvider.provider_resource_id && modelName === name}
              on:click={() => onModel(name)}
            ><span>{name}</span></button>
          {/each}
        {:else}<span class="diagnosis-model-empty">暂无可用模型</span>{/if}
      </div>
    </div>
  {/if}
</div>
