<script lang="ts">
  import type { AgentProfileVersion, Resource } from '../../lib/api';

  export let profiles: Resource[] = [];
  export let selectedProfileId = '';
  export let selectedVersionId = '';
  export let versions: AgentProfileVersion[] = [];
  export let profileName = '';
  export let profileInstruction = '';
  export let profileCapabilities = 'text, tool_calling, stream';
  export let profileAllowedTools = '';
  export let profileTargetKinds = 'Application';
  export let profileInputSchema = '{"type":"object","additionalProperties":true}';
  export let profileOutputSchema = '{"type":"object","additionalProperties":true}';
  export let busy = false;
  export let selectedScopeId = '';
  export let scopeName: (id: string) => string;
  export let formatDate: (value: string) => string;
  export let onLoadVersions: () => void | Promise<void>;
  export let onPublish: () => void | Promise<void>;
  export let onCreate: () => void | Promise<void>;
</script>

<section class="content-grid two-column ai-runtime">
  <section class="panel">
    <div class="panel-heading">
      <div><p class="eyebrow">AGENT PROFILES</p><h2>Agent 专家配置</h2></div>
      <span class="count">{profiles.length}</span>
    </div>
    <div class="stack-form compact-form">
      <label>AgentProfile<select bind:value={selectedProfileId} on:change={onLoadVersions}><option value="" disabled>选择 AgentProfile</option>{#each profiles as item}<option value={item.id}>{item.name} · {scopeName(item.scope_id)}</option>{/each}</select></label>
      <label>已发布版本<select bind:value={selectedVersionId}><option value="">选择版本</option>{#each versions as version}<option value={version.id}>v{version.version} · {version.status}</option>{/each}</select></label>
      <button class="primary" type="button" disabled={busy || !selectedProfileId || !selectedVersionId} on:click={onPublish}>发布版本</button>
      {#if versions.length === 0}<p class="muted-copy">版本发布后，AIEngine 执行时会固定使用已发布的专家指令和工具契约。</p>{/if}
    </div>
  </section>
  <section class="panel wide-panel">
    <div class="panel-heading"><div><p class="eyebrow">NEW PROFILE</p><h2>创建 AgentProfile</h2></div><span class="scope-type">版本化契约</span></div>
    <form class="stack-form" on:submit|preventDefault={onCreate}>
      <div class="form-row">
        <label>名称<input bind:value={profileName} required placeholder="例如：PostgreSQL 故障专家" /></label>
        <label>适用资源类型<input bind:value={profileTargetKinds} required placeholder="Application, PostgreSQL" /></label>
      </div>
      <label>专家指令<textarea bind:value={profileInstruction} rows="5" required placeholder="描述诊断范围、判断原则和输出要求"></textarea></label>
      <div class="form-row">
        <label>模型能力<input bind:value={profileCapabilities} placeholder="text, tool_calling, stream" /></label>
        <label>允许工具<input bind:value={profileAllowedTools} placeholder="connector_postgresql_inspect" /></label>
      </div>
      <div class="form-row">
        <label>输入 Schema<textarea bind:value={profileInputSchema} rows="4" spellcheck="false"></textarea></label>
        <label>输出 Schema<textarea bind:value={profileOutputSchema} rows="4" spellcheck="false"></textarea></label>
      </div>
      <button class="primary" disabled={busy || !selectedScopeId}>创建并发布 v1</button>
    </form>
  </section>
  <section class="panel wide-panel">
    <div class="panel-heading"><div><p class="eyebrow">RELEASE HISTORY</p><h2>版本历史</h2></div><span class="count">{versions.length}</span></div>
    <div class="table-list">
      {#each versions as version}
        <div class="list-row static"><span><strong>v{version.version}</strong><small>{formatDate(version.created_at)} · {version.status}</small></span><span class="status-label {version.status}">{version.status === 'published' ? '已发布' : version.status === 'disabled' ? '已停用' : '草稿'}</span></div>
      {:else}<div class="empty-state">选择 AgentProfile 后显示版本历史。</div>{/each}
    </div>
  </section>
</section>
