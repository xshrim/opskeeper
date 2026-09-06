<script lang="ts">
  import { api, ApiError, type MCPSnapshot, type OperationRequest, type Resource } from '../../lib/api';

  export let resources: Resource[] = [];
  export let scopeId = '';
  export let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  export let resourceSchemaName: (kind: string) => string;
  export let formatDate: (value: string) => string;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;

  let operationTargetId = '';
  let operationName = 'kubernetes.restart_workload';
  let operationRisk: 'low' | 'medium' | 'high' = 'medium';
  let operationParameters = '{\n  "namespace": "default",\n  "workload": ""\n}';
  let operationImpact = '';
  let operationRollback = '';
  let operationRequests: OperationRequest[] = [];
  let busy = false;
  let loadedScopeId = '';

  $: if (scopeId && scopeId !== loadedScopeId) {
    loadedScopeId = scopeId;
    void loadOperations(scopeId);
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      return error.message || fallback;
    }
    if (error instanceof SyntaxError) return '配置必须是有效的 JSON 对象。';
    if (error instanceof Error) return error.message || fallback;
    return fallback;
  }

  async function loadOperations(targetScopeId = scopeId) {
    try {
      const nextRequests = await api.operationRequests(targetScopeId);
      const mcpServers = resources.filter(
        (item) => item.kind === 'MCPServer' && item.scope_id === targetScopeId
      );
      const nextSnapshots = Object.fromEntries(
        await Promise.all(
          mcpServers.map(async (server) => [server.id, await api.mcpSnapshots(server.id)])
        )
      );
      if (targetScopeId !== scopeId) return;
      operationRequests = nextRequests;
      operationSnapshots = { ...operationSnapshots, ...nextSnapshots };
    } catch (error) {
      if (targetScopeId === scopeId) {
        onError(describeError(error, '受控操作数据加载失败'));
      }
    }
  }

  async function createRequest() {
    busy = true;
    onError('');
    try {
      const created = await api.createOperationRequest({
        scope_id: scopeId,
        target_resource_id: operationTargetId,
        operation_name: operationName,
        risk_level: operationRisk,
        parameters: JSON.parse(operationParameters),
        impact_summary: operationImpact,
        rollback_summary: operationRollback,
        dry_run: { requested: true },
        idempotency_key: crypto.randomUUID()
      });
      operationRequests = [created, ...operationRequests];
      onNotice('已创建操作请求；中高风险操作等待另一位有权限的审批人处理。');
    } catch (error) {
      onError(describeError(error, '创建操作请求失败'));
    } finally {
      busy = false;
    }
  }

  async function discoverMCP(resourceId: string) {
    busy = true;
    onError('');
    try {
      const snapshot = await api.discoverMCP(resourceId);
      operationSnapshots = {
        ...operationSnapshots,
        [resourceId]: [snapshot, ...(operationSnapshots[resourceId] ?? [])]
      };
      onNotice(snapshot.status === 'succeeded' ? '已发现并保存 MCP 工具快照。' : 'MCP 健康检查失败，已保存受限错误信息。');
    } catch (error) {
      onError(describeError(error, 'MCP Server 发现失败'));
    } finally {
      busy = false;
    }
  }

  async function approve(item: OperationRequest, decision: 'approved' | 'rejected') {
    busy = true;
    onError('');
    try {
      const updated = await api.approveOperation(item.id, { decision, parameters_hash: item.parameters_hash });
      operationRequests = operationRequests.map((current) => current.id === updated.id ? updated : current);
      onNotice(decision === 'approved' ? '操作请求已批准。' : '操作请求已拒绝。');
    } catch (error) {
      onError(describeError(error, '审批操作请求失败'));
    } finally {
      busy = false;
    }
  }

  async function start(item: OperationRequest) {
    busy = true;
    onError('');
    try {
      await api.startOperation(item.id, crypto.randomUUID());
      operationRequests = await api.operationRequests(scopeId);
      onNotice('执行已排队；实际变更只能由受限 Kubernetes Job 完成。');
    } catch (error) {
      onError(describeError(error, '启动操作失败'));
    } finally {
      busy = false;
    }
  }
</script>

