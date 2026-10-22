<script lang="ts">
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import ModalDialog from '../../components/ModalDialog.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import type { Resource } from '../../lib/api';
  import { api } from '../../lib/api';
  import {
    hostSecret,
    mcpSecret
  } from './resourceActions';
  import HostConnectionStep from './HostConnectionStep.svelte';
  import McpConnectionFields from './McpConnectionFields.svelte';
  import {
    hostConfigForSave,
    hostConnectionConfigurationValid,
    hostCredentialForSave,
    dockerTLSValueForSave,
    mcpConfigForSave,
    mcpConfigurationValid,
    parseMCPHeaders,
    type HostAccessMode
  } from './resourceWorkflow';
  import { parseLabels } from '../../lib/labels';

  export let scopeId = '';
  export let resources: Resource[] = [];
  export let onCreated: (resource: Resource) => void = () => {};
  export let onCancel: () => void = () => {};

  let name = '';
  let labels = '';
  let accessMode: HostAccessMode = 'direct';
  let host = '';
  let port = 22;
  let username = '';
  let authMethod: 'password' | 'key' = 'password';
  let password = '';
  let privateKey = '';
  let passphrase = '';
  let knownHosts = '';
  let timeoutSeconds = 30;
  let mcpServerResourceId = '';
  let connectionOverride = false;
  let attempted = false;
  let busy = false;
  let message = '';

  let mcpDialogOpen = false;
  let mcpName = '';
  let mcpLabels = '';
  let mcpTransport = 'streamable_http';
  let mcpURL = '';
  let mcpToken = '';
  let mcpRequestHeaders = '';
  let mcpToolAllowlist = '';
  let mcpTimeoutSeconds = 120;
  let mcpMaxResponseBytes = 4 * 1024 * 1024;
  let mcpTLSCA = '';
  let mcpTLSCert = '';
  let mcpTLSKey = '';
  let mcpTLSSkipVerify = false;
  let mcpAttempted = false;
  let mcpBusy = false;
  let mcpMessage = '';
  let createdMCPServers: Resource[] = [];

  $: mcpServers = [...resources, ...createdMCPServers]
    .filter((resource, index, all) =>
      resource.kind === 'MCPServer' &&
      all.findIndex((item) => item.id === resource.id) === index
    )
    .sort((left, right) => left.name.localeCompare(right.name));

  function hostDraft() {
    return {
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
    };
  }

  function selectAccessMode(value: string) {
    accessMode = value === 'agent' ? 'agent' : 'direct';
    if (accessMode === 'direct') {
      mcpServerResourceId = '';
      connectionOverride = false;
    }
    message = '';
  }

  function hostValid() {
    return Boolean(name.trim() && hostConnectionConfigurationValid(hostDraft()));
  }

  async function createHost() {
    attempted = true;
    message = '';
    if (!hostValid()) {
      message = '请填写 Host 名称并完成连接配置。';
      return;
    }
    busy = true;
    try {
      const draft = hostDraft();
      const credentialValues = hostCredentialForSave(draft);
      const credential = Object.keys(credentialValues).length
        ? hostSecret(credentialValues)
        : null;
      const created = await api.createResource({
        scope_id: scopeId,
        kind: 'Host',
        subtype: draft.accessMode === 'agent' ? 'Agent' : 'Direct',
        agent_ref:
          draft.accessMode === 'agent' ? draft.mcpServerResourceId : undefined,
        name: name.trim(),
        labels: parseLabels(labels),
        config: hostConfigForSave(draft),
        ...(credential ? { credential } : {})
      });
      onCreated(created);
    } catch (error) {
      message = error instanceof Error ? error.message : '创建 Host 资源失败。';
    } finally {
      busy = false;
    }
  }

  function openMCPDialog() {
    mcpName = '';
    mcpLabels = '';
    mcpTransport = 'streamable_http';
    mcpURL = '';
    mcpToken = '';
    mcpRequestHeaders = '';
    mcpToolAllowlist = '';
    mcpTimeoutSeconds = 120;
    mcpMaxResponseBytes = 4 * 1024 * 1024;
    mcpTLSCA = '';
    mcpTLSCert = '';
    mcpTLSKey = '';
    mcpTLSSkipVerify = false;
    mcpAttempted = false;
    mcpMessage = '';
    mcpDialogOpen = true;
  }

  async function createMCPServer() {
    mcpAttempted = true;
    mcpMessage = '';
    if (!mcpName.trim() || !mcpConfigurationValid(mcpTransport, mcpURL, mcpRequestHeaders)) {
      mcpMessage = '请填写 MCPServer 名称、有效的 Server 地址和请求 Header。';
      return;
    }
    mcpBusy = true;
    try {
      const headers = parseMCPHeaders(mcpRequestHeaders);
      const credential = mcpSecret(mcpToken, headers, {
        tls_ca: mcpTLSCA.trim() ? dockerTLSValueForSave(mcpTLSCA) : '',
        tls_cert: mcpTLSCert.trim() ? dockerTLSValueForSave(mcpTLSCert) : '',
        tls_key: mcpTLSKey.trim() ? dockerTLSValueForSave(mcpTLSKey) : '',
        tls_skip_verify: mcpTLSSkipVerify
      });
      const created = await api.createResource({
        scope_id: scopeId,
        kind: 'MCPServer',
        subtype: mcpTransport === 'sse' ? 'SSE' : 'StreamHTTP',
        name: mcpName.trim(),
        labels: parseLabels(mcpLabels),
        config: mcpConfigForSave({
          transport: mcpTransport,
          url: mcpURL,
          toolAllowlist: mcpToolAllowlist,
          timeoutSeconds: Number(mcpTimeoutSeconds),
          maxResponseBytes: Number(mcpMaxResponseBytes)
        }),
        ...(credential ? { credential } : {})
      });
      createdMCPServers = [created, ...createdMCPServers];
      mcpServerResourceId = created.id;
      onCreated(created);
      mcpDialogOpen = false;
    } catch (error) {
      mcpMessage = error instanceof Error ? error.message : '创建 MCPServer 失败。';
    } finally {
      mcpBusy = false;
    }
  }
