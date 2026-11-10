<script lang="ts">
  import { onDestroy } from 'svelte';
  import { AlertTriangle, ArrowUpRight, CircleCheck } from 'lucide-svelte';
  import EmptyState from '../../components/EmptyState.svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import { api, type ConnectionCheck, type DiagnosisSession, type InspectionRun, type Project, type Provider, type Resource, type SkillCatalogItem, type PersonaCatalogItem, type Team } from '../../lib/api';
  import { resourceHasConnector } from '../../lib/resources';

  export let teams: Team[] = [];
  export let projects: Project[] = [];
  export let visibleResources: Resource[] = [];
  export let resourceConnectionChecks: Record<string, ConnectionCheck | null> = {};
  export let scopeId = '';
  export let providers: Provider[] = [];
  export let skills: SkillCatalogItem[] = [];
  export let personas: PersonaCatalogItem[] = [];
  export let organizationTeamCount = 0;
  export let organizationMemberCount: number | null = null;
  export let resourceSchemaName: (kind: string) => string;
  export let resourceIcon: (kind: string) => string;
  export let onOpenProject: (project: Project) => void;
  export let onOpenDiagnosis: () => void;
  export let onOpenInspection: () => void;
  export let onOpenNotifications: () => void;
  export let onOpenModels: () => void;
  export let onOpenAccess: () => void;
  export let onOpenResources: () => void;
  export let onOpenResource: (resource: Resource) => void;

  let diagnosisSessions: DiagnosisSession[] = [];
  let inspectionRuns: InspectionRun[] = [];
  let activityLoading = false;
  let diagnosisReadFailed = false;
  let inspectionReadFailed = false;
  let requestVersion = 0;

  $: applicationCount = projects.reduce((sum, project) => sum + project.summary.applications, 0);
  $: healthyApplicationCount = projects.reduce((sum, project) => sum + project.summary.applications_healthy, 0);
  $: unhealthyApplicationCount = Math.max(0, applicationCount - healthyApplicationCount);
  $: instanceCount = projects.reduce((sum, project) => sum + project.summary.instances, 0);
  $: healthyInstanceCount = projects.reduce((sum, project) => sum + project.summary.instances_healthy, 0);
  $: instancesNeedingAttention = Math.max(0, instanceCount - healthyInstanceCount);
  $: alertCount = projects.reduce((sum, project) => sum + project.summary.alerts, 0);
  $: projectsWithAlerts = projects.filter((project) => project.summary.alerts > 0).length;
  $: connectorResources = visibleResources.filter(resourceHasConnector);
  $: testedResources = connectorResources.filter((resource) => resourceConnectionChecks[resource.id]);
  $: successfulConnections = testedResources.filter((resource) => resourceConnectionChecks[resource.id]?.status === 'succeeded').length;
  $: failedConnections = testedResources.length - successfulConnections;
  $: unhealthyObjectCount = unhealthyApplicationCount + failedConnections;
  $: untestedConnections = Math.max(0, connectorResources.length - testedResources.length);
  $: resourceKinds = Object.entries(visibleResources.reduce<Record<string, number>>((counts, resource) => {
    counts[resource.kind] = (counts[resource.kind] ?? 0) + 1;
    return counts;
  }, {})).sort((left, right) => right[1] - left[1]);
  $: exceptions = projects.filter((project) => project.summary.alerts > 0).sort((left, right) => right.summary.alerts - left.summary.alerts);
  $: activeDiagnosisCount = diagnosisSessions.filter((session) => ['queued', 'planning', 'collecting', 'analyzing'].includes(session.status)).length;
  $: successfulInspectionRuns = inspectionRuns.filter((run) => run.status === 'succeeded').length;
  $: pendingInspectionRuns = inspectionRuns.filter((run) => run.status === 'queued').length;
  $: inspectionSuccessRate = percentage(successfulInspectionRuns, inspectionRuns.length);
  $: applicationHealthRate = percentage(healthyApplicationCount, applicationCount);
  $: resourceHealthRate = percentage(successfulConnections, testedResources.length);
  $: applicationHealthStatus = applicationCount === 0 ? '暂无数据' : healthyApplicationCount === applicationCount ? '稳定' : '关注';
  $: instanceHealthStatus = instanceCount === 0 ? '暂无数据' : healthyInstanceCount === instanceCount ? '达标' : '关注';
  $: inspectionStatus = inspectionRuns.length === 0 ? '暂无数据' : inspectionSuccessRate >= 90 ? '稳定' : '关注';
  $: alertStatus = alertCount > 0 ? '需处理' : '正常';

  $: if (scopeId) void loadScopeActivity(scopeId);

  async function loadScopeActivity(currentScopeId: string) {
    const version = ++requestVersion;
    activityLoading = true;
    const [sessions, runs] = await Promise.allSettled([
      api.diagnosisSessions(currentScopeId),
      api.inspectionRuns(currentScopeId)
    ]);
    if (version !== requestVersion) return;
    diagnosisSessions = sessions.status === 'fulfilled' ? sessions.value : [];
    inspectionRuns = runs.status === 'fulfilled' ? runs.value : [];
    diagnosisReadFailed = sessions.status === 'rejected';
    inspectionReadFailed = runs.status === 'rejected';
    activityLoading = false;
  }

  onDestroy(() => { requestVersion += 1; });

  function percentage(value: number, total: number) {
    return total ? Math.round((value / total) * 100) : 0;
  }

  function percentageLabel(value: number, total: number) {
    return total ? `${percentage(value, total)}%` : '—';
  }

  function projectTeamName(project: Project) {
    return teams.find((team) => team.id === project.team_id)?.name ?? '团队';
  }

