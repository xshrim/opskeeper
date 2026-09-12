<script lang="ts">
  import { Eye, EyeOff } from 'lucide-svelte';
  import type { Resource } from '../../lib/api';
  import {
    hostConnectionConfigurationValid,
    type HostAccessMode
  } from './resourceWorkflow';

  export let accessMode: HostAccessMode = 'direct';
  export let host = '';
  export let port = 22;
  export let username = '';
  export let authMethod: 'password' | 'key' = 'password';
  export let password = '';
  export let privateKey = '';
  export let passphrase = '';
  export let knownHosts = '';
  export let timeoutSeconds = 30;
  export let mcpServerResourceId = '';
  export let connectionOverride = false;
  export let mcpServers: Resource[] = [];
  export let allowAddMCPServer = true;
  export let configurationAttempted = false;
  export let credentialLoading = false;
  export let onConfigurationChange: () => void = () => {};
  export let onAddMCPServer: () => void = () => {};

  type CredentialField = 'privateKey' | 'knownHosts';
  let selectedFileNames: Record<CredentialField, string> = { privateKey: '', knownHosts: '' };
  let fileErrors: Record<CredentialField, string> = { privateKey: '', knownHosts: '' };
  let privateKeyFileInput: HTMLInputElement;
  let knownHostsFileInput: HTMLInputElement;
  let passwordVisible = false;
  let passphraseVisible = false;

  function openFilePicker(input: HTMLInputElement) {
    input?.click();
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
      if (field === 'privateKey') privateKey = value;
      else knownHosts = value;
      selectedFileNames = { ...selectedFileNames, [field]: file.webkitRelativePath || file.name };
      onConfigurationChange();
    } catch (error) {
      fileErrors = { ...fileErrors, [field]: error instanceof Error ? error.message : '文件读取失败' };
      selectedFileNames = { ...selectedFileNames, [field]: '' };
    } finally {
      input.value = '';
    }
  }

  $: valid = hostConnectionConfigurationValid({
    accessMode,
    host,
    port,
    username,
    authMethod,
    password,
    privateKey,
    passphrase,
    knownHosts,
    timeoutSeconds,
    mcpServerResourceId,
    connectionOverride
  });
  $: sshMode = Boolean(host.trim());
  $: if (authMethod === 'password') privateKey = '';
  $: if (authMethod === 'key') password = '';
  function selectMCPServer(event: Event) {
    if (
      (event.currentTarget as HTMLSelectElement).value === '__add_mcp_server__'
    ) {
      mcpServerResourceId = '';
      onAddMCPServer();
    }
    onConfigurationChange();
  }
</script>

