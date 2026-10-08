<script lang="ts">
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import Switch from '../../components/Switch.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import McpServerSelect from './McpServerSelect.svelte';
  import TlsCredentialFields from './TlsCredentialFields.svelte';
  import type { Resource } from '../../lib/api';
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

  let selectedFileName = '';
  let fileError = '';
  let lastMcpServerResourceId = '';
  let kubeconfigFileInput: HTMLInputElement;

  function openFilePicker(input: HTMLInputElement) {
    input?.click();
  }

  async function importKubeconfigFile(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    fileError = '';
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      if (!bytes.length) throw new Error('文件为空');
      const value = new TextDecoder('utf-8', { fatal: true })
        .decode(bytes)
        .trim();
      if (!value) throw new Error('文件为空');
      kubeconfig = value;
      selectedFileName = file.webkitRelativePath || file.name;
      onConfigurationChange();
    } catch (error) {
      fileError = error instanceof Error ? error.message : '文件读取失败';
      selectedFileName = '';
    } finally {
      input.value = '';
    }
  }

  $: if (mcpServerResourceId !== '__add_mcp_server__')
    lastMcpServerResourceId = mcpServerResourceId;

  function selectMCPServer(event: CustomEvent<string>) {
    const value = event.detail;
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
        <McpServerSelect
          bind:value={mcpServerResourceId}
          resources={mcpServers}
          allowAdd={allowAddMCPServer}
          invalid={configurationAttempted && !mcpServerResourceId}
          ariaLabel="关联 MCPServer"
          on:change={selectMCPServer}
        />
        <label class="docker-agent-override-toggle">
          <span>使用自定义 Kubernetes 连接</span>
          <Switch
            bind:checked={connectionOverride}
            ariaLabel="使用自定义 Kubernetes 连接"
            on:change={onConfigurationChange}
          />
        </label>
      </div>
      <p class="docker-field-help">
        资源子类型已经决定 Agent 接入方式；MCPServer 只提供工具传输路径。
      </p>
      {#if !mcpServers.length}
        <div class="docker-agent-empty" role="status">
          <strong>没有可用的 MCPServer</strong><span
            >请先创建并启用一个 MCPServer。</span
          >
        </div>
      {/if}
    </div>
  {/if}

  {#if !isAgent || connectionOverride}
    <div
      class="kubernetes-connection-mode"
      role="group"
      aria-label="API 连接方式"
    >
      <button
        class:active={connectionMode === 'kubeconfig'}
        type="button"
        aria-pressed={connectionMode === 'kubeconfig'}
        on:click={() => {
          connectionMode = 'kubeconfig';
          onConfigurationChange();
        }}
      >
        <strong>Kubeconfig</strong><small
          >使用 kubeconfig 文件连接集群，支持粘贴或导入。</small
        >
      </button>
      <button
        class:active={connectionMode === 'endpoint'}
        type="button"
        aria-pressed={connectionMode === 'endpoint'}
        on:click={() => {
          connectionMode = 'endpoint';
          onConfigurationChange();
        }}
      >
        <strong>Endpoint</strong><small
          >直接填写 API Server 与 Endpoint TLS 凭据。</small
        >
      </button>
    </div>

    {#if connectionMode === 'kubeconfig'}
      <fieldset class="docker-tls-fieldset">
        <legend>Kubeconfig 配置</legend>
        <div class="docker-form-grid docker-tls-grid">
          <div class="docker-tls-certificate-row kubernetes-kubeconfig-row">
            <label class:invalid={configurationAttempted && !kubeconfig.trim()}>
              <span class="docker-credential-label">
                <span
                  >Kubeconfig<i class="required-mark" aria-hidden="true">*</i
                  ></span
                >
                <span class="docker-file-picker">
                  {#if selectedFileName}<small>{selectedFileName}</small>{/if}
                  <input
                    class="docker-file-input"
                    bind:this={kubeconfigFileInput}
                    type="file"
                    accept=".yaml,.yml,.conf,text/plain,application/yaml"
                    aria-label="选择 kubeconfig 文件"
                    on:change={(event) => void importKubeconfigFile(event)}
                  />
                  <button
                    class="docker-file-import"
                    type="button"
                    aria-label="导入 kubeconfig 文件"
                    on:click|stopPropagation|preventDefault={() =>
                      openFilePicker(kubeconfigFileInput)}>导入</button
                  >
                </span>
              </span>
              <TextArea
                bind:value={kubeconfig}
                on:input={onConfigurationChange}
                rows={6}
                required
                placeholder="粘贴 kubeconfig 文本或 Base64 编码"
                spellcheck={false}
              />
              {#if fileError}<small class="field-error">{fileError}</small>{/if}
            </label>
          </div>
        </div>
      </fieldset>
    {:else}
      <div class="docker-form-grid kubernetes-endpoint-fields">
        <FormField
          label="API Server URL"
          required
          invalid={configurationAttempted && !server.trim()}
        >
          <TextInput
            bind:value={server}
            on:input={onConfigurationChange}
            required
            placeholder="https://kubernetes.example:6443"
          />
        </FormField>
        <FormField label="Token（可选）"
          ><PasswordInput
            bind:value={token}
            on:input={onConfigurationChange}
            ariaLabel="Token"
          /></FormField
        >
      </div>
      <TlsCredentialFields
        bind:ca={caBase64}
        bind:cert={certBase64}
        bind:key={keyBase64}
        bind:skipVerify={skipTLSVerify}
        {configurationAttempted}
        legend="Endpoint TLS 凭据（可选）"
        onChange={onConfigurationChange}
      />
    {/if}
  {/if}
  {#if credentialLoading}<small class="docker-field-help"
      >正在读取已有 Kubernetes 凭据，请稍候…</small
    >{/if}
</div>
