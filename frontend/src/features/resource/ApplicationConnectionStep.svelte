<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import ApplicationTargetCombobox, {
    type ApplicationTargetOption
  } from './ApplicationTargetCombobox.svelte';
  import {
    api,
    ApiError,
    type ApplicationDockerContainerCandidate,
    type ApplicationHostProcessCandidate,
    type ApplicationKubernetesNamespaceCandidate,
    type ApplicationKubernetesWorkloadCandidate,
    type Project,
    type Resource,
    type Team
  } from '../../lib/api';

  export let accessMode = 'virtual_machine';
  export let projectId = '';
  export let teamId = '';
  export let instancesJSON = '[]';
  export let projects: Project[] = [];
  export let teams: Team[] = [];
  export let sourceResources: Resource[] = [];
  export let logResources: Resource[] = [];
  export let configurationAttempted = false;
  export let onConfigurationChange: () => void = () => {};
  export let onValidationChange: (invalid: boolean) => void = () => {};

  type Instance = Record<string, any>;
  type TargetState = { options: ApplicationTargetOption[]; loading: boolean; error: string };
  const workloadKinds = ['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob'];
  let instances: Instance[] = [];
  let lastJSON = '';
  let hostStates: Record<string, TargetState> = {};
  let dockerStates: Record<string, TargetState> = {};
  let namespaceStates: Record<string, TargetState> = {};
  let workloadStates: Record<string, TargetState> = {};
  let hostValidation: Record<string, string> = {};
  let workloadInputs: Record<string, string> = {};
  let timers: Record<string, ReturnType<typeof setTimeout>> = {};
  let requestTokens: Record<string, number> = {};

  const modeLabel = () => accessMode === 'virtual_machine' ? '虚拟机' : accessMode === 'containerized' ? '容器化' : '云原生';
  const sourceLabel = () => accessMode === 'virtual_machine' ? 'Host' : accessMode === 'containerized' ? 'Docker' : 'Kubernetes';
  const sourceKey = () => accessMode === 'virtual_machine' ? 'host_resource_id' : accessMode === 'containerized' ? 'docker_resource_id' : 'kubernetes_resource_id';
  const sourceValue = (instance: Instance) => String(instance[sourceKey()] ?? '');
  const teamName = (id: string) => teams.find((item) => item.id === id)?.name ?? id;
  const stateKey = (index: number) => `${accessMode}:${index}`;
  const processKeyword = (instance: Instance) => String(instance.process_keyword ?? '');
  const workloadValue = (index: number, instance: Instance) => workloadInputs[stateKey(index)] ?? (instance.workload_name ? `${instance.workload_kind ?? 'Deployment'} · ${instance.workload_name}` : '');
  const describeError = (error: unknown, fallback: string) => error instanceof ApiError && error.message ? error.message : fallback;

  function emptyInstance(): Instance {
    if (accessMode === 'virtual_machine') return { host_resource_id: '', process_keyword: '', log_source: { type: 'path', path: '' } };
    if (accessMode === 'containerized') return { docker_resource_id: '', container_name: '', log_source: { type: 'stdout' } };
    return { kubernetes_resource_id: '', namespace: '', workload_kind: 'Deployment', workload_name: '', log_source: { type: 'stdout' } };
  }

  function parseInstances(value: string) {
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) && parsed.length ? parsed.filter((item) => item && typeof item === 'object') : [emptyInstance()];
    } catch {
      return [emptyInstance()];
    }
  }

  function syncWorkloadInputs() {
    const next: Record<string, string> = {};
    instances.forEach((instance, index) => {
      if (accessMode === 'cloud_native' && instance.workload_name) next[stateKey(index)] = `${instance.workload_kind ?? 'Deployment'} · ${instance.workload_name}`;
    });
    workloadInputs = next;
  }

  function writeInstances() {
    instancesJSON = JSON.stringify(instances);
    lastJSON = instancesJSON;
    onConfigurationChange();
  }

  function setState(states: Record<string, TargetState>, key: string, next: TargetState) {
    states[key] = next;
    if (states === hostStates) hostStates = states;
    else if (states === dockerStates) dockerStates = states;
    else if (states === namespaceStates) namespaceStates = states;
    else workloadStates = states;
  }

  function clearInstanceDiscovery(index: number) {
    const key = stateKey(index);
    delete hostStates[key];
    delete dockerStates[key];
    delete namespaceStates[key];
    delete workloadStates[key];
    delete hostValidation[key];
    delete requestTokens[key];
    hostStates = hostStates;
    dockerStates = dockerStates;
    namespaceStates = namespaceStates;
    workloadStates = workloadStates;
    hostValidation = hostValidation;
    updateValidationState();
  }

  function updateValidationState() {
    onValidationChange(Object.values(hostValidation).some(Boolean));
  }

  function setHostValidation(index: number, message: string) {
    const key = stateKey(index);
    if (message) hostValidation[key] = message;
    else delete hostValidation[key];
    hostValidation = hostValidation;
    updateValidationState();
  }

  function updateInstance(index: number, key: string, value: any, resetDiscovery = false) {
    instances[index] = { ...instances[index], [key]: value };
    instances = instances;
    if (resetDiscovery) clearInstanceDiscovery(index);
    writeInstances();
  }

  function updateKeywords(index: number, value: string) {
    updateInstance(index, 'process_keyword', value);
    setHostValidation(index, '');
    scheduleHostDiscovery(index, value);
  }

  function updateContainerName(index: number, value: string) {
    updateInstance(index, 'container_name', value);
    scheduleDockerDiscovery(index, value);
  }

  function parseWorkloadValue(value: string, currentKind: string) {
    const separator = value.indexOf('·');
    if (separator < 0) return { kind: currentKind || 'Deployment', name: value.trim() };
    const kind = value.slice(0, separator).trim();
    const name = value.slice(separator + 1).trim();
    return { kind: workloadKinds.includes(kind) ? kind : currentKind || 'Deployment', name };
  }

  function updateWorkload(index: number, value: string) {
    const current = instances[index] ?? {};
    const parsed = parseWorkloadValue(value, String(current.workload_kind ?? 'Deployment'));
    workloadInputs[stateKey(index)] = value;
    workloadInputs = workloadInputs;
    instances[index] = { ...current, workload_kind: parsed.kind, workload_name: parsed.name };
    instances = instances;
    writeInstances();
    scheduleWorkloadDiscovery(index);
  }

  function updateNamespace(index: number, value: string) {
    workloadInputs[stateKey(index)] = '';
    workloadInputs = workloadInputs;
    instances[index] = { ...instances[index], namespace: value, workload_name: '' };
    instances = instances;
    const key = stateKey(index);
    delete workloadStates[key];
    workloadStates = workloadStates;
    writeInstances();
    scheduleWorkloadDiscovery(index);
  }

  function updateLogSource(index: number, type: string) {
    const current = instances[index]?.log_source ?? {};
    const next = type === 'stdout' ? { type: 'stdout' } : { type, [type === 'path' ? 'path' : 'query']: current.path ?? current.query ?? '', ...(type === 'query' ? { resource_id: current.resource_id ?? '' } : {}) };
    instances[index] = { ...instances[index], log_source: next };
    instances = instances;
    writeInstances();
  }

  function addInstance() {
    instances = [...instances, emptyInstance()];
    syncWorkloadInputs();
    writeInstances();
  }

  function removeInstance(index: number) {
    if (instances.length === 1) return;
    instances = instances.filter((_, itemIndex) => itemIndex !== index);
    syncWorkloadInputs();
    hostStates = {};
    dockerStates = {};
    namespaceStates = {};
    workloadStates = {};
    hostValidation = {};
    updateValidationState();
    writeInstances();
  }

  function switchMode() {
    instances = [emptyInstance()];
    hostStates = {};
    dockerStates = {};
    namespaceStates = {};
    workloadStates = {};
    hostValidation = {};
    workloadInputs = {};
    updateValidationState();
    writeInstances();
  }

  function schedule(key: string, callback: () => void, delay = 320) {
    if (timers[key]) clearTimeout(timers[key]);
    timers[key] = setTimeout(callback, delay);
  }

  function scheduleHostDiscovery(index: number, keyword: string) {
    const sourceID = sourceValue(instances[index] ?? {});
    const state = stateKey(index);
    if (!sourceID || !keyword.trim()) {
      delete hostStates[state];
      hostStates = hostStates;
      return;
    }
    schedule(`host:${state}`, () => void loadHostProcesses(index, keyword), 320);
  }

  function scheduleDockerDiscovery(index: number, keyword: string) {
    const sourceID = sourceValue(instances[index] ?? {});
    if (!sourceID) return;
    schedule(`docker:${stateKey(index)}`, () => void loadDockerContainers(index, keyword), 260);
  }

  function scheduleWorkloadDiscovery(index: number) {
    const sourceID = sourceValue(instances[index] ?? {});
    const namespace = String(instances[index]?.namespace ?? '').trim();
    if (!sourceID || !namespace) return;
    schedule(`workload:${stateKey(index)}`, () => void loadWorkloads(index, namespace), 320);
  }

  function nextToken(key: string) {
    requestTokens[key] = (requestTokens[key] ?? 0) + 1;
    return requestTokens[key];
  }

  function isCurrent(key: string, token: number) {
    return requestTokens[key] === token;
  }

  async function loadHostProcesses(index: number, keyword = processKeyword(instances[index] ?? {})) {
    keyword = keyword.trim();
    const resourceID = sourceValue(instances[index] ?? {});
    const key = stateKey(index);
    if (!resourceID || !keyword) return;
    const token = nextToken(`host:${key}`);
    setState(hostStates, key, { options: hostStates[key]?.options ?? [], loading: true, error: '' });
    try {
      const result = await api.applicationHostProcesses(resourceID, keyword, 50);
      if (!isCurrent(`host:${key}`, token)) return;
      const options = result.processes.map((process: ApplicationHostProcessCandidate) => ({
        value: keyword,
        label: `${process.name || process.executable || `PID ${process.pid}`} · PID ${process.pid}`,
        description: [process.executable, process.command_line].filter(Boolean).join(' · '),
        id: String(process.pid),
        metadata: { pid: process.pid }
      }));
      setState(hostStates, key, { options, loading: false, error: '' });
    } catch (error) {
      if (isCurrent(`host:${key}`, token)) setState(hostStates, key, { options: [], loading: false, error: describeError(error, 'Host 进程候选获取失败，可继续手动填写') });
    }
  }

  async function validateHostSelection(index: number, option: ApplicationTargetOption) {
    const keyword = processKeyword(instances[index] ?? {}).trim();
    const resourceID = sourceValue(instances[index] ?? {});
    const pid = Number(option.metadata?.pid ?? option.id ?? 0);
    if (!keyword || !resourceID || !pid) return;
    try {
      const result = await api.validateApplicationHostProcess(resourceID, { keyword, pid });
      if (processKeyword(instances[index] ?? {}).trim() !== keyword || sourceValue(instances[index] ?? {}) !== resourceID) return;
      setHostValidation(index, result.valid ? '' : result.message || '搜索关键字不能唯一定位选中的进程');
    } catch (error) {
      if (processKeyword(instances[index] ?? {}).trim() !== keyword || sourceValue(instances[index] ?? {}) !== resourceID) return;
      setHostValidation(index, describeError(error, '无法校验进程唯一性，请重新搜索'));
    }
  }

  async function loadDockerContainers(index: number, keyword = String(instances[index]?.container_name ?? '')) {
    const resourceID = sourceValue(instances[index] ?? {});
    const key = stateKey(index);
    if (!resourceID) return;
    const token = nextToken(`docker:${key}`);
    setState(dockerStates, key, { options: dockerStates[key]?.options ?? [], loading: true, error: '' });
    try {
      const result = await api.applicationDockerContainers(resourceID, keyword.trim(), 100);
      if (!isCurrent(`docker:${key}`, token)) return;
      const options = result.containers.map((container: ApplicationDockerContainerCandidate) => ({
        value: container.name,
        label: container.name,
        description: [container.image, container.status || container.state].filter(Boolean).join(' · '),
        id: container.id
      }));
      setState(dockerStates, key, { options, loading: false, error: '' });
    } catch (error) {
      if (isCurrent(`docker:${key}`, token)) setState(dockerStates, key, { options: [], loading: false, error: describeError(error, 'Docker 容器候选获取失败，可继续手动填写') });
    }
  }

  async function loadNamespaces(index: number) {
    const resourceID = sourceValue(instances[index] ?? {});
    const key = stateKey(index);
    if (!resourceID) return;
    const token = nextToken(`namespace:${key}`);
    setState(namespaceStates, key, { options: namespaceStates[key]?.options ?? [], loading: true, error: '' });
    try {
      const result = await api.applicationKubernetesNamespaces(resourceID, false, 100);
      if (!isCurrent(`namespace:${key}`, token)) return;
      const options = result.namespaces.map((namespace: ApplicationKubernetesNamespaceCandidate) => ({ value: namespace.name, label: namespace.name }));
      setState(namespaceStates, key, { options, loading: false, error: '' });
    } catch (error) {
      if (isCurrent(`namespace:${key}`, token)) setState(namespaceStates, key, { options: [], loading: false, error: describeError(error, 'Kubernetes 命名空间获取失败，可继续手动填写') });
    }
  }

  async function loadWorkloads(index: number, namespace = String(instances[index]?.namespace ?? '').trim()) {
    const resourceID = sourceValue(instances[index] ?? {});
    const key = stateKey(index);
    if (!resourceID || !namespace) return;
    const token = nextToken(`workload:${key}`);
    setState(workloadStates, key, { options: workloadStates[key]?.options ?? [], loading: true, error: '' });
    try {
      const result = await api.applicationKubernetesWorkloads(resourceID, namespace, 100);
      if (!isCurrent(`workload:${key}`, token)) return;
      const options = result.workloads.map((workload: ApplicationKubernetesWorkloadCandidate) => ({
        value: `${workload.kind} · ${workload.name}`,
        label: `${workload.kind} · ${workload.name}`,
        description: [workload.ready === undefined ? '' : workload.ready ? 'Ready' : 'Not ready', workload.phase, workload.age].filter(Boolean).join(' · '),
        metadata: { kind: workload.kind, name: workload.name }
      }));
      setState(workloadStates, key, { options, loading: false, error: '' });
    } catch (error) {
      if (isCurrent(`workload:${key}`, token)) setState(workloadStates, key, { options: [], loading: false, error: describeError(error, 'Kubernetes 工作负载获取失败，可继续手动填写') });
    }
  }

  function hostOptionSelected(index: number, option: ApplicationTargetOption) {
    void validateHostSelection(index, option);
  }

  function dockerOptionSelected(index: number, option: ApplicationTargetOption) {
    updateContainerName(index, option.value);
  }

  function namespaceOptionSelected(index: number, option: ApplicationTargetOption) {
    updateNamespace(index, option.value);
    void loadWorkloads(index, option.value);
  }

  function workloadOptionSelected(index: number, option: ApplicationTargetOption) {
    const kind = String(option.metadata?.kind ?? 'Deployment');
    const name = String(option.metadata?.name ?? option.value);
    workloadInputs[stateKey(index)] = `${kind} · ${name}`;
    workloadInputs = workloadInputs;
    instances[index] = { ...instances[index], workload_kind: kind, workload_name: name };
    instances = instances;
    writeInstances();
  }

  function switchSourceResource(index: number, value: string) {
    updateInstance(index, sourceKey(), value, true);
    if (accessMode === 'cloud_native' && value) void loadNamespaces(index);
  }

  $: if (instancesJSON !== lastJSON) {
    instances = parseInstances(instancesJSON);
    lastJSON = instancesJSON;
    syncWorkloadInputs();
  }
  $: if (projectId) {
    const project = projects.find((item) => item.id === projectId);
    if (project && teamId !== project.team_id) teamId = project.team_id;
  }

  onMount(() => {
    instances = parseInstances(instancesJSON);
    lastJSON = instancesJSON;
    syncWorkloadInputs();
  });

  onDestroy(() => Object.values(timers).forEach((timer) => clearTimeout(timer)));
