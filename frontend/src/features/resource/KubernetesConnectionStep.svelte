<script lang="ts">
  import type { Resource } from '../../lib/api';
  import { dockerTLSValueValid } from './resourceWorkflow';
  import type { KubernetesConnectionMode } from './resourceWorkflow';

  export let isAgent = false;
  export let connectionOverride = false;
  export let connectionMode: KubernetesConnectionMode = 'kubeconfig';
  export let server = '';
  export let caBase64 = '';
  export let token = '';
  export let certBase64 = '';
  export let keyBase64 = '';
  export let kubeconfig = '';
  export let skipTLSVerify = false;
  export let mcpServerResourceId = '';
  export let mcpServers: Resource[] = [];
  export let allowAddMCPServer = true;
  export let configurationAttempted = false;
  export let credentialLoading = false;
  export let onConfigurationChange: () => void = () => {};
  export let onAddMCPServer: () => void = () => {};

  type CredentialField = 'kubeconfig' | 'ca' | 'cert' | 'key';
  let selectedFileNames: Record<CredentialField, string> = { kubeconfig: '', ca: '', cert: '', key: '' };
  let fileErrors: Record<CredentialField, string> = { kubeconfig: '', ca: '', cert: '', key: '' };
  let lastMcpServerResourceId = '';
  let kubeconfigFileInput: HTMLInputElement;
  let caFileInput: HTMLInputElement;
  let certFileInput: HTMLInputElement;
  let keyFileInput: HTMLInputElement;

  function openFilePicker(input: HTMLInputElement) {
    input?.click();
  }

  function endpoint(resource: Resource) {
    return String(resource.config?.url ?? '未设置地址');
  }

  async function importCredentialFile(event: Event, field: CredentialField) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    fileErrors = { ...fileErrors, [field]: '' };
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      if (!bytes.length) throw new Error('文件为空');
      const value = new TextDecoder('utf-8', { fatal: true }).decode(bytes).trim();
      if (!value) throw new Error('文件为空');
      if (field === 'kubeconfig') {
        kubeconfig = value;
      }
      if (field === 'ca') caBase64 = value;
      if (field === 'cert') certBase64 = value;
      if (field === 'key') keyBase64 = value;
      selectedFileNames = { ...selectedFileNames, [field]: file.webkitRelativePath || file.name };
      onConfigurationChange();
    } catch (error) {
      fileErrors = { ...fileErrors, [field]: error instanceof Error ? error.message : '文件读取失败' };
      selectedFileNames = { ...selectedFileNames, [field]: '' };
    } finally {
      input.value = '';
    }
  }

  function tlsInvalid(value: string) {
    return configurationAttempted && Boolean(value.trim()) && !dockerTLSValueValid(value);
  }

  $: if (mcpServerResourceId !== '__add_mcp_server__') lastMcpServerResourceId = mcpServerResourceId;

  function selectMCPServer(event: Event) {
    const value = (event.currentTarget as HTMLSelectElement).value;
    if (value === '__add_mcp_server__') {
      mcpServerResourceId = lastMcpServerResourceId;
      onAddMCPServer();
      return;
    }
    onConfigurationChange();
  }
</script>

