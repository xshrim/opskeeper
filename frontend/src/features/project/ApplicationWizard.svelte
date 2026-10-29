<script lang="ts">
  import {
    CircleCheck,
    CircleHelp,
    CircleX,
    Plus,
    ScanSearch,
    Server,
    TriangleAlert,
    X
  } from 'lucide-svelte';
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

  type Workload = {
    namespace: string;
    name: string;
    kind: string;
    ready: boolean;
    phase?: string;
    restarts?: number;
    pods: Array<{ namespace: string; name: string; containers?: string[] }>;
  };
  type KubernetesApplicationDraft = {
    name: string;
    code: string;
    description: string;
    labels: string;
    icon: string;
    critical: boolean;
    technicalOwnerIds: string[];
    operationsOwnerIds: string[];
  };
  type BindingPreview = {
    resourceId: string;
    keyword: string;
    count: number;
    items: Array<{
      id: string;
      name: string;
      detail?: string;
      status?: string;
    }>;
    error?: string;
    loading?: boolean;
  };
  type BindingPreviewStatus = 'unknown' | 'success' | 'multiple' | 'none';

  let step = 0;
  let runtimeKind: ApplicationRuntimeKind = 'virtual_machine';
  let runtimeChosen = false;
  let manualKubernetesApplication = false;
  let name = '';
  let code = '';
  let nameTouched = false;
  let description = '';
  let labels = '';
  let icon = 'lucide:AppWindow';
  let critical = false;
  let technicalOwnerIds: string[] = [];
  let operationsOwnerIds: string[] = [];
  let instances: ApplicationInstanceDraft[] = [];
  let bindingPreviews: Record<string, BindingPreview> = {};
  let expandedBindingPreviews: Record<string, boolean> = {};
  let kubernetesResourceId = '';
  let namespace = 'default';
  let filters = '';
  let workloads: Workload[] = [];
  let selectedWorkloads = new Set<string>();
  let activeWorkloadKey = '';
  let kubernetesApplicationDrafts: Record<string, KubernetesApplicationDraft> =
    {};
  let loading = false;
  let workloadDiscoveryTimer: ReturnType<typeof setTimeout> | undefined;
  let workloadDiscoveryRequestId = 0;
  let saving = false;
  let reconfirming = false;
  let submitAttempted = false;
  let error = '';
  let createdResources: Resource[] = [];
  let handledResourceCreationRequestId = '';
  let editInitialized = false;

  $: availableResources = [...resources, ...createdResources].filter(
    (resource, index, all) =>
      all.findIndex((item) => item.id === resource.id) === index
  );
  $: if (initialApplication && !editInitialized) {
    runtimeKind = initialApplication.runtime_kind as ApplicationRuntimeKind;
    name = initialApplication.name;
    code = initialApplication.code;
    nameTouched = true;
    description = initialApplication.description;
    labels = formatLabels(initialApplication.labels);
    icon = initialApplication.icon || 'lucide:AppWindow';
    critical = initialApplication.critical;
    technicalOwnerIds = [...initialApplication.technical_owner_ids];
    operationsOwnerIds = [...initialApplication.operations_owner_ids];
    instances = initialApplication.instances.map((instance) => ({
      id: instance.id,
      name: instance.name,
      targetResourceId: instance.target_resource_id,
      selector: instance.selector,
      status: instance.status
    }));
    runtimeChosen = instances.length > 0;
    manualKubernetesApplication = true;
    step = instances.length > 0 ? 1 : 0;
    editInitialized = true;
  }
  $: runtimeResources = resourcesForRuntime(availableResources, runtimeKind);
  $: ownerOptions = users
    .filter((user) => user.status === 'active')
    .map((user) => ({
      value: user.id,
      label: user.display_name || user.username,
      description: user.display_name ? user.username : user.email || user.phone
    }));
  $: runtimeResourceOptions = [
    ...runtimeResources.map((resource) => ({
      value: resource.id,
      label: resource.name
    })),
    {
      value: '__add_runtime_resource__',
      label: `新增 ${runtimeLabel(runtimeKind)} 资源`
    }
  ];
  $: if (
    resourceCreationResult &&
    resourceCreationResult.requestId !== handledResourceCreationRequestId
  ) {
    handledResourceCreationRequestId = resourceCreationResult.requestId;
    handleResourceCreated(
      resourceCreationResult.resource,
      resourceCreationResult.instanceId
    );
  }
  $: if (runtimeKind === 'cloud_native' && !kubernetesResourceId) {
    kubernetesResourceId = runtimeResources[0]?.id ?? '';
    if (kubernetesResourceId && step === 1 && !manualKubernetesApplication) {
      queueWorkloadDiscovery();
    }
  }
  function newInstance(kind = runtimeKind): ApplicationInstanceDraft {
    return {
      id: crypto.randomUUID(),
      name: `instance-${instances.length + 1}`,
      targetResourceId:
        resourcesForRuntime(availableResources, kind)[0]?.id ?? '',
      selector: {}
    };
  }

  function applicationCreateBlockReason() {
    const kubernetesBatch =
      runtimeKind === 'cloud_native' &&
      !initialApplication &&
      !manualKubernetesApplication;
    if (!kubernetesBatch && (!name.trim() || !code.trim()))
      return '请填写应用名称和应用标识。';
    if (!runtimeChosen) return '请先选择运行时类型。';
    if (
      runtimeKind === 'cloud_native' &&
      !initialApplication &&
      !manualKubernetesApplication
    ) {
      const selected = workloads.filter((workload) =>
        selectedWorkloads.has(workloadKey(workload))
      );
      if (!selected.length) return '请至少选择一个 Kubernetes 工作负载。';
      const drafts = selected.map(
        (workload) => kubernetesApplicationDrafts[workloadKey(workload)]
      );
      if (drafts.some((draft) => !draft?.name.trim() || !draft.code.trim())) {
        return '请填写所有已选工作负载对应的应用名称和应用标识。';
      }
      const codes = drafts.map((draft) =>
        draft.code.trim().toLocaleLowerCase()
      );
      if (new Set(codes).size !== codes.length) return '应用标识不能重复。';
      return '';
    }
    for (const instance of instances) {
      if (!instance.name.trim()) return '请填写所有实例名称。';
      if (!instance.targetResourceId) return '请为所有实例选择运行资源。';
      if (runtimeKind === 'cloud_native') {
        if (!validSelector('cloud_native', instance.selector))
          return '请填写 Kubernetes 实例的命名空间、工作负载类型和名称。';
      } else {
        const selectorKey =
          runtimeKind === 'virtual_machine'
            ? 'process_keyword'
            : 'container_name';
        const selectorLabel =
          runtimeKind === 'virtual_machine' ? '进程关键字' : '容器名称';
        const keyword = String(instance.selector[selectorKey] ?? '').trim();
        if (!keyword) return '请填写所有实例的' + selectorLabel + '。';
        if (!initialApplication) {
          const preview = bindingPreviews[instance.id];
          if (
            !preview ||
            preview.resourceId !== instance.targetResourceId ||
            preview.keyword !== keyword
          )
            return '请重新预览所有实例的匹配结果。';
          if (preview.loading) return '正在匹配实例，请稍候。';
          if (preview.error) return '存在匹配失败的实例，请检查后重新预览。';
          if (preview.count !== 1) return '每个实例都必须恰好匹配一个对象。';
        }
      }
    }
    return '';
  }

  function instanceBindingIssue(instance: ApplicationInstanceDraft) {
    if (!instance.name.trim()) return '请填写实例名称';
    if (!instance.targetResourceId) return '请选择运行资源';
    if (runtimeKind === 'cloud_native')
      return validSelector(runtimeKind, instance.selector)
        ? ''
        : '请填写 Kubernetes 命名空间、工作负载类型和名称';
    const selectorKey =
      runtimeKind === 'virtual_machine' ? 'process_keyword' : 'container_name';
    const keyword = String(instance.selector[selectorKey] ?? '').trim();
    if (!keyword)
      return runtimeKind === 'virtual_machine'
        ? '请填写进程关键字'
        : '请填写容器名称';
    const preview = bindingPreviews[instance.id];
    if (
      !preview ||
      preview.resourceId !== instance.targetResourceId ||
      preview.keyword !== keyword
    ) {
      return '请预览匹配结果';
    }
    if (preview.loading) return '正在匹配，请稍候';
    if (preview.error) return '匹配失败，请检查条件后重试';
    if (preview.count !== 1) return '必须恰好匹配一个对象';
    return '';
  }

  function validSelector(
    kind: ApplicationRuntimeKind,
    selector: Record<string, unknown>
  ) {
    const required =
      kind === 'virtual_machine'
        ? ['process_keyword']
        : kind === 'containerized'
          ? ['container_name']
          : ['namespace', 'workload_kind', 'workload_name'];
    return required.every((key) => String(selector[key] ?? '').trim());
  }

  function changeRuntime(next: ApplicationRuntimeKind) {
    runtimeKind = next;
    runtimeChosen = true;
    step = 1;
    error = '';
    instances = [];
    manualKubernetesApplication = Boolean(initialApplication);
    bindingPreviews = {};
    expandedBindingPreviews = {};
    if (next !== 'cloud_native') {
      kubernetesResourceId = '';
      clearTimeout(workloadDiscoveryTimer);
      workloadDiscoveryRequestId += 1;
      loading = false;
    } else {
      kubernetesResourceId =
        resourcesForRuntime(availableResources, next)[0]?.id ?? '';
      if (!initialApplication) queueWorkloadDiscovery();
    }
  }

  function syncNameWithIdentifier() {
    if (!nameTouched) name = code;
  }

  function markNameTouched() {
    nameTouched = true;
  }

  function addInstance() {
    instances = [...instances, newInstance()];
  }

  function continueFromRuntimeStep() {
    if (!runtimeChosen) {
      error = '请先选择运行时类型。';
      return;
    }
    error = '';
    step = 1;
    if (
      runtimeKind === 'cloud_native' &&
      !initialApplication &&
      !manualKubernetesApplication
    ) {
      queueWorkloadDiscovery();
    }
  }

  function removeInstance(id: string) {
    instances = instances.filter((instance) => instance.id !== id);
    delete bindingPreviews[id];
    delete expandedBindingPreviews[id];
    bindingPreviews = { ...bindingPreviews };
    expandedBindingPreviews = { ...expandedBindingPreviews };
  }

  function updateSelector(
    instance: ApplicationInstanceDraft,
    key: string,
    value: string
  ) {
    instance.selector = { ...instance.selector, [key]: value };
    delete bindingPreviews[instance.id];
    bindingPreviews = { ...bindingPreviews };
    expandedBindingPreviews = {
      ...expandedBindingPreviews,
      [instance.id]: false
    };
    instances = [...instances];
  }

  function selectInstanceResource(
    instance: ApplicationInstanceDraft,
    value: string
  ) {
    if (value === '__add_runtime_resource__') {
      onRequestResourceCreate(
        runtimeLabel(runtimeKind) as RuntimeResourceKind,
        instance.id
      );
      return;
    }
    instance.targetResourceId = value;
    delete bindingPreviews[instance.id];
    bindingPreviews = { ...bindingPreviews };
    expandedBindingPreviews = {
      ...expandedBindingPreviews,
      [instance.id]: false
    };
    instances = [...instances];
  }

  function bindingPreviewStatus(
    preview: BindingPreview | undefined,
    instance: ApplicationInstanceDraft
  ): BindingPreviewStatus {
    const selectorKey =
      runtimeKind === 'virtual_machine'
        ? 'process_keyword'
        : runtimeKind === 'containerized'
          ? 'container_name'
          : 'workload_name';
    if (
      !preview ||
      preview.resourceId !== instance.targetResourceId ||
      preview.keyword !== String(instance.selector[selectorKey] ?? '').trim()
    ) {
      return ['active', 'healthy', 'normal', 'up'].includes(
        instance.status ?? ''
      )
        ? 'success'
        : ['warning', 'disabled', 'down', 'error'].includes(
              instance.status ?? ''
            )
          ? 'none'
          : 'unknown';
    }
    if (preview.loading) return 'unknown';
    if (preview.error || preview.count === 0) return 'none';
    return preview.count === 1 ? 'success' : 'multiple';
  }

  function bindingPreviewAccent(status: BindingPreviewStatus) {
    if (status === 'success') return 'var(--theme-success)';
    if (status === 'multiple') return 'var(--theme-warning)';
    if (status === 'none') return 'var(--theme-danger)';
    return 'var(--theme-fg-subtle)';
  }

  function toggleBindingPreview(instance: ApplicationInstanceDraft) {
    expandedBindingPreviews = {
      ...expandedBindingPreviews,
      [instance.id]: !expandedBindingPreviews[instance.id]
    };
  }

  function bindingPreviewTooltip(
    preview: BindingPreview | undefined,
    status: BindingPreviewStatus,
    instance: ApplicationInstanceDraft
  ) {
    if (!preview) {
      if (['active', 'healthy', 'normal', 'up'].includes(instance.status ?? ''))
        return '当前实例状态：正常';
      if (
        ['warning', 'disabled', 'down', 'error'].includes(instance.status ?? '')
      )
        return '当前实例状态：需关注';
      return '当前实例状态：未知';
    }
    if (status === 'unknown')
      return preview.loading ? '正在读取匹配对象…' : '尚未预览匹配';
    if (preview.error) return `匹配失败：${preview.error}`;
    const summary =
      preview.count === 1
        ? '匹配到 1 个对象'
        : preview.count === 0
          ? '匹配到 0 个对象，必须恰好匹配一个对象才可创建应用'
          : `匹配到 ${preview.count} 个对象，必须恰好匹配一个对象才可创建应用`;
    return [
      summary,
      ...preview.items.map(
        (item) => `${item.name}${item.detail ? ` · ${item.detail}` : ''}`
      )
    ].join('\n');
  }

  async function previewBinding(
    instance: ApplicationInstanceDraft,
    silent = false
  ) {
    const selectorKey =
      runtimeKind === 'virtual_machine'
        ? 'process_keyword'
        : runtimeKind === 'containerized'
          ? 'container_name'
          : 'workload_name';
    const keyword = String(instance.selector[selectorKey] ?? '').trim();
    const resourceId = instance.targetResourceId;
    if (!resourceId || !keyword) {
      bindingPreviews = {
        ...bindingPreviews,
        [instance.id]: {
          resourceId,
          keyword,
          count: 0,
          items: [],
          error: '请先选择资源并填写定位关键字。'
        }
      };
      expandedBindingPreviews = {
        ...expandedBindingPreviews,
        [instance.id]: true
      };
      return;
    }
    if (!silent) {
      bindingPreviews = {
        ...bindingPreviews,
        [instance.id]: {
          resourceId,
          keyword,
          count: 0,
          items: [],
          loading: true
        }
      };
      expandedBindingPreviews = {
        ...expandedBindingPreviews,
        [instance.id]: true
      };
    }
    try {
      const result = await api.applicationBindingPreview(
        resourceId,
        runtimeKind,
        keyword
      );
      if (
        instance.targetResourceId !== resourceId ||
        String(instance.selector[selectorKey] ?? '').trim() !== keyword
      )
        return;
      bindingPreviews = {
        ...bindingPreviews,
        [instance.id]: {
          resourceId,
          keyword,
          count: result.count,
          items: result.items
        }
      };
      if (silent && result.count !== 1) {
        expandedBindingPreviews = {
          ...expandedBindingPreviews,
          [instance.id]: true
        };
      }
    } catch (cause) {
      if (
        instance.targetResourceId !== resourceId ||
        String(instance.selector[selectorKey] ?? '').trim() !== keyword
      )
        return;
      bindingPreviews = {
        ...bindingPreviews,
        [instance.id]: {
          resourceId,
          keyword,
          count: 0,
          items: [],
          error: cause instanceof Error ? cause.message : '匹配预览失败。'
        }
      };
      if (silent)
        expandedBindingPreviews = {
          ...expandedBindingPreviews,
          [instance.id]: true
        };
    }
  }

  async function previewAllBindings() {
    await Promise.all(
      instances.map((instance) =>
        previewBinding(
          instance,
          bindingPreviewStatus(bindingPreviews[instance.id], instance) ===
            'success'
        )
      )
    );
  }

  function handleResourceCreated(resource: Resource, instanceId?: string) {
    createdResources = [
      resource,
      ...createdResources.filter((item) => item.id !== resource.id)
    ];
    onResourceCreated(resource);
    if (runtimeKind === 'cloud_native' && resource.kind === 'Kubernetes') {
      kubernetesResourceId = resource.id;
      queueWorkloadDiscovery();
      return;
    }
    if (resource.kind !== runtimeLabel(runtimeKind) || !instanceId) return;
    const instance = instances.find((item) => item.id === instanceId);
    if (instance) {
      instance.targetResourceId = resource.id;
      delete bindingPreviews[instance.id];
      bindingPreviews = { ...bindingPreviews };
      expandedBindingPreviews = {
        ...expandedBindingPreviews,
        [instance.id]: false
      };
      instances = [...instances];
    }
  }

  function selectKubernetesResource(value: string) {
    if (value === '__add_runtime_resource__') {
      onRequestResourceCreate('Kubernetes');
      return;
    }
    kubernetesResourceId = value;
    queueWorkloadDiscovery();
  }

  function workloadKey(workload: Workload) {
    return `${workload.namespace}/${workload.kind}/${workload.name}`;
  }

  function ensureKubernetesDrafts(items: Workload[]) {
    const next = { ...kubernetesApplicationDrafts };
    for (const workload of items) {
      const key = workloadKey(workload);
      next[key] ??= {
        name: workload.name,
        code: slugify(`${workload.name}-${workload.namespace}-${workload.kind}`)
          .slice(0, 64)
          .replace(/-$/, ''),
        description: `${workload.kind} · ${workload.namespace}`,
        labels,
        icon,
        critical,
        technicalOwnerIds: [...technicalOwnerIds],
        operationsOwnerIds: [...operationsOwnerIds]
      };
    }
    kubernetesApplicationDrafts = next;
  }

  function queueWorkloadDiscovery() {
    clearTimeout(workloadDiscoveryTimer);
    const requestId = ++workloadDiscoveryRequestId;
    const resourceId = kubernetesResourceId;
    const selectedNamespace = namespace.trim();
    const selectedFilters = filters.trim();
    if (!resourceId || !selectedNamespace) {
      workloads = [];
      selectedWorkloads = new Set();
      activeWorkloadKey = '';
      loading = false;
      return;
    }
    workloads = [];
    selectedWorkloads = new Set();
    activeWorkloadKey = '';
    loading = true;
    error = '';
    workloadDiscoveryTimer = setTimeout(() => {
      void discoverWorkloads(
        resourceId,
        selectedNamespace,
        selectedFilters,
        requestId
      );
    }, 250);
  }

  async function discoverWorkloads(
    resourceId: string,
    selectedNamespace: string,
    selectedFilters: string,
    requestId: number
  ) {
    loading = true;
    try {
      const result = await api.kubernetesWorkloads(resourceId, {
        namespace: selectedNamespace,
        filters: selectedFilters
      });
      if (requestId !== workloadDiscoveryRequestId) return;
      workloads = result.items;
      selectedWorkloads = new Set(workloads.map(workloadKey));
      ensureKubernetesDrafts(workloads);
      activeWorkloadKey = workloads.some(
        (workload) => workloadKey(workload) === activeWorkloadKey
      )
        ? activeWorkloadKey
        : workloads[0]
          ? workloadKey(workloads[0])
          : '';
    } catch (cause) {
      if (requestId === workloadDiscoveryRequestId) {
        error =
          cause instanceof Error
            ? cause.message
            : '工作负载读取失败，请检查资源连接和定位条件。';
      }
    } finally {
      if (requestId === workloadDiscoveryRequestId) loading = false;
    }
  }

  function toggleWorkload(key: string) {
    const next = new Set(selectedWorkloads);
    next.has(key) ? next.delete(key) : next.add(key);
    selectedWorkloads = next;
    if (!activeWorkloadKey || !next.has(activeWorkloadKey))
      activeWorkloadKey = key;
  }

  function updateKubernetesDraft(
    key: string,
    field: keyof KubernetesApplicationDraft,
    value: string | boolean | string[]
  ) {
    const draft = kubernetesApplicationDrafts[key];
    if (!draft) return;
    kubernetesApplicationDrafts = {
      ...kubernetesApplicationDrafts,
      [key]: { ...draft, [field]: value }
    };
  }

  function validate() {
    return !applicationCreateBlockReason();
  }

  async function save() {
    submitAttempted = true;
    error = '';
    reconfirming =
      !initialApplication &&
      runtimeKind !== 'cloud_native' &&
      instances.length > 0 &&
      instances.every(
        (instance) =>
          bindingPreviewStatus(bindingPreviews[instance.id], instance) ===
          'success'
      );
    if (!initialApplication && runtimeKind !== 'cloud_native')
      await previewAllBindings();
    if (!validate()) {
      reconfirming = false;
      return;
    }
    saving = true;
    try {
      const applicationLabels = parseLabels(labels);
      if (initialApplication) {
        await onUpdate(initialApplication, {
          name: name.trim(),
          code: code.trim(),
          description: description.trim(),
          icon,
          labels: applicationLabels,
          runtimeKind,
          critical,
          technicalOwnerIds,
          operationsOwnerIds,
          instances
        });
      } else if (
        runtimeKind === 'cloud_native' &&
        !manualKubernetesApplication
      ) {
        const drafts = workloads
          .filter((workload) => selectedWorkloads.has(workloadKey(workload)))
          .map((workload) => {
            const key = workloadKey(workload);
            const application = kubernetesApplicationDrafts[key];
            return {
              name: application.name.trim(),
              code: application.code.trim(),
              description: application.description.trim(),
              icon: application.icon,
              labels: parseLabels(application.labels),
              runtimeKind,
              externalUid: `${kubernetesResourceId}:${key}`,
              critical: application.critical,
              technicalOwnerIds: application.technicalOwnerIds,
              operationsOwnerIds: application.operationsOwnerIds,
              instances: workload.pods.map((pod) => ({
                id: crypto.randomUUID(),
                name: pod.name,
                targetResourceId: kubernetesResourceId,
                selector: {
                  namespace: workload.namespace,
                  workload_kind: workload.kind,
                  workload_name: workload.name,
                  pod_name: pod.name
                }
              }))
            } satisfies ApplicationDraft;
          });
        await onSave(drafts);
      } else {
        await onSave([
          {
            name: name.trim(),
            code: code.trim(),
            description: description.trim(),
            icon,
            labels: applicationLabels,
            runtimeKind,
            critical,
            technicalOwnerIds,
            operationsOwnerIds,
            instances
          }
        ]);
      }
    } catch (cause) {
      error =
        cause instanceof Error
          ? cause.message
          : '保存应用失败，请检查表单内容。';
    } finally {
      saving = false;
      reconfirming = false;
    }
  }
