<script lang="ts">
  import { onDestroy } from 'svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import FormField from '../../components/FormField.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import { api, type Application, type Resource, type User } from '../../lib/api';
  import { parseLabels } from '../../lib/labels';
  import {
    resourcesForRuntime,
    runtimeLabel,
    slugify,
    type ApplicationDraft,
    type ApplicationRuntimeKind
  } from './projectTypes';

  export let resources: Resource[] = [];
  export let users: User[] = [];
  export let runtimeKind: ApplicationRuntimeKind;
  export let initialApplication: Application | null = null;
  export let onSave: (draft: ApplicationDraft) => Promise<void> | void;
  export let onCancel: () => void;

  type CandidateInstance = {
    id?: string;
    name: string;
    target_resource_id: string;
    selector: Record<string, unknown>;
    log_binding?: Record<string, unknown>;
    status?: string;
  };
  type Candidate = {
    key: string;
    identity: string;
    name: string;
    kind?: string;
    namespace?: string;
    replicas: number;
    detail?: string;
    tooltip?: string;
    instances: CandidateInstance[];
  };

  let runtimeResourceIds: string[] = [];
  let keyword = '';
  let namespace = '';
  let namespaces: Array<{ value: string; label: string }> = [];
  let namespaceLoading = false;
  let labelSelector = '';
  let candidates: Candidate[] = [];
  let selectedCandidateKey = '';
  let loading = false;
  let saving = false;
  let attempted = false;
  let error = '';
  let namespaceRequestId = 0;
  let discoveryRequestId = 0;
  let discoveryTimer: ReturnType<typeof setTimeout> | undefined;

  let name = '';
  let code = '';
  let icon = 'lucide:AppWindow';
  let critical = false;
  let technicalOwnerIds: string[] = [];
  let operationsOwnerIds: string[] = [];
  let labels = '';
  let description = '';
  let logPath = '';

  $: runtimeResources = resourcesForRuntime(resources, runtimeKind);
  $: runtimeOptions = runtimeResources.map((resource) => ({ value: resource.id, label: resource.name }));
  $: namespaceOptions = namespaces;
  $: candidateOptions = candidates.map((candidate) => ({
    value: candidate.key,
    label: candidate.name,
    meta: `${candidate.replicas} 个副本`,
    description: candidateDescription(candidate),
    tooltip: candidate.tooltip
  }));
  $: ownerOptions = users
    .filter((user) => user.status === 'active')
    .map((user) => ({
      value: user.id,
      label: user.display_name || user.username,
      description: user.display_name ? user.username : user.email || user.phone
    }));
  $: selectedCandidate = candidates.find((candidate) => candidate.key === selectedCandidateKey);
  $: isKubernetes = runtimeKind === 'cloud_native';
  $: runtimeLabelText = runtimeLabel(runtimeKind);

  $: if (initialApplication && !selectedCandidateKey) {
    runtimeResourceIds = [...new Set(initialApplication.instances.map((instance) => instance.target_resource_id))];
    name = initialApplication.name;
    code = initialApplication.code;
    icon = initialApplication.icon || 'lucide:AppWindow';
    critical = initialApplication.critical;
    technicalOwnerIds = [...initialApplication.technical_owner_ids];
    operationsOwnerIds = [...initialApplication.operations_owner_ids];
    labels = Object.entries(initialApplication.labels ?? {}).map(([key, value]) => `${key}=${value}`).join(', ');
    description = initialApplication.description;
    logPath = String(initialApplication.instances[0]?.log_binding?.path ?? '');
    selectedCandidateKey = `existing:${initialApplication.id}`;
    candidates = [{
      key: selectedCandidateKey,
      identity: initialApplication.external_uid ?? initialApplication.id,
      name: initialApplication.name,
      kind: runtimeKind === 'cloud_native' ? 'Workload' : undefined,
      replicas: initialApplication.instances.length,
      instances: initialApplication.instances.map((instance) => ({
        id: instance.id,
        name: instance.name,
        target_resource_id: instance.target_resource_id,
        selector: instance.selector,
        log_binding: instance.log_binding,
        status: instance.status
      }))
    }];
  }

  onDestroy(() => {
    namespaceRequestId++;
    discoveryRequestId++;
    clearTimeout(discoveryTimer);
  });

  function changeRuntimeResources(value: string | string[]) {
    runtimeResourceIds = Array.isArray(value) ? value : value ? [value] : [];
    clearCandidate();
    if (isKubernetes) {
      namespaceRequestId++;
      namespace = '';
      namespaces = [];
      const resourceId = runtimeResourceIds[0];
      if (resourceId) void loadNamespaces(resourceId);
    }
    queueDiscovery();
  }

  async function loadNamespaces(resourceId: string) {
    const currentRequest = ++namespaceRequestId;
    namespaceLoading = true;
    try {
      const result = await api.kubernetesNamespaces(resourceId);
      if (currentRequest !== namespaceRequestId) return;
      namespaces = result.items.map((item) => ({ value: item.name, label: item.name }));
    } catch (cause) {
      if (currentRequest === namespaceRequestId) error = cause instanceof Error ? cause.message : '读取 Kubernetes 命名空间失败。';
    } finally {
      if (currentRequest === namespaceRequestId) namespaceLoading = false;
    }
  }

  function selectCandidate(candidate: Candidate) {
    selectedCandidateKey = candidate.key;
    name = candidate.name;
    code = slugify(candidate.name);
    icon = 'lucide:AppWindow';
    critical = false;
    technicalOwnerIds = [];
    operationsOwnerIds = [];
    labels = '';
    description = '';
    logPath = String(candidate.instances[0]?.log_binding?.path ?? '');
  }

  function clearCandidate() {
    candidates = [];
    selectedCandidateKey = '';
    name = '';
    code = '';
    icon = 'lucide:AppWindow';
    critical = false;
    technicalOwnerIds = [];
    operationsOwnerIds = [];
    labels = '';
    description = '';
    logPath = '';
  }

  function canDiscoverCandidates() {
    return runtimeResourceIds.length > 0 &&
      (runtimeKind !== 'virtual_machine' || Boolean(keyword.trim())) &&
      (!isKubernetes || Boolean(namespace.trim()));
  }

  function queueDiscovery() {
    clearTimeout(discoveryTimer);
    discoveryRequestId++;
    clearCandidate();
    loading = false;
    error = '';
    if (!canDiscoverCandidates()) return;
    discoveryTimer = setTimeout(() => void discoverCandidates(), 250);
  }

  async function discoverCandidates() {
    if (!canDiscoverCandidates()) return;
    const currentRequest = ++discoveryRequestId;
    loading = true;
    try {
      const result = await api.applicationCandidates({
        runtime_kind: runtimeKind,
        resource_ids: runtimeResourceIds,
        ...(isKubernetes
          ? { namespace: namespace.trim(), filters: labelSelector.trim() }
          : { keyword: keyword.trim() })
      });
      if (currentRequest !== discoveryRequestId) return;
      if (result.truncated) {
        error = '匹配对象过多，请缩小初筛范围后重试。';
        return;
      }
      candidates = result.items;
    } catch (cause) {
      if (currentRequest === discoveryRequestId) error = cause instanceof Error ? cause.message : '读取应用候选失败。';
    } finally {
      if (currentRequest === discoveryRequestId) loading = false;
    }
  }

  function candidateDescription(candidate: Candidate) {
    if (runtimeKind === 'cloud_native') {
      return candidate.detail || candidate.kind || '工作负载';
    }
    return candidate.detail || `${candidate.instances.length} 个运行资源`;
  }

  async function save() {
    attempted = true;
    error = '';
    const candidate = selectedCandidate;
    if (!candidate) {
      error = '请选择一个应用候选。';
      return;
    }
    if (!name.trim() || !code.trim()) {
      error = '请填写应用名称和应用标识。';
      return;
    }
    saving = true;
    try {
      await onSave({
        name: name.trim(),
        code: code.trim(),
        description: description.trim(),
        icon,
        labels: parseLabels(labels),
        runtimeKind,
        externalUid: initialApplication?.external_uid ?? (isKubernetes && candidate.kind
          ? `${runtimeResourceIds[0]}:${candidate.identity}`
          : ''),
        critical,
        technicalOwnerIds,
        operationsOwnerIds,
        dependencies: initialApplication?.dependencies.map((dependency) => ({
          dependency_kind: dependency.dependency_kind,
          target_resource_id: dependency.target_resource_id,
          binding: Object.fromEntries(Object.entries(dependency.binding ?? {}).map(([key, value]) => [key, String(value)])),
          required: dependency.required
        })) ?? [],
        relations: initialApplication?.relations.map((relation) => ({
          target_application_id: relation.target_application_id,
          relation_kind: relation.relation_kind as 'depends_on' | 'depended_by' | 'mutual',
          status: relation.status
        })) ?? [],
        instances: candidate.instances.map((instance, index) => ({
          id: instance.id ?? crypto.randomUUID(),
          ...(initialApplication?.instances.some((saved) => saved.id === instance.id) ? { persistedId: instance.id } : {}),
          name: instance.name || `instance-${index + 1}`,
          targetResourceId: instance.target_resource_id,
          selector: instance.selector,
          logBinding: {
            ...(instance.log_binding ?? {}),
            path: logPath.trim() || String(instance.log_binding?.path ?? (runtimeKind === 'virtual_machine' ? '' : 'stdout'))
          },
          ...(instance.status ? { status: instance.status } : {})
        }))
      });
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '新增应用失败，请检查表单内容。';
    } finally {
      saving = false;
    }
  }
