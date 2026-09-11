<script lang="ts">
  import { Eye, EyeOff } from 'lucide-svelte';

  export let url = '';
  export let token = '';
  export let requestHeaders = '';
  export let toolAllowlist = '';
  export let timeoutSeconds = 120;
  export let maxResponseBytes = 4 * 1024 * 1024;
  export let tokenPlaceholder = '保存于加密凭据';
  export let configurationAttempted = false;
  let tokenVisible = false;

  export let tlsCA = '';
  export let tlsCert = '';
  export let tlsKey = '';
  export let skipTLSVerify = false;

  type TLSField = 'ca' | 'cert' | 'key';
  let selectedFileNames: Record<TLSField, string> = { ca: '', cert: '', key: '' };
  let fileErrors: Record<TLSField, string> = { ca: '', cert: '', key: '' };
  let caFileInput: HTMLInputElement;
  let certFileInput: HTMLInputElement;
  let keyFileInput: HTMLInputElement;

  async function importTLSFile(event: Event, field: TLSField) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      const value = new TextDecoder('utf-8', { fatal: true }).decode(bytes).trim();
      if (!value) throw new Error('文件为空');
      if (field === 'ca') tlsCA = value;
      if (field === 'cert') tlsCert = value;
      if (field === 'key') tlsKey = value;
      selectedFileNames = { ...selectedFileNames, [field]: file.name };
      fileErrors = { ...fileErrors, [field]: '' };
    } catch (error) {
      fileErrors = { ...fileErrors, [field]: error instanceof Error ? error.message : '文件读取失败' };
    } finally {
      input.value = '';
    }
  }
</script>

<div class="mcp-resource-form">
  <label class="mcp-url-field" class:invalid={configurationAttempted && !url.trim()}><span><i>*</i>Server 地址</span><input bind:value={url} type="url" required placeholder="https://mcp.example.com/mcp" autocomplete="off" /></label>
  <label><span>Token</span><span class="resource-secret-control"><input bind:value={token} type={tokenVisible ? 'text' : 'password'} placeholder={tokenPlaceholder} autocomplete="new-password" /><button class="resource-secret-toggle" type="button" aria-label={tokenVisible ? '隐藏 Token' : '显示 Token'} aria-pressed={tokenVisible} data-tooltip={tokenVisible ? '隐藏 Token' : '显示 Token'} on:click={() => (tokenVisible = !tokenVisible)}>{#if tokenVisible}<EyeOff size={16} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={16} strokeWidth={1.8} aria-hidden="true" />{/if}</button></span></label>
  <div class="mcp-number-grid">
    <label><span>超时时间（秒）</span><input bind:value={timeoutSeconds} type="number" min="1" max="600" /></label>
    <label><span>响应体大小限制（字节）</span><input bind:value={maxResponseBytes} type="number" min="1" max="16777216" step="1024" /></label>
  </div>
  <label><span>请求 Header</span><textarea bind:value={requestHeaders} rows="3" placeholder="每行一个 Header，例如 X-Tenant: production"></textarea></label>
  <label class="mcp-tools-field"><span>工具白名单</span><textarea bind:value={toolAllowlist} rows="4" placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具" spellcheck="false"></textarea></label>
  <fieldset class="docker-tls-fieldset mcp-tls-fieldset">
    <legend>TLS 客户端配置（可选）</legend>
    <div class="docker-form-grid docker-tls-grid">
      <div class="docker-tls-certificate-row">
        <label>
          <span class="docker-credential-label"><span class="docker-tls-label">CA 证书 <button class="docker-switch inline" class:active={skipTLSVerify} type="button" aria-pressed={skipTLSVerify} aria-label="跳过 TLS 证书校验" on:click|stopPropagation|preventDefault={() => skipTLSVerify = !skipTLSVerify}><span></span></button><small>跳过校验</small></span><span class="docker-file-picker">{#if selectedFileNames.ca}<small>{selectedFileNames.ca}</small>{/if}<input class="docker-file-input" bind:this={caFileInput} type="file" accept=".pem,.crt,.cer,text/plain,application/x-pem-file" aria-label="选择 MCP CA 证书文件" on:change={(event) => void importTLSFile(event, 'ca')} /><button class="docker-file-import" type="button" on:click|stopPropagation|preventDefault={() => caFileInput?.click()}>导入</button></span></span>
          <textarea bind:value={tlsCA} rows="6" placeholder="粘贴 CA 证书 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>
          {#if fileErrors.ca}<small class="field-error">{fileErrors.ca}</small>{/if}
        </label>
        <label>
          <span class="docker-credential-label"><span>客户端证书</span><span class="docker-file-picker">{#if selectedFileNames.cert}<small>{selectedFileNames.cert}</small>{/if}<input class="docker-file-input" bind:this={certFileInput} type="file" accept=".pem,.crt,.cer,text/plain,application/x-pem-file" aria-label="选择 MCP 客户端证书文件" on:change={(event) => void importTLSFile(event, 'cert')} /><button class="docker-file-import" type="button" on:click|stopPropagation|preventDefault={() => certFileInput?.click()}>导入</button></span></span>
          <textarea bind:value={tlsCert} rows="6" placeholder="粘贴客户端证书 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>
          {#if fileErrors.cert}<small class="field-error">{fileErrors.cert}</small>{/if}
        </label>
        <label>
          <span class="docker-credential-label"><span>客户端私钥</span><span class="docker-file-picker">{#if selectedFileNames.key}<small>{selectedFileNames.key}</small>{/if}<input class="docker-file-input" bind:this={keyFileInput} type="file" accept=".pem,.key,.crt,text/plain,application/x-pem-file" aria-label="选择 MCP 客户端私钥文件" on:change={(event) => void importTLSFile(event, 'key')} /><button class="docker-file-import" type="button" on:click|stopPropagation|preventDefault={() => keyFileInput?.click()}>导入</button></span></span>
          <textarea bind:value={tlsKey} rows="6" placeholder="粘贴客户端私钥 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>
          {#if fileErrors.key}<small class="field-error">{fileErrors.key}</small>{/if}
        </label>
      </div>
    </div>
  </fieldset>
</div>