</script>

<ModalDialog
  dialogClass="project-dialog application-wizard-dialog"
  size="wide"
  overflowMode="expand"
  initialCenterRatio={0.38}
  title={initialApplication ? '编辑应用' : '新增应用'}
  description={initialApplication
    ? '更新应用信息与实例绑定。'
    : '选择运行时类型并填写应用信息。'}
  confirmLabel={step === 0 ? '下一步' : initialApplication ? '保存' : '创建'}
  busyLabel={initialApplication ? '保存中…' : '创建中…'}
  busy={saving || reconfirming}
  message={error}
  {onCancel}
  onConfirm={step === 0 ? continueFromRuntimeStep : save}
>
  <nav class="application-steps" aria-label="新增应用步骤">
    <button
      class:active={step === 0}
      class:done={step > 0}
      type="button"
      disabled={instances.length > 0}
      on:click={() => instances.length === 0 && (step = 0)}
      ><span>1</span>运行时类型</button
    >
    <button
      class:active={step === 1}
      type="button"
      disabled={!runtimeChosen}
      on:click={() => runtimeChosen && (step = 1)}
      ><span>2</span>应用与实例</button
    >
  </nav>
  <div class="project-dialog-content">
    {#if step === 0}
      <div class="runtime-choice-grid">
        {#each [['virtual_machine', 'Host', '虚拟机', '逐个绑定主机实例'], ['containerized', 'Docker', '容器化', '逐个绑定容器实例'], ['cloud_native', 'Kubernetes', '云原生', '批量发现工作负载和实例']] as option}
          <button
            class:selected={runtimeChosen && runtimeKind === option[0]}
            disabled={instances.length > 0}
            type="button"
            on:click={() => changeRuntime(option[0] as ApplicationRuntimeKind)}
          >
            <ResourceBrandIcon resource={{ kind: option[1] }} size={26} />
            <strong
              >{option[1]}<small class="runtime-choice-kind">{option[2]}</small
              ></strong
            ><small>{option[3]}</small>
          </button>
        {/each}
      </div>
    {:else if runtimeKind === 'cloud_native' && !initialApplication && !manualKubernetesApplication}
      <div class="kubernetes-runtime-fields">
        <FormField label="运行资源" required
          ><DropdownSelect
            value={kubernetesResourceId}
            options={runtimeResourceOptions}
            placeholder="选择 Kubernetes 资源"
            ariaLabel="运行资源"
            invalid={submitAttempted && !kubernetesResourceId}
            invalidMode="bubble"
            invalidMessage="请选择 Kubernetes 运行资源"
            on:change={(event) =>
              selectKubernetesResource(String(event.detail))}
          /></FormField
        >
        <FormField label="命名空间" required
          ><TextInput
            value={namespace}
            placeholder="default"
            on:value={(event) => {
              namespace = String(event.detail);
              queueWorkloadDiscovery();
            }}
          /></FormField
        >
        <FormField label="标签选择器" className="kubernetes-filter-field"
          ><TextInput
            value={filters}
            placeholder="app.kubernetes.io/part-of=payments"
            on:value={(event) => {
              filters = String(event.detail);
              queueWorkloadDiscovery();
            }}
          /></FormField
        >
      </div>
      <div class="kubernetes-batch-toolbar">
        <div>
          <strong>工作负载与应用配置</strong><small
            >勾选决定创建范围；选择名称编辑对应应用</small
          >
        </div>
        <span class="kubernetes-discovery-status" aria-live="polite"
          >{loading
            ? '正在同步工作负载…'
            : `${workloads.length} 个工作负载 · ${selectedWorkloads.size} 个将创建`}</span
        >
      </div>
      {#if error}<p class="kubernetes-discovery-error" role="alert">
          {error}
        </p>{/if}
      {#if workloads.length}
        <div class="kubernetes-batch-layout">
          <aside
            class="kubernetes-workload-pane"
            class:invalid={submitAttempted && selectedWorkloads.size === 0}
            aria-label="工作负载列表"
          >
            <div class="kubernetes-workload-pane-heading">
              <strong>工作负载</strong><span
                >{selectedWorkloads.size}/{workloads.length}</span
              >
            </div>
            <div class="kubernetes-workload-list">
              {#each workloads as workload}{@const key = workloadKey(workload)}
                <div
                  class="kubernetes-workload-item"
                  class:active={activeWorkloadKey === key}
                  class:checked={selectedWorkloads.has(key)}
                >
                  <input
                    type="checkbox"
                    checked={selectedWorkloads.has(key)}
                    aria-label={`将 ${workload.name} 纳入创建`}
                    on:change={() => toggleWorkload(key)}
                  />
                  <button
                    class="kubernetes-workload-focus"
                    type="button"
                    aria-current={activeWorkloadKey === key
                      ? 'true'
                      : undefined}
                    on:click={() => (activeWorkloadKey = key)}
                  >
                    <span class="workload-icon"
                      ><EntityBrandIcon
                        kind="Application"
                        fallback="lucide:AppWindow"
                        size={16}
                      /></span
                    >
                    <span class="kubernetes-workload-copy"
                      ><strong>{workload.name}</strong><small
                        >{workload.kind} · {workload.namespace}</small
                      ><small>{workload.pods.length} 个 Pod 实例</small></span
                    >
                  </button>
                </div>
              {/each}
            </div>
          </aside>
          <section
            class="kubernetes-application-pane"
            aria-label="当前工作负载对应的应用配置"
          >
            {#if activeWorkloadKey && workloads.find((workload) => workloadKey(workload) === activeWorkloadKey)}
              {@const activeWorkload = workloads.find(
                (workload) => workloadKey(workload) === activeWorkloadKey
              )!}
              {@const application =
                kubernetesApplicationDrafts[activeWorkloadKey]}
              <div class="kubernetes-application-heading">
                <div>
                  <h3>{application?.name || activeWorkload.name}</h3>
                  <p>
                    {activeWorkload.kind} · {activeWorkload.namespace} · 自动关联
                    {activeWorkload.pods.length} 个 Pod 实例
                  </p>
                </div>
                <span
                  class:included={selectedWorkloads.has(activeWorkloadKey)}
                  class="kubernetes-inclusion-state"
                  >{selectedWorkloads.has(activeWorkloadKey)
                    ? '将创建应用'
                    : '未纳入创建'}</span
                >
              </div>
              {#if application}
                <div class="application-form-grid">
                  <div class="application-identity-row">
                    <div class="application-icon-code-fields">
                      <FormField label="应用图标"
                        ><IconPicker
                          value={application.icon}
                          onSelect={(value) =>
                            updateKubernetesDraft(
                              activeWorkloadKey,
                              'icon',
                              value
                            )}
                          ariaLabel="选择应用图标"
                        /></FormField
                      >
                      <FormField
                        label="关键应用"
                        className="application-critical-field"
                        asLabel={false}
                        ><DropdownSelect
                          value={String(application.critical)}
                          options={[
                            { value: 'false', label: '否' },
                            { value: 'true', label: '是' }
                          ]}
                          ariaLabel="是否为关键应用"
                          on:change={(event) =>
                            updateKubernetesDraft(
                              activeWorkloadKey,
                              'critical',
                              event.detail === 'true'
                            )}
                        /></FormField
                      >
                      <FormField label="应用标识" required
                        ><TextInput
                          value={application.code}
                          on:value={(event) =>
                            updateKubernetesDraft(
                              activeWorkloadKey,
                              'code',
                              String(event.detail)
                            )}
                          required
                          placeholder="payments-api"
                          invalid={submitAttempted && !application.code.trim()}
                          invalidMode="bubble"
                          invalidMessage="请填写应用标识"
                        /></FormField
                      >
                    </div>
                    <FormField
                      label="应用名称"
                      required
                      className="application-name-field"
                      ><TextInput
                        value={application.name}
                        on:value={(event) =>
                          updateKubernetesDraft(
                            activeWorkloadKey,
                            'name',
                            String(event.detail)
                          )}
                        required
                        placeholder="支付 API"
                        invalid={submitAttempted && !application.name.trim()}
                        invalidMode="bubble"
                        invalidMessage="请填写应用名称"
                      /></FormField
                    >
                  </div>
                  <div class="owner-fields-row">
                    <FormField label="技术负责人" asLabel={false}
                      ><DropdownSelect
                        options={ownerOptions}
                        value={application.technicalOwnerIds}
                        multiple
                        taggedSelection
                        reorderableSelection
                        searchable
                        placeholder="选择负责人(主备排序)"
                        ariaLabel="选择技术负责人"
                        on:change={(event) =>
                          updateKubernetesDraft(
                            activeWorkloadKey,
                            'technicalOwnerIds',
                            event.detail
                          )}
                      /></FormField
                    >
                    <FormField label="运维负责人" asLabel={false}
                      ><DropdownSelect
                        options={ownerOptions}
                        value={application.operationsOwnerIds}
                        multiple
                        taggedSelection
                        reorderableSelection
                        searchable
                        placeholder="选择负责人(主备排序)"
                        ariaLabel="选择运维负责人"
                        on:change={(event) =>
                          updateKubernetesDraft(
                            activeWorkloadKey,
                            'operationsOwnerIds',
                            event.detail
                          )}
                      /></FormField
                    >
                  </div>
                  <FormField
                    label="应用标签"
                    className="full-field"
                    asLabel={false}
                    ><TextInput
                      value={application.labels}
                      on:value={(event) =>
                        updateKubernetesDraft(
                          activeWorkloadKey,
                          'labels',
                          String(event.detail)
                        )}
                      tagged
                      placeholder="填写 key=value、key:value 或 key，按回车确认"
                      autocomplete="off"
                    /></FormField
                  >
                  <FormField label="应用描述" className="full-field"
                    ><TextArea
                      bind:value={application.description}
                      on:input={() =>
                        updateKubernetesDraft(
                          activeWorkloadKey,
                          'description',
                          application.description
                        )}
                      rows={2}
                      placeholder="应用职责和运行边界"
                    /></FormField
                  >
                </div>
              {/if}
            {:else}
              <div class="kubernetes-application-empty">
                <CircleHelp size={18} /><span
                  >选择一个工作负载查看并编辑对应的应用配置。</span
                >
              </div>
            {/if}
          </section>
        </div>
        {#if submitAttempted && selectedWorkloads.size === 0}<p
            class="application-validation-bubble"
            role="alert"
          >
            请至少选择一个工作负载
          </p>{/if}
      {:else}
        <div class="kubernetes-batch-empty">
          <Server size={18} /><span
            >{loading
              ? '正在读取所选范围内的工作负载…'
              : '选择运行资源、命名空间和标签选择器后，将自动显示匹配的工作负载。'}</span
          >
        </div>
      {/if}
    {:else}
      <div class="application-form-grid">
        <div class="application-identity-row">
          <div class="application-icon-code-fields">
            <FormField label="应用图标"
              ><IconPicker
                value={icon}
                onSelect={(value) => (icon = value)}
                ariaLabel="选择应用图标"
              /></FormField
            >
            <FormField
              label="关键应用"
              className="application-critical-field"
              asLabel={false}
              ><DropdownSelect
                value={String(critical)}
                options={[
                  { value: 'false', label: '否' },
                  { value: 'true', label: '是' }
                ]}
                ariaLabel="是否为关键应用"
                on:change={(event) => (critical = event.detail === 'true')}
              /></FormField
            >
            <FormField label="应用标识" required
              ><TextInput
                bind:value={code}
                on:input={syncNameWithIdentifier}
                required
                placeholder="payments-api"
                invalid={submitAttempted && !code.trim()}
                invalidMode="bubble"
                invalidMessage="请填写应用标识"
              /></FormField
            >
          </div>
          <FormField
            label="应用名称"
            required
            className="application-name-field"
            ><TextInput
              bind:value={name}
              on:input={markNameTouched}
              required
              placeholder="支付 API"
              invalid={submitAttempted && !name.trim()}
              invalidMode="bubble"
              invalidMessage="请填写应用名称"
            /></FormField
          >
        </div>
        <div class="owner-fields-row">
          <FormField label="技术负责人" asLabel={false}
            ><DropdownSelect
              options={ownerOptions}
              bind:value={technicalOwnerIds}
              multiple
              taggedSelection
              reorderableSelection
              searchable
              placeholder="选择负责人(主备排序)"
              ariaLabel="选择技术负责人"
            /></FormField
          >
          <FormField label="运维负责人" asLabel={false}
            ><DropdownSelect
              options={ownerOptions}
              bind:value={operationsOwnerIds}
              multiple
              taggedSelection
              reorderableSelection
              searchable
              placeholder="选择负责人(主备排序)"
              ariaLabel="选择运维负责人"
            /></FormField
          >
        </div>
        <FormField label="应用标签" className="full-field" asLabel={false}
          ><TextInput
            bind:value={labels}
            tagged
            placeholder="填写 key=value、key:value 或 key，按回车确认"
            autocomplete="off"
          /></FormField
        >
        <FormField label="应用描述" className="full-field"
          ><TextArea
            bind:value={description}
            rows={2}
            placeholder="应用职责和运行边界"
          /></FormField
        >
      </div>
      <div class="discovery-toolbar">
        <span
          >实例绑定 <small
            >每个实例绑定一个唯一的 {runtimeLabel(runtimeKind)} 资源</small
          ></span
        ><button class="secondary" type="button" on:click={addInstance}
          ><Plus size={14} />新增实例</button
        >
      </div>
      <div class="instance-draft-head" aria-hidden="true">
        <span></span><span
          >实例名称<i class="required-mark" aria-hidden="true">*</i></span
        ><span
          >{runtimeLabel(runtimeKind)} 资源<i
            class="required-mark"
            aria-hidden="true">*</i
          ></span
        ><span
          >{runtimeKind === 'virtual_machine'
            ? '进程关键字'
            : runtimeKind === 'containerized'
              ? '容器名称'
              : '工作负载名称'}<i class="required-mark" aria-hidden="true">*</i
          ></span
        ><span></span><span></span>
      </div>
      <div class="instance-draft-list">
        {#each instances as instance}
          {@const preview = bindingPreviews[instance.id]}
          {@const previewStatus = bindingPreviewStatus(preview, instance)}
          {@const bindingIssue = instanceBindingIssue(instance)}
          <div
            class="instance-draft-entry"
            class:preview-open={expandedBindingPreviews[instance.id]}
            class:invalid={submitAttempted && Boolean(bindingIssue)}
          >
            <div class="instance-draft-row">
              <button
                class={`binding-preview-state ${previewStatus}`}
                style={`--binding-preview-accent:${bindingPreviewAccent(previewStatus)}`}
                type="button"
                data-binding-preview-id={instance.id}
                data-tooltip={bindingPreviewTooltip(
                  preview,
                  previewStatus,
                  instance
                )}
                aria-label={bindingPreviewTooltip(
                  preview,
                  previewStatus,
                  instance
                )}
                aria-expanded={expandedBindingPreviews[instance.id] ?? false}
                aria-controls={`binding-preview-${instance.id}`}
                on:click={() => toggleBindingPreview(instance)}
              >
                {#if previewStatus === 'success'}<CircleCheck
                    size={16}
                    color={bindingPreviewAccent(previewStatus)}
                  />
                {:else if previewStatus === 'multiple'}<TriangleAlert
                    size={16}
                    color={bindingPreviewAccent(previewStatus)}
                  />
                {:else if previewStatus === 'none'}<CircleX
                    size={16}
                    color={bindingPreviewAccent(previewStatus)}
                  />
                {:else}<CircleHelp
                    size={16}
                    color={bindingPreviewAccent(previewStatus)}
                  />{/if}
              </button>
              <div class="instance-cell">
                <TextInput
                  bind:value={instance.name}
                  ariaLabel="实例名称"
                  placeholder="instance-1"
                  invalid={submitAttempted && !instance.name.trim()}
                  invalidMode="bubble"
                  invalidMessage="请填写实例名称"
                />
              </div>
              <div class="instance-cell">
                <DropdownSelect
                  value={instance.targetResourceId}
                  options={runtimeResourceOptions}
                  placeholder="选择资源"
                  invalid={submitAttempted && !instance.targetResourceId}
                  invalidMode="bubble"
                  invalidMessage="请选择运行资源"
                  ariaLabel={`${runtimeLabel(runtimeKind)} 资源`}
                  on:change={(event) =>
                    selectInstanceResource(instance, String(event.detail))}
                />
              </div>
              <div class="instance-cell">
                <TextInput
                  value={String(
                    instance.selector[
                      runtimeKind === 'virtual_machine'
                        ? 'process_keyword'
                        : runtimeKind === 'containerized'
                          ? 'container_name'
                          : 'workload_name'
                    ] ?? ''
                  )}
                  placeholder={runtimeKind === 'virtual_machine'
                    ? 'java,payments.jar'
                    : runtimeKind === 'containerized'
                      ? 'payments-api'
                      : 'payments'}
                  invalid={submitAttempted && Boolean(bindingIssue)}
                  invalidMode="bubble"
                  invalidMessage={bindingIssue}
                  ariaLabel={runtimeKind === 'virtual_machine'
                    ? '进程关键字'
                    : runtimeKind === 'containerized'
                      ? '容器名称'
                      : '工作负载名称'}
                  on:value={(event) =>
                    updateSelector(
                      instance,
                      runtimeKind === 'virtual_machine'
                        ? 'process_keyword'
                        : runtimeKind === 'containerized'
                          ? 'container_name'
                          : 'workload_name',
                      String(event.detail)
                    )}
                />
              </div>
              {#if runtimeKind !== 'cloud_native'}<button
                  class="icon-button edit-action binding-preview-button"
                  type="button"
                  data-binding-preview-id={instance.id}
                  data-tooltip={bindingPreviews[instance.id]?.loading
                    ? '正在预览匹配'
                    : '预览匹配'}
                  aria-label={bindingPreviews[instance.id]?.loading
                    ? '正在预览匹配'
                    : '预览匹配'}
                  disabled={bindingPreviews[instance.id]?.loading}
                  on:click={() => previewBinding(instance)}
                  ><ScanSearch size={15} /></button
                >{:else}<span></span>{/if}
              <button
                class="icon-button danger-action delete-action"
                type="button"
                aria-label="移除实例"
                data-tooltip="移除实例"
                on:click={() => removeInstance(instance.id)}
                ><X size={15} /></button
              >
            </div>
            {#if runtimeKind === 'cloud_native'}
              <div class="instance-kubernetes-selector">
                {#each [['namespace', '命名空间'], ['workload_kind', '工作负载类型']] as field}
                  <TextInput
                    value={String(instance.selector[field[0]] ?? '')}
                    placeholder={field[1]}
                    ariaLabel={field[1]}
                    on:value={(event) =>
                      updateSelector(instance, field[0], String(event.detail))}
                  />
                {/each}
              </div>
            {/if}
            {#if expandedBindingPreviews[instance.id]}
              <div
                id={`binding-preview-${instance.id}`}
                class={`binding-preview-result ${previewStatus}`}
                style={`--binding-preview-accent:${bindingPreviewAccent(previewStatus)}`}
                data-binding-preview-id={instance.id}
              >
                {#if !preview}<span>尚未预览匹配，请点击“预览匹配”。</span>
                {:else if preview.loading}<span>正在读取匹配对象…</span>
                {:else if preview.error}<span class="binding-preview-status"
                    ><CircleX
                      class="binding-preview-icon"
                      size={14}
                      color={bindingPreviewAccent(previewStatus)}
                    /><strong>{preview.error}</strong></span
                  >
                {:else if preview.count === 1}<span
                    class="binding-preview-status"
                    ><CircleCheck
                      class="binding-preview-icon"
                      size={14}
                      color={bindingPreviewAccent(previewStatus)}
                    /><span class="binding-preview-summary"
                      ><strong>匹配到 1 个对象</strong></span
                    ></span
                  >
                {:else if preview.count === 0}<span
                    class="binding-preview-status"
                    ><CircleX
                      class="binding-preview-icon"
                      size={14}
                      color={bindingPreviewAccent(previewStatus)}
                    /><span class="binding-preview-summary"
                      ><strong>匹配到 0 个对象</strong><span
                        >（必须恰好匹配一个对象才可创建应用）</span
                      ></span
                    ></span
                  >
                {:else}<span class="binding-preview-status"
                    ><TriangleAlert
                      class="binding-preview-icon"
                      size={14}
                      color={bindingPreviewAccent(previewStatus)}
                    /><span class="binding-preview-summary"
                      ><strong>匹配到 {preview.count} 个对象</strong><span
                        >（必须恰好匹配一个对象才可创建应用）</span
                      ></span
                    ></span
                  >{/if}
                {#if preview && !preview.loading && !preview.error && preview.items.length}
                  <ul>
                    {#each preview.items as item}<li>
                        <span class="binding-preview-bullet" aria-hidden="true"
                        ></span><strong>{item.name}</strong><span
                          >{item.detail || item.status || item.id}</span
                        >
                      </li>{/each}
                  </ul>
                  {#if preview.count > preview.items.length}<small
                      >仅展示前 {preview.items.length} 条</small
                    >{/if}
                {/if}
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</ModalDialog>