</script>

<div class="schema-inputs application-config-step">
  <p class="eyebrow">APPLICATION INSTANCES</p>
  <div class="application-owner-grid">
    <label class:invalid={configurationAttempted && !teamId}>
      <span><i>*</i>所属团队</span>
      <select bind:value={teamId} on:change={() => { projectId = ''; onConfigurationChange(); }}>
        <option value="">选择团队</option>
        {#each teams as team}<option value={team.id}>{team.name}</option>{/each}
      </select>
    </label>
    <label class:invalid={configurationAttempted && !projectId}>
      <span><i>*</i>归属项目</span>
      <select bind:value={projectId} on:change={() => onConfigurationChange()}>
        <option value="">选择项目</option>
        {#each projects.filter((project) => !teamId || project.team_id === teamId) as project}
          <option value={project.id}>{project.name} · {teamName(project.team_id)}</option>
        {/each}
      </select>
    </label>
  </div>
  <label>
    <span><i>*</i>接入方式</span>
    <select bind:value={accessMode} on:change={switchMode}>
      <option value="virtual_machine">虚拟机</option>
      <option value="containerized">容器化</option>
      <option value="cloud_native">云原生</option>
    </select>
  </label>

  <div class="application-instance-list">
    {#each instances as instance, index}
      {@const state = stateKey(index)}
      <article class="application-instance-editor">
        <header><strong>Instance {index + 1}</strong><button class="secondary" type="button" disabled={instances.length === 1} on:click={() => removeInstance(index)}>删除</button></header>
        <label class:invalid={configurationAttempted && !sourceValue(instance)}>
          <span><i>*</i>{sourceLabel()}资源</span>
          <select value={sourceValue(instance)} on:change={(event) => switchSourceResource(index, (event.currentTarget as HTMLSelectElement).value)}>
            <option value="">选择{sourceLabel()}资源</option>
            {#each sourceResources as resource}<option value={resource.id}>{resource.name}</option>{/each}
          </select>
        </label>

        {#if accessMode === 'virtual_machine'}
          <label class:invalid={Boolean(hostValidation[state]) || (configurationAttempted && !processKeyword(instance).trim())}>
            <span><i>*</i>进程搜索关键字</span>
            <ApplicationTargetCombobox
              value={processKeyword(instance)}
              options={hostStates[state]?.options ?? []}
              loading={hostStates[state]?.loading ?? false}
              error={hostValidation[state] || hostStates[state]?.error || ''}
              invalid={Boolean(hostValidation[state]) || (configurationAttempted && !processKeyword(instance).trim())}
              placeholder="输入关键字，支持 &、|、逗号或空格"
              onInput={(value) => updateKeywords(index, value)}
              onFocus={() => scheduleHostDiscovery(index, processKeyword(instance))}
              onRefresh={() => void loadHostProcesses(index)}
              onSelect={(option) => hostOptionSelected(index, option)}
            />
          </label>
        {:else if accessMode === 'containerized'}
          <label class:invalid={configurationAttempted && !String(instance.container_name ?? '').trim()}>
            <span><i>*</i>容器名称</span>
            <ApplicationTargetCombobox
              value={instance.container_name ?? ''}
              options={dockerStates[state]?.options ?? []}
              loading={dockerStates[state]?.loading ?? false}
              error={dockerStates[state]?.error ?? ''}
              invalid={configurationAttempted && !String(instance.container_name ?? '').trim()}
              placeholder="搜索运行中的容器，也可直接填写名称"
              onInput={(value) => updateContainerName(index, value)}
              onFocus={() => void loadDockerContainers(index)}
              onRefresh={() => void loadDockerContainers(index)}
              onSelect={(option) => dockerOptionSelected(index, option)}
            />
          </label>
        {:else}
          <label class:invalid={configurationAttempted && !String(instance.namespace ?? '').trim()}>
            <span><i>*</i>命名空间</span>
            <ApplicationTargetCombobox
              value={instance.namespace ?? ''}
              options={namespaceStates[state]?.options ?? []}
              loading={namespaceStates[state]?.loading ?? false}
              error={namespaceStates[state]?.error ?? ''}
              invalid={configurationAttempted && !String(instance.namespace ?? '').trim()}
              placeholder="搜索命名空间，也可直接填写"
              onInput={(value) => updateNamespace(index, value)}
              onFocus={() => void loadNamespaces(index)}
              onRefresh={() => void loadNamespaces(index)}
              onSelect={(option) => namespaceOptionSelected(index, option)}
            />
          </label>
          <label class:invalid={configurationAttempted && !String(instance.workload_name ?? '').trim()}>
            <span><i>*</i>工作负载（类型 · 名称）</span>
            <ApplicationTargetCombobox
              value={workloadValue(index, instance)}
              options={workloadStates[state]?.options ?? []}
              loading={workloadStates[state]?.loading ?? false}
              error={workloadStates[state]?.error ?? ''}
              invalid={configurationAttempted && !String(instance.workload_name ?? '').trim()}
              placeholder="选择或填写 Deployment · workload-name"
              onInput={(value) => updateWorkload(index, value)}
              onFocus={() => void loadWorkloads(index)}
              onRefresh={() => void loadWorkloads(index)}
              onSelect={(option) => workloadOptionSelected(index, option)}
            />
          </label>
        {/if}

        <label>
          <span>{modeLabel()}日志来源</span>
          <select value={instance.log_source?.type ?? (accessMode === 'virtual_machine' ? 'path' : 'stdout')} on:change={(event) => updateLogSource(index, (event.currentTarget as HTMLSelectElement).value)}>
            {#if accessMode !== 'virtual_machine'}<option value="stdout">容器标准输出</option>{/if}
            <option value="path">{accessMode === 'virtual_machine' ? 'Host主机路径' : '容器内路径'}</option>
            <option value="query">日志平台查询</option>
          </select>
        </label>
        {#if instance.log_source?.type === 'path'}
          <label><span><i>{accessMode === 'virtual_machine' ? '*' : ''}</i>日志路径</span><input value={instance.log_source.path ?? ''} placeholder="/var/log/application.log" on:input={(event) => { instances[index] = { ...instance, log_source: { ...instance.log_source, path: (event.currentTarget as HTMLInputElement).value } }; instances = instances; writeInstances(); }} /></label>
        {:else if instance.log_source?.type === 'query'}
          <label><span><i>*</i>日志平台</span><select value={instance.log_source.resource_id ?? ''} on:change={(event) => { instances[index] = { ...instance, log_source: { ...instance.log_source, resource_id: (event.currentTarget as HTMLSelectElement).value } }; instances = instances; writeInstances(); }}><option value="">选择 Loki</option>{#each logResources as resource}<option value={resource.id}>{resource.name}</option>{/each}</select></label>
          <label><span><i>*</i>查询语句</span><textarea rows="2" value={instance.log_source.query ?? ''} on:input={(event) => { instances[index] = { ...instance, log_source: { ...instance.log_source, query: (event.currentTarget as HTMLTextAreaElement).value } }; instances = instances; writeInstances(); }}></textarea></label>
        {/if}
      </article>
    {/each}
  </div>
  <button class="secondary" type="button" on:click={addInstance}>+ 添加 Instance</button>
  <small class="muted">Application 必须归属于项目，并通过一种接入方式关联现有 {sourceLabel()} 资源。候选项仅用于辅助填写，保存时仍会校验目标是否存在且唯一。</small>
  {#if sourceResources.length === 0}<small class="muted">当前可见范围没有可关联的 {sourceLabel()} 资源。</small>{/if}
</div>
