<script lang="ts">
  import { Pencil, Trash2 } from 'lucide-svelte';

  type ProviderModel = {
    name: string;
    contextWindowTokens: number;
    maxOutputTokens: number;
    temperature: number;
    temperatureMutable: boolean;
    capabilities: string[];
    enabled: boolean;
    priority: number;
  };
  type Capability = { value: string; label: string };

  export let providerModelConfigurationAttempted = false;
  export let providerModelDraft: ProviderModel;
  export let providerModels: ProviderModel[] = [];
  export let editingProviderModelName = '';
  export let providerDefaultModel = '';
  export let providerCapabilityOptions: Capability[] = [];
  export let toggleProviderModelCapability: (capability: string) => void;
  export let addProviderModel: () => void;
  export let setProviderDefaultModel: (name: string) => void;
  export let setProviderModelEnabled: (name: string, enabled: boolean) => void;
  export let editProviderModel: (model: ProviderModel) => void;
  export let removeProviderModel: (name: string) => void;
</script>

<p class="resource-add-description">
  添加此 Provider 可用的模型；第一个添加的模型会自动设为默认模型，也可在下方调整。
</p>
<div class="provider-model-editor">
  <div class="provider-model-grid">
    <label class:invalid={providerModelConfigurationAttempted && (!providerModelDraft.name.trim() || providerModels.some((model) => model.name === providerModelDraft.name.trim() && model.name !== editingProviderModelName))}>
      <span><i>*</i>Model 名称</span><input bind:value={providerModelDraft.name} required placeholder="例如 gpt-4.1" autocomplete="off" />
    </label>
    <label class:invalid={providerModelConfigurationAttempted && providerModelDraft.contextWindowTokens <= 0}>
      <span><i>*</i>上下文窗口</span><input bind:value={providerModelDraft.contextWindowTokens} min="1" required type="number" />
    </label>
    <label><span>最大输出 Token</span><input bind:value={providerModelDraft.maxOutputTokens} min="1" type="number" /></label>
    <label><span>优先级</span><input bind:value={providerModelDraft.priority} min="0" type="number" /></label>
    <label>
      <span>温度</span><span class="provider-temperature-control"><input bind:value={providerModelDraft.temperature} min="0" max="2" step="0.1" type="number" /><span class="provider-temperature-toggle" data-tooltip="允许调用时调整温度参数"><input type="checkbox" bind:checked={providerModelDraft.temperatureMutable} aria-label="温度可调" /><i aria-hidden="true"></i></span></span>
    </label>
  </div>
  <div class="provider-model-capabilities-row">
    <div class="provider-model-flags" class:invalid={providerModelConfigurationAttempted && providerModelDraft.capabilities.length === 0}>
      <span><i>*</i>支持能力</span>
      {#each providerCapabilityOptions as capability}
        <button class:active={providerModelDraft.capabilities.includes(capability.value)} type="button" on:click={() => toggleProviderModelCapability(capability.value)}>{capability.label}</button>
      {/each}
    </div>
    <button class="secondary" type="button" on:click={addProviderModel}>{editingProviderModelName ? '保存修改' : '添加模型'}</button>
  </div>
</div>
<div class="provider-model-list">
  <div class="provider-model-list-heading"><strong>已配置模型</strong><span>{providerModels.length} 个</span></div>
  {#each providerModels as model}
    <div class="provider-model-row">
      <strong>{model.name}</strong><span>{model.contextWindowTokens.toLocaleString()} Token · 温度 {model.temperature}</span>
      <span>{model.capabilities.map((capability) => providerCapabilityOptions.find((item) => item.value === capability)?.label ?? capability).join('、')}</span>
      <label class="provider-model-default"><input type="radio" name="provider-default-model" value={model.name} checked={providerDefaultModel === model.name} on:change={() => setProviderDefaultModel(model.name)} /> 默认</label>
      <label class="provider-model-enabled" data-tooltip={model.name === providerDefaultModel ? '默认模型必须保持启用' : model.enabled ? '模型已启用，可被 Provider 调用' : '模型已停用，不会被 Provider 调用'}>
        <span class="visually-hidden">启用 {model.name}</span><span class="provider-toggle-control"><input type="checkbox" checked={model.enabled} disabled={model.name === providerDefaultModel} aria-label={'启用 ' + model.name} on:change={(event) => setProviderModelEnabled(model.name, (event.currentTarget as HTMLInputElement).checked)} /><i aria-hidden="true"></i></span>
      </label>
      <button class="icon-button" type="button" aria-label={`编辑 ${model.name}`} title="编辑模型" on:click={() => editProviderModel(model)}><Pencil size={14} aria-hidden="true" /></button>
      <button class="icon-button danger-action" type="button" aria-label={`删除 ${model.name}`} title="删除模型" on:click={() => removeProviderModel(model.name)}><Trash2 size={14} aria-hidden="true" /></button>
    </div>
  {:else}
    <div class="empty-state">尚未添加模型。</div>
  {/each}
</div>