<div class="docker-connection-form">
  {#if isAgent}
    <div class="docker-agent-form">
      <div class="docker-agent-connection-row">
        <label class:invalid={configurationAttempted && !mcpServerResourceId}>
          <span><i>*</i>关联 MCPServer</span>
          <select bind:value={mcpServerResourceId} on:change={selectMCPServer}>
            <option value="">请选择活动的 MCPServer</option>
            {#each mcpServers as server}
              <option value={server.id}>{server.name} · {endpoint(server)}</option>
            {/each}
            {#if allowAddMCPServer}<option value="__add_mcp_server__">添加 MCPServer</option>{/if}
          </select>
        </label>
        <label class="docker-agent-override-toggle">
          <span>使用自定义 Kubernetes 连接</span>
          <button class="docker-switch" class:active={connectionOverride} type="button" title="填写下方连接参数后，将其随工具调用传给通用 Kubernetes MCPServer。" aria-pressed={connectionOverride} aria-label="使用自定义 Kubernetes 连接" on:click={() => { connectionOverride = !connectionOverride; onConfigurationChange(); }}><span></span></button>
        </label>
      </div>
      <p class="docker-field-help">资源子类型已经决定 Agent 接入方式；MCPServer 只提供工具传输路径。</p>
      {#if !mcpServers.length}<div class="docker-agent-empty" role="status"><strong>没有可用的 MCPServer</strong><span>请先创建并启用一个 MCPServer。</span></div>{/if}
    </div>
  {/if}

  {#if !isAgent || connectionOverride}
  <div class="kubernetes-connection-mode" role="group" aria-label="API 连接方式">
    <button class:active={connectionMode === 'kubeconfig'} type="button" aria-pressed={connectionMode === 'kubeconfig'} on:click={() => { connectionMode = 'kubeconfig'; onConfigurationChange(); }}>
      <strong>Kubeconfig</strong>
      <small>使用 kubeconfig 文件连接集群，支持粘贴或导入。</small>
    </button>
    <button class:active={connectionMode === 'endpoint'} type="button" aria-pressed={connectionMode === 'endpoint'} on:click={() => { connectionMode = 'endpoint'; onConfigurationChange(); }}>
      <strong>API Server Endpoint</strong>
      <small>直接填写 API Server 与 Endpoint TLS 凭据。</small>
    </button>
  </div>

  {#if connectionMode === 'kubeconfig'}
    <fieldset class="docker-tls-fieldset">
      <legend>Kubeconfig 配置</legend>
      <div class="docker-form-grid docker-tls-grid">
        <div class="docker-tls-certificate-row kubernetes-kubeconfig-row">
          <label class:invalid={configurationAttempted && !kubeconfig.trim()}>
            <span class="docker-credential-label"><span><i>*</i>Kubeconfig</span><span class="docker-file-picker">{#if selectedFileNames.kubeconfig}<small>{selectedFileNames.kubeconfig}</small>{/if}<input class="docker-file-input" bind:this={kubeconfigFileInput} type="file" accept=".yaml,.yml,.conf,text/plain,application/yaml" aria-label="选择 kubeconfig 文件" on:change={(event) => void importCredentialFile(event, 'kubeconfig')} /><button class="docker-file-import" type="button" aria-label="导入 kubeconfig 文件" on:click|stopPropagation|preventDefault={() => openFilePicker(kubeconfigFileInput)}>导入</button></span></span>
            <textarea bind:value={kubeconfig} on:input={onConfigurationChange} rows="6" placeholder="粘贴 kubeconfig 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>
            {#if fileErrors.kubeconfig}<small class="field-error">{fileErrors.kubeconfig}</small>{/if}
          </label>
        </div>
      </div>
    </fieldset>
  {:else}
    <div class="docker-form-grid kubernetes-endpoint-fields">
      <label class:invalid={configurationAttempted && !server.trim()} class="kubernetes-api-server-field"><span><i>*</i>API Server URL</span><input bind:value={server} on:input={onConfigurationChange} placeholder="https://kubernetes.example:6443" /></label>
      <label><span>Token（可选）</span><input type="password" bind:value={token} on:input={onConfigurationChange} autocomplete="off" /></label>
    </div>
    <fieldset class="docker-tls-fieldset">
      <legend>Endpoint TLS 凭据（可选）</legend>
      <div class="docker-form-grid docker-tls-grid">
        <div class="docker-tls-certificate-row">
          <label class:invalid={tlsInvalid(caBase64)}><span class="docker-credential-label"><span class="docker-tls-label">CA 证书 <button class="docker-switch inline" class:active={skipTLSVerify} type="button" aria-pressed={skipTLSVerify} aria-label="跳过 TLS 证书校验" on:click={() => { skipTLSVerify = !skipTLSVerify; onConfigurationChange(); }}><span></span></button><small>跳过校验</small></span><span class="docker-file-picker">{#if selectedFileNames.ca}<small>{selectedFileNames.ca}</small>{/if}<input class="docker-file-input" bind:this={caFileInput} type="file" accept=".pem,.crt,.cer,text/plain,application/x-pem-file" aria-label="选择 CA 证书文件" on:change={(event) => void importCredentialFile(event, 'ca')} /><button class="docker-file-import" type="button" aria-label="导入 CA 证书文件" on:click|stopPropagation|preventDefault={() => openFilePicker(caFileInput)}>导入</button></span></span><textarea bind:value={caBase64} on:input={onConfigurationChange} rows="6" placeholder="粘贴 CA 证书 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>{#if fileErrors.ca}<small class="field-error">{fileErrors.ca}</small>{/if}</label>
          <label class:invalid={tlsInvalid(certBase64)}><span class="docker-credential-label"><span>客户端证书</span><span class="docker-file-picker">{#if selectedFileNames.cert}<small>{selectedFileNames.cert}</small>{/if}<input class="docker-file-input" bind:this={certFileInput} type="file" accept=".pem,.crt,.cer,text/plain,application/x-pem-file" aria-label="选择客户端证书文件" on:change={(event) => void importCredentialFile(event, 'cert')} /><button class="docker-file-import" type="button" aria-label="导入客户端证书文件" on:click|stopPropagation|preventDefault={() => openFilePicker(certFileInput)}>导入</button></span></span><textarea bind:value={certBase64} on:input={onConfigurationChange} rows="6" placeholder="粘贴客户端证书 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>{#if fileErrors.cert}<small class="field-error">{fileErrors.cert}</small>{/if}</label>
          <label class:invalid={tlsInvalid(keyBase64)}><span class="docker-credential-label"><span>客户端私钥</span><span class="docker-file-picker">{#if selectedFileNames.key}<small>{selectedFileNames.key}</small>{/if}<input class="docker-file-input" bind:this={keyFileInput} type="file" accept=".pem,.key,.crt,text/plain,application/x-pem-file" aria-label="选择客户端私钥文件" on:change={(event) => void importCredentialFile(event, 'key')} /><button class="docker-file-import" type="button" aria-label="导入客户端私钥文件" on:click|stopPropagation|preventDefault={() => openFilePicker(keyFileInput)}>导入</button></span></span><textarea bind:value={keyBase64} on:input={onConfigurationChange} rows="6" placeholder="粘贴客户端私钥 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>{#if fileErrors.key}<small class="field-error">{fileErrors.key}</small>{/if}</label>
        </div>
      </div>
    </fieldset>
  {/if}
  {/if}
  {#if credentialLoading}<small class="docker-field-help">正在读取已有 Kubernetes 凭据，请稍候…</small>{/if}
</div>
