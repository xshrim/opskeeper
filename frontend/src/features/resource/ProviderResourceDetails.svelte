<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor, resourceLabelsText } from './resourceCatalog';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let formatDate: (value: string) => string;
  export let modelsForResource: (resource: Resource) => Array<Record<string, unknown>>;
  export let modelCapabilities: (model: Record<string, unknown> | undefined) => string[];
  export let providerTypeLabel: (type: unknown) => string;
  export let providerBindingsFor: (resource: Resource) => Array<{ tag: string }>;
  export let providerPurposeLabel: (tag: string) => string;

  $: config = resource.config ?? {};
  $: models = modelsForResource(resource);
  $: labelText = resourceLabelsText(resource);
  $: defaultModel = models.find((model) => String(model.name ?? '').trim() === String(config.default_model ?? '').trim()) ?? models.find((model) => model.enabled !== false) ?? models[0];
  $: roleText = providerBindingsFor(resource).map((binding) => providerPurposeLabel(binding.tag)).join('、');
  $: capabilityText = modelCapabilities(defaultModel).join('、');
</script>

<div class="provider-resource-details">
  <div class="provider-resource-meta">
    <div><span>Provider 类型</span><strong>{providerTypeLabel(config.provider_type)}</strong></div>
    <div><span>协议</span><strong>{String(config.protocol ?? 'chat_completions')}</strong></div>
    <div class="provider-resource-address"><span>服务地址</span><strong>{resourceEndpointFor(resource)}</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>超时时间</span><strong>{Number(config.timeout_seconds ?? 60)} 秒</strong></div>
    <div><span>角色</span><strong>{roleText || '未设置'}</strong></div>
    <div><span>能力</span><strong>{capabilityText || '未声明'}</strong></div>
    <div><span>默认模型</span><strong>{String(config.default_model ?? models[0]?.name ?? '未设置')}</strong></div>
    <div><span>API Key</span><strong>{resource.credential_id ? '已配置凭据' : '未配置凭据'}</strong></div>
    <div class="provider-resource-labels" data-tooltip={labelText || undefined}><span>标签</span><strong>{labelText || '未设置标签'}</strong></div>
    <div class="provider-resource-connection"><span>连接状态</span><strong>{resourceCheck ? resourceCheck.status === 'succeeded' ? `正常 · ${resourceCheck.latency_ms} ms` : '异常' : '尚未测试'}</strong>{#if resourceCheck?.status === 'failed'}<small>{resourceCheck.message}</small>{/if}</div>
    <div><span>启用状态</span><strong>{resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知'}</strong></div>
  </div>
  <div class="provider-resource-models">
    <div class="provider-resource-models-heading"><strong>模型列表</strong><span>{models.length} 个</span></div>
    {#each models as model}
      {@const name = String(model.name ?? '未命名 Model')}
      {@const isDefault = name === String(config.default_model ?? '').trim() || (!config.default_model && model === models[0])}
      <div class="provider-resource-model-row">
        <div class="provider-resource-model-name"><span>模型</span><strong>{name}{#if isDefault}<em class="provider-default-model-tag">默认</em>{/if}</strong></div>
        <div><span>能力</span><strong>{modelCapabilities(model).join('、') || '未声明'}</strong></div>
        <div><span>上下文</span><strong>{Number(model.context_window_tokens ?? model.context_window ?? 128000).toLocaleString()} Token</strong></div>
        <div><span>最大输出</span><strong>{Number(model.max_output_tokens ?? 128000).toLocaleString()} Token</strong></div>
        <div><span>温度</span><strong>{Number(model.temperature ?? 0.7)}</strong></div>
        <div><span>优先级</span><strong>{Number(model.priority ?? 0)}</strong></div>
      </div>
    {:else}<div class="empty-state">尚未配置 Model。</div>{/each}
  </div>
</div>
