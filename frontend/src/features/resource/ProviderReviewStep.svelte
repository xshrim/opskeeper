<script lang="ts">
  import type { ProviderModel } from './resourceWorkflow';
  export let resourceName = '';
  export let providerTypeLabel = '';
  export let providerStatus = 'active';
  export let baseURL = '';
  export let protocol = '';
  export let timeoutSeconds = 60;
  export let maxConcurrency = 5;
  export let defaultModel = '';
  export let models: ProviderModel[] = [];
  export let scopeSummary = '';
  export let labelsConfigured = false;
  export let purposeLabels: string[] = [];
  export let testBusy = false;
  export let testPassed = false;
  export let testLatency: number | undefined;
  export let testMessage = '';
  export let testError = '';
  export let capabilityLabel: (value: string) => string = (value) => value;
  export let onTest: () => void = () => {};
  export let onSubmit: () => void = () => {};
</script>

<p class="resource-add-description">使用默认模型完成连接核验后，才可创建 Provider 并发布以下模型列表。</p>
<form id="provider-create-form" class="provider-summary" on:submit|preventDefault={onSubmit}>
  <div><span>Provider</span><strong>{resourceName}</strong><small>{providerTypeLabel} · {providerStatus === 'active' ? '已启用' : '未启用'}</small></div>
  <div><span>服务地址</span><strong>{baseURL}</strong><small>{protocol} · 超时 {timeoutSeconds} 秒 · 并发 {maxConcurrency}</small></div>
  <div><span>默认 Model</span><strong>{defaultModel}</strong><small>共 {models.length} 个 Model，凭据将加密保存</small></div>
  <div class="provider-test-summary">
    <span>连接核验</span>
    {#if testBusy}<strong>正在测试默认 Model...</strong><small>请求正在发送至 {defaultModel}。</small>
    {:else if testPassed}<strong class="success">连接正常 · {testLatency} ms</strong><small>{testMessage}</small>
    {:else if testError}<strong class="failed">连接失败</strong><small>{testError}</small>
    {:else}<strong>尚未核验</strong><small>需验证默认 Model 可成功响应后才能创建。</small>{/if}
    <button class="secondary provider-test-button" type="button" disabled={testBusy} on:click={onTest}>{testBusy ? '测试中' : '连接测试'}</button>
  </div>
  <div><span>资源属性</span><strong>{scopeSummary}</strong><small>{labelsConfigured ? '已配置的资源标签' : '未配置资源标签'}</small></div>
  <div><span>Provider角色</span><strong>{purposeLabels.length > 0 ? purposeLabels.join('、') : '未设置'}</strong><small>同级别同一角色会自动路由至此 Provider。</small></div>
  <div class="provider-summary-models"><span>模型列表</span>{#each models as model}<div><strong>{model.name}</strong><small>{model.contextWindowTokens.toLocaleString()} Token · {model.capabilities.map(capabilityLabel).join('、')}</small></div>{/each}</div>
</form>
