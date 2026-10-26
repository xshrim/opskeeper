<script lang="ts">
  import Tabs from '../../components/Tabs.svelte';
  import FormActions from '../../components/FormActions.svelte';
  import ModalDialog from '../../components/ModalDialog.svelte';
  import {
    Bot,
    Check,
    ChevronRight,
    Copy,
    Edit3,
    Plus,
    RefreshCw,
    Shield,
    ShieldCheck,
    Star,
    X
  } from 'lucide-svelte';
  import {
    api,
    type PersonaCatalogItem,
    type PersonaVersion
  } from '../../lib/api';
  import { errorMessage as formatErrorMessage } from '../../lib/errorMessage';
  import SearchInput from '../../components/SearchInput.svelte';
  import FormField from '../../components/FormField.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import TagList from '../../components/TagList.svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import { resourceDomainKinds } from '../../lib/resources';

  export let profiles: PersonaCatalogItem[] = [];
  export let scopeId = '';
  export let scopeName: (id: string) => string;
  export let formatDate: (value: string) => string;
  export let onReload: () => void | Promise<void> = () => {};
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;

  let query = '';
  let selectedProfileId = '';
  let selectedVersionId = '';
  let versions: PersonaVersion[] = [];
  let profileInstruction = '';
  let profileCapabilities = 'text, tool_calling, stream';
  let profileAllowedTools = '';
  let profileRole = 'worker';
  let profileDomainValue = '';
  let profileTags = '';
  let profileWorkerPersonaIds: string[] = [];
  let profileInputSchema = '{"type":"object","additionalProperties":true}';
  let profileOutputSchema = '{"type":"object","additionalProperties":true}';
  let detailTab: 'overview' | 'versions' | 'contract' = 'overview';
  let editorOpen = false;
  let busy = false;
  let loading = false;
  let favoriteIds: string[] = [];
  let personaFilter: 'all' | 'manager' | 'worker' = 'all';
  let createPersonaOpen = false;
  let createPersonaBusy = false;
  let createIdentifier = '';
  let createName = '';
  let createDescription = '';
  let createTags = '';
  let createRole: 'worker' | 'manager' = 'worker';
  let createDomain = '';
  let createWorkerPersonaIds: string[] = [];
  let createInstruction = '';
  const domainSuggestions = [...resourceDomainKinds];
  const personaTagSuggestions = ['内置', '只读', '跨域', '数据库', '基础设施', '巡检', '安全', '性能', '容量', '可委派'];

  $: filteredProfiles = profiles.filter((profile) =>
    `${profile.identifier} ${profile.name} ${profile.tags.join(' ')} ${profile.description} ${profile.status} ${scopeName(profile.scope_id)}`
      .toLowerCase()
      .includes(query.trim().toLowerCase())
  );
  $: visibleProfiles = filteredProfiles.filter(
    (profile) => personaFilter === 'all' || profileType(profile).toLowerCase() === personaFilter
  );
  $: selectedProfile =
    profiles.find((profile) => profile.id === selectedProfileId) ?? null;
  $: selectedVersion =
    versions.find((version) => version.id === selectedVersionId) ?? null;
  function describeError(error: unknown, fallback: string) {
    return formatErrorMessage(error, fallback, {
      forbidden: '当前账号没有执行此操作的权限。',
      unauthorized: '会话已过期，请重新登录。',
      invalidJson: '配置必须是有效的 JSON 对象。'
    });
  }
  function selectedConfig(key: string, fallback: unknown = '') {
    return (
      selectedVersion?.config?.[key] ??
      selectedProfile?.config?.[key] ??
      fallback
    );
  }
  function configText(key: string, fallback = '') {
    const value = selectedConfig(key, fallback);
    return Array.isArray(value) ? value.join(', ') : String(value ?? fallback);
  }
  function profileType(profile: PersonaCatalogItem) {
    const role = String(profile.config?.role ?? '').toLowerCase();
    if (role === 'manager') return 'Manager';
    if (role === 'worker') return 'Worker';
    return /manager|管理|协同|审计/i.test(
      `${profile.name} ${profile.description}`
    )
      ? 'Manager'
      : 'Worker';
  }
  function profileEngine(profile: PersonaCatalogItem) {
    return String(
      profile.config?.engine ??
        profile.config?.provider ??
        (profileType(profile) === 'Manager' ? 'Anthropic AI SDK' : '默认引擎')
    );
  }
  function profileDomain(profile: PersonaCatalogItem) {
    return String(
      profile.config?.domain ??
        profile.config?.resource_kind ??
        (profile.name.replace(/专家|协同|审计|诊断|优化/g, '').trim() || '通用')
    );
  }
  function profileCapabilitiesText(profile: PersonaCatalogItem) {
    return String(
      profile.config?.capabilities ??
        profile.config?.skills ??
        (profileType(profile) === 'Manager' ? '理解 · 分派 · 汇总' : '资源诊断')
    );
  }
  function profileDisplayTags(profile: PersonaCatalogItem) {
    const tags = [...profile.tags];
    if (profile.status === 'active' && !tags.includes('内置')) tags.unshift('内置');
    return tags;
  }
  function profileVersionLabel(profile: PersonaCatalogItem) {
    const version = profile.config?.version;
    return version === undefined || version === null || version === '' ? '版本待定' : `v${version}`;
  }
  function toggleFavorite(id: string) {
    favoriteIds = favoriteIds.includes(id)
      ? favoriteIds.filter((item) => item !== id)
      : [...favoriteIds, id];
  }
  async function loadVersions(profileID = selectedProfileId, open = false) {
    if (!profileID) return;
    loading = true;
    try {
      versions = await api.personaVersions(profileID);
      selectedVersionId =
        versions.find((item) => item.status === 'published')?.id ||
        versions[0]?.id ||
        '';
      if (open) {
        selectedProfileId = profileID;
        detailTab = 'overview';
        editorOpen = false;
      }
    } catch (error) {
      onError(describeError(error, 'Persona 版本加载失败'));
    } finally {
      loading = false;
    }
  }
  function selectProfile(profile: PersonaCatalogItem) {
    selectedProfileId = profile.id;
    void loadVersions(profile.id, true);
  }
  async function refreshProfiles() {
    await onReload();
    if (selectedProfileId) await loadVersions();
  }
  async function openEdit(profile: PersonaCatalogItem) {
    await loadVersions(profile.id, true);
    resetEditor(versions.find((item) => item.id === selectedVersionId));
    editorOpen = true;
  }
  function resetEditor(version = selectedVersion) {
    const config = version?.config ?? {};
    profileInstruction = String(config.instruction ?? '');
    profileCapabilities = Array.isArray(config.capabilities)
      ? config.capabilities.join(', ')
      : String(config.capabilities ?? 'text, tool_calling, stream');
    profileAllowedTools = Array.isArray(config.allowed_tools)
      ? config.allowed_tools.join(', ')
      : String(config.allowed_tools ?? '');
    profileRole = String(
      config.role ?? (selectedProfile && profileType(selectedProfile) === 'Manager' ? 'manager' : 'worker')
    );
    profileDomainValue = String(config.domain ?? config.resource_kind ?? '');
    profileTags = selectedProfile?.tags?.join(', ') ?? '';
    profileWorkerPersonaIds = Array.isArray(config.worker_persona_ids)
      ? config.worker_persona_ids.map(String)
      : [];
    profileInputSchema = JSON.stringify(
      config.input_schema ?? { type: 'object', additionalProperties: true },
      null,
      2
    );
    profileOutputSchema = JSON.stringify(
      config.output_schema ?? { type: 'object', additionalProperties: true },
      null,
      2
    );
  }
  function startCreate() {
    createIdentifier = '';
    createName = '';
    createDescription = '';
    createTags = '';
    createRole = personaFilter === 'manager' ? 'manager' : 'worker';
    createDomain = '';
    createWorkerPersonaIds = [];
    createInstruction = '';
    createPersonaOpen = true;
  }
  async function createPersona() {
    if (!scopeId || !createIdentifier.trim() || !createName.trim() || !createInstruction.trim()) return;
    createPersonaBusy = true;
    onError('');
    try {
      const config = {
        description: createDescription.trim(),
        version: 1,
        instruction: createInstruction.trim(),
        capabilities: ['text', 'tool_calling', 'stream'],
        allowed_tools: [],
        role: createRole,
        domain: createRole === 'worker' ? createDomain.trim() : '',
        worker_persona_ids: createRole === 'manager' ? createWorkerPersonaIds : [],
        input_schema: { type: 'object', additionalProperties: true },
        output_schema: { type: 'object', additionalProperties: true },
        enabled: true
      };
      const item = await api.createPersona({
        scope_id: scopeId,
        identifier: createIdentifier.trim(),
        name: createName.trim(),
        description: createDescription.trim(),
        tags: createTags.split(',').map((item) => item.trim()).filter(Boolean),
        config,
        status: 'active'
      });
      createPersonaOpen = false;
      await onReload();
      selectedProfileId = item.id;
      await loadVersions(item.id, true);
      onNotice(`专家“${item.name}”已创建`);
    } catch (error) {
      onError(describeError(error, '新增专家失败'));
    } finally {
      createPersonaBusy = false;
    }
  }
  function startEdit() {
    resetEditor();
    editorOpen = true;
  }
  async function createVersion() {
    if (!selectedProfileId || !profileInstruction.trim()) return;
    busy = true;
    onError('');
    try {
      const config = {
        version: 1,
        instruction: profileInstruction.trim(),
        capabilities: profileCapabilities
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
        allowed_tools:
          profileRole === 'worker'
            ? profileAllowedTools.split(',').map((item) => item.trim()).filter(Boolean)
            : [],
        role: profileRole,
        domain: profileRole === 'worker' ? profileDomainValue.trim() : '',
        worker_persona_ids:
          profileRole === 'manager'
            ? profileWorkerPersonaIds
            : [],
        input_schema: JSON.parse(profileInputSchema),
        output_schema: JSON.parse(profileOutputSchema),
        enabled: true
      };
      if (selectedProfile) {
        await api.updatePersona(selectedProfile.id, {
          description: selectedProfile.description,
          tags: profileTags.split(',').map((item) => item.trim()).filter(Boolean)
        });
      }
      const version = await api.createPersonaVersion(selectedProfileId, config);
      selectedVersionId = version.id;
      versions = [version, ...versions];
      editorOpen = false;
      await onReload();
      onNotice(`专家 v${version.version} 草稿已创建`);
    } catch (error) {
      onError(describeError(error, '创建专家版本失败'));
    } finally {
      busy = false;
    }
  }
  async function publishVersion() {
    if (!selectedProfileId || !selectedVersionId) return;
    busy = true;
    onError('');
    try {
      await api.publishPersonaVersion(selectedProfileId, selectedVersionId);
      await loadVersions();
      onNotice('Persona 版本已发布');
    } catch (error) {
      onError(describeError(error, '发布 Persona 版本失败'));
    } finally {
      busy = false;
    }
  }
  function closeDetail() {
    selectedProfileId = '';
    selectedVersionId = '';
    versions = [];
    editorOpen = false;
  }
