<script lang="ts">
  import type { KubernetesConnectionMode } from './resourceWorkflow';

  export let resourceName = '';
  export let resourceStatus = 'active';
  export let isAgent = false;
  export let connectionOverride = false;
  export let connectionMode: KubernetesConnectionMode = 'kubeconfig';
  export let server = '';
  export let mcpServerName = '';
  export let credentialConfigured = false;
  export let scopeSummary = '';
  export let labelsConfigured = false;
  export let testBusy = false;
  export let testStatus = '';
  export let testMessage = '';
  export let testError = '';
  export let testLatency: number | undefined;
  export let onSubmit = () => {};
  const tools = ['kubernetes_cluster_info', 'kubernetes_api_resources', 'kubernetes_namespaces', 'kubernetes_nodes', 'kubernetes_pods', 'kubernetes_workloads', 'kubernetes_pod_stat', 'kubernetes_node_stat', 'kubernetes_services', 'kubernetes_configmaps', 'kubernetes_ingresses', 'kubernetes_endpoint_slices', 'kubernetes_events', 'kubernetes_pod_logs', 'kubernetes_resource_get', 'kubernetes_health'];
</script>

<form id="kubernetes-review-form" class="provider-summary docker-summary" on:submit|preventDefault={onSubmit}>
  <div><span>Kubernetes 资源</span><strong>{resourceName}</strong><small>{resourceStatus === 'active' ? '已启用' : '已停用'} · {scopeSummary}</small></div>
  <div><span>资源接入</span><strong>{isAgent ? 'Agent · MCP 代理' : 'Direct · 直接连接'}</strong><small>{isAgent ? `${mcpServerName || '未选择 MCPServer'}${connectionOverride ? ' · 自定义 Kubernetes 连接' : ''}` : '后端直接连接 Kubernetes API'}</small></div>
  <div><span>API 连接方式</span><strong>{!isAgent || connectionOverride ? connectionMode === 'kubeconfig' ? 'Kubeconfig' : 'Endpoint' : '由 MCPServer 管理'}</strong><small>{!isAgent || connectionOverride ? connectionMode === 'endpoint' ? server || '未设置 API Server' : '凭据以 Base64 加密保存' : '使用关联 MCPServer 的 Kubernetes 工具'}</small></div>
  <div><span>连接凭据</span><strong>{isAgent && !connectionOverride ? '由 MCPServer 管理' : credentialConfigured ? '已配置' : '使用环境或默认配置'}</strong></div>
  <div><span>资源属性</span><strong>{labelsConfigured ? '已配置标签' : '未配置标签'}</strong></div>
  <div class="docker-tool-summary"><span>可用只读工具（{tools.length}）</span><div>{#each tools as tool}<code>{tool}</code>{/each}</div></div>
  <div class="provider-test-summary"><span>连接核验</span>{#if testBusy}<strong>正在测试连接...</strong>{:else if testStatus === 'succeeded'}<strong class="success">连接正常 · {testLatency ?? 0} ms</strong><small>{testMessage}</small>{:else if testError}<strong class="failed">连接失败</strong><small>{testError}</small>{:else}<strong>尚未核验</strong><small>进入此步骤后自动执行连接测试。</small>{/if}</div>
</form>
