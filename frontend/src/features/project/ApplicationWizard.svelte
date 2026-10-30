<script lang="ts">
  import { ArrowRight, Server } from 'lucide-svelte';
  import { onDestroy } from 'svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import FormField from '../../components/FormField.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import ModalDialog from '../../components/ModalDialog.svelte';
  import { formatLabels, parseLabels } from '../../lib/labels';
  import type { Application, Resource, User } from '../../lib/api';
  import { api } from '../../lib/api';
  import {
    resourcesForRuntime,
    runtimeLabel,
    slugify,
    type ApplicationResourceCreationResult,
    type ApplicationDraft,
    type ApplicationInstanceDraft,
    type ApplicationRuntimeKind,
    type RuntimeResourceKind
  } from './projectTypes';

  export let resources: Resource[] = [];
  export let users: User[] = [];
  export let initialApplication: Application | null = null;
  export let onSave: (applications: ApplicationDraft[]) => Promise<void> | void;
  export let onUpdate: (
    application: Application,
    draft: ApplicationDraft
  ) => Promise<void> | void = () => {};
  export let onCancel: () => void;
  export let onResourceCreated: (resource: Resource) => void = () => {};
  export let onRequestResourceCreate: (
    kind: RuntimeResourceKind,
    instanceId?: string
  ) => void = () => {};
  export let resourceCreationResult: ApplicationResourceCreationResult | null =
    null;

  type CandidateInstance = {
    name: string;
    target_resource_id: string;
    selector: Record<string, unknown>;
    log_binding?: Record<string, unknown>;
    status?: string;
  };
  type Candidate = {
    key: string;
    identity: string;
    runtime_kind: string;
    name: string;
    detail?: string;
    kind?: string;
    namespace?: string;
    replicas: number;
    ready_replicas?: number;
    ambiguous?: boolean;
    instances: CandidateInstance[];
  };
  type CandidateDraft = {
    name: string;
    code: string;
    description: string;
    icon: string;
    labels: string;
    critical: boolean;
    technicalOwnerIds: string[];
    operationsOwnerIds: string[];
  };
  const runtimeChoices: Array<{
    kind: ApplicationRuntimeKind;
    label: string;
    subtitle: string;
    description: string;
  }> = [
    { kind: 'virtual_machine', label: 'Host', subtitle: '虚拟机', description: '按进程关键字初筛' },
    { kind: 'containerized', label: 'Docker', subtitle: '容器化', description: '按容器名称初筛' },
    { kind: 'cloud_native', label: 'Kubernetes', subtitle: '云原生', description: '按命名空间和标签筛选' }
  ];

  let step = 0;
  let runtimeKind: ApplicationRuntimeKind = 'virtual_machine';
  let runtimeChosen = false;
  let runtimeResourceIds: string[] = [];
  let processKeyword = '';
  let containerKeyword = '';
  let namespace = '';
  let namespaceOptions: Array<{ value: string; label: string }> = [];
  let namespaceLoading = false;
  let labelSelector = '';
  let discoveredWorkloads: Candidate[] = [];
  let selectedCandidateKeys: string[] = [];
  let candidates: Candidate[] = [];
  let candidateDrafts: Record<string, CandidateDraft> = {};
  let activeCandidateKey = '';
  let loading = false;
  let saving = false;
  let submitAttempted = false;
  let error = '';
  let createdResources: Resource[] = [];
  let handledResourceCreationRequestId = '';
  let editInitialized = false;
  let bindingsCleared = false;
  let namespaceRequestId = 0;
  let discoveryRequestId = 0;
  let workloadRequestId = 0;
  let workloadTimer: ReturnType<typeof setTimeout> | undefined;

  $: availableResources = [...resources, ...createdResources].filter(
    (resource, index, all) => all.findIndex((item) => item.id === resource.id) === index
  );
  $: runtimeResources = resourcesForRuntime(availableResources, runtimeKind);
  $: runtimeResourceOptions = [
    ...runtimeResources.map((resource) => ({ value: resource.id, label: resource.name })),
    { value: '__add_runtime_resource__', label: `新增 ${runtimeLabel(runtimeKind)} 资源` }
  ];
  $: namespaceDropdownOptions = namespaceOptions;
  $: ownerOptions = users.filter((user) => user.status === 'active').map((user) => ({
    value: user.id,
    label: user.display_name || user.username,
    description: user.display_name ? user.username : user.email || user.phone
  }));
  $: activeCandidate = candidates.find((candidate) => candidate.key === activeCandidateKey);
  $: activeDraft = activeCandidate ? candidateDrafts[activeCandidate.key] : undefined;
  $: runtimeLocked = Boolean(
    initialApplication?.instances.length
  ) && !bindingsCleared;

  $: if (initialApplication && !editInitialized) initializeEdit(initialApplication);
  $: if (resourceCreationResult && resourceCreationResult.requestId !== handledResourceCreationRequestId) {
    handledResourceCreationRequestId = resourceCreationResult.requestId;
    handleResourceCreated(resourceCreationResult.resource);
  }

  onDestroy(() => {
    clearTimeout(workloadTimer);
    namespaceRequestId++;
    discoveryRequestId++;
    workloadRequestId++;
  });

  function initializeEdit(application: Application) {
    editInitialized = true;
    runtimeKind = application.runtime_kind as ApplicationRuntimeKind;
    const externalWorkload = runtimeKind === 'cloud_native'
      ? parseExternalWorkloadIdentity(application.external_uid)
      : null;
    runtimeChosen = application.instances.length > 0;
    step = runtimeChosen ? 1 : 0;
    const first = application.instances[0];
    runtimeResourceIds = [...new Set(application.instances.map((instance) => instance.target_resource_id))];
    if (runtimeResourceIds.length === 0 && externalWorkload) runtimeResourceIds = [externalWorkload.resourceId];
    if (runtimeKind === 'virtual_machine') {
      const stored = String(first?.selector.process_command_line ?? '');
      processKeyword = stored.trim().split(/\s+/)[0]?.slice(0, 128) ?? '';
    } else if (runtimeKind === 'containerized') {
      containerKeyword = String(first?.selector.container_name ?? '');
    } else if (runtimeKind === 'cloud_native' && (first || externalWorkload)) {
      namespace = String(first?.selector.namespace ?? externalWorkload?.namespace ?? '');
      void loadNamespaces(first?.target_resource_id ?? externalWorkload!.resourceId, true);
    }
    if (application.instances.length === 0) setUnboundCandidate(application);
    else setEditCandidate(application, externalWorkload);
  }

  function setEditCandidate(application: Application, externalWorkload: ReturnType<typeof parseExternalWorkloadIdentity>) {
    const first = application.instances[0];
    const candidate: Candidate = {
      key: 'edit-application', identity: existingIdentity(application), runtime_kind: runtimeKind,
      name: application.name,
      kind: runtimeKind === 'cloud_native' ? String(first?.selector.workload_kind ?? externalWorkload?.kind ?? '') : undefined,
      namespace: runtimeKind === 'cloud_native' ? String(first?.selector.namespace ?? externalWorkload?.namespace ?? '') : undefined,
      replicas: application.instances.length,
      instances: application.instances.map((instance) => ({
        name: instance.name, target_resource_id: instance.target_resource_id,
        selector: instance.selector, log_binding: instance.log_binding, status: instance.status
      }))
    };
    candidates = [candidate];
    selectedCandidateKeys = [candidate.key];
    activeCandidateKey = candidate.key;
    candidateDrafts = { ...candidateDrafts, [candidate.key]: draftFromApplication(application) };
  }

  function draftFromApplication(application: Application): CandidateDraft {
    return {
      name: application.name,
      code: application.code,
      description: application.description,
      icon: application.icon || 'lucide:AppWindow',
      labels: formatLabels(application.labels),
      critical: application.critical,
      technicalOwnerIds: [...application.technical_owner_ids],
      operationsOwnerIds: [...application.operations_owner_ids]
    };
  }

  function setUnboundCandidate(application: Application) {
    const key = 'unbound-application';
    candidates = [{ key, identity: key, runtime_kind: runtimeKind, name: application.name, replicas: 0, instances: [] }];
    selectedCandidateKeys = [key];
    activeCandidateKey = key;
    candidateDrafts = { ...candidateDrafts, [key]: draftFromApplication(application) };
  }

  function setNewUnboundCandidate() {
    const key = 'unbound-new-application';
    candidates = [{ key, identity: key, runtime_kind: runtimeKind, name: '未命名应用', replicas: 0, instances: [] }];
    selectedCandidateKeys = [key];
    activeCandidateKey = key;
    candidateDrafts = {
      ...candidateDrafts,
      [key]: candidateDrafts[key] ?? {
        name: '',
        code: '',
        description: '',
        icon: 'lucide:AppWindow',
        labels: '',
        critical: false,
        technicalOwnerIds: [],
        operationsOwnerIds: []
      }
    };
  }

  function clearExistingBindings() {
    if (!initialApplication) return;
    clearTimeout(workloadTimer);
    workloadRequestId++;
    namespaceRequestId++;
    discoveryRequestId++;
    loading = false;
    namespaceLoading = false;
    bindingsCleared = true;
    runtimeChosen = false;
    runtimeResourceIds = [];
    namespace = '';
    namespaceOptions = [];
    discoveredWorkloads = [];
    selectedCandidateKeys = [];
    candidates = [];
    activeCandidateKey = '';
    error = '';
    step = 0;
  }

  function selectRuntime(kind: ApplicationRuntimeKind) {
    if (runtimeLocked) return;
    const changed = !runtimeChosen || kind !== runtimeKind;
    runtimeKind = kind;
    runtimeChosen = true;
    error = '';
    if (!changed) return;
    clearTimeout(workloadTimer);
    workloadRequestId++;
    namespaceRequestId++;
    loading = false;
    namespaceLoading = false;
    runtimeResourceIds = [];
    const first = resourcesForRuntime(availableResources, kind)[0];
    if (first) runtimeResourceIds = [first.id];
    processKeyword = '';
    containerKeyword = '';
    namespace = '';
    labelSelector = '';
    namespaceOptions = [];
    selectedCandidateKeys = [];
    candidates = [];
    activeCandidateKey = '';
    if (initialApplication) {
      setUnboundCandidate(initialApplication);
    } else {
      candidateDrafts = {};
    }
    if (kind === 'cloud_native' && first) void loadNamespaces(first.id, false);
  }

  function changeRuntimeResources(value: string | string[]) {
    const selected = Array.isArray(value) ? value : value ? [value] : [];
    if (selected.includes('__add_runtime_resource__')) {
      onRequestResourceCreate(runtimeLabel(runtimeKind) as RuntimeResourceKind);
      return;
    }
    if (initialApplication && selected.length === 0) bindingsCleared = true;
    runtimeResourceIds = selected;
    if (runtimeKind === 'cloud_native') {
      const id = selected[0] ?? '';
      if (id) void loadNamespaces(id, false);
      else {
        clearTimeout(workloadTimer);
        workloadRequestId++;
        namespaceRequestId++;
        loading = false;
        namespaceLoading = false;
        namespaceOptions = [];
        namespace = '';
      }
    }
    error = '';
  }

  function handleResourceCreated(resource: Resource) {
    createdResources = [resource, ...createdResources.filter((item) => item.id !== resource.id)];
    onResourceCreated(resource);
    if (resource.kind !== runtimeLabel(runtimeKind)) return;
    runtimeResourceIds = runtimeKind === 'cloud_native'
      ? [resource.id]
      : [...new Set([...runtimeResourceIds, resource.id])];
    if (runtimeKind === 'cloud_native') void loadNamespaces(resource.id, false);
  }

  async function loadNamespaces(resourceId: string, preserve: boolean) {
    const requestId = ++namespaceRequestId;
    namespaceLoading = true;
    namespaceOptions = [];
    try {
      const result = await api.kubernetesNamespaces(resourceId);
      if (requestId !== namespaceRequestId) return;
      namespaceOptions = result.items.map((item) => ({ value: item.name, label: item.name }));
      const names = namespaceOptions.map((item) => item.value);
      namespace = preserve && names.includes(namespace)
        ? namespace
        : names.includes('default') ? 'default' : names[0] ?? '';
      if (namespace) queueWorkloadDiscovery();
    } catch (cause) {
      if (requestId === namespaceRequestId) error = cause instanceof Error ? cause.message : '读取 Kubernetes 命名空间失败。';
    } finally {
      if (requestId === namespaceRequestId) namespaceLoading = false;
    }
  }

  function queueWorkloadDiscovery() {
    clearTimeout(workloadTimer);
    const requestId = ++workloadRequestId;
    const resourceId = runtimeResourceIds[0] ?? '';
    const selectedNamespace = namespace.trim();
    discoveredWorkloads = [];
    selectedCandidateKeys = [];
    if (!resourceId || !selectedNamespace) return;
    loading = true;
    error = '';
    workloadTimer = setTimeout(() => {
      void discoverKubernetesWorkloads(resourceId, selectedNamespace, labelSelector.trim(), requestId);
    }, 250);
  }

  async function discoverKubernetesWorkloads(resourceId: string, selectedNamespace: string, filters: string, requestId: number) {
    try {
      const result = await api.applicationCandidates({
        runtime_kind: 'cloud_native', resource_ids: [resourceId], namespace: selectedNamespace, filters,
        ...(initialApplication && runtimeLocked ? { exact_identity: existingIdentity(initialApplication) } : {})
      });
      if (requestId !== workloadRequestId) return;
      if (result.truncated) {
        error = '工作负载数量超过读取上限，请缩小筛选范围。';
        return;
      }
      discoveredWorkloads = result.items;
      if (initialApplication && runtimeLocked) {
        const identity = existingIdentity(initialApplication);
        selectedCandidateKeys = result.items.filter((item) => item.identity === identity).map((item) => item.key);
      } else {
        selectedCandidateKeys = result.items.map((item) => item.key);
      }
      ensureDrafts(result.items);
    } catch (cause) {
      if (requestId === workloadRequestId) error = cause instanceof Error ? cause.message : '工作负载读取失败。';
    } finally {
      if (requestId === workloadRequestId) loading = false;
    }
  }

  function kubernetesIdentity(namespaceValue: string, kind: string, name: string) {
    return namespaceValue && kind && name ? `${namespaceValue}/${kind}/${name}` : '';
  }

  function existingIdentity(application: Application) {
    const instance = application.instances[0];
    if (!instance) return parseExternalWorkloadIdentity(application.external_uid)?.identity ?? '';
    if (application.runtime_kind === 'virtual_machine') return String(instance.selector.process_command_line ?? '');
    if (application.runtime_kind === 'containerized') return String(instance.selector.container_name ?? '');
    return kubernetesIdentity(String(instance.selector.namespace ?? ''), String(instance.selector.workload_kind ?? ''), String(instance.selector.workload_name ?? ''));
  }

  function parseExternalWorkloadIdentity(value = '') {
    const separator = value.indexOf(':');
    if (separator <= 0) return null;
    const resourceId = value.slice(0, separator).trim();
    const identity = value.slice(separator + 1).trim();
    const [namespaceValue, kind, name] = identity.split('/');
    if (!resourceId || !namespaceValue || !kind || !name) return null;
    return { resourceId, identity, namespace: namespaceValue, kind };
  }

  function ensureDrafts(items: Candidate[]) {
    const next = { ...candidateDrafts };
    items.forEach((candidate, index) => {
      if (next[candidate.key]) return;
      if (initialApplication && (
        candidate.identity === existingIdentity(initialApplication) || items.length === 1
      )) {
        next[candidate.key] = draftFromApplication(initialApplication);
      } else {
        const suffix = items.length > 1 ? `-${index + 1}` : '';
        const stem = slugify(candidate.name).slice(0, 63 - suffix.length).replace(/-$/, '');
        next[candidate.key] = {
          name: candidate.name,
          code: `${stem}${suffix}`,
          description: '',
          icon: 'lucide:AppWindow',
          labels: '',
          critical: false,
          technicalOwnerIds: [],
          operationsOwnerIds: []
        };
      }
    });
    candidateDrafts = next;
  }

  function updateDraft(key: string, field: keyof CandidateDraft, value: string | boolean | string[]) {
    const current = candidateDrafts[key];
    if (current) candidateDrafts = { ...candidateDrafts, [key]: { ...current, [field]: value } };
  }

  function toggleCandidate(key: string) {
    const selected = new Set(selectedCandidateKeys);
    if (selected.has(key)) selected.delete(key);
    else selected.add(key);
    selectedCandidateKeys = [...selected];
  }

  async function continueFromRuntimeStep() {
    submitAttempted = true;
    error = '';
    if (!runtimeChosen) {
      error = '请先选择运行时类型。';
      return;
    }
    if (runtimeResourceIds.length === 0) {
      if (initialApplication) setUnboundCandidate(initialApplication);
      else setNewUnboundCandidate();
      step = 1;
      return;
    }
    if (runtimeKind === 'virtual_machine' && !runtimeLocked && !processKeyword.trim()) {
      error = '请填写 Host 进程初筛关键字。';
      return;
    }
    if (runtimeKind === 'cloud_native') {
      if (!namespace.trim()) {
        error = '请选择 Kubernetes 命名空间。';
        return;
      }
      if (namespaceLoading || loading) {
        error = '正在读取 Kubernetes 工作负载，请稍候。';
        return;
      }
    }

    // Kubernetes workload discovery already applied the namespace and label
    // filters. Keep only the workloads the user selected instead of querying
    // the full list again and silently replacing that selection.
    if (runtimeKind === 'cloud_native') {
      const selected = discoveredWorkloads.filter((candidate) =>
        selectedCandidateKeys.includes(candidate.key) ||
        (runtimeLocked && candidate.identity === existingIdentity(initialApplication!))
      );
      candidates = selected;
      ensureDrafts(candidates);
      if (candidates.length === 0) {
        if (initialApplication && runtimeLocked) {
          error = '当前筛选条件没有找到已绑定的工作负载。';
          return;
        }
        if (initialApplication) setUnboundCandidate(initialApplication);
        else setNewUnboundCandidate();
      }
      activeCandidateKey = chooseActiveCandidate(candidates);
      step = 1;
      return;
    }

    await discoverRuntimeCandidates();
    if (error) return;
    activeCandidateKey = chooseActiveCandidate(candidates);
    step = 1;
  }

  async function discoverRuntimeCandidates() {
    const requestId = ++discoveryRequestId;
    loading = true;
    candidates = [];
    activeCandidateKey = '';
    try {
      const body = {
        runtime_kind: runtimeKind,
        resource_ids: runtimeResourceIds,
        keyword: runtimeKind === 'virtual_machine' ? processKeyword.trim() : containerKeyword.trim(),
        ...(runtimeKind === 'cloud_native' ? { namespace: namespace.trim(), filters: labelSelector.trim() } : {}),
        ...(initialApplication && runtimeLocked ? { exact_identity: existingIdentity(initialApplication) } : {})
      };
      const result = await api.applicationCandidates(body);
      if (requestId !== discoveryRequestId) return;
      if (result.truncated) {
        error = '匹配结果过多，无法保证完整筛选；请缩小初筛条件后重试。';
        return;
      }
      candidates = result.items;
      selectedCandidateKeys = candidates.map((candidate) => candidate.key);
      ensureDrafts(candidates);
      if (candidates.length === 0) {
        if (initialApplication && runtimeLocked) error = '当前筛选条件没有找到已绑定的运行目标。';
        else if (initialApplication) setUnboundCandidate(initialApplication);
        else setNewUnboundCandidate();
      }
    } catch (cause) {
      if (requestId === discoveryRequestId) error = cause instanceof Error ? cause.message : '应用候选读取失败。';
    } finally {
      if (requestId === discoveryRequestId) loading = false;
    }
  }

  function chooseActiveCandidate(items: Candidate[]) {
    if (initialApplication) {
      const match = items.find((item) => item.identity === existingIdentity(initialApplication!));
      if (match) return match.key;
      if (items.length === 1) return items[0].key;
    }
    return items[0]?.key ?? '';
  }

  function runtimeCandidateDraft(candidate: Candidate): ApplicationDraft {
    const draft = candidateDrafts[candidate.key];
    const instances: ApplicationInstanceDraft[] = candidate.instances.map((instance, index) => ({
      id: crypto.randomUUID(),
      name: instance.name || `instance-${index + 1}`,
      targetResourceId: instance.target_resource_id,
      selector: instance.selector,
      ...(instance.log_binding ? { logBinding: instance.log_binding } : {}),
      ...(instance.status ? { status: instance.status } : {})
    }));
    if (initialApplication) {
      for (const instance of instances) {
        const prior = initialApplication.instances.find((item) =>
          item.target_resource_id === instance.targetResourceId &&
          Object.keys(instance.selector).every((key) => String(item.selector[key] ?? '') === String(instance.selector[key] ?? ''))
        );
        if (prior?.status) instance.status = prior.status;
        if (prior?.log_binding) instance.logBinding = prior.log_binding;
      }
    }
    return {
      name: draft.name.trim(),
      code: draft.code.trim(),
      description: draft.description.trim(),
      icon: draft.icon,
      labels: parseLabels(draft.labels),
      runtimeKind,
      externalUid: runtimeKind === 'cloud_native' && candidate.kind
        ? `${runtimeResourceIds[0]}:${candidate.identity}`
        : '',
      critical: draft.critical,
      technicalOwnerIds: draft.technicalOwnerIds,
      operationsOwnerIds: draft.operationsOwnerIds,
      instances
    };
  }

  function validateDrafts() {
    const selected = initialApplication
      ? candidates.filter((candidate) => candidate.key === activeCandidateKey)
      : candidates.filter((candidate) => selectedCandidateKeys.includes(candidate.key));
    if (selected.length === 0) return '没有可保存的应用候选。';
    if (selected.some((candidate) => !candidateDrafts[candidate.key]?.name.trim() || !candidateDrafts[candidate.key]?.code.trim())) return '请填写应用名称和应用标识。';
    const codes = selected.map((candidate) => candidateDrafts[candidate.key].code.trim().toLowerCase());
    if (new Set(codes).size !== codes.length) return '应用标识不能重复。';
    return '';
  }

  async function save() {
    submitAttempted = true;
    error = validateDrafts();
    if (error) return;
    saving = true;
    try {
      if (initialApplication) {
        const candidate = candidates.find((item) => item.key === activeCandidateKey);
        if (!candidate) throw new Error('没有找到可编辑的应用候选。');
        await onUpdate(initialApplication, runtimeCandidateDraft(candidate));
      } else {
        const selected = candidates.filter((candidate) => selectedCandidateKeys.includes(candidate.key));
        await onSave(selected.map(runtimeCandidateDraft));
      }
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '保存应用失败，请检查表单内容。';
    } finally {
      saving = false;
    }
  }
