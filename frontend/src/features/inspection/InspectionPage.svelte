<script lang="ts">
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import {
    api,
    ApiError,
    type InspectionFinding,
    type InspectionPolicy,
    type InspectionRun,
    type PersonaCatalogItem,
    type Resource
  } from '../../lib/api';

  export let scopeId = '';
  export let executableTargets: Resource[] = [];
  export let personas: PersonaCatalogItem[] = [];
  export let scopeName: (id: string) => string;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;

  let policies: InspectionPolicy[] = [];
  let runs: InspectionRun[] = [];
  let findings: InspectionFinding[] = [];
  let policyName = '';
  let cron = '0 * * * *';
  let timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  let targetIds: string[] = [];
  let personaId = '';
  let targetLabels = '{}';
  let timeoutSeconds = 120;
  let retries = 1;
  let maxConcurrent = 2;
  let maxToolCalls = 12;
  let maxTokens = 20000;
  let busy = false;
  let loadedScopeId = '';
  $: personaOptions = personas.filter((item) => item.status === 'active').map((profile) => ({
    value: profile.id,
    label: profile.name,
    description: scopeName(profile.scope_id)
  }));

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
      const [nextPolicies, nextRuns, nextFindings] = await Promise.all([
        api.inspectionPolicies(targetScopeId),
        api.inspectionRuns(targetScopeId),
        api.inspectionFindings(targetScopeId)
      ]);
      if (targetScopeId !== scopeId) return;
      policies = nextPolicies;
      runs = nextRuns;
      findings = nextFindings;
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
        persona_id: personaId || undefined,
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
</script>

<section class="content-grid">
  <section class="panel wide-panel">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">NEW POLICY</p>
        <h2>创建巡检策略</h2>
      </div>
    </div>
    <form class="stack-form" on:submit|preventDefault={createPolicy}>
      <div class="form-grid">
        <FormField label="名称" required><TextInput bind:value={policyName} required maxlength={200} /></FormField>
        <FormField label="Cron" required><TextInput bind:value={cron} required /></FormField>
        <FormField label="时区" required><TextInput bind:value={timezone} required /></FormField>
        <FormField label="超时（秒）"><TextInput type="number" min="1" max="3600" bind:value={timeoutSeconds} /></FormField>
        <FormField label="重试次数"><TextInput type="number" min="0" max="10" bind:value={retries} /></FormField>
        <FormField label="目标并发"><TextInput type="number" min="1" max="64" bind:value={maxConcurrent} /></FormField>
        <FormField label="Tool 预算"><TextInput type="number" min="1" max="100" bind:value={maxToolCalls} /></FormField>
        <FormField label="Token 预算"><TextInput type="number" min="1" max="200000" bind:value={maxTokens} /></FormField>
      </div>
      <FormField label="标签选择器（JSON 对象）"><textarea rows="3" bind:value={targetLabels}></textarea></FormField>
      <fieldset>
        <legend>目标资源</legend>
        <div class="check-grid">
          {#each executableTargets as target}<label class="check-row"
              ><input
                type="checkbox"
                checked={targetIds.includes(target.id)}
                on:change={() =>
                  (targetIds = toggleSelection(targetIds, target.id))}
              />{target.name} · {target.kind}</label
            >{/each}
        </div>
      </fieldset>
      <FormField label="解释 Persona（可选）"><DropdownSelect bind:value={personaId} options={personaOptions} placeholder="使用内置巡检解释 Persona" ariaLabel="解释 Persona" /></FormField>
      <button
        class="primary"
        disabled={busy ||
          !policyName ||
          (targetIds.length === 0 && targetLabels.trim() === '{}')}
        >创建策略</button
      >
    </form>
  </section>
  <section class="panel">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">POLICIES</p>
        <h2>巡检策略</h2>
      </div>
      <span class="count">{policies.length}</span>
    </div>
    <div class="table-list">
      {#each policies as policy}<article class="list-row">
          <div>
            <strong>{policy.name}</strong>
            <p>
              {policy.cron} · {policy.timezone} · {policy.target_resource_ids
                .length} 个目标 · {policy.status}
            </p>
          </div>
          <div class="inline-actions">
            <button
              class="quiet-button"
              disabled={busy || policy.status !== 'active'}
              on:click={() => rerunInspection(policy.id)}>立即运行</button
            ><button
              class="quiet-button"
              disabled={busy}
              on:click={() =>
                setPolicyStatus(
                  policy.id,
                  policy.status === 'active' ? 'disabled' : 'active'
                )}>{policy.status === 'active' ? '停止' : '恢复'}</button
            >
          </div>
        </article>{:else}<p class="empty-state">
          当前作用域还没有巡检策略。
        </p>{/each}
    </div>
  </section>
  <section class="panel">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">HEALTH</p>
        <h2>最近运行</h2>
      </div>
      <span class="count">{runs.length}</span>
    </div>
    <div class="table-list">
      {#each runs as run}<article class="list-row">
          <div>
            <strong>{run.score ?? '—'} 分 · {run.status}</strong>
            <p>
              {new Date(run.window_start).toLocaleString()} · LLM {run.llm_status}
            </p>
          </div>
        </article>{:else}<p class="empty-state">尚无运行记录。</p>{/each}
    </div>
  </section>
  <section class="panel wide-panel">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">FINDINGS</p>
        <h2>异常与恢复</h2>
      </div>
      <span class="count">{findings.length}</span>
    </div>
    <div class="table-list">
      {#each findings as finding}<article class="list-row">
          <div>
            <strong>{finding.severity} · {finding.rule}</strong>
            <p>{finding.message || '无补充说明'} · {finding.status}</p>
          </div>
        </article>{:else}<p class="empty-state">没有已记录的异常。</p>{/each}
    </div>
  </section>
</section>
