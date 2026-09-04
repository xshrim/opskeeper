<script lang="ts">
  import MessageBanner from '../../components/MessageBanner.svelte';

  export let activeMessage = '';
  export let activeMessageTone: 'success' | 'error' = 'success';
  export let editingProviderResourceId = '';
  export let editingResourceId = '';
  export let resourceAddStep = 1;
  export let resourceKind = '';
  export let busy = false;
  export let selectedScopeId = '';
  export let resourceAddStepTitle: (step: number, kind: string) => string;
  export let resourceAddStepValidationMessage: () => string;
  export let onContinueResourceAdd: () => void;
  export let onContinueProviderAdd: () => void;
  export let onPrevious: (step: number) => void;
  export let onContinueMCP: () => void;
  export let onSubmitMCP: () => void;
</script>

<div class="resource-add-step-heading">
  {#if activeMessage}
    <MessageBanner message={activeMessage} tone={activeMessageTone} />
  {/if}
  <h3>{resourceAddStepTitle(resourceAddStep, resourceKind)}</h3>
  {#if resourceAddStepValidationMessage()}
    <p class="resource-add-step-validation" role="alert">
      {resourceAddStepValidationMessage()}
    </p>
  {/if}
  <div class="resource-add-step-actions">
    {#if resourceAddStep === 1}
      <button class="primary" type="button" on:click={onContinueResourceAdd}>下一步</button>
    {:else if resourceKind === 'AIProvider' && resourceAddStep === 2}
      <button class="secondary" type="button" on:click={() => onPrevious(1)}>上一步</button>
      <button class="primary" type="button" on:click={onContinueProviderAdd}>下一步</button>
    {:else if resourceKind === 'AIProvider' && resourceAddStep === 3}
      <button class="secondary" type="button" on:click={() => onPrevious(2)}>上一步</button>
      <button class="primary" type="button" on:click={onContinueProviderAdd}>下一步</button>
    {:else if resourceKind === 'AIProvider' && resourceAddStep === 4}
      <button class="secondary" type="button" on:click={() => onPrevious(3)}>上一步</button>
      <button class="primary" type="submit" form="provider-create-form" disabled={busy || !selectedScopeId}>{editingProviderResourceId ? '保存' : '创建'}</button>
    {:else if resourceKind === 'MCPServer' && resourceAddStep === 2}
      <button class="secondary" type="button" on:click={() => onPrevious(1)}>上一步</button>
      <button class="primary" type="button" on:click={onContinueMCP}>下一步</button>
    {:else if resourceKind === 'MCPServer' && resourceAddStep === 3}
      <button class="secondary" type="button" on:click={() => onPrevious(2)}>上一步</button>
      <button class="primary" type="button" on:click={onSubmitMCP} disabled={busy || !selectedScopeId}>{editingResourceId ? '保存' : '创建'}</button>
    {:else if resourceAddStep === 2}
      <button class="secondary" type="button" on:click={() => onPrevious(1)}>上一步</button>
      <button class="primary" type="submit" form="resource-create-form" disabled={busy || !selectedScopeId}>创建</button>
    {/if}
  </div>
</div>
