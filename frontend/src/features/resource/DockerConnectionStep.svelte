<script lang="ts">
  import type { Resource } from '../../lib/api';
  import { dockerHostSupportsTLS, dockerHostValid, dockerTLSValueValid, type DockerAccessMode } from './resourceWorkflow';

  export let accessMode: DockerAccessMode = 'direct';
  export let host = '';
  export let timeoutSeconds = 10;
  export let caBase64 = '';
  export let certBase64 = '';
  export let keyBase64 = '';
  export let skipTLSVerify = false;
export let mcpServerResourceId = '';
  export let connectionOverride = false;
  export let mcpServers: Resource[] = [];
  export let allowAddMCPServer = true;
  export let configurationAttempted = false;
  export let credentialLoading = false;
  export let onConfigurationChange: () => void = () => {};
  export let onAddMCPServer: () => void = () => {};

  type TLSField = 'ca' | 'cert' | 'key';
  let selectedFileNames: Record<TLSField, string> = { ca: '', cert: '', key: '' };
  let fileErrors: Record<TLSField, string> = { ca: '', cert: '', key: '' };
  let lastMcpServerResourceId = '';
  let caFileInput: HTMLInputElement;
  let certFileInput: HTMLInputElement;
  let keyFileInput: HTMLInputElement;

  function openFilePicker(input: HTMLInputElement) {
    input?.click();
  }

  async function importTLSFile(event: Event, field: TLSField) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    fileErrors = { ...fileErrors, [field]: '' };
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      if (!bytes.length) throw new Error('文件为空');
      const pem = new TextDecoder('utf-8', { fatal: true }).decode(bytes).trim();
      if (!pem) throw new Error('文件为空');
      if (field === 'ca') caBase64 = pem;
      if (field === 'cert') certBase64 = pem;
      if (field === 'key') keyBase64 = pem;
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

  function endpoint(resource: Resource) {
    return String(resource.config?.url ?? '未设置地址');
  }

  $: tlsSupported = dockerHostSupportsTLS(host);
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

