<script lang="ts">
  import type { AIConnectionResult } from '../../lib/api';

  type ProviderModel = {
    name: string;
    contextWindowTokens: number;
    capabilities: string[];
  };
  type ProviderTypeOption = { value: string; label: string };
  type DraftTest = { result?: AIConnectionResult; error?: string } | null;

  export let resourceName = '';
  export let providerTypeOptions: ProviderTypeOption[] = [];
  export let providerType = '';
  export let resourceStatus = 'active';
  export let providerBaseURL = '';
  export let providerProtocol = 'chat_completions';
  export let providerTimeoutSeconds = 60;
  export let providerMaxConcurrency = 5;
  export let providerDefaultModel = '';
  export let providerModels: ProviderModel[] = [];
  export let providerDraftTestBusy = false;
  export let providerDraftTestPassedState = false;
  export let providerDraftTest: DraftTest = null;
  export let resourceLabelsConfigured = false;
  export let activeScopeSummary: () => string;
  export let providerPurposeTags: string[] = [];
  export let providerPurposeLabel: (tag: string) => string;
  export let providerCapabilityOptions: ProviderTypeOption[] = [];
  export let onTestConnection: () => void;
  export let onSubmit: () => void;
</script>

<p class="resource-add-description">
  使用默认 Model 完成连接核验后，才可创建 Provider 并发布以下 Model 列表。
</p>
<form id="provider-create-form" class="provider-summary" on:submit|preventDefault={onSubmit}>
  <div>
    <span>Provider</span><strong>{resourceName}</strong><small>{providerTypeOptions.find((item) => item.value === providerType)?.label} · {resourceStatus === 'active' ? '已启用' : '未启用'}</small>
  </div>
  <div>
    <span>服务地址</span><strong>{providerBaseURL}</strong><small>{providerProtocol} · 超时 {providerTimeoutSeconds} 秒 · 并发 {providerMaxConcurrency}</small>
  </div>
  <div>
    <span>默认 Model</span><strong>{providerDefaultModel}</strong><small>共 {providerModels.length} 个 Model，凭据将加密保存</small>
  </div>
  <div class="provider-test-summary">
    <span>连接核验</span>
    {#if providerDraftTestBusy}
      <strong>正在测试默认 Model...</strong><small>请求正在发送至 {providerDefaultModel}。</small>
    {:else if providerDraftTestPassedState}
      <strong class="success">连接正常 · {providerDraftTest?.result?.latency_ms} ms</strong><small>{providerDraftTest?.result?.message}</small>
    {:else if providerDraftTest?.error}
      <strong class="failed">连接失败</strong><small>{providerDraftTest.error}</small>
    {:else}
      <strong>尚未核验</strong><small>需验证默认 Model 可成功响应后才能创建。</small>
    {/if}
    <button class="secondary provider-test-button" type="button" disabled={providerDraftTestBusy} on:click={onTestConnection}>{providerDraftTestBusy ? '测试中' : '连接测试'}</button>
  </div>
  <div>
    <span>资源属性</span><strong>{activeScopeSummary()}</strong><small>{resourceLabelsConfigured ? '已配置的资源标签' : '未配置资源标签'}</small>
  </div>
  <div>
    <span>Provider角色</span><strong>{providerPurposeTags.length > 0 ? providerPurposeTags.map(providerPurposeLabel).join('、') : '未设置'}</strong><small>同级别同一角色会自动路由至此 Provider。</small>
  </div>
  <div class="provider-summary-models">
    <span>Model 列表</span>
    {#each providerModels as model}
      <div><strong>{model.name}</strong><small>{model.contextWindowTokens.toLocaleString()} Token · {model.capabilities.map((capability) => providerCapabilityOptions.find((item) => item.value === capability)?.label ?? capability).join('、')}</small></div>
    {/each}
  </div>
</form>
