<script lang="ts">
  import type { MCPSnapshot, OperationRequest, Resource } from '../../lib/api';

  export let resources: Resource[] = [];
  export let selectedScopeId = '';
  export let operationTargetId = '';
  export let operationName = 'kubernetes.restart_workload';
  export let operationRisk: 'low' | 'medium' | 'high' = 'medium';
  export let operationParameters = '{\n  "namespace": "default",\n  "workload": ""\n}';
  export let operationImpact = '';
  export let operationRollback = '';
  export let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  export let operationRequests: OperationRequest[] = [];
  export let busy = false;
  export let resourceSchemaName: (kind: string) => string;
  export let formatDate: (value: string) => string;
  export let onCreateRequest: () => void | Promise<void>;
  export let onDiscoverMCP: (resourceID: string) => void | Promise<void>;
  export let onApprove: (item: OperationRequest, decision: 'approved' | 'rejected') => void | Promise<void>;
  export let onStart: (item: OperationRequest) => void | Promise<void>;
</script>

<section class="content-grid two-column">
  <section class="panel">
    <div class="panel-heading"><div><p class="eyebrow">APPROVAL WORKFLOW</p><h2>创建受控操作</h2></div><span class="scope-type">Medium+ 需人工审批</span></div>
    <form class="stack-form compact-form" on:submit|preventDefault={onCreateRequest}>
      <label>目标资源<select bind:value={operationTargetId} required><option value="" disabled>选择可访问资源</option>{#each resources.filter((item) => item.scope_id === selectedScopeId && item.status === 'active') as item}<option value={item.id}>{item.name} · {resourceSchemaName(item.kind)}</option>{/each}</select></label>
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
      {#each resources.filter((item) => item.kind === 'MCPServer' && item.scope_id === selectedScopeId) as server}
        <article class="list-row"><div><strong>{server.name}</strong><p>{(operationSnapshots[server.id] ?? [])[0]?.tools?.length ?? 0} 个已发现且允许的工具；描述和响应一律按不可信文本处理。</p>{#if (operationSnapshots[server.id] ?? [])[0]}<small>快照 {(operationSnapshots[server.id] ?? [])[0].content_hash.slice(0, 12)} · {(operationSnapshots[server.id] ?? [])[0].status}</small>{/if}</div><button class="quiet-button" disabled={busy} on:click={() => onDiscoverMCP(server.id)}>发现 / 健康检查</button></article>
      {:else}<p class="empty-state">当前作用域没有 MCPServer 资源。先在资源目录以 HTTPS URL 和工具白名单登记。</p>{/each}
    </div>
  </section>
  <section class="panel wide-panel">
    <div class="panel-heading"><div><p class="eyebrow">AUDITABLE REQUESTS</p><h2>操作请求与审批</h2></div><span class="count">{operationRequests.length}</span></div>
    <div class="table-list">
      {#each operationRequests as item}
        <article class="list-row"><div><strong>{item.operation_name} · {item.risk_level} · {item.status}</strong><p>目标 {resources.find((resource) => resource.id === item.target_resource_id)?.name ?? item.target_resource_id.slice(0, 8)} · 参数哈希 {item.parameters_hash.slice(0, 12)} · {item.expires_at ? `有效至 ${formatDate(item.expires_at)}` : '无需人工审批'}</p><small>影响：{item.impact_summary || '未填写'}；回滚：{item.rollback_summary || '未填写'}</small><pre class="config-preview">{JSON.stringify(item.parameters, null, 2)}</pre></div><div class="inline-actions">{#if item.status === 'pending'}<button class="quiet-button" disabled={busy} on:click={() => onApprove(item, 'approved')}>批准</button><button class="quiet-button" disabled={busy} on:click={() => onApprove(item, 'rejected')}>拒绝</button>{:else if item.status === 'approved'}<button class="primary" disabled={busy} on:click={() => onStart(item)}>开始执行</button>{/if}</div></article>
      {:else}<p class="empty-state">尚无操作请求。所有请求、审批和执行结果都会进入审计记录。</p>{/each}
    </div>
  </section>
</section>
