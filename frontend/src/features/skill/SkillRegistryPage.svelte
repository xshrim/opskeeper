<script lang="ts">
  import { BookOpen, Check, ChevronRight, Clipboard, Code2, Copy, Edit3, FileJson, Filter, Play, Plus, Search, ShieldCheck, Sparkles, Wrench, X } from 'lucide-svelte';
  import { api, ApiError, type SkillCatalogItem, type SkillVersion } from '../../lib/api';

  type SkillToolOption = {
    name: string;
    title: string;
    description: string;
    inputSchema: Record<string, unknown>;
  };
  type Category = 'all' | 'diagnosis' | 'monitoring' | 'optimization' | 'maintenance';
  type DetailTab = 'overview' | 'schema' | 'versions' | 'logs';

  const toolOptions: SkillToolOption[] = [
    { name: 'connector_kubernetes_read', title: 'Kubernetes 只读查询', description: '查询目标 Kubernetes 资源；不会修改集群。', inputSchema: { type: 'object', required: ['resource'], properties: { target_resource_id: { type: 'string' }, resource: { type: 'string' }, namespace: { type: 'string' }, name: { type: 'string' }, label_selector: { type: 'string' }, limit: { type: 'integer' } }, additionalProperties: false } },
    { name: 'connector_metrics_query', title: '指标查询', description: '通过已关联的 Prometheus 查询时间序列指标。', inputSchema: { type: 'object', required: ['query', 'start', 'end', 'step_seconds'], properties: { target_resource_id: { type: 'string' }, query: { type: 'string' }, start: { type: 'string', format: 'date-time' }, end: { type: 'string', format: 'date-time' }, step_seconds: { type: 'integer' } }, additionalProperties: false } },
    { name: 'connector_logs_query', title: '日志查询', description: '通过已关联的 Loki 查询限定范围内的日志。', inputSchema: { type: 'object', required: ['query', 'start', 'end', 'limit'], properties: { target_resource_id: { type: 'string' }, query: { type: 'string' }, start: { type: 'string', format: 'date-time' }, end: { type: 'string', format: 'date-time' }, limit: { type: 'integer' }, keyword: { type: 'string' } }, additionalProperties: false } },
    { name: 'connector_traces_query', title: '链路查询', description: '通过已关联的追踪平台查询服务调用链。', inputSchema: { type: 'object', required: ['service', 'start', 'end', 'limit'], properties: { target_resource_id: { type: 'string' }, service: { type: 'string' }, operation: { type: 'string' }, start: { type: 'string', format: 'date-time' }, end: { type: 'string', format: 'date-time' }, limit: { type: 'integer' } }, additionalProperties: false } },
    { name: 'connector_alerts_get', title: '告警查询', description: '查询目标关联监控平台中的当前告警。', inputSchema: { type: 'object', properties: { target_resource_id: { type: 'string' }, active_only: { type: 'boolean' } }, additionalProperties: false } }
  ];

  const categoryLabels: Record<string, string> = { diagnosis: '诊断', monitoring: '监控', optimization: '优化', maintenance: '维护' };
  const categoryIcons: Record<string, typeof Search> = { diagnosis: Sparkles, monitoring: Play, optimization: Code2, maintenance: Wrench };

  export let skills: SkillCatalogItem[] = [];
  export let scopeId = '';
  export let scopeName: (id: string) => string;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;

  let query = '';
  let category: Category = 'all';
  let selectedSkillId = '';
  let selectedVersionId = '';
  let versions: SkillVersion[] = [];
  let instruction = '';
  let description = '';
  let inputSchema = '{"type":"object","additionalProperties":true}';
  let outputSchema = '{"type":"object","additionalProperties":true}';
  let selectedToolNames: string[] = [];
  let detailTab: DetailTab = 'overview';
  let editing = false;
  let busy = false;
  let copied = false;
  let versionsLoading = false;

  $: filteredSkills = skills.filter((skill) => {
    const text = `${skill.name} ${skill.identifier} ${skill.category} ${skill.tags.join(' ')} ${skill.maintainer}`.toLowerCase();
    const matchesQuery = !query.trim() || text.includes(query.trim().toLowerCase());
    return matchesQuery && (category === 'all' || skill.category === category);
  });
  $: selectedSkill = skills.find((skill) => skill.id === selectedSkillId) ?? null;
  $: selectedVersion = versions.find((version) => version.id === selectedVersionId) ?? null;
  $: publishedCount = skills.filter((skill) => skill.status === 'active').length;
  $: totalTools = selectedVersion?.tools.length ?? 0;

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) { if (error.status === 403) return '当前账号没有执行此操作的权限。'; if (error.status === 401) return '会话已过期，请重新登录。'; return error.message || fallback; }
    return error instanceof Error ? error.message || fallback : fallback;
  }
  function categoryLabel(value: string) { return categoryLabels[value] ?? (value || '未分类'); }
  function versionStatus(status: string) { return status === 'published' ? '已发布' : status === 'disabled' ? '已停用' : '草稿'; }
  function pretty(value: unknown) { try { return JSON.stringify(value, null, 2); } catch { return String(value); } }
  function resetEditor(version?: SkillVersion) {
    instruction = version?.manifest.instruction ?? '';
    description = version?.manifest.description ?? '';
    inputSchema = pretty(version?.input_schema ?? { type: 'object', additionalProperties: true });
    outputSchema = pretty(version?.output_schema ?? { type: 'object', additionalProperties: true });
    selectedToolNames = version?.tools.map((tool) => tool.name) ?? [];
  }
  async function loadVersions(skillID = selectedSkillId, open = false) {
    if (!skillID) return;
    versionsLoading = true;
    try {
      versions = await api.skillVersions(skillID);
      selectedVersionId = versions.find((item) => item.status === 'published')?.id || versions[0]?.id || '';
      resetEditor(versions.find((item) => item.id === selectedVersionId));
      if (open) { selectedSkillId = skillID; editing = false; detailTab = 'overview'; }
    } catch (error) { onError(describeError(error, 'Skill 版本加载失败')); }
    finally { versionsLoading = false; }
  }
  function selectSkill(skill: SkillCatalogItem) { selectedSkillId = skill.id; void loadVersions(skill.id, true); }
  function startNewVersion() {
    if (!selectedSkill) { if (filteredSkills[0]) selectSkill(filteredSkills[0]); return; }
    selectedVersionId = '';
    resetEditor();
    editing = true;
    detailTab = 'overview';
  }
  function startEdit() { editing = true; resetEditor(selectedVersion ?? undefined); }
  function cancelEdit() { editing = false; resetEditor(selectedVersion ?? undefined); }
  function toggleTool(name: string) { selectedToolNames = selectedToolNames.includes(name) ? selectedToolNames.filter((item) => item !== name) : [...selectedToolNames, name]; }
  async function createVersion() {
    if (!selectedSkillId) return;
    busy = true; onError('');
    try {
      const version = await api.createSkillVersion(selectedSkillId, { manifest: { name: selectedSkill?.name || '技能', description: description || '由 OpsKeeper 管理的声明式技能', instruction }, input_schema: JSON.parse(inputSchema), output_schema: JSON.parse(outputSchema), tools: toolOptions.filter((tool) => selectedToolNames.includes(tool.name)).map((tool) => ({ name: tool.name, description: tool.description, input_schema: tool.inputSchema })), risk_level: 'read_only' });
      versions = [version, ...versions]; selectedVersionId = version.id; editing = false; detailTab = 'overview'; onNotice(`技能 v${version.version} 草稿已创建`);
    } catch (error) { onError(describeError(error, '创建技能版本失败')); }
    finally { busy = false; }
  }
  async function publishVersion() {
    if (!selectedSkillId || !selectedVersionId) return;
    busy = true; onError('');
    try { await api.publishSkillVersion(selectedSkillId, selectedVersionId); await loadVersions(); onNotice('Skill 版本已发布'); }
    catch (error) { onError(describeError(error, '发布 Skill 版本失败')); }
    finally { busy = false; }
  }
  async function setDefault() {
    if (!scopeId || !selectedSkillId || !selectedVersionId) return;
    busy = true; onError('');
    try { await api.setSkillDefault({ scope_id: scopeId, skill_id: selectedSkillId, skill_version_id: selectedVersionId }); onNotice('默认技能已更新'); }
    catch (error) { onError(describeError(error, '设置默认技能失败')); }
    finally { busy = false; }
  }
  async function copyBody() {
    if (!selectedVersion?.manifest.instruction) return;
    try { await navigator.clipboard.writeText(selectedVersion.manifest.instruction); copied = true; setTimeout(() => (copied = false), 1400); } catch { onError('无法访问剪贴板，请手动复制。'); }
  }
  function closeDetail() { selectedSkillId = ''; selectedVersionId = ''; versions = []; editing = false; }