<section class="content-grid two-column">
  <section class="panel">
    <div class="panel-heading"><div><p class="eyebrow">APPROVAL WORKFLOW</p><h2>创建受控操作</h2></div><span class="scope-type">Medium+ 需人工审批</span></div>
    <form class="stack-form compact-form" on:submit|preventDefault={createRequest}>
      <label>目标资源<select bind:value={operationTargetId} required><option value="" disabled>选择可访问资源</option>{#each resources.filter((item) => item.scope_id === scopeId && item.status === 'active') as item}<option value={item.id}>{item.name} · {resourceSchemaName(item.kind)}</option>{/each}</select></label>
      <div class="form-row"><label>操作<select bind:value={operationName}><option value="kubernetes.restart_workload">重启 Kubernetes 工作负载</option><option value="kubernetes.scale_workload">扩缩容 Kubernetes 工作负载</option></select></label><label>风险<select bind:value={operationRisk}><option value="low">Low</option><option value="medium">Medium（默认审批）</option><option value="high">High（默认审批）</option></select></label></div>
      <label>精确参数 JSON<textarea bind:value={operationParameters} rows="5" spellcheck="false" required></textarea></label>
      <label>影响范围<input bind:value={operationImpact} placeholder="说明可能影响的应用、副本或访问窗口" /></label>
      <label>回滚建议<input bind:value={operationRollback} placeholder="例如恢复到原副本数" /></label>
      <p class="form-hint">提交会生成参数哈希；参数变更后原审批自动无效。删除和写 SQL 被系统永久拒绝。</p>
      <button class="primary" disabled={busy || !operationTargetId}>创建 dry-run 请求</button>
    </form>
  </section>
  <section class="panel">
    <div class="panel-heading"><div><p class="eyebrow">MCP SERVERS</p><h2>MCP 工具快照</h2></div><span class="scope-type">外部内容不可信</span></div>
    <div class="table-list">
      {#each resources.filter((item) => item.kind === 'MCPServer' && item.scope_id === scopeId) as server}
        <article class="list-row"><div><strong>{server.name}</strong><p>{(operationSnapshots[server.id] ?? [])[0]?.tools?.length ?? 0} 个已发现且允许的工具；描述和响应一律按不可信文本处理。</p>{#if (operationSnapshots[server.id] ?? [])[0]}<small>快照 {(operationSnapshots[server.id] ?? [])[0].content_hash.slice(0, 12)} · {(operationSnapshots[server.id] ?? [])[0].status}</small>{/if}</div><button class="quiet-button" disabled={busy} on:click={() => discoverMCP(server.id)}>发现 / 健康检查</button></article>
      {:else}<p class="empty-state">当前作用域没有 MCPServer 资源。先在资源目录以 HTTPS URL 和工具白名单登记。</p>{/each}
    </div>
  </section>
  <section class="panel wide-panel">
    <div class="panel-heading"><div><p class="eyebrow">AUDITABLE REQUESTS</p><h2>操作请求与审批</h2></div><span class="count">{operationRequests.length}</span></div>
    <div class="table-list">
      {#each operationRequests as item}
        <article class="list-row"><div><strong>{item.operation_name} · {item.risk_level} · {item.status}</strong><p>目标 {resources.find((resource) => resource.id === item.target_resource_id)?.name ?? item.target_resource_id.slice(0, 8)} · 参数哈希 {item.parameters_hash.slice(0, 12)} · {item.expires_at ? `有效至 ${formatDate(item.expires_at)}` : '无需人工审批'}</p><small>影响：{item.impact_summary || '未填写'}；回滚：{item.rollback_summary || '未填写'}</small><pre class="config-preview">{JSON.stringify(item.parameters, null, 2)}</pre></div><div class="inline-actions">{#if item.status === 'pending'}<button class="quiet-button" disabled={busy} on:click={() => approve(item, 'approved')}>批准</button><button class="quiet-button" disabled={busy} on:click={() => approve(item, 'rejected')}>拒绝</button>{:else if item.status === 'approved'}<button class="primary" disabled={busy} on:click={() => start(item)}>开始执行</button>{/if}</div></article>
      {:else}<p class="empty-state">尚无操作请求。所有请求、审批和执行结果都会进入审计记录。</p>{/each}
    </div>
  </section>
</section>
