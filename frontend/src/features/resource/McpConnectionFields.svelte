<script lang="ts">
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import TlsCredentialFields from './TlsCredentialFields.svelte';

  export let url = '';
  export let token = '';
  export let requestHeaders = '';
  export let toolAllowlist = '';
  export let timeoutSeconds = 120;
  export let maxResponseBytes = 4 * 1024 * 1024;
  export let tokenPlaceholder = '保存于资源加密字段';
  export let configurationAttempted = false;
  export let tlsCA = '';
  export let tlsCert = '';
  export let tlsKey = '';
  export let skipTLSVerify = false;
</script>

<div class="mcp-resource-form">
  <FormField
    label="Server 地址"
    required
    invalid={configurationAttempted && !url.trim()}
    className="mcp-url-field"
  >
    <TextInput
      bind:value={url}
      type="url"
      required
      placeholder="https://mcp.example.com/mcp"
      autocomplete="off"
      ariaLabel="Server 地址"
    />
  </FormField>

  <FormField label="Token">
    <PasswordInput
      bind:value={token}
      placeholder={tokenPlaceholder}
      autocomplete="new-password"
      ariaLabel="Token"
      secretLabel="Token"
    />
  </FormField>

  <div class="mcp-number-grid">
    <FormField label="超时时间（秒）">
      <TextInput
        bind:value={timeoutSeconds}
        type="number"
        min="1"
        max="600"
        ariaLabel="超时时间（秒）"
      />
    </FormField>
    <FormField label="响应体大小限制（字节）">
      <TextInput
        bind:value={maxResponseBytes}
        type="number"
        min="1"
        max="16777216"
        step={1024}
        ariaLabel="响应体大小限制（字节）"
      />
    </FormField>
  </div>

  <FormField label="请求 Header">
    <TextArea
      bind:value={requestHeaders}
      rows={3}
      placeholder="每行一个 Header，例如 X-Tenant: production"
    />
  </FormField>

  <FormField label="工具白名单" className="mcp-tools-field">
    <TextArea
      bind:value={toolAllowlist}
      rows={4}
      placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具"
      spellcheck={false}
    />
  </FormField>

  <TlsCredentialFields
    bind:ca={tlsCA}
    bind:cert={tlsCert}
    bind:key={tlsKey}
    bind:skipVerify={skipTLSVerify}
    {configurationAttempted}
    legend="TLS 客户端配置（可选）"
    showSkipVerify
  />
</div>
