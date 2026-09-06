<script lang="ts">
  import { api, ApiError, type AgentProfileVersion, type Resource } from '../../lib/api';

  export let profiles: Resource[] = [];
  export let scopeId = '';
  export let scopeName: (id: string) => string;
  export let formatDate: (value: string) => string;
  export let onResourceCreated: (resource: Resource) => void;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;
  let selectedProfileId = ''; let selectedVersionId = ''; let versions: AgentProfileVersion[] = [];
  let profileName = ''; let profileInstruction = ''; let profileCapabilities = 'text, tool_calling, stream'; let profileAllowedTools = ''; let profileTargetKinds = 'Application'; let profileInputSchema = '{"type":"object","additionalProperties":true}'; let profileOutputSchema = '{"type":"object","additionalProperties":true}'; let busy = false;
  $: if (!selectedProfileId && profiles[0]) { selectedProfileId = profiles[0].id; void loadVersions(); }
  function describeError(error: unknown, fallback: string) { if (error instanceof ApiError) { if (error.status === 403) return '当前账号没有执行此操作的权限。'; if (error.status === 401) return '会话已过期，请重新登录。'; return error.message || fallback; } if (error instanceof SyntaxError) return '配置必须是有效的 JSON 对象。'; return error instanceof Error ? error.message || fallback : fallback; }
  async function loadVersions() { if (!selectedProfileId) return; try { versions = await api.agentProfileVersions(selectedProfileId); selectedVersionId = versions.find((item) => item.status === 'published')?.id || versions[0]?.id || ''; } catch (error) { onError(describeError(error, 'AgentProfile 版本加载失败')); } }
  async function createProfile() { if (!scopeId || !profileName.trim() || !profileInstruction.trim()) return; busy = true; onError(''); try { const config = { version: 1, instruction: profileInstruction.trim(), capabilities: profileCapabilities.split(',').map((item) => item.trim()).filter(Boolean), allowed_tools: profileAllowedTools.split(',').map((item) => item.trim()).filter(Boolean), target_kinds: profileTargetKinds.split(',').map((item) => item.trim()).filter(Boolean), input_schema: JSON.parse(profileInputSchema), output_schema: JSON.parse(profileOutputSchema), enabled: true }; const resource = await api.createResource({ scope_id: scopeId, kind: 'AgentProfile', schema_version: 1, name: profileName.trim(), labels: {}, config, status: 'active' }); const version = await api.createAgentProfileVersion(resource.id, config); await api.publishAgentProfileVersion(resource.id, version.id); onResourceCreated(resource); profiles = [resource, ...profiles]; selectedProfileId = resource.id; selectedVersionId = version.id; versions = [version]; profileName = ''; profileInstruction = ''; onNotice('AgentProfile 已创建并发布 v1'); } catch (error) { onError(describeError(error, '创建 AgentProfile 失败')); } finally { busy = false; } }
  async function publishVersion() { if (!selectedProfileId || !selectedVersionId) return; busy = true; onError(''); try { await api.publishAgentProfileVersion(selectedProfileId, selectedVersionId); await loadVersions(); onNotice('AgentProfile 版本已发布'); } catch (error) { onError(describeError(error, '发布 AgentProfile 版本失败')); } finally { busy = false; } }
</script>

<section class="content-grid two-column">
  <section class="panel">
    <div class="panel-heading">
      <div><p class="eyebrow">AGENT PROFILES</p><h2>Agent 专家配置</h2></div>
      <span class="count">{profiles.length}</span>
    </div>
    <div class="stack-form compact-form">
      <label>AgentProfile<select bind:value={selectedProfileId} on:change={loadVersions}><option value="" disabled>选择 AgentProfile</option>{#each profiles as item}<option value={item.id}>{item.name} · {scopeName(item.scope_id)}</option>{/each}</select></label>
      <label>已发布版本<select bind:value={selectedVersionId}><option value="">选择版本</option>{#each versions as version}<option value={version.id}>v{version.version} · {version.status}</option>{/each}</select></label>
      <button class="primary" type="button" disabled={busy || !selectedProfileId || !selectedVersionId} on:click={publishVersion}>发布版本</button>
      {#if versions.length === 0}<p class="muted-copy">版本发布后，AIEngine 执行时会固定使用已发布的专家指令和工具契约。</p>{/if}
    </div>
  </section>
  <section class="panel wide-panel">
    <div class="panel-heading"><div><p class="eyebrow">NEW PROFILE</p><h2>创建 AgentProfile</h2></div><span class="scope-type">版本化契约</span></div>
    <form class="stack-form" on:submit|preventDefault={createProfile}>
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
      <button class="primary" disabled={busy || !scopeId}>创建并发布 v1</button>
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