{#if accessMode === 'agent'}
  <div class="docker-agent-form">
    <div class="docker-agent-connection-row">
      <label class:invalid={configurationAttempted && !mcpServerResourceId}>
        <span><i>*</i>关联 MCPServer</span>
        <select bind:value={mcpServerResourceId} required on:change={selectMCPServer} aria-describedby="docker-agent-help">
          <option value="">请选择活动的 MCPServer</option>
          {#each mcpServers as server}
            <option value={server.id}>{server.name} · {endpoint(server)}</option>
          {/each}
          {#if allowAddMCPServer}<option value="__add_mcp_server__">添加 MCPServer</option>{/if}
        </select>
      </label>
      <label class="docker-agent-override-toggle">
        <span>使用自定义 Docker 连接</span>
        <button class="docker-switch" class:active={connectionOverride} type="button" title="填写下方连接参数后，将其随工具调用传给通用 Docker MCPServer。" aria-pressed={connectionOverride} aria-label="使用自定义 Docker 连接" on:click={() => { connectionOverride = !connectionOverride; onConfigurationChange(); }}><span></span></button>
      </label>
    </div>
    <p id="docker-agent-help" class="docker-field-help">逻辑 Docker 资源仍是权限和审计主体；MCPServer 只提供传输路径。</p>
    {#if !mcpServers.length}
      <div class="docker-agent-empty" role="status">
        <strong>没有可用的 MCPServer</strong>
        <span>请先创建并启用一个 MCPServer，再选择 Agent 接入。</span>
      </div>
    {/if}
  </div>
{/if}

{#if accessMode === 'direct' || connectionOverride}
  <form id="docker-create-form" class="docker-connection-form" on:submit|preventDefault>
    <div class="docker-form-grid">
      <label class:invalid={configurationAttempted && (!host.trim() || !dockerHostValid(host))} class="docker-host-field">
        <span><i>*</i>Docker Host URL</span>
        <input
          bind:value={host}
          on:input={onConfigurationChange}
          type="text"
          required
          placeholder="例如 unix:///var/run/docker.sock 或 https://docker.example.com:2376"
          autocomplete="off"
          aria-describedby="docker-host-help"
        />
      </label>
      <label class="docker-timeout-field">
        <span>超时时间（秒）</span>
        <input bind:value={timeoutSeconds} on:input={onConfigurationChange} min="1" max="300" type="number" />
      </label>
      <p id="docker-host-help" class="docker-field-help">
        支持 <code>unix://</code>、<code>tcp://</code>、<code>http://</code> 和 <code>https://</code>；<code>unix://</code> 和 <code>http://</code> 不支持 TLS，<code>tcp://</code> 和 <code>https://</code> 支持 TLS；不能包含用户名或密码。
      </p>
    </div>

    <fieldset class="docker-tls-fieldset" disabled={!tlsSupported}>
      <legend>TLS 客户端配置（可选）</legend>
      <p class="docker-field-help">{tlsSupported ? 'CA 证书、客户端证书和私钥支持粘贴 PEM 文本或 Base64 编码，也支持导入 PEM 文件。数据库和工具参数统一使用 Base64；客户端证书和私钥必须成对。' : '当前 Docker Host URL 不支持 TLS 配置。请选择 tcp:// 或 https:// 后再配置。'}</p>
      <div class="docker-form-grid docker-tls-grid">
        <div class="docker-tls-certificate-row">
          <label class:invalid={tlsInvalid(caBase64)}>
          <span class="docker-credential-label"><span class="docker-tls-label">CA 证书 <button class="docker-switch inline" class:active={skipTLSVerify} type="button" aria-pressed={skipTLSVerify} aria-label="跳过 TLS 证书校验" on:click={() => { skipTLSVerify = !skipTLSVerify; onConfigurationChange(); }}><span></span></button><small>跳过校验</small></span><span class="docker-file-picker">{#if selectedFileNames.ca}<small>{selectedFileNames.ca}</small>{/if}<input class="docker-file-input" bind:this={caFileInput} type="file" accept=".pem,.crt,.cer,text/plain,application/x-pem-file" aria-label="选择 CA 证书文件" on:change={(event) => void importTLSFile(event, 'ca')} /><button class="docker-file-import" type="button" aria-label="导入 CA 证书文件" on:click|stopPropagation|preventDefault={() => openFilePicker(caFileInput)}>导入</button></span></span>
          <textarea bind:value={caBase64} on:input={onConfigurationChange} rows="6" placeholder="粘贴 CA 证书 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>
          {#if fileErrors.ca}<small class="field-error">{fileErrors.ca}</small>{/if}
          </label>
          <label class:invalid={tlsInvalid(certBase64)}>
          <span class="docker-credential-label"><span>客户端证书</span><span class="docker-file-picker">{#if selectedFileNames.cert}<small>{selectedFileNames.cert}</small>{/if}<input class="docker-file-input" bind:this={certFileInput} type="file" accept=".pem,.crt,.cer,text/plain,application/x-pem-file" aria-label="选择客户端证书文件" on:change={(event) => void importTLSFile(event, 'cert')} /><button class="docker-file-import" type="button" aria-label="导入客户端证书文件" on:click|stopPropagation|preventDefault={() => openFilePicker(certFileInput)}>导入</button></span></span>
          <textarea bind:value={certBase64} on:input={onConfigurationChange} rows="6" placeholder="粘贴客户端证书 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>
          {#if fileErrors.cert}<small class="field-error">{fileErrors.cert}</small>{/if}
          </label>
          <label class:invalid={tlsInvalid(keyBase64)}>
          <span class="docker-credential-label"><span>客户端私钥</span><span class="docker-file-picker">{#if selectedFileNames.key}<small>{selectedFileNames.key}</small>{/if}<input class="docker-file-input" bind:this={keyFileInput} type="file" accept=".pem,.key,.crt,text/plain,application/x-pem-file" aria-label="选择客户端私钥文件" on:change={(event) => void importTLSFile(event, 'key')} /><button class="docker-file-import" type="button" aria-label="导入客户端私钥文件" on:click|stopPropagation|preventDefault={() => openFilePicker(keyFileInput)}>导入</button></span></span>
          <textarea bind:value={keyBase64} on:input={onConfigurationChange} rows="6" placeholder="粘贴客户端私钥 PEM 文本或 Base64 编码" autocomplete="off" spellcheck="false"></textarea>
          {#if fileErrors.key}<small class="field-error">{fileErrors.key}</small>{/if}
          </label>
        </div>
      </div>
      {#if credentialLoading}<p class="docker-field-help">正在读取已有 TLS 凭据，请稍候…</p>{/if}
    </fieldset>
  </form>
{/if}