</script>

<section class="overview-dashboard" aria-label="运行驾驶舱">
  <svg class="overview-sparkline-symbols" aria-hidden="true">
    <symbol id="overview-metric-sparkline" viewBox="0 0 70 23">
      <path d="M1 18 L10 14 L18 16 L27 8 L36 12 L45 5 L53 10 L62 4 L69 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
      <path d="M1 22H69" fill="none" stroke="var(--theme-divider)" stroke-width="1" />
    </symbol>
  </svg>
  <div class="overview-metrics">
    <article class="overview-metric"><div class="overview-metric-label"><span>项目</span><svg class="overview-sparkline" viewBox="0 0 70 23" aria-hidden="true"><use href="#overview-metric-sparkline" /></svg></div><strong>{projects.length}</strong><small class="overview-note" class:danger={projectsWithAlerts > 0} class:good={projectsWithAlerts === 0}>{projectsWithAlerts} 个异常</small></article>
    <article class="overview-metric"><div class="overview-metric-label"><span>应用</span><svg class="overview-sparkline" viewBox="0 0 70 23" aria-hidden="true"><use href="#overview-metric-sparkline" /></svg></div><strong>{applicationCount}</strong><small class="overview-note" class:danger={unhealthyApplicationCount > 0} class:good={unhealthyApplicationCount === 0}>{unhealthyApplicationCount} 个异常</small></article>
    <article class="overview-metric"><div class="overview-metric-label"><span>实例</span><svg class="overview-sparkline" viewBox="0 0 70 23" aria-hidden="true"><use href="#overview-metric-sparkline" /></svg></div><strong>{instanceCount}</strong><small class="overview-note" class:attention={instancesNeedingAttention > 0} class:good={instancesNeedingAttention === 0}>{instancesNeedingAttention} 个需关注</small></article>
    <article class="overview-metric"><div class="overview-metric-label"><span>资源</span><svg class="overview-sparkline" viewBox="0 0 70 23" aria-hidden="true"><use href="#overview-metric-sparkline" /></svg></div><strong>{visibleResources.length}</strong><small class="overview-note" class:danger={failedConnections > 0} class:attention={failedConnections === 0 && untestedConnections > 0} class:good={connectorResources.length > 0 && failedConnections === 0 && untestedConnections === 0} class:neutral={connectorResources.length === 0}>{failedConnections > 0 ? `${failedConnections} 个连接异常` : connectorResources.length === 0 ? '无可测试连接' : untestedConnections > 0 ? `${untestedConnections} 个连接待测试` : '连接均正常'}</small></article>
    <article class="overview-metric"><div class="overview-metric-label"><span>告警</span><svg class="overview-sparkline" viewBox="0 0 70 23" aria-hidden="true"><use href="#overview-metric-sparkline" /></svg></div><strong class:warning={alertCount > 0}>{alertCount}</strong><small class="overview-note" class:danger={alertCount > 0} class:good={alertCount === 0}>{alertCount} 个待处理</small></article>
  </div>

  <div class="overview-middle">
    <section class="overview-card resource-star-card" aria-labelledby="resource-star-heading">
      <header class="overview-card-heading">
        <h2 id="resource-star-heading">资源星图</h2>
        <button class="overview-link" on:click={onOpenResources}>资源管理 <ArrowUpRight size={14} aria-hidden="true" /></button>
      </header>
      {#if visibleResources.length}
        <div class="resource-star-content">
          <button class="resource-orbit" on:click={onOpenResources} aria-label={`查看当前范围的 ${visibleResources.length} 个资源`}>
            <span class="orbit-ring ring-one"></span><span class="orbit-ring ring-two"></span><span class="orbit-ring ring-three"></span>
            <span class="orbit-node node-one"></span><span class="orbit-node node-two"></span><span class="orbit-node node-three"></span><span class="orbit-node node-four"></span>
            <span class="orbit-center"><strong>{visibleResources.length}</strong><small>可见资源</small></span>
          </button>
          <div class="resource-kind-list" aria-label="资源类型分布">
            {#each resourceKinds.slice(0, 6) as [kind, count]}
              <button class="resource-kind-row" on:click={() => { const resource = visibleResources.find((item) => item.kind === kind); if (resource) onOpenResource(resource); }}>
                <span class="kind-icon"><ResourceBrandIcon resource={visibleResources.find((item) => item.kind === kind)!} fallback={resourceIcon(kind)} /></span>
                <span class="kind-copy"><strong>{resourceSchemaName(kind)}</strong><small>{kind}</small></span>
                <b>{count}</b>
              </button>
            {/each}
            {#if resourceKinds.length > 6}<button class="overview-link more-resources" on:click={onOpenResources}>另外 {resourceKinds.length - 6} 种资源 →</button>{/if}
          </div>
        </div>
      {:else}
        <EmptyState>当前范围还没有可见资源。</EmptyState>
      {/if}
    </section>

    <section class="overview-card scorecard" aria-labelledby="scorecard-heading">
      <header class="overview-card-heading">
        <h2 id="scorecard-heading">关键指标</h2>
        <button class="overview-link" on:click={onOpenNotifications}>通知管理 <ArrowUpRight size={14} aria-hidden="true" /></button>
      </header>
      <div class="score-table-wrap">
        <table class="score-table">
          <thead><tr><th>指标</th><th>当前</th><th>趋势</th><th>状态</th></tr></thead>
          <tbody>
            <tr><td>健康应用</td><td>{healthyApplicationCount} / {applicationCount}</td><td class="no-trend">—</td><td><span class:attention={applicationHealthStatus === '关注'} class:neutral={applicationHealthStatus === '暂无数据'} class="metric-status">{applicationHealthStatus}</span></td></tr>
            <tr><td>实例可用性</td><td>{percentageLabel(healthyInstanceCount, instanceCount)}</td><td class="no-trend">—</td><td><span class:attention={instanceHealthStatus === '关注'} class:neutral={instanceHealthStatus === '暂无数据'} class="metric-status">{instanceHealthStatus}</span></td></tr>
            <tr><td>巡检执行成功率</td><td>{percentageLabel(successfulInspectionRuns, inspectionRuns.length)}</td><td class="no-trend">—</td><td><span class:attention={inspectionStatus === '关注'} class:neutral={inspectionStatus === '暂无数据'} class="metric-status">{inspectionStatus}</span></td></tr>
            <tr><td>未处理告警</td><td>{alertCount}</td><td class="no-trend">—</td><td><span class:attention={alertCount > 0} class="metric-status">{alertStatus}</span></td></tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="overview-card risk-card" aria-labelledby="risk-events-heading">
      <header class="overview-card-heading">
        <h2 id="risk-events-heading">风险事件</h2>
        <span class="exception-count">{alertCount}</span>
      </header>
      {#if exceptions.length}
        <div class="exception-list">
          {#each exceptions.slice(0, 4) as project (project.id)}
            <button class="exception-row" on:click={() => onOpenProject(project)}>
              <span class="exception-icon"><AlertTriangle size={14} aria-hidden="true" /></span>
              <span class="exception-copy"><strong>{project.name}</strong><small>{projectTeamName(project)} · {project.summary.applications} 个应用</small></span>
              <b>{project.summary.alerts}</b>
            </button>
          {/each}
        </div>
        {#if exceptions.length > 4}<button class="overview-link card-footer-link" on:click={onOpenInspection}>查看其余 {exceptions.length - 4} 个项目 →</button>{/if}
      {:else}
        <div class="empty-exceptions"><CircleCheck size={18} aria-hidden="true" /><span>当前范围没有未处理告警</span></div>
      {/if}
    </section>

    <section class="overview-card runtime-health-card" aria-labelledby="runtime-health-heading">
      <header class="overview-card-heading">
        <h2 id="runtime-health-heading">运行健康</h2>
        <span class="exception-count">{unhealthyObjectCount}</span>
      </header>
      <div class="health-rate-list">
        <div class="health-rate">
          <div class="health-rate-heading"><span>应用健康率</span><strong>{percentageLabel(healthyApplicationCount, applicationCount)}</strong></div>
          <div class="health-rate-bar" role="progressbar" aria-label="应用健康率" aria-valuemin="0" aria-valuemax="100" aria-valuenow={applicationCount ? applicationHealthRate : 0}><span style={`width:${applicationCount ? applicationHealthRate : 0}%`}></span></div>
          <small>{healthyApplicationCount} / {applicationCount} 个应用正常</small>
        </div>
        <div class="health-rate">
          <div class="health-rate-heading"><span>资源健康率</span><strong>{percentageLabel(successfulConnections, testedResources.length)}</strong></div>
          <div class="health-rate-bar" role="progressbar" aria-label="资源健康率" aria-valuemin="0" aria-valuemax="100" aria-valuenow={testedResources.length ? resourceHealthRate : 0}><span style={`width:${testedResources.length ? resourceHealthRate : 0}%`}></span></div>
          <small>{#if connectorResources.length === 0}当前范围无可测试资源{:else if testedResources.length === 0}{connectorResources.length} 个连接器尚未测试{:else}{successfulConnections} / {testedResources.length} 个已测连接器正常{#if untestedConnections > 0} · {untestedConnections} 个待测{/if}{/if}</small>
        </div>
      </div>
    </section>
  </div>

  <div class="overview-bottom">
    <section class="overview-card activity-card" aria-labelledby="inspection-heading">
      <header class="overview-card-heading"><h2 id="inspection-heading">智能巡检</h2><button class="overview-link" on:click={onOpenInspection}>查看全部 <ArrowUpRight size={14} aria-hidden="true" /></button></header>
      <div class="preview-card-content">
        <strong class="summary-primary">{inspectionRuns.length}</strong>
        <p class="summary-secondary">
          {#if activityLoading}正在读取巡检记录…{:else if inspectionReadFailed}巡检记录暂不可用{:else}<span>次巡检执行</span>{' '}<span>·</span>{' '}<b>{percentageLabel(successfulInspectionRuns, inspectionRuns.length)}</b>{' '}<span>执行成功率</span>{' '}<span>·</span>{' '}<b>{pendingInspectionRuns}</b>{' '}<span>个任务待执行</span>{/if}
        </p>
      </div>
    </section>

    <section class="overview-card activity-card diagnosis-card" aria-labelledby="diagnosis-heading">
      <header class="overview-card-heading"><h2 id="diagnosis-heading">诊断活动</h2><button class="overview-link" on:click={onOpenDiagnosis}>进入诊断 <ArrowUpRight size={14} aria-hidden="true" /></button></header>
      <div class="preview-card-content diagnosis-summary">
        <strong class="summary-primary">{activeDiagnosisCount}</strong>
        <p class="summary-secondary">
          {#if activityLoading}正在读取诊断活动…{:else if diagnosisReadFailed}诊断记录暂不可用{:else}<span>个活跃会话</span>{' '}<span>·</span>{' '}<b>{diagnosisSessions.length}</b>{' '}<span>个会话</span>{' '}<span>·</span>{' '}<b>{alertCount}</b>{' '}<span>条告警事件</span>{/if}
        </p>
      </div>
    </section>

    <section class="overview-card capability-card organization-card" aria-labelledby="organization-heading">
      <header class="overview-card-heading"><h2 id="organization-heading">组织成员</h2><button class="overview-link" on:click={onOpenAccess}>管理权限 <ArrowUpRight size={14} aria-hidden="true" /></button></header>
      <div class="preview-card-content">
        <strong class="summary-primary">{organizationTeamCount}</strong>
        <p class="summary-secondary"><span>个团队</span>{' '}<span>·</span>{' '}<b>{organizationMemberCount ?? '—'}</b>{' '}<span>名成员</span></p>
      </div>
    </section>

    <section class="overview-card capability-card" aria-labelledby="capability-heading">
      <header class="overview-card-heading"><h2 id="capability-heading">平台能力</h2><button class="overview-link" on:click={onOpenModels}>配置模型 <ArrowUpRight size={14} aria-hidden="true" /></button></header>
      <div class="preview-card-content capability-summary">
        <strong class="summary-primary">{providers.length}</strong>
        <p class="summary-secondary"><span>个模型引擎</span>{' '}<span>·</span>{' '}<b>{skills.length}</b>{' '}<span>项技能</span>{' '}<span>·</span>{' '}<b>{personas.length}</b>{' '}<span>位专家</span></p>
      </div>
    </section>
  </div>
</section>

<style>
  .overview-dashboard { --overview-accent: color-mix(in srgb, var(--theme-teal) 86%, var(--theme-fg-strong)); --overview-success: color-mix(in srgb, var(--theme-success) 86%, var(--theme-fg-strong)); --overview-warning: color-mix(in srgb, var(--theme-warning) 80%, var(--theme-fg-strong)); --overview-danger: color-mix(in srgb, var(--theme-danger) 86%, var(--theme-fg-strong)); --overview-info: color-mix(in srgb, var(--theme-info) 86%, var(--theme-fg-strong)); display: grid; gap: 14px; padding-top: 18px; }
  .overview-metrics { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; }
  .overview-metric, .overview-card { min-width: 0; color: var(--theme-fg-strong); background: var(--theme-bg-raised); border: 1px solid var(--theme-border); border-radius: 8px; }
  .overview-sparkline-symbols { position: absolute; width: 0; height: 0; overflow: hidden; }
  .overview-metric { padding: 11px 12px; }
  .overview-metric-label { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-width: 0; color: var(--theme-fg-muted); font-size: 9px; }
  .overview-metric-label > span { position: relative; top: -2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .overview-sparkline { width: 70px; height: 23px; flex: 0 0 70px; color: var(--overview-accent); }
  .overview-metric small { display: block; color: var(--theme-fg-muted); font-size: 9px; }
  .overview-metric small.overview-note { color: var(--theme-fg-muted); }
  .overview-metric small.overview-note.good { color: var(--overview-success); }
  .overview-metric small.overview-note.attention { color: var(--overview-warning); }
  .overview-metric small.overview-note.danger { color: var(--overview-danger); }
  .overview-metric small.overview-note.neutral { color: var(--theme-fg-muted); }
  .overview-metric strong { display: block; margin: 5px 0 0; font-size: 23px; line-height: 1; font-variant-numeric: tabular-nums; }
  .overview-metric small.overview-note { margin-top: 7px; }
  .overview-metric strong.warning { color: var(--overview-danger); }
  .overview-middle { display: grid; grid-template-columns: minmax(0, 2fr) minmax(160px, 1fr) minmax(0, 2fr); grid-template-rows: repeat(2, minmax(0, auto)); gap: 10px; align-items: stretch; }
  .overview-card { overflow: hidden; }
  .overview-card-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-height: 37px; padding: 0 11px; border-bottom: 1px solid var(--theme-divider); }
  .overview-card-heading h2 { margin: 0; font-size: 10px; font-weight: 700; }
  .overview-link { display: inline-flex; align-items: center; justify-content: center; gap: 4px; padding: 5px 0; color: var(--overview-accent); background: transparent; border: 0; font-size: 10px; white-space: nowrap; }
  .overview-link:hover { color: var(--theme-fg-strong); }
  .resource-star-card { grid-column: 1; grid-row: 1 / span 2; }
  .scorecard { grid-column: 3; grid-row: 1 / span 2; }
  .risk-card { grid-column: 2; grid-row: 1; }
  .runtime-health-card { grid-column: 2; grid-row: 2; }
  .scorecard, .risk-card, .runtime-health-card { min-width: 0; }
  .resource-star-content { display: grid; grid-template-columns: minmax(160px, .9fr) minmax(0, 1.1fr); align-items: center; gap: 14px; min-height: 280px; padding: 14px; }
  .resource-orbit { position: relative; display: grid; place-items: center; width: min(100%, 250px); aspect-ratio: 1; justify-self: center; color: var(--theme-fg-strong); background: transparent; border: 0; }
  .orbit-ring { position: absolute; border: 1px solid color-mix(in srgb, var(--overview-accent) 44%, var(--theme-border)); border-radius: 50%; }
  .ring-one { inset: 7%; }.ring-two { inset: 21%; border-style: dashed; }.ring-three { inset: 35%; border-color: color-mix(in srgb, var(--overview-accent) 66%, var(--theme-border)); }
  .orbit-center { z-index: 1; display: grid; place-items: center; width: 29%; aspect-ratio: 1; background: var(--theme-bg-surface); border: 1px solid var(--theme-border); border-radius: 50%; box-shadow: var(--theme-shadow-soft); }
  .orbit-center strong { font-size: 20px; line-height: 1; }.orbit-center small { margin-top: -15px; color: var(--theme-fg-muted); font-size: 8px; }
  .orbit-node { position: absolute; z-index: 1; width: 10px; height: 10px; background: var(--overview-accent); border: 2px solid var(--theme-bg-raised); border-radius: 50%; box-shadow: 0 0 0 3px color-mix(in srgb, var(--overview-accent) 26%, transparent); }
  .node-one { top: 12%; right: 26%; }.node-two { bottom: 18%; right: 13%; background: var(--overview-info); }.node-three { bottom: 13%; left: 25%; background: var(--overview-warning); }.node-four { top: 30%; left: 8%; background: var(--overview-success); }
  .resource-kind-list { display: grid; align-content: center; gap: 2px; }
  .resource-kind-row { display: flex; align-items: center; gap: 9px; min-width: 0; padding: 8px 5px; color: var(--theme-fg-strong); background: transparent; border: 0; border-bottom: 1px solid var(--theme-divider); text-align: left; }
  .resource-kind-row:hover { background: var(--theme-bg-hover); }
  .kind-icon { display: grid; place-items: center; width: 28px; height: 28px; flex: 0 0 auto; color: var(--overview-accent); background: var(--theme-bg-subtle); border-radius: 7px; }
  .kind-copy { min-width: 0; flex: 1; }.kind-copy strong, .kind-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.kind-copy strong { font-size: 10px; }.kind-copy small { margin-top: 2px; color: var(--theme-fg-muted); font-size: 8px; }
  .resource-kind-row > b { font-size: 11px; font-variant-numeric: tabular-nums; }
  .more-resources { justify-self: start; }
  .scorecard, .risk-card, .runtime-health-card { min-height: 0; }
  .score-table-wrap { overflow-x: auto; padding: 2px 11px 8px; }
  .score-table { width: 100%; border-collapse: collapse; text-align: left; }
  .score-table th { padding: 7px 8px; color: var(--theme-fg-muted); background: var(--theme-bg-subtle); border-bottom: 1px solid var(--theme-divider); font-size: 9px; font-weight: 650; white-space: nowrap; }
  .score-table td { padding: 9px 8px; border-bottom: 1px solid var(--theme-divider); font-size: 10px; white-space: nowrap; }
  .score-table tr:last-child td { border-bottom: 0; }.score-table td:first-child { font-weight: 650; }.score-table .no-trend { color: var(--theme-fg-muted); text-align: center; }
  .metric-status { display: inline-flex; align-items: center; padding: 3px 6px; color: var(--overview-success); background: var(--theme-bg-success-soft); border-radius: 4px; font-size: 9px; font-weight: 650; }
  .metric-status.attention { color: var(--overview-warning); background: var(--theme-bg-warning-soft); }
  .metric-status.neutral { color: var(--theme-fg-muted); background: var(--theme-bg-subtle); }
  .exception-count { display: grid; place-items: center; min-width: 22px; height: 22px; padding: 0 5px; color: var(--overview-danger); background: var(--theme-bg-danger-soft); border-radius: 6px; font-size: 10px; font-weight: 700; }
  .exception-list { padding: 4px 13px; }
  .exception-row { display: flex; align-items: center; gap: 8px; width: 100%; min-width: 0; padding: 7px 0; color: var(--theme-fg-strong); background: transparent; border: 0; border-bottom: 1px solid var(--theme-divider); text-align: left; }
  .exception-row:hover { background: var(--theme-bg-hover); }.exception-icon { display: grid; place-items: center; width: 24px; height: 24px; flex: 0 0 auto; color: var(--overview-warning); background: var(--theme-bg-warning-soft); border-radius: 6px; }
  .exception-copy { min-width: 0; flex: 1; }.exception-copy strong, .exception-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.exception-copy strong { font-size: 9px; }.exception-copy small { margin-top: 2px; color: var(--theme-fg-muted); font-size: 8px; }.exception-row > b { color: var(--overview-danger); font-size: 10px; }
  .card-footer-link { margin: 2px 13px 7px; }.empty-exceptions { display: flex; align-items: center; gap: 8px; min-height: 70px; padding: 14px; color: var(--overview-success); font-size: 10px; }
  .health-rate-list { display: grid; gap: 16px; padding: 14px; }
  .health-rate-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; color: var(--theme-fg-muted); font-size: 10px; }
  .health-rate-heading strong { color: var(--theme-fg-strong); font-size: 13px; font-variant-numeric: tabular-nums; }
  .health-rate-bar { height: 6px; margin-top: 7px; overflow: hidden; background: var(--theme-bg-subtle); border-radius: 99px; }
  .health-rate-bar span { display: block; height: 100%; background: var(--overview-accent); border-radius: inherit; }
  .health-rate > small { display: block; margin-top: 6px; color: var(--theme-fg-muted); font-size: 9px; }
  .overview-bottom { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; width: 100%; }
  .overview-bottom .overview-card { border: 1px solid var(--theme-border); border-radius: 6px; }
  .overview-bottom .overview-card-heading { min-height: 37px; }
  .preview-card-content { min-height: 72px; padding: 11px 13px 12px; }
  .summary-primary { display: block; color: var(--theme-fg-strong); font-size: 24px; font-weight: 750; line-height: 1; font-variant-numeric: tabular-nums; }
  .summary-secondary { margin: 7px 0 0; color: var(--theme-fg-muted); font-size: 9px; line-height: 1.35; }
  .summary-secondary b { color: var(--theme-fg-strong); font-size: 14px; font-weight: 700; font-variant-numeric: tabular-nums; }
  @media (max-width: 1080px) { .overview-middle { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto; }.resource-star-card, .scorecard, .risk-card, .runtime-health-card { grid-column: auto; grid-row: auto; }.resource-star-content { min-height: 230px; }.overview-bottom { grid-template-columns: repeat(2, minmax(0, 1fr)); }.capability-card { grid-column: auto; } }
  @media (max-width: 720px) { .overview-dashboard { padding-top: 12px; }.overview-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }.overview-metric:last-child { grid-column: 1 / -1; }.resource-star-content { grid-template-columns: minmax(120px, .8fr) minmax(0, 1.2fr); gap: 6px; padding: 10px; }.overview-bottom { grid-template-columns: minmax(0, 1fr); } }
</style>
