<script lang="ts">
  import { Pencil, Trash2 } from 'lucide-svelte';
  import type { ProviderModel } from './resourceWorkflow';

  export let draft: ProviderModel;
  export let models: ProviderModel[] = [];
  export let defaultModel = '';
  export let capabilityOptions: Array<{ value: string; label: string }> = [];
  export let configurationAttempted = false;
  export let editingModelName = '';
  export let radioName = 'provider-default-model';
  export let showEnabledControl = true;
  export let onToggleCapability: (value: string) => void = () => {};
  export let onAddModel: () => void = () => {};
  export let onSetDefault: (name: string) => void = () => {};
  export let onSetEnabled: (name: string, enabled: boolean) => void = () => {};
  export let onEditModel: (model: ProviderModel) => void = () => {};
  export let onRemoveModel: (name: string) => void = () => {};
</script>

<p class="resource-add-description">添加此 Provider 可用的模型；第一个添加的模型会自动设为默认模型，也可在下方调整。</p>
<div class="provider-model-editor">
  <div class="provider-model-grid">
    <label class:invalid={configurationAttempted && (!draft.name.trim() || models.some((model) => model.name === draft.name.trim() && model.name !== editingModelName))}><span><i>*</i>Model 名称</span><input bind:value={draft.name} required placeholder="例如 gpt-4.1" autocomplete="off" /></label>
    <label class:invalid={configurationAttempted && draft.contextWindowTokens <= 0}><span><i>*</i>上下文窗口</span><input bind:value={draft.contextWindowTokens} min="1" required type="number" /></label>
    <label><span>最大输出 Token</span><input bind:value={draft.maxOutputTokens} min="1" type="number" /></label>
    <label><span>优先级</span><input bind:value={draft.priority} min="0" type="number" /></label>
    <label><span>温度</span><span class="provider-temperature-control"><input bind:value={draft.temperature} min="0" max="2" step="0.1" type="number" /><span class="provider-temperature-toggle" data-tooltip="允许调用时调整温度参数"><input type="checkbox" bind:checked={draft.temperatureMutable} aria-label="温度可调" /><i aria-hidden="true"></i></span></span></label>
  </div>
  <div class="provider-model-capabilities-row">
    <div class="provider-model-flags" class:invalid={configurationAttempted && draft.capabilities.length === 0}>
      <span><i>*</i>支持能力</span>
      {#each capabilityOptions as capability}<button class:active={draft.capabilities.includes(capability.value)} type="button" on:click={() => onToggleCapability(capability.value)}>{capability.label}</button>{/each}
    </div>
    <button class="secondary" type="button" on:click={onAddModel}>{editingModelName ? '保存修改' : '添加模型'}</button>
  </div>
</div>
<div class="provider-model-list">
  <div class="provider-model-list-heading"><strong>已配置模型</strong><span>{models.length} 个</span></div>
  {#each models as model}
    <div class="provider-model-row">
      <strong>{model.name}</strong><span>{model.contextWindowTokens.toLocaleString()} Token · 温度 {model.temperature}</span><span>{model.capabilities.map((capability) => capabilityOptions.find((item) => item.value === capability)?.label ?? capability).join('、')}</span>
      <label class="provider-model-default"><input type="radio" name={radioName} value={model.name} checked={defaultModel === model.name} on:change={() => onSetDefault(model.name)} /> 默认</label>
      {#if showEnabledControl}<label class="provider-model-enabled" data-tooltip={model.name === defaultModel ? '默认模型必须保持启用' : model.enabled ? '模型已启用，可被 Provider 调用' : '模型已停用，不会被 Provider 调用'}><span class="visually-hidden">启用 {model.name}</span><span class="provider-toggle-control"><input type="checkbox" checked={model.enabled} disabled={model.name === defaultModel} aria-label={'启用 ' + model.name} on:change={(event) => onSetEnabled(model.name, (event.currentTarget as HTMLInputElement).checked)} /><i aria-hidden="true"></i></span></label>{/if}
      <button class="icon-button" type="button" aria-label={`编辑 ${model.name}`} title="编辑模型" on:click={() => onEditModel(model)}><Pencil size={14} aria-hidden="true" /></button>
      <button class="icon-button danger-action" type="button" aria-label={`删除 ${model.name}`} title="删除模型" on:click={() => onRemoveModel(model.name)}><Trash2 size={14} aria-hidden="true" /></button>
    </div>
  {:else}<div class="empty-state">尚未添加模型。</div>{/each}
</div>