</script>

<section class="topology-application-create panel" aria-label={initialApplication ? '编辑应用' : '新增应用'}>
  <header class="topology-application-create-heading">
    <div><h2>{initialApplication ? '编辑应用' : '新增应用'}</h2><p>{initialApplication ? '更新应用信息及其运行实例。' : '筛选一个运行目标并补充应用基础信息。'}</p></div>
    <div class="topology-application-create-actions">
      <button class="secondary" type="button" disabled={saving} on:click={onCancel}>取消</button>
      <button class="primary" type="button" disabled={saving} on:click={save}>{saving ? (initialApplication ? '保存中…' : '创建中…') : (initialApplication ? '保存' : '创建')}</button>
    </div>
  </header>
  {#if error}<div class="topology-application-error" role="alert">{error}</div>{/if}
  <div class="topology-application-create-content">
    <section class="topology-application-runtime">
      <div class="topology-application-section-heading">
        <span><ResourceBrandIcon resource={{ kind: runtimeLabelText }} size={16} /></span>
        <div><strong>{runtimeLabelText} 运行时</strong><small>{runtimeKind === 'virtual_machine' ? '按进程关键字筛选' : runtimeKind === 'containerized' ? '按容器名称筛选' : '按命名空间和标签筛选工作负载'}</small></div>
      </div>
      <div class="topology-application-runtime-fields" class:kubernetes-runtime-fields={isKubernetes}>
        {#if isKubernetes}
          <FormField label="运行资源" required>
            <DropdownSelect value={runtimeResourceIds[0] ?? ''} options={runtimeOptions} searchable placeholder="选择 Kubernetes 资源" ariaLabel="Kubernetes 运行资源" invalid={attempted && runtimeResourceIds.length !== 1} invalidMode="bubble" invalidMessage="请选择一个 Kubernetes 运行资源" on:change={(event) => changeRuntimeResources(String(event.detail))} />
          </FormField>
          <FormField label="命名空间" required>
            <DropdownSelect value={namespace} options={namespaceOptions} searchable disabled={namespaceLoading || !runtimeResourceIds[0]} placeholder={namespaceLoading ? '读取命名空间…' : '选择命名空间'} ariaLabel="Kubernetes 命名空间" invalid={attempted && !namespace} invalidMode="bubble" invalidMessage="请选择 Kubernetes 命名空间" on:change={(event) => { namespace = String(event.detail); queueDiscovery(); }} />
          </FormField>
          <FormField label="标签选择器">
          <TextInput bind:value={labelSelector} placeholder="可选，如：app.kubernetes.io/part-of=payments" on:input={queueDiscovery} />
          </FormField>
        {:else}
          <FormField label="运行资源" required>
            <DropdownSelect value={runtimeResourceIds} options={runtimeOptions} multiple taggedSelection searchable placeholder={`选择 ${runtimeLabelText} 资源`} ariaLabel="运行资源" invalid={attempted && runtimeResourceIds.length === 0} invalidMode="bubble" invalidMessage="请至少选择一个运行资源" on:change={(event) => changeRuntimeResources(event.detail)} />
          </FormField>
          <FormField label="初筛关键字" required={runtimeKind === 'virtual_machine'}>
            <TextInput bind:value={keyword} placeholder={runtimeKind === 'virtual_machine' ? '多个关键字以逗号分隔，如：java,pay.jar' : '可选，如：payments-api'} invalid={attempted && runtimeKind === 'virtual_machine' && !keyword.trim()} invalidMode="bubble" invalidMessage="请填写 Host 进程初筛关键字" on:input={queueDiscovery} />
          </FormField>
        {/if}
        <FormField label="匹配应用" required>
          <DropdownSelect value={selectedCandidateKey} options={candidateOptions} searchable disabled={loading || !candidateOptions.length} placeholder={loading ? '正在匹配…' : canDiscoverCandidates() ? '选择匹配应用' : '请先完成筛选条件'} ariaLabel="匹配应用" invalid={attempted && !selectedCandidateKey} invalidMode="bubble" invalidMessage="请选择一个应用候选" on:change={(event) => {
            const candidate = candidates.find((item) => item.key === String(event.detail));
            if (candidate) selectCandidate(candidate);
          }} />
        </FormField>
      </div>
    </section>

    {#if selectedCandidate}
      <section class="topology-application-details">
        <div class="topology-application-section-heading"><span><IconPicker value={icon} onSelect={(value) => (icon = value)} ariaLabel="选择应用图标" /></span><div><strong>应用信息</strong><small>{initialApplication ? '编辑应用信息及关联运行目标。' : '将创建并关联到选定的运行目标。'}</small></div></div>
        <div class="topology-application-form">
          <div class="topology-application-identity">
            <div class="topology-application-identity-half">
              <FormField label="应用标识" required><TextInput bind:value={code} placeholder="payments-api" invalid={attempted && !code.trim()} invalidMode="bubble" invalidMessage="请填写应用标识" /></FormField>
              <FormField label="应用名称" required><TextInput bind:value={name} placeholder="支付 API" invalid={attempted && !name.trim()} invalidMode="bubble" invalidMessage="请填写应用名称" /></FormField>
            </div>
            <div class="application-identity-half application-labels-half">
              <FormField label="应用标签" asLabel={false}><TextInput bind:value={labels} tagged placeholder="填写 key=value，按回车确认" autocomplete="off" /></FormField>
              <div class="critical-radio-field"><span class="form-field-label">关键</span><div class="critical-radio-group"><button class:checked={critical} class="critical-radio-toggle" type="button" role="radio" aria-label="关键" aria-checked={critical} on:click={() => (critical = !critical)}></button></div></div>
            </div>
          </div>
          <FormField label="技术负责人" asLabel={false}><DropdownSelect options={ownerOptions} value={technicalOwnerIds} multiple taggedSelection reorderableSelection searchable placeholder="选择负责人(主备排序)" ariaLabel="选择技术负责人" on:change={(event) => (technicalOwnerIds = Array.isArray(event.detail) ? event.detail : [])} /></FormField>
          <FormField label="运维负责人" asLabel={false}><DropdownSelect options={ownerOptions} value={operationsOwnerIds} multiple taggedSelection reorderableSelection searchable placeholder="选择负责人(主备排序)" ariaLabel="选择运维负责人" on:change={(event) => (operationsOwnerIds = Array.isArray(event.detail) ? event.detail : [])} /></FormField>
          <FormField label="应用日志配置" className="topology-application-full-field"><TextInput bind:value={logPath} placeholder={runtimeKind === 'virtual_machine' ? 'Host 日志路径（可选）' : 'stdout'} /></FormField>
          <FormField label="应用描述" asLabel={false} className="topology-application-full-field"><TextArea bind:value={description} rows={3} placeholder="应用职责和运行边界" /></FormField>
        </div>
      </section>
    {/if}
  </div>
</section>

<style>
  .topology-application-create { position:relative; display:grid; align-content:start; gap:14px; min-width:0; padding:0; overflow:visible; }
  .topology-application-create-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; min-height:60px; padding:10px 14px; border-bottom:1px solid var(--theme-divider); }
  .topology-application-create-heading h2 { margin:0; color:var(--theme-fg-strong); font-size:15px; }
  .topology-application-create-heading p { margin:4px 0 0; color:var(--theme-fg-muted); font-size:11px; }
  .topology-application-create-actions { display:flex; flex:none; gap:8px; }
  .topology-application-error { margin:0 14px; padding:8px 10px; color:var(--theme-danger); background:var(--theme-bg-danger-soft); border:1px solid var(--theme-danger); border-radius:4px; font-size:12px; }
  .topology-application-create-content { display:grid; gap:16px; padding:0 14px 14px; }
  .topology-application-runtime,
  .topology-application-details { display:grid; gap:12px; padding-bottom:16px; border-bottom:1px solid var(--theme-divider); }
  .topology-application-details { padding-bottom:0; border-bottom:0; }
  .topology-application-section-heading { display:flex; align-items:center; gap:8px; }.topology-application-section-heading > span { display:grid; place-items:center; color:var(--theme-accent); }.topology-application-section-heading > div { display:grid; gap:2px; }.topology-application-section-heading strong { color:var(--theme-fg-strong); font-size:13px; }.topology-application-section-heading small { color:var(--theme-fg-muted); font-size:10px; }
  .topology-application-runtime .topology-application-section-heading > span { color:var(--theme-application); }
  .topology-application-runtime-fields { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:12px; }.topology-application-runtime-fields.kubernetes-runtime-fields { grid-template-columns:repeat(4,minmax(0,1fr)); }.topology-application-form { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:12px; }.topology-application-form > :global(.topology-application-full-field) { grid-column:1/-1; }
  .topology-application-identity { display:grid; grid-column:1/-1; grid-template-columns:repeat(2,minmax(0,1fr)); align-items:start; gap:12px; }
  .topology-application-identity-half { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); align-items:end; gap:12px; min-width:0; }
  .topology-application-identity > :global(.form-field) { min-width:0; }
  @media (max-width: 700px) { .topology-application-runtime-fields,.topology-application-runtime-fields.kubernetes-runtime-fields,.topology-application-form,.topology-application-identity,.application-identity-half { grid-template-columns:1fr; } }
</style>
