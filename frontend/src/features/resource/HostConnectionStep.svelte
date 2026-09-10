<script lang="ts">
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
        ><span>SSH 端口</span><input
          bind:value={port}
          on:input={onConfigurationChange}
          min="1"
          max="65535"
          type="number"
          disabled={!sshMode}
        /></label
      >
      <label
        ><span>SSH 用户</span><input
          bind:value={username}
          on:input={onConfigurationChange}
          placeholder="例如 opskeeper"
          autocomplete="username"
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
      <div class="docker-form-grid">
        <label
          ><span>认证方式</span><select
            bind:value={authMethod}
            on:change={onConfigurationChange}
            ><option value="password">密码</option><option value="key"
              >私钥</option
            ></select
          ></label
        >
        {#if authMethod === 'password'}<label
            ><span>SSH 密码</span><input
              bind:value={password}
              on:input={onConfigurationChange}
              type="password"
              autocomplete="current-password"
              placeholder="不会回显"
            /></label
          >{:else}<label
            ><span>私钥</span><textarea
              bind:value={privateKey}
              on:input={onConfigurationChange}
              rows="4"
              placeholder="PEM 或 Base64 编码私钥"
              spellcheck="false"
            ></textarea></label
          ><label
            ><span>私钥口令</span><input
              bind:value={passphrase}
              on:input={onConfigurationChange}
              type="password"
              autocomplete="off"
            /></label
          >{/if}
        <label
          ><span>known_hosts</span><textarea
            bind:value={knownHosts}
            on:input={onConfigurationChange}
            rows="3"
            placeholder="文件路径或 known_hosts 内容"
            spellcheck="false"
          ></textarea></label
        >
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