{#if accessMode === 'agent'}
  <div class="docker-agent-form">
    <div class="docker-agent-connection-row">
      <label class:invalid={configurationAttempted && !mcpServerResourceId}
        ><span><i>*</i>关联 MCPServer</span><select
          bind:value={mcpServerResourceId}
          required
          on:change={selectMCPServer}
          ><option value="">请选择活动的 MCPServer</option
          >{#each mcpServers as server}<option value={server.id}
              >{server.name}</option
            >{/each}{#if allowAddMCPServer}<option value="__add_mcp_server__"
              >添加 MCPServer</option
            >{/if}</select
        ></label
      >
      <label class="docker-agent-override-toggle"
        ><span>使用自定义 Host 连接</span><button
          class="docker-switch"
          class:active={connectionOverride}
          type="button"
          aria-label="使用自定义 Host 连接"
          aria-pressed={connectionOverride}
          on:click={() => {
            connectionOverride = !connectionOverride;
            onConfigurationChange();
          }}><span></span></button
        ></label
      >
    </div>
    <p class="docker-field-help">
      Host Agent 统一支持本机和 SSH：工具参数未提供时，MCP Agent 读取 HOST_MCP_*
      环境变量，仍未配置则采集 Agent 所在主机。
    </p>
  </div>
{/if}

{#if accessMode === 'direct' || connectionOverride}
  <form
    id="host-create-form"
    class="docker-connection-form"
    on:submit|preventDefault
  >
    <div class="docker-form-grid">
      <label class:invalid={configurationAttempted && !valid}
        ><span>目标主机</span><input
          bind:value={host}
          on:input={onConfigurationChange}
          placeholder="留空采集本机；填写主机名后使用 SSH"
          autocomplete="off"
        /></label
      >
      <label
        ><span>{#if sshMode}<i>*</i>{/if}SSH 端口</span><input
          bind:value={port}
          on:input={onConfigurationChange}
          min="1"
          max="65535"
          type="number"
          required={sshMode}
          disabled={!sshMode}
        /></label
      >
      <label
        ><span>{#if sshMode}<i>*</i>{/if}SSH 用户</span><input
          bind:value={username}
          on:input={onConfigurationChange}
          placeholder="例如 opskeeper"
          autocomplete="username"
          required={sshMode}
          disabled={!sshMode}
        /></label
      >
      <label
        ><span>超时时间（秒）</span><input
          bind:value={timeoutSeconds}
          on:input={onConfigurationChange}
          min="1"
          max="300"
          type="number"
        /></label
      >
    </div>
    {#if sshMode}
      <div class="host-ssh-form-grid">
        <label class="host-auth-method">
          <span><i>*</i>认证方式</span>
          <select bind:value={authMethod} on:change={onConfigurationChange} required>
            <option value="password">密码</option>
            <option value="key">私钥</option>
          </select>
        </label>
        {#if authMethod === 'password'}
          <label class="host-password">
            <span><i>*</i>SSH 密码</span>
            <span class="resource-secret-control"><input bind:value={password} on:input={onConfigurationChange} type={passwordVisible ? 'text' : 'password'} autocomplete="current-password" placeholder="不会回显" required /><button class="resource-secret-toggle" type="button" aria-label={passwordVisible ? '隐藏 SSH 密码' : '显示 SSH 密码'} aria-pressed={passwordVisible} data-tooltip={passwordVisible ? '隐藏 SSH 密码' : '显示 SSH 密码'} on:click={() => (passwordVisible = !passwordVisible)}>{#if passwordVisible}<EyeOff size={16} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={16} strokeWidth={1.8} aria-hidden="true" />{/if}</button></span>
          </label>
          <label class="host-known-hosts">
            <span class="docker-credential-label"><span>known_hosts（可选）</span><span class="docker-file-picker">{#if selectedFileNames.knownHosts}<small>{selectedFileNames.knownHosts}</small>{/if}<input class="docker-file-input" bind:this={knownHostsFileInput} type="file" accept=".conf,.txt,text/plain" aria-label="选择 known_hosts 文件" on:change={(event) => void importCredentialFile(event, 'knownHosts')} /><button class="docker-file-import" type="button" aria-label="导入 known_hosts 文件" on:click|stopPropagation|preventDefault={() => openFilePicker(knownHostsFileInput)}>导入</button></span></span>
            <textarea bind:value={knownHosts} on:input={onConfigurationChange} rows="4" placeholder="可选；填写后校验 SSH 主机密钥" spellcheck="false"></textarea>
            {#if fileErrors.knownHosts}<small class="field-error">{fileErrors.knownHosts}</small>{/if}
          </label>
        {:else}
          <label class="host-passphrase">
            <span>私钥口令（可选）</span>
            <span class="resource-secret-control"><input bind:value={passphrase} on:input={onConfigurationChange} type={passphraseVisible ? 'text' : 'password'} autocomplete="off" placeholder="可选" /><button class="resource-secret-toggle" type="button" aria-label={passphraseVisible ? '隐藏私钥口令' : '显示私钥口令'} aria-pressed={passphraseVisible} data-tooltip={passphraseVisible ? '隐藏私钥口令' : '显示私钥口令'} on:click={() => (passphraseVisible = !passphraseVisible)}>{#if passphraseVisible}<EyeOff size={16} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={16} strokeWidth={1.8} aria-hidden="true" />{/if}</button></span>
          </label>
          <label class="host-private-key">
            <span class="docker-credential-label"><span><i>*</i>私钥</span><span class="docker-file-picker">{#if selectedFileNames.privateKey}<small>{selectedFileNames.privateKey}</small>{/if}<input class="docker-file-input" bind:this={privateKeyFileInput} type="file" accept=".pem,.key,.crt,text/plain,application/x-pem-file" aria-label="选择 SSH 私钥文件" on:change={(event) => void importCredentialFile(event, 'privateKey')} /><button class="docker-file-import" type="button" aria-label="导入 SSH 私钥文件" on:click|stopPropagation|preventDefault={() => openFilePicker(privateKeyFileInput)}>导入</button></span></span>
            <textarea bind:value={privateKey} on:input={onConfigurationChange} rows="4" placeholder="粘贴 PEM 或 Base64 编码私钥" spellcheck="false" required></textarea>
            {#if fileErrors.privateKey}<small class="field-error">{fileErrors.privateKey}</small>{/if}
          </label>
          <label class="host-known-hosts">
            <span class="docker-credential-label"><span>known_hosts（可选）</span><span class="docker-file-picker">{#if selectedFileNames.knownHosts}<small>{selectedFileNames.knownHosts}</small>{/if}<input class="docker-file-input" bind:this={knownHostsFileInput} type="file" accept=".conf,.txt,text/plain" aria-label="选择 known_hosts 文件" on:change={(event) => void importCredentialFile(event, 'knownHosts')} /><button class="docker-file-import" type="button" aria-label="导入 known_hosts 文件" on:click|stopPropagation|preventDefault={() => openFilePicker(knownHostsFileInput)}>导入</button></span></span>
            <textarea bind:value={knownHosts} on:input={onConfigurationChange} rows="4" placeholder="可选；填写后校验 SSH 主机密钥" spellcheck="false"></textarea>
            {#if fileErrors.knownHosts}<small class="field-error">{fileErrors.knownHosts}</small>{/if}
          </label>
        {/if}
      </div>
    {:else}
      <p class="docker-field-help">
        未填写目标主机时使用当前运行环境的 Linux procfs；SSH
        凭据不会进入资源配置或模型上下文。
      </p>
    {/if}
    {#if credentialLoading}<p class="docker-field-help">
        正在读取已有 Host 凭据，请稍候…
      </p>{/if}
  </form>
{/if}
