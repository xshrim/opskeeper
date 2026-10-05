<script lang="ts">
  import { Plus, Server, X } from 'lucide-svelte';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import type { Resource } from '../../lib/api';
  import { api } from '../../lib/api';
  import {
    resourcesForRuntime,
    runtimeLabel,
    slugify,
    type ApplicationDraft,
    type ApplicationInstanceDraft,
    type ApplicationRuntimeKind
  } from './projectTypes';

  export let resources: Resource[] = [];
  export let onSave: (applications: ApplicationDraft[]) => Promise<void> | void;
  export let onCancel: () => void;

  type Workload = {
    namespace: string;
    name: string;
    kind: string;
    ready: boolean;
    phase?: string;
    restarts?: number;
    pods: Array<{ namespace: string; name: string; containers?: string[] }>;
  };

  let step = 0;
  let runtimeKind: ApplicationRuntimeKind = 'virtual_machine';
  let name = '';
  let code = '';
  let description = '';
  let icon = 'lucide:AppWindow';
  let instances: ApplicationInstanceDraft[] = [];
  let kubernetesResourceId = '';
  let namespace = 'default';
  let filters = '';
  let workloads: Workload[] = [];
  let selectedWorkloads = new Set<string>();
  let workloadNames: Record<string, string> = {};
  let loading = false;
  let saving = false;
  let error = '';

  $: runtimeResources = resourcesForRuntime(resources, runtimeKind);
  $: if (runtimeKind !== 'cloud_native' && instances.length === 0 && runtimeResources.length) {
    instances = [newInstance()];
  }
  $: if (runtimeKind === 'cloud_native' && !kubernetesResourceId) {
    kubernetesResourceId = runtimeResources[0]?.id ?? '';
  }

  function newInstance(kind = runtimeKind): ApplicationInstanceDraft {
    return {
      id: crypto.randomUUID(),
      name: `instance-${instances.length + 1}`,
      targetResourceId: resourcesForRuntime(resources, kind)[0]?.id ?? '',
      selector: {}
    };
  }

  function changeRuntime(next: ApplicationRuntimeKind) {
    runtimeKind = next;
    step = 1;
    error = '';
    instances = next === 'cloud_native' ? [] : [newInstance(next)];
    if (next !== 'cloud_native') kubernetesResourceId = '';
  }

  function addInstance() {
    instances = [...instances, newInstance()];
  }

  function removeInstance(id: string) {
    instances = instances.filter((instance) => instance.id !== id);
  }

  function updateSelector(instance: ApplicationInstanceDraft, key: string, value: string) {
    instance.selector = { ...instance.selector, [key]: value };
    instances = [...instances];
  }

  function workloadKey(workload: Workload) {
    return `${workload.namespace}/${workload.kind}/${workload.name}`;
  }

  async function discoverWorkloads() {
    error = '';
    if (!kubernetesResourceId || !namespace.trim()) {
      error = '请选择 Kubernetes 资源并填写命名空间。';
      return;
    }
    loading = true;
    try {
      const result = await api.kubernetesWorkloads(kubernetesResourceId, { namespace: namespace.trim(), filters });
      workloads = result.items;
      selectedWorkloads = new Set(workloads.map(workloadKey));
      workloadNames = Object.fromEntries(workloads.map((item) => [workloadKey(item), item.name]));
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '工作负载读取失败，请检查资源连接和定位条件。';
    } finally {
      loading = false;
    }
  }

  function toggleWorkload(key: string) {
    const next = new Set(selectedWorkloads);
    next.has(key) ? next.delete(key) : next.add(key);
    selectedWorkloads = next;
  }

  function validate() {
    if (runtimeKind === 'cloud_native') return selectedWorkloads.size > 0;
    if (!name.trim() || !code.trim()) return false;
    return instances.length > 0 && instances.every((instance) => {
      const selectorKey = runtimeKind === 'virtual_machine' ? 'process_keyword' : 'container_name';
      return instance.name.trim() && instance.targetResourceId && String(instance.selector[selectorKey] ?? '').trim();
    });
  }

  async function save() {
    error = '';
    if (!validate()) {
      error = runtimeKind === 'cloud_native'
        ? '请先加载并选择至少一个 Kubernetes 工作负载。'
        : '请填写应用名称、编码，并为每个实例绑定资源和唯一定位字段。';
      return;
    }
    saving = true;
    try {
      if (runtimeKind === 'cloud_native') {
        const drafts = workloads.filter((workload) => selectedWorkloads.has(workloadKey(workload))).map((workload) => {
          const key = workloadKey(workload);
          const appName = (workloadNames[key] || workload.name).trim();
          return {
            name: appName,
            code: slugify(`${workload.name}-${workload.namespace}-${workload.kind}`).slice(0, 64).replace(/-$/, ''),
            description: `${workload.kind} · ${workload.namespace}`,
            icon: 'lucide:AppWindow',
            runtimeKind,
            externalUid: `${kubernetesResourceId}:${key}`,
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
        await onSave([{ name: name.trim(), code: code.trim(), description: description.trim(), icon, runtimeKind, instances }]);
      }
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '保存应用失败，请检查表单内容。';
    } finally {
      saving = false;
    }
  }
</script>

<div class="project-overlay" role="presentation" on:click={(event) => event.target === event.currentTarget && onCancel()}>
  <div class="project-dialog application-wizard-dialog" role="dialog" aria-modal="true" aria-labelledby="application-wizard-title" tabindex="-1">
    <header class="project-dialog-header">
      <div><h2 id="application-wizard-title">添加应用</h2><p>先选择运行时类型，再完成应用信息与实例绑定。</p></div>
      <div class="project-dialog-actions"><button class="secondary" type="button" on:click={onCancel}>取消</button>{#if step === 1}<button class="secondary" type="button" on:click={() => (step = 0)}>上一步</button>{/if}<button class="primary" type="button" disabled={saving} on:click={step === 0 ? () => (step = 1) : save}>{step === 0 ? '下一步' : saving ? '保存中…' : '完成添加'}</button></div>
    </header>
    <nav class="application-steps" aria-label="添加应用步骤">
      <button class:active={step === 0} class:done={step > 0} type="button" on:click={() => (step = 0)}><span>1</span>运行时类型</button>
      <button class:active={step === 1} type="button" on:click={() => (step = 1)}><span>2</span>应用与实例</button>
    </nav>
    {#if error}<div class="form-error" role="alert">{error}</div>{/if}
    <div class="project-dialog-content">
      {#if step === 0}
        <div class="runtime-choice-grid">
          {#each [['virtual_machine', 'Host', '逐个绑定主机实例'], ['containerized', 'Docker', '逐个绑定容器实例'], ['cloud_native', 'Kubernetes', '批量发现工作负载和实例']] as option}
            <button class:selected={runtimeKind === option[0]} type="button" on:click={() => changeRuntime(option[0] as ApplicationRuntimeKind)}>
              <ResourceBrandIcon resource={{ kind: option[1] }} size={26} />
              <strong>{option[1]}</strong><small>{option[2]}</small>
            </button>
          {/each}
        </div>
      {:else if runtimeKind === 'cloud_native'}
        <div class="wizard-form-grid">
          <label><span>运行资源<i class="required-mark">*</i></span><select bind:value={kubernetesResourceId}><option value="">选择 Kubernetes 资源</option>{#each runtimeResources as resource}<option value={resource.id}>{resource.name}</option>{/each}</select></label>
          <label><span>命名空间<i class="required-mark">*</i></span><input bind:value={namespace} placeholder="default" /></label>
          <label class="full-field"><span>标签选择器</span><input bind:value={filters} placeholder="app.kubernetes.io/part-of=payments" /></label>
        </div>
        <div class="discovery-toolbar"><span>工作负载列表</span><button class="secondary" type="button" disabled={loading} on:click={discoverWorkloads}><Server size={14} />{loading ? '读取中…' : '读取工作负载'}</button></div>
        {#if workloads.length}
          <div class="workload-list">{#each workloads as workload}{@const key = workloadKey(workload)}<label class="workload-row"><input type="checkbox" checked={selectedWorkloads.has(key)} on:change={() => toggleWorkload(key)} /><span class="workload-icon"><EntityBrandIcon kind="Application" fallback="lucide:AppWindow" size={16} /></span><span><strong>{workload.name}</strong><small>{workload.kind} · {workload.namespace} · {workload.pods.length} 个实例</small></span><input class="workload-name" value={workloadNames[key] ?? workload.name} aria-label={`${workload.name} 的应用名称`} on:input={(event) => (workloadNames = { ...workloadNames, [key]: (event.currentTarget as HTMLInputElement).value })} /></label>{/each}</div>
        {:else}<div class="empty-inline"><Server size={18} /><span>填写定位条件后读取 Kubernetes 工作负载。</span></div>{/if}
      {:else}
        <div class="wizard-form-grid">
          <label><span>应用名称<i class="required-mark">*</i></span><input bind:value={name} placeholder="支付 API" /></label>
          <label><span>应用编码<i class="required-mark">*</i></span><input bind:value={code} placeholder="payments-api" /></label>
          <label><span>应用图标</span><IconPicker value={icon} onSelect={(value) => (icon = value)} ariaLabel="选择应用图标" /></label>
          <label class="full-field"><span>应用描述</span><textarea bind:value={description} rows="2" placeholder="应用职责和运行边界"></textarea></label>
        </div>
        <div class="discovery-toolbar"><span>实例绑定 <small>每个实例绑定一个唯一的 {runtimeLabel(runtimeKind)} 资源</small></span><button class="secondary" type="button" on:click={addInstance}><Plus size={14} />添加实例</button></div>
        <div class="instance-draft-list">{#each instances as instance, index}<div class="instance-draft-row"><span class="instance-index">{index + 1}</span><label><span>实例名称</span><input bind:value={instance.name} placeholder="instance-1" /></label><label><span>{runtimeLabel(runtimeKind)} 资源<i class="required-mark">*</i></span><select bind:value={instance.targetResourceId}><option value="">选择资源</option>{#each runtimeResources as resource}<option value={resource.id}>{resource.name}</option>{/each}</select></label><label><span>{runtimeKind === 'virtual_machine' ? '进程关键字' : '容器名称'}<i class="required-mark">*</i></span><input value={String(instance.selector[runtimeKind === 'virtual_machine' ? 'process_keyword' : 'container_name'] ?? '')} placeholder={runtimeKind === 'virtual_machine' ? 'java -jar payments.jar' : 'payments-api'} on:input={(event) => updateSelector(instance, runtimeKind === 'virtual_machine' ? 'process_keyword' : 'container_name', (event.currentTarget as HTMLInputElement).value)} /></label><button class="icon-button" type="button" aria-label="移除实例" data-tooltip="移除实例" disabled={instances.length === 1} on:click={() => removeInstance(instance.id)}><X size={15} /></button></div>{/each}</div>
      {/if}
    </div>
  </div>
</div>