</script>

<section class="persona-page">
  <header class="persona-page-header">
    <div>
      <div class="persona-title">
        <Shield size={21} aria-hidden="true" />
        <h1>专家库</h1>
      </div>
      <p>
        专家库 · 共 {profiles.length} 个，已匹配 {filteredProfiles.length}。职能
        Agent 负责拆解与汇总，资源 Agent 执行域内诊断。
      </p>
    </div>
    <div class="persona-header-actions">
      <button
        class="secondary"
        type="button"
        on:click={refreshProfiles}
        ><RefreshCw size={15} />刷新</button
      ><button
        class="primary"
        type="button"
        on:click={startCreate}
        ><Plus size={16} />新增专家</button
      >
    </div>
  </header>
  <div class="persona-toolbar">
    <SearchInput
      bind:value={query}
      width="min(288px, 100%)"
      height="40px"
      placeholder="搜索名称 / 领域 / 提示词"
      ariaLabel="搜索专家"
    />
    <div class="persona-type-filters" aria-label="专家类型">
      <button class:active={personaFilter === 'all'} type="button" on:click={() => (personaFilter = 'all')}>全部</button>
      <button class:active={personaFilter === 'manager'} type="button" on:click={() => (personaFilter = 'manager')}>职能专家</button>
      <button class:active={personaFilter === 'worker'} type="button" on:click={() => (personaFilter = 'worker')}>资源专家</button>
    </div>
    <span class="persona-toolbar-count">{favoriteIds.length} 个收藏 · {visibleProfiles.length} / {profiles.length} 个专家</span>
  </div>
  {#if createPersonaOpen}
    <ModalDialog
      title="新增专家"
      description="创建一个可用于诊断编排的职能专家或资源专家。"
      confirmLabel="创建"
      confirmForm="create-persona-form"
      busy={createPersonaBusy}
      confirmDisabled={!scopeId || !createIdentifier.trim() || !createName.trim() || !createInstruction.trim() || (createRole === 'worker' && !createDomain.trim()) || (createRole === 'manager' && createWorkerPersonaIds.length === 0)}
      onCancel={() => (createPersonaOpen = false)}
    >
      <form id="create-persona-form" class="persona-editor-form" on:submit|preventDefault={createPersona}>
        <div class="persona-editor-grid">
          <FormField label="唯一标识" required><TextInput bind:value={createIdentifier} required placeholder="如 postgres-health" ariaLabel="专家唯一标识" /></FormField>
          <FormField label="专家名称" required><TextInput bind:value={createName} required placeholder="如 PostgreSQL诊断" ariaLabel="专家名称" /></FormField>
        </div>
        <div class="persona-editor-grid">
          <FormField label="专家职责" required asLabel={false}><DropdownSelect bind:value={createRole} options={[{ value: 'worker', label: '资源专家 · Worker' }, { value: 'manager', label: '职能专家 · Manager' }]} ariaLabel="专家职责" /></FormField>
          {#if createRole === 'worker'}<FormField label="资源领域" required asLabel={false}><TextInput bind:value={createDomain} tagged tagSeparators={[',']} suggestions={domainSuggestions} required placeholder="选择或输入资源领域，回车确认" ariaLabel="资源领域" /></FormField>{:else}<FormField label="可委派资源专家" required asLabel={false}><DropdownSelect bind:value={createWorkerPersonaIds} options={profiles.filter((profile) => profile.status === 'active' && profileType(profile) === 'Worker').map((profile) => ({ value: profile.id, label: `${profile.name}（${profile.identifier}） · ${profileDomain(profile)}` }))} multiple taggedSelection reorderableSelection searchable placeholder="选择资源专家" ariaLabel="可委派资源专家" /></FormField>{/if}
        </div>
        <FormField label="专家标签" asLabel={false}><TextInput bind:value={createTags} tagged tagSeparators={[',']} suggestions={personaTagSuggestions} placeholder="选择或输入标签，回车确认" ariaLabel="专家标签" /></FormField>
        <FormField label="专家说明"><TextArea bind:value={createDescription} rows={3} placeholder="说明职责范围和使用场景" /></FormField>
        <FormField label="专家指令" required><TextArea bind:value={createInstruction} rows={6} required placeholder="描述诊断范围、判断原则和输出要求" /></FormField>
      </form>
    </ModalDialog>
  {/if}
  {#if visibleProfiles.length === 0}<div class="persona-empty panel">
      <Bot size={28} /><strong>没有匹配的专家</strong><span
        >请调整搜索条件，或选择一个已有专家创建版本。</span
      >
    </div>{/if}
  {#if visibleProfiles.length > 0}<div class="persona-card-grid">
          {#each visibleProfiles as profile}
            <!-- svelte-ignore a11y_no_noninteractive_tabindex a11y_no_noninteractive_element_interactions a11y_no_noninteractive_element_to_interactive_role -->
            <article
              class:selected={selectedProfileId === profile.id}
              class="persona-card persona-band-card"
              on:click={() => selectProfile(profile)}
              role="button"
              tabindex="0"
              on:keydown={(event) =>
                (event.key === 'Enter' || event.key === ' ') &&
                selectProfile(profile)}
            >
              <div class="persona-card-band">
                <div class="persona-card-band-main">
                  <Bot size={16} aria-hidden="true" />
                  <div class="persona-card-name">
                    <h2>{profile.name}</h2>
                    <small>{profile.identifier}</small>
                  </div>
                </div>
                <div class="persona-card-band-meta">
                  <span class={`persona-card-type ${profileType(profile).toLowerCase()}`}>{profileType(profile) === 'Manager' ? '职能专家' : '资源专家'}</span>
                  <button
                    class="icon-button"
                    type="button"
                    aria-label="收藏专家"
                    on:click|stopPropagation={() => toggleFavorite(profile.id)}
                    ><Star
                      size={15}
                      fill={favoriteIds.includes(profile.id)
                        ? 'currentColor'
                        : 'none'}
                    /></button
                  >
                </div>
              </div>
              <div class="persona-card-body">
                <div class="persona-card-domain"><span>资源领域</span><strong>{profileDomain(profile)}</strong></div>
                <TagList tags={profileDisplayTags(profile)} className="persona-card-tags" ariaLabel="专家标签" maxItems={4} />
                <p class="persona-card-description">{profile.description || (profileType(profile) === 'Manager' ? '负责拆解问题、委派资源专家并汇总诊断结论。' : '负责指定资源领域的只读诊断与证据分析。')}</p>
              </div>
              <div class="persona-card-footer">
                <div class="persona-card-status"><span class:active={profile.status === 'active'}>{profile.status === 'active' ? '已启用' : profile.status}</span><small>{profileVersionLabel(profile)}</small></div>
                <div class="persona-card-actions">
                  <button
                    class="primary"
                    type="button"
                    on:click|stopPropagation={() =>
                      onNotice(`已选择专家：${profile.name}`)}
                    ><Bot size={14} />使用</button
                  ><button
                    class="secondary"
                    type="button"
                    on:click|stopPropagation={() => openEdit(profile)}
                    ><Edit3 size={14} />编辑</button
                  >
                </div>
              </div>
            </article>{/each}
        </div>{/if}
  {#if selectedProfile}<aside class="persona-detail-panel panel">
      <div class="persona-detail-header">
        <div>
          <p class="eyebrow">PERSONA PROFILE</p>
          <h2>{selectedProfile.name}</h2>
          <small
            >{profileType(selectedProfile)} · {scopeName(
              selectedProfile.scope_id
            )}</small
          >
        </div>
        <button
          class="icon-button"
          type="button"
          aria-label="关闭详情"
          on:click={closeDetail}><X size={17} /></button
        >
      </div>
      <div class="persona-detail-meta">
        <span class="persona-built-in"
          >{selectedProfile.status === 'active'
            ? '内置'
            : selectedProfile.status}</span
        ><span>{versions.length} 个版本</span><span
          >{profileEngine(selectedProfile)}</span
        >
      </div>
      <Tabs className="persona-detail-tabs" value={detailTab} ariaLabel="Persona 详情" items={[{ value: 'overview', label: '概览' }, { value: 'contract', label: '配置契约' }, { value: 'versions', label: '版本历史' }]} onChange={(value) => (detailTab = value as typeof detailTab)} />
      <div class="persona-detail-scroll">
        {#if loading}<div class="persona-detail-empty">
            正在加载版本...
          </div>{:else if detailTab === 'overview'}<section
            class="persona-detail-section"
          >
            <h3>说明</h3>
            <p>{selectedProfile.description || '暂无专家说明。'}</p>
          </section>
          <section class="persona-detail-section">
            <h3>专家能力</h3>
            <div class="persona-detail-chips">
              <span>{profileType(selectedProfile)}</span><span
                >{profileDomain(selectedProfile)}</span
              ><span>{profileCapabilitiesText(selectedProfile)}</span>
            </div>
          </section>
          <section class="persona-detail-section">
            <h3>当前版本</h3>
            <div class="persona-version-summary">
              <strong
                >{selectedVersion
                  ? `v${selectedVersion.version}`
                  : '尚未创建版本'}</strong
              ><span
                >{selectedVersion
                  ? selectedVersion.status === 'published'
                    ? '已发布'
                    : '草稿'
                  : '创建版本后可供 Engine 使用'}</span
              >
            </div>
            <p>
              {selectedVersion
                ? String(selectedVersion.config.instruction || '暂无专家指令。')
                : '请在右下角创建第一个版本。'}
            </p>
          </section>{:else if detailTab === 'contract'}<section
            class="persona-detail-section"
          >
            <h3>模型能力</h3>
            <pre>{JSON.stringify(
                selectedVersion?.config.capabilities ??
                  selectedProfile.config.capabilities ??
                  [],
                null,
                2
              )}</pre>
          </section>
          <section class="persona-detail-section">
            <h3>允许工具</h3>
            <pre>{JSON.stringify(
                selectedVersion?.config.allowed_tools ??
                  selectedProfile.config.allowed_tools ??
                  [],
                null,
                2
              )}</pre>
          </section>{:else}<section class="persona-detail-section">
            <div class="persona-version-list">
              {#each versions as version}<button
                  class:selected={version.id === selectedVersionId}
                  on:click={() => (selectedVersionId = version.id)}
                  ><span
                    ><strong>v{version.version}</strong><small
                      >{version.status === 'published'
                        ? '已发布'
                        : version.status === 'disabled'
                          ? '已停用'
                          : '草稿'} · {formatDate(version.created_at)}</small
                    ></span
                  ><ChevronRight size={14} /></button
                >{:else}<span class="muted-copy">暂无版本历史。</span>{/each}
            </div>
          </section>{/if}
      </div>
      <FormActions className="persona-detail-footer">
        <button
          class="secondary"
          type="button"
          on:click={publishVersion}
          disabled={busy ||
            !selectedVersionId ||
            selectedVersion?.status === 'published'}
          ><ShieldCheck size={14} />发布版本</button
        ><button class="primary" type="button" on:click={startEdit}
          ><Edit3 size={14} />编辑</button
        >
      </FormActions>
      {#if editorOpen}<ModalDialog title="新增专家版本" description={`为“${selectedProfile.name}”创建新的专家版本草稿。`} confirmLabel="创建" confirmForm="persona-version-form" {busy} confirmDisabled={!selectedProfileId} onCancel={() => (editorOpen = false)}>
          <form
            id="persona-version-form"
            class="persona-editor-form"
            on:submit|preventDefault={createVersion}
          >
            <FormField label="专家指令" required><TextArea bind:value={profileInstruction} rows={9} required placeholder="描述诊断范围、判断原则和输出要求" /></FormField>
            <div class="persona-editor-grid">
              <FormField label="模型能力"><TextInput bind:value={profileCapabilities} placeholder="text, tool_calling, stream" ariaLabel="模型能力" /></FormField>{#if profileRole === 'worker'}<FormField label="允许工具"><TextInput bind:value={profileAllowedTools} placeholder="connector_postgresql_inspect" ariaLabel="允许工具" /></FormField>{/if}
            </div>
            <div class="persona-editor-grid">
              <FormField label="专家职责" asLabel={false}><DropdownSelect bind:value={profileRole} options={[{ value: 'worker', label: '资源专家 · Worker' }, { value: 'manager', label: '职能专家 · Manager' }]} ariaLabel="专家职责" /></FormField>
              {#if profileRole === 'worker'}<FormField label="资源领域" required asLabel={false}><TextInput bind:value={profileDomainValue} tagged tagSeparators={[',']} suggestions={domainSuggestions} required placeholder="选择或输入资源领域，回车确认" ariaLabel="资源领域" /></FormField>{:else}<FormField label="可委派资源专家" required asLabel={false}><DropdownSelect bind:value={profileWorkerPersonaIds} options={profiles.filter((profile) => profile.id !== selectedProfileId && profile.status === 'active' && profileType(profile) === 'Worker' && Boolean(profile.config?.domain ?? profile.config?.resource_kind)).map((profile) => ({ value: profile.id, label: `${profile.name}（${profile.identifier}） · ${profileDomain(profile)}` }))} multiple taggedSelection searchable placeholder="选择资源专家" ariaLabel="可委派资源专家" /></FormField>{/if}
            </div>
            <FormField label="专家标签" asLabel={false}><TextInput bind:value={profileTags} tagged tagSeparators={[',']} suggestions={personaTagSuggestions} placeholder="选择或输入标签，回车确认" ariaLabel="专家标签" /></FormField>
            <div class="persona-editor-grid">
              <FormField label="输入 Schema"><TextArea bind:value={profileInputSchema} rows={7} spellcheck={false} /></FormField><FormField label="输出 Schema"><TextArea bind:value={profileOutputSchema} rows={7} spellcheck={false} /></FormField>
            </div>
          </form>
        </ModalDialog>{/if}
    </aside>{/if}
</section>
