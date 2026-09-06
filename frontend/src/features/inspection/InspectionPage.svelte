<script lang="ts">
  import {
    api,
    ApiError,
    type InspectionFinding,
    type InspectionPolicy,
    type InspectionRun,
    type NotificationChannel,
    type Resource
  } from '../../lib/api';

  export let scopeId = '';
  export let executableTargets: Resource[] = [];
  export let agentProfiles: Resource[] = [];
  export let scopeName: (id: string) => string;
  export let resourceInActiveWorkspace: (resource: Resource) => boolean;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;

  let policies: InspectionPolicy[] = [];
  let runs: InspectionRun[] = [];
  let findings: InspectionFinding[] = [];
  let channels: NotificationChannel[] = [];
  let policyName = '';
  let cron = '0 * * * *';
  let timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  let targetIds: string[] = [];
  let agentProfileId = '';
  let targetLabels = '{}';
  let timeoutSeconds = 120;
  let retries = 1;
  let maxConcurrent = 2;
  let maxToolCalls = 12;
  let maxTokens = 20000;
  let channelName = '';
  let channelWebhookURL = '';
  let channelRateLimit = 30;
  let busy = false;
  let loadedScopeId = '';

  $: if (scopeId && scopeId !== loadedScopeId) {
    loadedScopeId = scopeId;
    void loadInspection(scopeId);
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

  async function loadInspection(targetScopeId = scopeId) {
    try {
      const [nextPolicies, nextRuns, nextFindings, nextChannels] =
        await Promise.all([
          api.inspectionPolicies(targetScopeId),
          api.inspectionRuns(targetScopeId),
          api.inspectionFindings(targetScopeId),
          api.notificationChannels(targetScopeId)
        ]);
      if (targetScopeId !== scopeId) return;
      policies = nextPolicies;
      runs = nextRuns;
      findings = nextFindings;
      channels = nextChannels;
    } catch (error) {
      if (targetScopeId === scopeId) {
        onError(describeError(error, '巡检数据加载失败'));
      }
    }
  }

  function toggleSelection(list: string[], id: string) {
    return list.includes(id)
      ? list.filter((item) => item !== id)
      : [...list, id];
  }

  async function rerunInspection(policyID: string) {
    busy = true;
    onError('');
    try {
      await api.startInspectionRun(policyID, scopeId);
      onNotice('已创建手动巡检任务。');
      await loadInspection();
    } catch (error) {
      onError(describeError(error, '创建巡检任务失败'));
    } finally {
      busy = false;
    }
  }

  async function setPolicyStatus(policyID: string, status: string) {
    busy = true;
    onError('');
    try {
      await api.setInspectionPolicyStatus(policyID, scopeId, status);
      onNotice(status === 'disabled' ? '已停止周期巡检。' : '已恢复周期巡检。');
      await loadInspection();
    } catch (error) {
      onError(describeError(error, '更新巡检策略失败'));
    } finally {
      busy = false;
    }
  }

  async function createPolicy() {
    busy = true;
    onError('');
    try {
      const created = await api.createInspectionPolicy({
        scope_id: scopeId,
        name: policyName,
        cron,
        timezone,
        target_resource_ids: targetIds,
        target_labels: JSON.parse(targetLabels),
        agent_profile_resource_id: agentProfileId || undefined,
        timeout_seconds: timeoutSeconds,
        retries,
        max_concurrent: maxConcurrent,
        max_tool_calls: maxToolCalls,
        max_tokens: maxTokens,
        maintenance: []
      });
      policies = [created, ...policies];
      policyName = '';
      onNotice('巡检策略已创建。');
    } catch (error) {
      onError(describeError(error, '创建巡检策略失败'));
    } finally {
      busy = false;
    }
  }

  async function createChannel() {
    busy = true;
    onError('');
    try {
      const created = await api.createNotificationChannel({
        scope_id: scopeId,
        name: channelName,
        webhook_url: channelWebhookURL,
        status: 'active',
        rate_limit_per_minute: channelRateLimit
      });
      channels = [created, ...channels];
      channelName = '';
      channelWebhookURL = '';
      onNotice('Webhook 通知渠道已创建。');
    } catch (error) {
      onError(describeError(error, '创建通知渠道失败'));
    } finally {
      busy = false;
    }
  }
</script>

<section class="content-grid">
  <section class="panel wide-panel">
    <div class="panel-heading"><div><p class="eyebrow">NEW POLICY</p><h2>创建巡检策略</h2></div></div>
    <form class="stack-form" on:submit|preventDefault={createPolicy}>
      <div class="form-grid">
        <label>名称<input bind:value={policyName} required maxlength="200" /></label>
        <label>Cron<input bind:value={cron} required /></label>
        <label>时区<input bind:value={timezone} required /></label>
        <label>超时（秒）<input type="number" min="1" max="3600" bind:value={timeoutSeconds} /></label>
        <label>重试次数<input type="number" min="0" max="10" bind:value={retries} /></label>
        <label>目标并发<input type="number" min="1" max="64" bind:value={maxConcurrent} /></label>
        <label>Tool 预算<input type="number" min="1" max="100" bind:value={maxToolCalls} /></label>
        <label>Token 预算<input type="number" min="1" max="200000" bind:value={maxTokens} /></label>
      </div>
      <label>标签选择器（JSON 对象）<textarea rows="3" bind:value={targetLabels}></textarea></label>
      <fieldset><legend>目标资源</legend><div class="check-grid">{#each executableTargets as target}<label class="check-row"><input type="checkbox" checked={targetIds.includes(target.id)} on:change={() => (targetIds = toggleSelection(targetIds, target.id))} />{target.name} · {target.kind}</label>{/each}</div></fieldset>
      <label>解释 AgentProfile（可选）<select bind:value={agentProfileId}><option value="">使用内置巡检解释 Agent</option>{#each agentProfiles.filter((item) => resourceInActiveWorkspace(item) && item.status === 'active') as profile}<option value={profile.id}>{profile.name} · {scopeName(profile.scope_id)}</option>{/each}</select></label>
      <button class="primary" disabled={busy || !policyName || (targetIds.length === 0 && targetLabels.trim() === '{}')}>创建策略</button>
    </form>
  </section>
  <section class="panel"><div class="panel-heading"><div><p class="eyebrow">POLICIES</p><h2>巡检策略</h2></div><span class="count">{policies.length}</span></div><div class="table-list">{#each policies as policy}<article class="list-row"><div><strong>{policy.name}</strong><p>{policy.cron} · {policy.timezone} · {policy.target_resource_ids.length} 个目标 · {policy.status}</p></div><div class="inline-actions"><button class="quiet-button" disabled={busy || policy.status !== 'active'} on:click={() => rerunInspection(policy.id)}>立即运行</button><button class="quiet-button" disabled={busy} on:click={() => setPolicyStatus(policy.id, policy.status === 'active' ? 'disabled' : 'active')}>{policy.status === 'active' ? '停止' : '恢复'}</button></div></article>{:else}<p class="empty-state">当前作用域还没有巡检策略。</p>{/each}</div></section>
  <section class="panel"><div class="panel-heading"><div><p class="eyebrow">HEALTH</p><h2>最近运行</h2></div><span class="count">{runs.length}</span></div><div class="table-list">{#each runs as run}<article class="list-row"><div><strong>{run.score ?? '—'} 分 · {run.status}</strong><p>{new Date(run.window_start).toLocaleString()} · LLM {run.llm_status}</p></div></article>{:else}<p class="empty-state">尚无运行记录。</p>{/each}</div></section>
  <section class="panel wide-panel"><div class="panel-heading"><div><p class="eyebrow">FINDINGS</p><h2>异常与恢复</h2></div><span class="count">{findings.length}</span></div><div class="table-list">{#each findings as finding}<article class="list-row"><div><strong>{finding.severity} · {finding.rule}</strong><p>{finding.message || '无补充说明'} · {finding.status}</p></div></article>{:else}<p class="empty-state">没有已记录的异常。</p>{/each}</div></section>
  <section class="panel"><div class="panel-heading"><div><p class="eyebrow">WEBHOOKS</p><h2>通知渠道</h2></div><span class="count">{channels.length}</span></div><div class="table-list">{#each channels as channel}<article class="list-row"><div><strong>{channel.name}</strong><p>{channel.kind} · {channel.status} · 每分钟 {channel.rate_limit_per_minute} 次</p></div></article>{:else}<p class="empty-state">当前作用域没有启用的通知渠道。</p>{/each}</div><form class="stack-form compact-form" on:submit|preventDefault={createChannel}><label>名称<input bind:value={channelName} required maxlength="120" /></label><label>HTTPS Webhook<input type="url" pattern="https://.*" bind:value={channelWebhookURL} required /></label><label>每分钟上限<input type="number" min="1" max="10000" bind:value={channelRateLimit} /></label><button class="primary" disabled={busy}>添加渠道</button></form></section>
</section>