</script>

<ModalDialog
  dialogClass="project-dialog application-wizard-dialog"
  size="wide"
  overflowMode="expand"
  initialCenterRatio={0.38}
  title={initialApplication ? '编辑应用' : '新增应用'}
  description={initialApplication ? '调整应用配置与运行实例。' : '选择运行时并批量配置匹配到的应用。'}
  confirmLabel={step === 0 ? '下一步' : initialApplication ? '保存' : '创建'}
  busyLabel={initialApplication ? '保存中…' : '创建中…'}
  busy={saving || loading}
  message={error}
  {onCancel}
  onConfirm={step === 0 ? continueFromRuntimeStep : save}
>
  <nav class="application-steps" aria-label="新增应用步骤">
    <button class:active={step === 0} class:done={step > 0} type="button" on:click={() => (step = 0)}><span>1</span>运行时类型</button>
    <button class:active={step === 1} type="button" disabled={!runtimeChosen} on:click={() => runtimeChosen && (step = 1)}><span>2</span>应用与实例</button>
  </nav>

  <div class="project-dialog-content">
    {#if step === 0}
      <div class="runtime-choice-grid">
        {#each runtimeChoices as option}
          <button class:selected={runtimeChosen && runtimeKind === option.kind} class:locked={runtimeLocked} type="button" disabled={runtimeLocked} on:click={() => selectRuntime(option.kind)}>
            <ResourceBrandIcon resource={{ kind: option.label }} size={26} />
            <strong>{option.label}<small class="runtime-choice-kind">{option.subtitle}</small></strong>
            <small>{option.description}</small>
          </button>
        {/each}
      </div>

      {#if runtimeChosen}
        <div class="runtime-binding-fields">
          <div class="runtime-binding-fields-grid">
            {#if runtimeKind !== 'cloud_native'}
              <FormField label="运行资源" required>
                <DropdownSelect value={runtimeResourceIds} options={runtimeResourceOptions} multiple taggedSelection searchable placeholder={`选择 ${runtimeLabel(runtimeKind)} 资源`} ariaLabel="运行资源" invalid={submitAttempted && runtimeResourceIds.length === 0} invalidMode="bubble" invalidMessage="请至少选择一个运行资源" on:change={(event) => changeRuntimeResources(event.detail)} />
              </FormField>
              {#if runtimeKind === 'virtual_machine'}
                <FormField label="初筛关键字" required>
                  <TextInput bind:value={processKeyword} placeholder="多个关键字以逗号分隔，如：java,pay.jar" invalid={submitAttempted && !processKeyword.trim()} invalidMode="bubble" invalidMessage="请填写 Host 进程初筛关键字" />
              </FormField>
            {:else}
                <FormField label="初筛关键字"><TextInput bind:value={containerKeyword} placeholder="可选，如：payments-api" /></FormField>
              {/if}
            {:else}
              <FormField label="运行资源" required>
                <DropdownSelect value={runtimeResourceIds[0] ?? ''} options={runtimeResourceOptions} searchable disabled={runtimeLocked} placeholder="选择 Kubernetes 资源" ariaLabel="Kubernetes 运行资源" invalid={submitAttempted && runtimeResourceIds.length !== 1} invalidMode="bubble" invalidMessage="请选择一个 Kubernetes 运行资源" on:change={(event) => changeRuntimeResources(String(event.detail))} />
              </FormField>
              <FormField label="命名空间" required>
                <DropdownSelect value={namespace} options={namespaceDropdownOptions} searchable disabled={namespaceLoading || !runtimeResourceIds[0] || runtimeLocked} placeholder={namespaceLoading ? '读取命名空间…' : '选择命名空间'} ariaLabel="Kubernetes 命名空间" invalid={submitAttempted && !namespace} invalidMode="bubble" invalidMessage="请选择 Kubernetes 命名空间" on:change={(event) => { namespace = String(event.detail); queueWorkloadDiscovery(); }} />
              </FormField>
              <FormField label="标签选择器">
                <TextInput bind:value={labelSelector} disabled={runtimeLocked} placeholder="可选，如：app.kubernetes.io/part-of=payments" on:value={queueWorkloadDiscovery} />
              </FormField>
            {/if}
          </div>
          <button class="runtime-continue-button" type="button" aria-label="进入应用与实例配置" data-tooltip="进入应用与实例配置" on:click={continueFromRuntimeStep}><ArrowRight size={18} /></button>
        </div>
      {/if}
    {:else if candidates.length > 0}
      <div class="kubernetes-batch-toolbar">
        <div><strong>应用与实例配置</strong><small>左侧选择应用，右侧设置应用信息；每个副本对应一个运行实例。</small></div>
        <span class="kubernetes-discovery-status">{loading ? '正在读取…' : `${candidates.length} 个应用候选`}</span>
      </div>
      <div class="kubernetes-batch-layout">
        <aside class="application-candidate-pane" aria-label="应用列表">
          <div class="kubernetes-workload-pane-heading"><strong>应用</strong><span>{candidates.length}</span></div>
          <div class="application-candidate-list">
            {#each candidates as candidate}
              <button class="application-candidate" class:active={activeCandidateKey === candidate.key} type="button" on:click={() => (activeCandidateKey = candidate.key)}>
                {#if runtimeKind === 'cloud_native' && !initialApplication}
                  <input
                    class="application-candidate-check"
                    type="checkbox"
                    checked={selectedCandidateKeys.includes(candidate.key)}
                    aria-label={`选择 ${candidate.name}`}
                    on:click|stopPropagation
                    on:change={() => toggleCandidate(candidate.key)}
                  />
                {/if}
                <EntityBrandIcon kind="Application" fallback="lucide:AppWindow" size={17} />
                <span><strong>{candidate.name}</strong><small>{candidate.kind ? `${candidate.kind} · ` : ''}{candidate.replicas} 个副本</small></span>
              </button>
            {/each}
          </div>
          {#if initialApplication && !bindingsCleared}
            <button class="text-button application-clear-bindings" type="button" on:click={clearExistingBindings}>清除所有实例绑定</button>
          {/if}
        </aside>
        <section class="kubernetes-application-pane" aria-label="应用配置">
          {#if activeCandidate && activeDraft}
            <div class="kubernetes-application-heading">
              <div><h3>{activeDraft.name || activeCandidate.name}</h3><p>{runtimeLabel(runtimeKind)} · {activeCandidate.replicas} 个副本{activeCandidate.namespace ? ` · ${activeCandidate.namespace}` : ''}</p></div>
              <span class="kubernetes-inclusion-state included">{initialApplication ? '编辑中' : '待新增'}</span>
            </div>
            <div class="application-form-grid">
              <div class="application-identity-row">
                <div class="application-icon-code-fields">
                  <FormField label="应用图标"><IconPicker value={activeDraft.icon} onSelect={(value) => updateDraft(activeCandidate.key, 'icon', value)} ariaLabel="选择应用图标" /></FormField>
                  <FormField label="关键应用" asLabel={false}><DropdownSelect value={String(activeDraft.critical)} options={[{ value: 'false', label: '否' }, { value: 'true', label: '是' }]} ariaLabel="是否为关键应用" on:change={(event) => updateDraft(activeCandidate.key, 'critical', event.detail === 'true')} /></FormField>
                  <FormField label="应用标识" required><TextInput value={activeDraft.code} placeholder="payments-api" invalid={submitAttempted && !activeDraft.code.trim()} invalidMode="bubble" invalidMessage="请填写应用标识" on:value={(event) => updateDraft(activeCandidate.key, 'code', String(event.detail))} /></FormField>
                </div>
                <FormField label="应用名称" required className="application-name-field"><TextInput value={activeDraft.name} placeholder="支付 API" invalid={submitAttempted && !activeDraft.name.trim()} invalidMode="bubble" invalidMessage="请填写应用名称" on:value={(event) => updateDraft(activeCandidate.key, 'name', String(event.detail))} /></FormField>
              </div>
              <div class="owner-fields-row">
                <FormField label="技术负责人" asLabel={false}><DropdownSelect options={ownerOptions} value={activeDraft.technicalOwnerIds} multiple taggedSelection reorderableSelection searchable placeholder="选择负责人(主备排序)" ariaLabel="选择技术负责人" on:change={(event) => updateDraft(activeCandidate.key, 'technicalOwnerIds', Array.isArray(event.detail) ? event.detail : [])} /></FormField>
                <FormField label="运维负责人" asLabel={false}><DropdownSelect options={ownerOptions} value={activeDraft.operationsOwnerIds} multiple taggedSelection reorderableSelection searchable placeholder="选择负责人(主备排序)" ariaLabel="选择运维负责人" on:change={(event) => updateDraft(activeCandidate.key, 'operationsOwnerIds', Array.isArray(event.detail) ? event.detail : [])} /></FormField>
              </div>
              <FormField label="应用标签" className="full-field" asLabel={false}><TextInput value={activeDraft.labels} tagged placeholder="填写 key=value、key:value 或 key，按回车确认" autocomplete="off" on:value={(event) => updateDraft(activeCandidate.key, 'labels', String(event.detail))} /></FormField>
              <FormField label="应用描述" className="full-field"><TextArea value={activeDraft.description} rows={2} placeholder="应用职责和运行边界" on:input={(event) => updateDraft(activeCandidate.key, 'description', (event.currentTarget as HTMLTextAreaElement).value)} /></FormField>
            </div>
          {:else}
            <div class="kubernetes-application-empty"><Server size={18} /><span>选择左侧应用候选，配置其名称、负责人、标签与描述。</span></div>
          {/if}
        </section>
      </div>
    {:else}
      <div class="kubernetes-batch-empty"><Server size={18} /><span>{loading ? '正在读取运行目标…' : initialApplication ? '当前筛选没有找到可编辑的唯一运行目标，请返回调整条件。' : '没有在所有选定运行资源中匹配到共同目标，请返回调整初筛条件。'}</span></div>
    {/if}
  </div>
</ModalDialog>
