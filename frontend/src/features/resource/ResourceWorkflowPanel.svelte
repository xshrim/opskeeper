<script lang="ts">
  import MessageBanner from '../../components/MessageBanner.svelte';

  export let step = 1;
  export let kind = '';
  export let category = '';
  export let subtype = '';
  export let editingProvider = false;
  export let editingResource = false;
  export let editingDocker = false;
  export let editingKubernetes = false;
  export let basicConfigurationComplete = false;
  export let mcpConfigurationComplete = false;
  export let dockerConfigurationComplete = false;
  export let kubernetesConfigurationComplete = false;
  export let providerModelCount = 0;
  export let busy = false;
  export let scopeSelected = false;
  export let message = '';
  export let messageTone: 'success' | 'error' = 'success';
  export let stepTitle = '';
  export let validationMessage = '';
  export let onCancel: () => void = () => {};
  export let onSelectStep: (step: number) => void = () => {};
  export let onContinueBasic: () => void = () => {};
  export let onContinueProvider: () => void = () => {};
  export let onContinueMcp: () => void = () => {};
  export let onContinueDocker: () => void = () => {};
  export let onSubmitMcp: () => void = () => {};
  export let onSubmitDocker: () => void = () => {};
</script>

<section class="panel resource-add-workflow" aria-labelledby="resource-add-title">
  <header class="resource-add-main-heading">
    <div>
      <p class="eyebrow">{editingProvider || editingResource || editingDocker || editingKubernetes ? 'EDIT RESOURCE' : 'ADD RESOURCE'}</p>
      <h2 id="resource-add-title">
        <span>{editingProvider || editingResource || editingDocker || editingKubernetes ? '编辑资源' : '添加资源'}</span>
        {#if category && subtype}<small>{category} · {subtype}</small>{/if}
      </h2>
    </div>
    <button class="secondary" type="button" on:click={onCancel}>取消</button>
  </header>

  <aside class="resource-add-steps" aria-label="添加资源步骤">
    <button class:active={step === 1} class:done={step > 1} type="button" on:click={() => onSelectStep(1)}><b>1</b><span>基础配置</span></button>
    {#if kind === 'AIProvider'}
      <button class:active={step === 2} class:done={step > 2} disabled={!basicConfigurationComplete} type="button" on:click={() => basicConfigurationComplete && onSelectStep(2)}><b>2</b><span>Provider 配置</span></button>
      <button class:active={step === 3} class:done={step > 3} type="button" on:click={() => onSelectStep(3)}><b>3</b><span>Model 配置</span></button>
      <button class:active={step === 4} disabled={providerModelCount === 0} type="button" on:click={() => providerModelCount > 0 && onSelectStep(4)}><b>4</b><span>总结核验</span></button>
    {:else if kind === 'MCPServer'}
      <button class:active={step === 2} class:done={step > 2} disabled={!basicConfigurationComplete} type="button" on:click={() => basicConfigurationComplete && onSelectStep(2)}><b>2</b><span>MCP 配置</span></button>
      <button class:active={step === 3} disabled={!mcpConfigurationComplete} type="button" on:click={() => mcpConfigurationComplete && onSelectStep(3)}><b>3</b><span>总结核验</span></button>
    {:else if kind === 'Docker'}
      <button class:active={step === 2} class:done={step > 2} disabled={!basicConfigurationComplete} type="button" on:click={() => basicConfigurationComplete && onSelectStep(2)}><b>2</b><span>Docker 配置</span></button>
      <button class:active={step === 3} disabled={!dockerConfigurationComplete} type="button" on:click={() => dockerConfigurationComplete && onSelectStep(3)}><b>3</b><span>总结核验</span></button>
    {:else if kind === 'Kubernetes'}
      <button class:active={step === 2} class:done={step > 2} disabled={!basicConfigurationComplete} type="button" on:click={() => basicConfigurationComplete && onSelectStep(2)}><b>2</b><span>Kubernetes 配置</span></button>
      <button class:active={step === 3} disabled={!kubernetesConfigurationComplete} type="button" on:click={() => kubernetesConfigurationComplete && onSelectStep(3)}><b>3</b><span>总结核验</span></button>
    {:else}
      <button class:active={step === 2} disabled={!basicConfigurationComplete} type="button" on:click={() => basicConfigurationComplete && onSelectStep(2)}><b>2</b><span>配置资源</span></button>
    {/if}
  </aside>

  <div class="resource-add-content" class:type-selection-content={step === 1}>
    {#if message}<MessageBanner {message} tone={messageTone} />{/if}
    <div class="resource-add-step-heading">
      <h3>{stepTitle}</h3>
      {#if validationMessage}<p class="resource-add-step-validation" role="alert">{validationMessage}</p>{/if}
      <div class="resource-add-step-actions">
      {#if step === 1}
        <button class="primary" type="button" on:click={onContinueBasic}>下一步</button>
      {:else if kind === 'AIProvider' && step === 2}
        <button class="secondary" type="button" on:click={() => onSelectStep(1)}>上一步</button><button class="primary" type="button" on:click={onContinueProvider}>下一步</button>
      {:else if kind === 'AIProvider' && step === 3}
        <button class="secondary" type="button" on:click={() => onSelectStep(2)}>上一步</button><button class="primary" type="button" on:click={onContinueProvider}>下一步</button>
      {:else if kind === 'AIProvider' && step === 4}
        <button class="secondary" type="button" on:click={() => onSelectStep(3)}>上一步</button><button class="primary" type="submit" form="provider-create-form" disabled={busy || !scopeSelected}>{editingProvider ? '保存' : '创建'}</button>
      {:else if kind === 'MCPServer' && step === 2}
        <button class="secondary" type="button" on:click={() => onSelectStep(1)}>上一步</button><button class="primary" type="button" on:click={onContinueMcp}>下一步</button>
      {:else if kind === 'MCPServer' && step === 3}
        <button class="secondary" type="button" on:click={() => onSelectStep(2)}>上一步</button><button class="primary" type="button" on:click={onSubmitMcp} disabled={busy || !scopeSelected}>{editingResource ? '保存' : '创建'}</button>
      {:else if kind === 'Docker' && step === 2}
        <button class="secondary" type="button" on:click={() => onSelectStep(1)}>上一步</button><button class="primary" type="button" on:click={onContinueDocker}>下一步</button>
      {:else if kind === 'Docker' && step === 3}
        <button class="secondary" type="button" on:click={() => onSelectStep(2)}>上一步</button><button class="primary" type="button" on:click={onSubmitDocker} disabled={busy || !scopeSelected}>{editingDocker ? '保存' : '创建'}</button>
      {:else if kind === 'Kubernetes' && step === 2}
        <button class="secondary" type="button" on:click={() => onSelectStep(1)}>上一步</button><button class="primary" type="button" on:click={onContinueDocker}>下一步</button>
      {:else if kind === 'Kubernetes' && step === 3}
        <button class="secondary" type="button" on:click={() => onSelectStep(2)}>上一步</button><button class="primary" type="button" on:click={onSubmitDocker} disabled={busy || !scopeSelected}>{editingKubernetes ? '保存' : '创建'}</button>
      {:else if step === 2}
        <button class="secondary" type="button" on:click={() => onSelectStep(1)}>上一步</button><button class="primary" type="submit" form="resource-create-form" disabled={busy || !scopeSelected}>创建</button>
      {/if}
      </div>
    </div>
    <slot />
  </div>
</section>
