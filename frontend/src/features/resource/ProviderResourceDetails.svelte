<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor, resourceLabelsText } from './resourceCatalog';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let formatDate: (value: string) => string;
  export let modelsForResource: (resource: Resource) => Array<Record<string, unknown>>;
  export let modelCapabilities: (model: Record<string, unknown> | undefined) => string[];
  export let providerTypeLabel: (type: unknown) => string;

  $: config = resource.config ?? {};
  $: models = modelsForResource(resource);
  $: labelText = resourceLabelsText(resource);
</script>

<div class="provider-resource-details">
  <div class="provider-resource-meta">
    <div><span>Provider 类型</span><strong>{providerTypeLabel(config.provider_type)}</strong></div>
    <div><span>协议</span><strong>{String(config.protocol ?? 'chat_completions')}</strong></div>
    <div class="provider-resource-address"><span>服务地址</span><strong>{resourceEndpointFor(resource)}</strong></div>
    <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
    <div><span>请求超时</span><strong>{Number(config.timeout_seconds ?? 60)} 秒</strong></div>
    <div><span>最大并发</span><strong>{Number(config.max_concurrency ?? 5)}</strong></div>
    <div><span>限流</span><strong>{Number(config.rate_limit_per_minute ?? 0) > 0 ? `${Number(config.rate_limit_per_minute)} 次/分钟` : '不限流'}</strong></div>
    <div><span>默认 Model</span><strong>{String(config.default_model ?? models[0]?.name ?? '未设置')}</strong></div>
    <div><span>API Key</span><strong>{resource.credential_id ? '已配置凭据' : '未配置凭据'}</strong></div>
    <div><span>Provider 状态</span><strong>{resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知'}</strong></div>
    <div class="provider-resource-labels" data-tooltip={labelText || undefined}><span>标签</span><strong>{labelText || '未设置标签'}</strong></div>
    <div class="provider-resource-connection"><span>连接测试</span><strong>{resourceCheck ? resourceCheck.status === 'succeeded' ? `正常 · ${resourceCheck.latency_ms} ms` : '失败' : '尚未测试'}</strong></div>
  </div>
  <div class="provider-resource-models">
    <div class="provider-resource-models-heading"><strong>Model 列表</strong><span>{models.length} 个</span></div>
    {#each models as model}
      {@const name = String(model.name ?? '未命名 Model')}
      {@const isDefault = name === String(config.default_model ?? '').trim() || (!config.default_model && model === models[0])}
      <div class="provider-resource-model-row">
        <div class="provider-resource-model-name"><strong>{name}</strong><small>{isDefault ? '默认 Model' : '备用 Model'} · {model.enabled === false ? '已停用' : '已启用'}</small></div>
        <div><span>能力</span><strong>{modelCapabilities(model).join('、') || '未声明'}</strong></div>
        <div><span>上下文</span><strong>{Number(model.context_window_tokens ?? model.context_window ?? 128000).toLocaleString()} Token</strong></div>
        <div><span>最大输出</span><strong>{Number(model.max_output_tokens ?? 128000).toLocaleString()} Token</strong></div>
        <div><span>温度</span><strong>{Number(model.temperature ?? 0.7)}</strong></div>
        <div><span>优先级</span><strong>{Number(model.priority ?? 0)}</strong></div>
      </div>
    {:else}<div class="empty-state">尚未配置 Model。</div>{/each}
  </div>
</div>
