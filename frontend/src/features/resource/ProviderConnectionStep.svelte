<script lang="ts">
  import { Eye, EyeOff } from 'lucide-svelte';

  type ProviderOption = { value: string; label: string; baseURL: string };
  type PurposeOption = { value: string; label: string; requiredCapabilities?: string[] };

  export let type = 'openai_compatible';
  export let protocol = 'chat_completions';
  export let baseURL = '';
  export let apiKey = '';
  export let apiKeyVisible = false;
  export let apiKeyLoading = false;
  export let timeoutSeconds = 60;
  export let maxConcurrency = 5;
  export let rateLimitPerMinute = 0;
  export let purposeTags: string[] = [];
  export let typeOptions: ProviderOption[] = [];
  export let purposeOptions: PurposeOption[] = [];
  export let configurationAttempted = false;
  export let baseURLValid = true;
  export let showAPIKey = true;
  export let onSelectType: (type: string) => void = () => {};
  export let onTogglePurpose: (purpose: string) => void = () => {};
</script>

<div class="provider-config-form">
  <label class="provider-config-type" class:invalid={configurationAttempted && !type}>
    <span><i>*</i>Provider类型</span>
    <select bind:value={type} on:change={(event) => onSelectType((event.currentTarget as HTMLSelectElement).value)}>
      {#each typeOptions as option}<option value={option.value}>{option.label}</option>{/each}
    </select>
  </label>
  <label class="provider-config-protocol"><span>Provider协议</span><select bind:value={protocol}><option value="chat_completions">Chat Completions</option></select></label>
  <div class="provider-purpose-options provider-config-purpose">
    <span>Provider角色</span>
    <div>
      {#each purposeOptions as purpose}<button class:active={purposeTags.includes(purpose.value)} type="button" aria-pressed={purposeTags.includes(purpose.value)} on:click={() => onTogglePurpose(purpose.value)}>{purpose.label}</button>{/each}
    </div>
  </div>
  <label class="provider-config-url" class:invalid={configurationAttempted && !baseURLValid}>
    <span><i>*</i>服务地址</span><input bind:value={baseURL} required type="url" placeholder="https://api.example.com/v1" autocomplete="off" />
  </label>
  {#if showAPIKey}<label class="provider-config-api-key">
      <span>API Key</span>
      <span class="provider-api-key-control">
        <input bind:value={apiKey} type={apiKeyVisible ? 'text' : 'password'} placeholder={apiKeyLoading ? '正在读取 API Key…' : '请输入 API Key（可选）'} autocomplete="new-password" />
        <button class="provider-api-key-toggle" type="button" aria-label={apiKeyVisible ? '隐藏 API Key' : '显示 API Key'} aria-pressed={apiKeyVisible} data-tooltip={apiKeyVisible ? '隐藏 API Key' : '显示 API Key'} on:click={() => (apiKeyVisible = !apiKeyVisible)}>
          {#if apiKeyVisible}<EyeOff size={16} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={16} strokeWidth={1.8} aria-hidden="true" />{/if}
        </button>
      </span>
    </label>{/if}
  <label class="provider-config-timeout"><span>超时时间（秒）</span><input bind:value={timeoutSeconds} min="1" max="300" type="number" /></label>
  <label class="provider-config-concurrency"><span>最大并发</span><input bind:value={maxConcurrency} min="1" type="number" /></label>
  <label class="provider-config-rate-limit"><span>限流（请求/分钟）</span><input bind:value={rateLimitPerMinute} min="0" type="number" /></label>
</div>