</script>

<ModalDialog
  title="新增 Host 资源"
  description="创建后将自动回填到当前应用实例。"
  confirmLabel="创建"
  busyLabel="创建中…"
  {busy}
  message={message}
  size="wide"
  dialogClass="application-resource-dialog"
  onCancel={onCancel}
  onConfirm={createHost}
>
  <div class="host-resource-form">
    <div class="host-resource-identity">
      <FormField label="资源名称" required invalid={attempted && !name.trim()}>
        <TextInput bind:value={name} required placeholder="payments-host-01" ariaLabel="Host 资源名称" />
      </FormField>
      <FormField label="接入方式" required>
        <DropdownSelect
          value={accessMode}
          options={[
            { value: 'direct', label: 'Direct', description: '本机或 SSH 直接连接' },
            { value: 'agent', label: 'Agent', description: '通过 MCPServer 访问' }
          ]}
          ariaLabel="Host 接入方式"
          on:change={(event) => selectAccessMode(String(event.detail))}
        />
      </FormField>
    </div>
    <FormField label="标签" asLabel={false}>
      <TextInput bind:value={labels} tagged placeholder="填写 key=value、key:value 或 key，按回车确认" ariaLabel="Host 标签" />
    </FormField>
    <HostConnectionStep
      {accessMode}
      bind:host
      bind:port
      bind:username
      bind:authMethod
      bind:password
      bind:privateKey
      bind:passphrase
      bind:knownHosts
      bind:timeoutSeconds
      bind:mcpServerResourceId
      bind:connectionOverride
      {mcpServers}
      configurationAttempted={attempted}
      onConfigurationChange={() => (message = '')}
      onAddMCPServer={openMCPDialog}
    />
  </div>
</ModalDialog>

{#if mcpDialogOpen}
  <ModalDialog
    title="新增 MCPServer"
    description="创建后将自动关联到正在新增的 Host 资源。"
    confirmLabel="创建"
    busyLabel="创建中…"
    busy={mcpBusy}
    message={mcpMessage}
    size="wide"
    dialogClass="application-resource-dialog"
    onCancel={() => (mcpDialogOpen = false)}
    onConfirm={createMCPServer}
  >
    <div class="host-resource-form">
      <div class="host-resource-identity">
        <FormField label="资源名称" required invalid={mcpAttempted && !mcpName.trim()}>
          <TextInput bind:value={mcpName} required placeholder="host-agent-mcp" ariaLabel="MCPServer 资源名称" />
        </FormField>
        <FormField label="传输方式" required>
          <DropdownSelect
            bind:value={mcpTransport}
            options={[
              { value: 'streamable_http', label: 'Streamable HTTP' },
              { value: 'sse', label: 'SSE' }
            ]}
            ariaLabel="MCPServer 传输方式"
          />
        </FormField>
      </div>
      <FormField label="标签" asLabel={false}>
        <TextInput bind:value={mcpLabels} tagged placeholder="填写 key=value、key:value 或 key，按回车确认" ariaLabel="MCPServer 标签" />
      </FormField>
      <McpConnectionFields
        bind:url={mcpURL}
        bind:token={mcpToken}
        bind:requestHeaders={mcpRequestHeaders}
        bind:toolAllowlist={mcpToolAllowlist}
        bind:timeoutSeconds={mcpTimeoutSeconds}
        bind:maxResponseBytes={mcpMaxResponseBytes}
        bind:tlsCA={mcpTLSCA}
        bind:tlsCert={mcpTLSCert}
        bind:tlsKey={mcpTLSKey}
        bind:skipTLSVerify={mcpTLSSkipVerify}
        configurationAttempted={mcpAttempted}
      />
    </div>
  </ModalDialog>
{/if}

<style>
  .host-resource-form { display: grid; gap: 12px; }
  .host-resource-identity { display: grid; grid-template-columns: minmax(0, 2fr) minmax(180px, 1fr); gap: 12px; }
  @media (max-width: 720px) { .host-resource-identity { grid-template-columns: 1fr; } }
</style>