</script>

<section class="skill-page">
  <header class="skill-page-header">
    <div><div class="skill-breadcrumb"><span>项目</span><ChevronRight size={13} /><strong>Skills</strong></div><p>诊断定位 · LLM 可调用的能力与扩展</p></div>
    <button class="primary skill-create-button" type="button" on:click={startNewVersion} disabled={skills.length === 0}><Plus size={16} aria-hidden="true" />新建 Skill</button>
  </header>

  <div class="skill-stat-grid">
    <article class="skill-stat-card"><div><span>总 Skills</span><strong>{skills.length}</strong><small><i class="dot purple"></i> 内置 {skills.filter((skill) => skill.maintainer === 'native' || skill.maintainer === 'system').length} <i class="dot green"></i> 自定义 {Math.max(0, skills.length - skills.filter((skill) => skill.maintainer === 'native' || skill.maintainer === 'system').length)}</small></div><BookOpen size={17} aria-hidden="true" /></article>
    <article class="skill-stat-card"><div><span>7 天调用次数</span><strong>0</strong><small>→ 0.0% vs 上周</small></div><Play size={17} aria-hidden="true" /></article>
    <article class="skill-stat-card"><div><span>平均 Token 消耗</span><strong>0 <em>tokens / 次</em></strong><small>≈ ¥0.00 / 次 → 0.0% vs 上周</small></div><Sparkles size={17} aria-hidden="true" /></article>
  </div>

  <div class="skill-filterbar">
    <label class="skill-search"><Search size={16} aria-hidden="true" /><span class="sr-only">搜索 Skills</span><input bind:value={query} placeholder="搜索 name / 描述 / 标签..." /></label>
    <span class="skill-filter-label">类别</span>
    <div class="skill-category-filters"><button class:active={category === 'all'} on:click={() => (category = 'all')}>全部 {skills.length}</button>{#each Object.entries(categoryLabels) as [key, label]}<button class:active={category === key} on:click={() => (category = key as Category)}><svelte:component this={categoryIcons[key]} size={13} aria-hidden="true" />{label} {skills.filter((skill) => skill.category === key).length}</button>{/each}</div>
    <span class="skill-filter-result"><Filter size={14} aria-hidden="true" />{filteredSkills.length} 个技能</span>
  </div>

  <div class:has-detail={Boolean(selectedSkill)} class="skill-content">
    <section class="skill-list-panel panel">
      <div class="skill-list-header"><label><input type="checkbox" aria-label="选择全部技能" /><span>名称 / KEY</span></label><span>分类</span><span>类型</span><span>来源</span><span>上次调用</span><span>平均 TOKEN</span><span>风险</span><span>描述</span><span>操作</span></div>
      {#if filteredSkills.length === 0}<div class="skill-empty"><BookOpen size={25} aria-hidden="true" /><strong>没有匹配的 Skill</strong><span>请清除筛选条件或创建一个新的技能版本。</span></div>{/if}
      {#each filteredSkills as skill}
        {@const Category = categoryIcons[skill.category] ?? BookOpen}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex a11y_no_noninteractive_element_interactions -->
        <article class:selected={selectedSkillId === skill.id} class="skill-row" on:click={() => selectSkill(skill)} role="listitem" tabindex="0" on:keydown={(event) => (event.key === 'Enter' || event.key === ' ') && selectSkill(skill)}>
          <div class="skill-name-cell"><input type="checkbox" aria-label={`选择 ${skill.name}`} on:click|stopPropagation /><span class="skill-row-icon"><BookOpen size={15} aria-hidden="true" /></span><div><strong>{skill.name}</strong><small>{skill.identifier} · {skill.tags.slice(0, 3).join(' · ')}</small></div></div>
          <span><span class="skill-category-chip"><Category size={13} aria-hidden="true" />{categoryLabel(skill.category)}</span></span>
          <span><span class="skill-type-chip">Skill</span></span>
          <span class="skill-source">{skill.maintainer || 'native'}</span>
          <span class="skill-last-call">从未调用</span>
          <span class="skill-token-cost">—</span>
          <span><span class="skill-risk safe">safe</span></span>
          <span class="skill-description">{skill.tags.slice(0, 2).join(' · ') || '声明式能力'}</span>
          <span class="skill-row-actions"><button type="button" class="icon-button" aria-label={`执行 ${skill.name}`} data-tooltip="执行 Skill" on:click|stopPropagation={() => onNotice(`已选择 Skill：${skill.name}`)}><Play size={14} /></button><button type="button" class="icon-button" aria-label={`编辑 ${skill.name}`} data-tooltip="查看与编辑" on:click|stopPropagation={() => selectSkill(skill)}><Edit3 size={14} /></button><button type="button" class="icon-button" aria-label="复制 Skill 标识" data-tooltip="复制标识" on:click|stopPropagation={() => onNotice(`Skill 标识：${skill.identifier}`)}><Copy size={14} /></button></span>
        </article>
      {/each}
    </section>

    {#if selectedSkill}
      <aside class="skill-detail-panel panel">
        <div class="skill-detail-header"><div class="skill-detail-title"><span class="skill-detail-icon"><BookOpen size={18} /></span><div><h2>{selectedSkill.name}</h2><small>{selectedSkill.identifier} · {selectedVersion ? `v${selectedVersion.version}` : '暂无版本'}</small></div></div><button class="icon-button" type="button" aria-label="关闭详情" on:click={closeDetail}><X size={17} /></button></div>
        <div class="skill-detail-meta"><span class="skill-category-chip"><svelte:component this={categoryIcons[selectedSkill.category] ?? BookOpen} size={13} />{categoryLabel(selectedSkill.category)}</span><span class="skill-status-chip"><Check size={13} />{selectedSkill.status === 'active' ? '安全' : selectedSkill.status}</span><span>{selectedSkill.tags.length} 个标签</span></div>
        <nav class="skill-detail-tabs" aria-label="Skill 详情"><button class:active={detailTab === 'overview'} on:click={() => (detailTab = 'overview')}>概览</button><button class:active={detailTab === 'schema'} on:click={() => (detailTab = 'schema')}>输入输出</button><button class:active={detailTab === 'versions'} on:click={() => (detailTab = 'versions')}>版本</button><button class:active={detailTab === 'logs'} on:click={() => (detailTab = 'logs')}>调用日志</button></nav>
        <div class="skill-detail-scroll">
          {#if versionsLoading}<div class="skill-detail-loading">正在加载版本...</div>{:else if detailTab === 'overview'}
            <section class="skill-detail-section"><h3>说明</h3><p>{selectedVersion?.manifest.description || '该 Skill 尚未发布版本说明。'}</p></section>
            <section class="skill-detail-section"><h3>元数据</h3><div class="skill-meta-grid"><div><span>唯一标识</span><strong>{selectedSkill.identifier}</strong></div><div><span>维护者</span><strong>{selectedSkill.maintainer || '未指定'}</strong></div><div><span>当前级别</span><strong>{scopeName(selectedSkill.scope_id)}</strong></div><div><span>风险级别</span><strong>只读</strong></div></div></section>
            <section class="skill-detail-section"><h3>标签</h3><div class="skill-tags">{#each selectedSkill.tags as tag}<span>{tag}</span>{:else}<em>暂无标签</em>{/each}</div></section>
            <section class="skill-detail-section"><div class="skill-section-title"><h3>正文</h3><button class="icon-button" type="button" data-tooltip={copied ? '已复制' : '复制正文'} on:click={copyBody}>{#if copied}<Check size={14} />{:else}<Clipboard size={14} />{/if}</button></div><div class="skill-body-preview"><small>格式 · markdown</small><pre>{selectedVersion?.manifest.instruction || '暂无正文内容'}</pre></div></section>
            <section class="skill-detail-section"><h3>关联工具 <span>{totalTools}</span></h3><div class="skill-tool-summary">{#each selectedVersion?.tools ?? [] as tool}<span><Wrench size={12} />{tool.name.replace('connector_', '')}</span>{:else}<em>当前版本未声明工具</em>{/each}</div></section>
          {:else if detailTab === 'schema'}
            <section class="skill-detail-section"><h3>输入 Schema</h3><pre class="schema-preview">{pretty(selectedVersion?.input_schema ?? {})}</pre></section><section class="skill-detail-section"><h3>输出 Schema</h3><pre class="schema-preview">{pretty(selectedVersion?.output_schema ?? {})}</pre></section>
          {:else if detailTab === 'versions'}
            <section class="skill-detail-section"><div class="version-list">{#each versions as version}<button class:selected={selectedVersionId === version.id} on:click={() => { selectedVersionId = version.id; resetEditor(version); }}><span><strong>v{version.version}</strong><small>{versionStatus(version.status)} · {new Date(version.created_at).toLocaleDateString('zh-CN')}</small></span><ChevronRight size={14} /></button>{:else}<em>暂无版本</em>{/each}</div></section>
          {:else}<section class="skill-detail-section skill-logs-empty"><Clipboard size={22} /><strong>暂无调用日志</strong><span>Skill 被诊断、巡检或工作流调用后，日志会显示在这里。</span></section>{/if}
        </div>
        <div class="skill-detail-footer"><button class="secondary" type="button" on:click={setDefault} disabled={busy || !selectedVersionId}>设为默认</button>{#if selectedVersion?.status !== 'published'}<button class="secondary" type="button" on:click={publishVersion} disabled={busy || !selectedVersionId}>发布版本</button>{/if}<button class="primary" type="button" on:click={startEdit}><Edit3 size={14} />编辑 Skill</button></div>

        {#if editing}<div class="skill-editor-overlay"><div class="skill-editor-heading"><div><p class="eyebrow">SKILL VERSION</p><h3>{selectedSkill.name}</h3><small>创建不可变版本草稿</small></div><button class="icon-button" type="button" aria-label="关闭编辑" on:click={cancelEdit}><X size={17} /></button></div><form class="skill-editor-form" on:submit|preventDefault={createVersion}><label>正文内容<textarea bind:value={instruction} rows="11" required placeholder="描述目标、边界、步骤与输出要求"></textarea></label><div class="skill-editor-grid"><label>输入 Schema<textarea bind:value={inputSchema} rows="8" spellcheck="false"></textarea></label><label>输出 Schema<textarea bind:value={outputSchema} rows="8" spellcheck="false"></textarea></label></div><label>版本说明<input bind:value={description} placeholder="说明本版本的用途和变化" /></label><fieldset class="skill-tool-picker"><legend>可使用工具 <span>{selectedToolNames.length}</span></legend><div class="skill-tool-grid">{#each toolOptions as tool}<label class:selected={selectedToolNames.includes(tool.name)}><input type="checkbox" checked={selectedToolNames.includes(tool.name)} on:change={() => toggleTool(tool.name)} /><span><strong>{tool.title}</strong><small>{tool.description}</small></span></label>{/each}</div></fieldset><div class="skill-editor-actions"><button class="secondary" type="button" on:click={cancelEdit}>取消</button><button class="primary" disabled={busy || !selectedSkillId}><Plus size={14} />创建版本</button></div></form></div>{/if}
      </aside>
    {/if}
  </div>
</section>
