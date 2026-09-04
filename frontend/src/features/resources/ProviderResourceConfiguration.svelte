<script lang="ts">
  import { Eye, EyeOff } from 'lucide-svelte';

  type SelectOption = { value: string; label: string };

  export let providerTypeOptions: SelectOption[] = [];
  export let providerPurposeOptions: SelectOption[] = [];
  export let providerConfigurationAttempted = false;
  export let providerType = '';
  export let providerProtocol = 'chat_completions';
  export let providerPurposeTags: string[] = [];
  export let providerBaseURL = '';
  export let providerAPIKey = '';
  export let providerAPIKeyVisible = false;
  export let providerAPIKeyLoading = false;
  export let providerTimeoutSeconds = 60;
  export let providerMaxConcurrency = 5;
  export let providerRateLimitPerMinute = 0;
  export let providerBaseURLValid: () => boolean;
  export let onSelectProviderType: (type: string) => void;
  export let onTogglePurpose: (purpose: string) => void;
</script>

<p class="resource-add-description">
  配置 Provider 连接、运行边界和角色。凭据会作为独立加密对象保存，不会写入资源配置。
</p>
<div class="provider-config-form">
  <label class="provider-config-type" class:invalid={providerConfigurationAttempted && !providerType}>
    <span><i>*</i>Provider类型</span>
    <select bind:value={providerType} on:change={(event) => onSelectProviderType((event.currentTarget as HTMLSelectElement).value)}>
      {#each providerTypeOptions as option}<option value={option.value}>{option.label}</option>{/each}
    </select>
  </label>
  <label class="provider-config-protocol">
    <span>Provider协议</span>
    <select bind:value={providerProtocol}><option value="chat_completions">Chat Completions</option></select>
  </label>
  <div class="provider-purpose-options provider-config-purpose">
    <span>Provider角色</span>
    <div>
      {#each providerPurposeOptions as purpose}
        <button class:active={providerPurposeTags.includes(purpose.value)} type="button" aria-pressed={providerPurposeTags.includes(purpose.value)} on:click={() => onTogglePurpose(purpose.value)}>{purpose.label}</button>
      {/each}
    </div>
  </div>
  <label class="provider-config-url" class:invalid={providerConfigurationAttempted && !providerBaseURLValid()}>
    <span><i>*</i>服务地址</span>
    <input bind:value={providerBaseURL} required type="url" placeholder="https://api.example.com/v1" autocomplete="off" />
  </label>
  <label class="provider-config-api-key" class:invalid={providerConfigurationAttempted && !providerAPIKey.trim()}>
    <span>API Key</span>
    <span class="provider-api-key-control">
      <input bind:value={providerAPIKey} required type={providerAPIKeyVisible ? 'text' : 'password'} placeholder={providerAPIKeyLoading ? '正在读取 API Key…' : '请输入 API Key'} autocomplete="new-password" />
      <button class="provider-api-key-toggle" type="button" aria-label={providerAPIKeyVisible ? '隐藏 API Key' : '显示 API Key'} aria-pressed={providerAPIKeyVisible} data-tooltip={providerAPIKeyVisible ? '隐藏 API Key' : '显示 API Key'} on:click={() => (providerAPIKeyVisible = !providerAPIKeyVisible)}>
        {#if providerAPIKeyVisible}<EyeOff size={16} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={16} strokeWidth={1.8} aria-hidden="true" />{/if}
      </button>
    </span>
  </label>
  <label class="provider-config-timeout"><span>请求超时（秒）</span><input bind:value={providerTimeoutSeconds} min="1" max="300" type="number" /></label>
  <label class="provider-config-concurrency"><span>最大并发</span><input bind:value={providerMaxConcurrency} min="1" type="number" /></label>
  <label class="provider-config-rate-limit"><span>限流（请求/分钟）</span><input bind:value={providerRateLimitPerMinute} min="0" type="number" /></label>
</div>
