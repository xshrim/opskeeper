<script lang="ts">
  import {
    AppWindow,
    AlertTriangle,
    ArrowRight,
    Boxes,
    Check,
    CircleHelp,
    Cloud,
    Database,
    HardDrive,
    Layers3,
    Link2,
    Network,
    Pencil,
    Plus,
    Server,
    StickyNote,
    Trash2,
    X
  } from 'lucide-svelte';
  import DeleteConfirmationDialog from '../../components/DeleteConfirmationDialog.svelte';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
import type { Application, Project, ProjectTopology, Resource } from '../../lib/api';
import { api } from '../../lib/api';
import { resourceCategoryFor, resourceCategoryIcon, resourceCategoryOptions } from '../resource/resourceCatalog';

  export let project: Project;
  export let topology: ProjectTopology | null = null;
  export let applications: Application[] = [];
  export let resources: Resource[] = [];
  export let loading = false;
  export let mode: 'browse' | 'edit' = 'browse';
  export let configurationOpen = false;
  export let onRefresh: () => void = () => {};
  type ComponentIntent = { kind: 'application' | 'resource' | 'annotation'; id?: string; resourceKind?: string; runtimeKind?: string; position?: Point };
  export let onConfigureNode: (intent: ComponentIntent) => void = () => {};
  export let onEditApplication: (application: Application) => void = () => {};
  export let applicationAlertCounts: Record<string, number> = {};
  export let onSelectionChange: (selection: { node: ProjectTopology['nodes'][number] | null; edge: ProjectTopology['edges'][number] | null }) => void = () => {};
  export let onNotice: (message: string) => void = () => {};
  export let onError: (message: string) => void = () => {};

  type Node = ProjectTopology['nodes'][number];
  type Edge = ProjectTopology['edges'][number];
  type Point = { x: number; y: number };
  const resourceKinds = Object.keys(resourceCategoryOptions).filter((kind) => kind !== '全部' && !['Host', 'Docker', 'Kubernetes'].includes(kind));
  const applicationRuntimes = [
    { kind: 'virtual_machine', label: '虚拟机应用' },
    { kind: 'containerized', label: '容器化应用' },
    { kind: 'cloud_native', label: '云原生应用' }
  ];

  let selectedNodeID = '';
  let selectedEdgeID = '';
  let connectionSource = '';
  let connectionPointer: Point | null = null;
  let nodeDrag: { id: string; start: Point; origin: Point; moved: boolean } | null = null;
  let suppressNodeClickID = '';
  let dragPosition: { id: string; point: Point } | null = null;
  let connectionDraft: { sourceID: string; targetID: string } | null = null;
  let edgeKind = 'other';
  let edgeBinding = '';
  let resourceRelationKind = 'depends_on';
  let busy = false;
  let deletingNode: Node | null = null;
  $: nodes = topology?.nodes ?? [];
  $: edges = topology?.edges ?? [];
  $: selectedNode = nodes.find((node) => node.id === selectedNodeID) ?? null;
  $: selectedEdge = edges.find((edge) => edge.id === selectedEdgeID) ?? null;
  $: if (mode === 'browse') {
    connectionDraft = null;
    connectionSource = '';
    connectionPointer = null;
  }
  $: displayNodes = nodes.filter((node) => node.kind !== 'project' || node.id !== project.id);
  $: positions = new Map(displayNodes.map((node, index) => [node.id, dragPosition?.id === node.id ? dragPosition.point : node.position ?? pointFor(index)]));
  $: graphHeight = Math.max(370, Math.ceil(displayNodes.length / 5) * 100 + 70, ...[...positions.values()].map((point) => point.y + 74));
  $: displayEdges = edges.filter((edge) => positions.has(edge.source_id) && positions.has(edge.target_id));
  $: highlightedNodeIDs = selectedNode ? linkedNodes(selectedNode.id) : new Set<string>();
  $: if (connectionDraft) {
    const source = nodes.find((node) => node.id === connectionDraft?.sourceID);
    const target = nodes.find((node) => node.id === connectionDraft?.targetID);
    if ((source?.kind === 'resource' && target?.kind === 'application') || (source?.kind === 'application' && source.project_id !== project.id && target?.kind === 'application' && target.project_id === project.id)) {
      connectionDraft = { sourceID: target.id, targetID: source.id };
    }
  }

  function pointFor(index: number): Point {
    return { x: 36 + (index % 5) * 220, y: 42 + Math.floor(index / 5) * 100 };
  }

  function isExternal(node: Node) {
    return node.kind === 'project'
      || (node.kind === 'application' && node.project_id !== project.id)
      || Boolean(node.metadata?.external);
  }

  function isRuntime(node: Node) {
    return node.kind === 'resource' && ['Host', 'Docker', 'Kubernetes'].includes(node.resource_kind ?? '');
  }

  function linkedNodes(nodeID: string) {
    const linked = new Set([nodeID]);
    for (const edge of edges) {
      if (edge.source_id === nodeID) linked.add(edge.target_id);
      if (edge.target_id === nodeID) linked.add(edge.source_id);
    }
    return linked;
  }

  function edgePath(edge: Edge) {
    const source = positions.get(edge.source_id);
    const target = positions.get(edge.target_id);
    if (!source || !target) return '';
    const sourceIsLeft = source.x <= target.x;
    const x1 = source.x + (sourceIsLeft ? 109 : 0);
    const y1 = source.y + 22;
    const x2 = target.x + (sourceIsLeft ? 0 : 109);
    const y2 = target.y + 22;
    const bend = Math.max(48, Math.abs(x2 - x1) * 0.42);
    return `M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}`;
  }

  function edgeLabel(edge: Edge) {
    if (edge.relation === 'runs_on') return '运行于';
    if (edge.relation === 'uses') return String(edge.metadata?.kind ?? '资源关联');
    if (edge.relation === 'depends_on') return '依赖';
    if (edge.relation === 'contains') return '包含';
    return edge.relation.replaceAll('_', ' ');
  }

  function nodeSubtitle(node: Node) {
    if (node.kind === 'application') {
      const runtime = String(node.metadata?.runtime_kind ?? '');
      const runtimeName = runtime === 'virtual_machine' ? 'Host' : runtime === 'containerized' ? 'Docker' : runtime === 'cloud_native' ? 'Kubernetes' : '运行时未定';
      return `${String(node.metadata?.code ?? '')} · ${runtimeName}`;
    }
    if (node.kind === 'project') return String(node.metadata?.code ?? '外部项目');
    if (node.kind === 'annotation') return String(node.metadata?.description ?? '拓扑说明');
    return `${node.resource_kind ?? '资源'}${node.metadata?.subtype ? ` · ${String(node.metadata.subtype)}` : ''}`;
  }

  function nodeIcon(node: Node) {
    if (node.kind === 'application') return AppWindow;
    if (node.kind === 'project') return Layers3;
    if (node.kind === 'annotation') return StickyNote;
    if (node.resource_kind === 'Host') return Server;
    if (node.resource_kind === 'Docker') return Boxes;
    if (node.resource_kind === 'Kubernetes') return Cloud;
    if (['PostgreSQL', 'MySQL', 'Oracle', 'OceanBase'].includes(node.resource_kind ?? '')) return Database;
    return HardDrive;
  }

  function selectNode(node: Node) {
    if (suppressNodeClickID === node.id) {
      suppressNodeClickID = '';
      return;
    }
    selectedNodeID = node.id;
    selectedEdgeID = '';
    onSelectionChange({ node, edge: null });
  }

  function beginNodeDrag(event: PointerEvent, node: Node) {
    if (mode !== 'edit' || event.button !== 0 || (event.target as HTMLElement).closest('[data-topology-anchor]')) return;
    const point = positions.get(node.id);
    if (!point) return;
    nodeDrag = { id: node.id, start: { x: event.clientX, y: event.clientY }, origin: point, moved: false };
  }

  function moveNode(event: PointerEvent) {
    if (!nodeDrag) return;
    const dx = event.clientX - nodeDrag.start.x;
    const dy = event.clientY - nodeDrag.start.y;
    if (!nodeDrag.moved && Math.hypot(dx, dy) < 4) return;
    nodeDrag.moved = true;
    suppressNodeClickID = nodeDrag.id;
    const point = {
      x: Math.max(0, Math.min(1171, nodeDrag.origin.x + dx)),
      y: Math.max(0, Math.min(9960, nodeDrag.origin.y + dy))
    };
    dragPosition = { id: nodeDrag.id, point };
    positions = new Map(positions).set(nodeDrag.id, point);
  }

  async function finishNodeDrag() {
    if (!nodeDrag) return;
    const drag = nodeDrag;
    nodeDrag = null;
    if (!drag.moved || !dragPosition || dragPosition.id !== drag.id) return;
    window.setTimeout(() => {
      if (suppressNodeClickID === drag.id) suppressNodeClickID = '';
    }, 0);
    try {
      await api.setProjectTopologyPosition(project.id, drag.id, { x: Math.round(dragPosition.point.x), y: Math.round(dragPosition.point.y) });
      await onRefresh();
      dragPosition = null;
    } catch (error) {
      dragPosition = null;
      onError(error instanceof Error ? error.message : '保存节点位置失败');
    }
  }

  function beginConnection(event: PointerEvent, node: Node) {
    if (mode !== 'edit') return;
    event.preventDefault();
    event.stopPropagation();
    connectionSource = node.id;
    connectionPointer = { x: event.clientX, y: event.clientY };
    selectedNodeID = node.id;
    selectedEdgeID = '';
  }

  function moveConnection(event: PointerEvent) {
    if (connectionSource) connectionPointer = { x: event.clientX, y: event.clientY };
  }

  async function finishConnection(event: PointerEvent) {
    if (!connectionSource) return;
    const anchor = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>('[data-topology-anchor]');
    const targetID = anchor?.dataset.topologyAnchor ?? '';
    const sourceID = connectionSource;
    connectionSource = '';
    connectionPointer = null;
    if (!targetID || targetID === sourceID) return;
    const source = nodes.find((node) => node.id === sourceID);
    const target = nodes.find((node) => node.id === targetID);
    if (!source || !target || source.kind === 'project' || target.kind === 'project') return;
    connectionDraft = { sourceID, targetID };
    edgeKind = 'other';
    edgeBinding = '';
    resourceRelationKind = 'depends_on';
  }

  function dropKind(event: DragEvent) {
    event.preventDefault();
    if (configurationOpen || mode !== 'edit') return;
    const raw = event.dataTransfer?.getData('application/x-opskeeper-topology-component');
    if (!raw) return;
    try {
      const container = event.currentTarget as HTMLElement;
      const canvas = container.querySelector<HTMLElement>('.topology-canvas');
      const bounds = (canvas ?? container).getBoundingClientRect();
      const point = { x: Math.max(0, Math.min(1171, event.clientX - bounds.left - 54)), y: Math.max(0, event.clientY - bounds.top - 22) };
      onConfigureNode({ ...JSON.parse(raw), position: point });
    } catch { onError('无法识别拖入的拓扑组件'); }
  }

  function dragComponent(event: DragEvent, intent: ComponentIntent) {
    event.dataTransfer?.setData('application/x-opskeeper-topology-component', JSON.stringify(intent));
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'copy';
  }

  function detailFloatStyle(node: Node) {
    const point = positions.get(node.id) ?? { x: 0, y: 0 };
    const horizontal = point.x < 640 ? 'left:auto;right:14px' : 'left:14px;right:auto';
    const vertical = point.y < graphHeight / 2 ? 'top:auto;bottom:14px' : 'top:14px;bottom:auto';
    return `${horizontal};${vertical}`;
  }


  function editRuntimeBinding(nodeID: string) {
    const node = nodes.find((item) => item.id === nodeID);
    connectionDraft = null;
    if (node?.kind === 'application') onEditApplication(node as unknown as Application);
  }

  function applicationPayload(application: Application, relations = application.relations, dependencies = application.dependencies) {
    return {
      name: application.name,
      code: application.code,
      description: application.description,
      icon: application.icon,
      runtime_kind: application.runtime_kind,
      external_uid: application.external_uid,
      labels: application.labels,
      critical: application.critical,
      technical_owner_ids: application.technical_owner_ids,
      operations_owner_ids: application.operations_owner_ids,
      relations: relations.map((item) => ({ target_application_id: item.target_application_id, relation_kind: item.relation_kind, status: item.status })),
      dependencies: dependencies.map((item) => ({ target_resource_id: item.target_resource_id, dependency_kind: item.dependency_kind, binding: item.binding, required: item.required, status: item.status })),
      instances: application.instances.map((item) => ({ name: item.name, runtime_kind: item.runtime_kind, target_resource_id: item.target_resource_id, selector: item.selector, log_binding: item.log_binding, status: item.status }))
    };
  }

  function parseBinding(value: string) {
    return Object.fromEntries(value.split(',').map((item) => item.trim()).filter(Boolean).map((item) => {
      const split = item.indexOf('=');
      return split < 0 ? [item, ''] : [item.slice(0, split).trim(), item.slice(split + 1).trim()];
    }));
  }

  async function saveConnection() {
    if (!connectionDraft || busy) return;
    const source = nodes.find((node) => node.id === connectionDraft?.sourceID);
    const target = nodes.find((node) => node.id === connectionDraft?.targetID);
    if (!source || !target) return;
    busy = true;
    try {
      if (source.kind === 'application' && target.kind === 'application') {
        const app = await api.application(project.id, source.id);
        if (app.relations.some((item) => item.target_application_id === target.id)) throw new Error('这两个应用已存在依赖关系');
        await api.updateApplication(project.id, app.id, applicationPayload(app, [...app.relations, { target_application_id: target.id, relation_kind: 'depends_on', status: 'active' } as Application['relations'][number]]));
      } else if (source.kind === 'application' && target.kind === 'resource') {
        if (isRuntime(target)) throw new Error('运行时绑定请通过应用编辑向导配置，以保留实例唯一定位校验');
        const app = await api.application(project.id, source.id);
        if (app.dependencies.some((item) => item.target_resource_id === target.id)) throw new Error('该应用已关联此资源');
        await api.updateApplication(project.id, app.id, applicationPayload(app, app.relations, [...app.dependencies, { target_resource_id: target.id, dependency_kind: edgeKind, binding: parseBinding(edgeBinding), required: false, status: 'active' } as Application['dependencies'][number]]));
      } else if (source.kind === 'resource' && target.kind === 'resource') {
        await api.createRelation(source.id, { target_resource_id: target.id, relation_type: resourceRelationKind, attributes: {}, discovery_source: 'manual', confidence: 1, confirmed: true });
      } else {
        throw new Error('该连线方向不支持直接创建，请从应用端拖出连线');
      }
      connectionDraft = null;
      await onRefresh();
      onNotice('拓扑关系已保存');
    } catch (error) {
      onError(error instanceof Error ? error.message : '创建拓扑关系失败');
    } finally {
      busy = false;
    }
  }

  async function removeEdge(edge: Edge) {
    if (busy) return;
    busy = true;
    try {
      if (edge.id.startsWith('resource_relation:')) {
        await api.deleteRelation(edge.source_id, edge.id.slice('resource_relation:'.length));
      } else if (edge.id.startsWith('instance:')) {
        const app = await api.application(project.id, edge.source_id);
        const instance = app.instances.find((item) => item.id === edge.id.slice('instance:'.length));
        if (!instance) throw new Error('实例绑定已不存在');
        await api.deleteApplicationInstance(project.id, app.id, instance.id);
      } else {
        const app = await api.application(project.id, edge.source_id);
        if (edge.id.startsWith('binding:')) {
          const dependencyID = edge.id.slice('binding:'.length);
          await api.updateApplication(project.id, app.id, applicationPayload(app, app.relations, app.dependencies.filter((item) => item.id !== dependencyID)));
        } else if (edge.id.startsWith('relation:')) {
          const relationID = edge.id.slice('relation:'.length);
          await api.updateApplication(project.id, app.id, applicationPayload(app, app.relations.filter((item) => item.id !== relationID)));
        }
      }
      selectedEdgeID = '';
      await onRefresh();
      onNotice('拓扑连线已移除');
    } catch (error) {
      onError(error instanceof Error ? error.message : '移除拓扑连线失败');
    } finally {
      busy = false;
    }
  }

  async function confirmRemoveNode() {
    const node = deletingNode;
    if (!node || busy) return;
    busy = true;
    try {
      if (node.kind === 'application') await api.deleteApplication(project.id, node.id);
      else if (node.kind === 'resource') await api.removeProjectTopologyResource(project.id, node.id);
      else if (node.kind === 'annotation') await api.deleteProjectTopologyAnnotation(project.id, node.id);
      else throw new Error('项目节点不可删除');
      deletingNode = null;
      selectedNodeID = '';
      await onRefresh();
      onNotice('拓扑节点已删除');
    } catch (error) {
      onError(error instanceof Error ? error.message : '删除拓扑节点失败');
    } finally {
      busy = false;
    }
  }
</script>

<svelte:window on:pointermove={moveConnection} on:pointermove={moveNode} on:pointerup={finishConnection} on:pointerup={finishNodeDrag} />

<section class="topology-session">
  <div class:editing={mode === 'edit'} class:configuring={configurationOpen} class="topology-workspace">
    {#if mode === 'edit'}
      <aside class="topology-palette">
        <div class="topology-palette-title"><strong>组件</strong><small>拖到画布</small></div>
        <details class="topology-palette-section" open>
          <summary><StickyNote size={14} />通用</summary>
          <button class="topology-palette-item annotation" draggable="true" on:dragstart={(event) => dragComponent(event, { kind: 'annotation' })}>
            <StickyNote size={16} /><span><strong>说明块</strong><small>添加文字说明</small></span>
          </button>
        </details>
        <details class="topology-palette-section" open>
          <summary><EntityBrandIcon kind="Application" fallback="lucide:AppWindow" size={15} />应用</summary>
          {#each applicationRuntimes as runtime (runtime.kind)}
            {@const runtimeApps = applications.filter((application) => application.runtime_kind === runtime.kind)}
            <details class="topology-palette-subsection">
              <summary><span>{runtime.label}</span><small>{runtimeApps.length}</small></summary>
              <button class="topology-palette-item application template" draggable="true" on:dragstart={(event) => dragComponent(event, { kind: 'application', runtimeKind: runtime.kind })}>
                <EntityBrandIcon kind="Application" fallback="lucide:AppWindow" size={16} /><span><strong>新增{runtime.label}</strong><small>拖入后配置并创建</small></span>
              </button>
              {#each runtimeApps as application (application.id)}
                <button class="topology-palette-item application" disabled={nodes.some((node) => node.kind === 'application' && node.id === application.id)} draggable={!nodes.some((node) => node.kind === 'application' && node.id === application.id)} on:dragstart={(event) => dragComponent(event, { kind: 'application', id: application.id, runtimeKind: runtime.kind })}>
                  <EntityBrandIcon kind="Application" fallback={application.icon || 'lucide:AppWindow'} size={16} /><span><strong>{application.name}</strong><small>{application.code}</small></span>
                </button>
              {/each}
              {#if !runtimeApps.length}<small class="topology-palette-empty">暂无此类应用</small>{/if}
            </details>
          {/each}
        </details>
        <details class="topology-palette-section" open>
          <summary><HardDrive size={14} />资源</summary>
          {#each resourceKinds as kind (kind)}
            {@const kindResources = resources.filter((resource) => resourceCategoryFor(resource) === kind)}
            <details class="topology-palette-subsection">
              <summary><span>{kind}</span><small>{kindResources.length}</small></summary>
              <button class="topology-palette-item resource template" draggable="true" on:dragstart={(event) => dragComponent(event, { kind: 'resource', resourceKind: kind })}>
                <ResourceBrandIcon resource={{ kind }} fallback={resourceCategoryIcon(kind)} size={16} /><span><strong>新增{kind}资源</strong><small>拖入后打开资源配置</small></span>
              </button>
              {#each kindResources as resource (resource.id)}
                {@const alreadyAdded = nodes.some((node) => node.kind === 'resource' && node.id === resource.id)}
                <button class:already-added={alreadyAdded} class="topology-palette-item resource" disabled={alreadyAdded} draggable={!alreadyAdded} on:dragstart={(event) => dragComponent(event, { kind: 'resource', id: resource.id, resourceKind: kind })}>
                  <ResourceBrandIcon {resource} fallback={resourceCategoryIcon(kind)} size={16} /><span><strong>{resource.name}</strong><small>{resource.subtype || kind}</small></span>
                </button>
              {/each}
              {#if !kindResources.length}<small class="topology-palette-empty">暂无此类资源</small>{/if}
            </details>
          {/each}
        </details>
      </aside>
    {/if}

    <main class:configuration-open={configurationOpen} class="topology-canvas-wrap" on:dragover|preventDefault on:drop={dropKind}>
      {#if loading}<div class="topology-state">正在加载项目拓扑…</div>
      {:else if !displayNodes.length}<div class="topology-state"><Network size={28} /><strong>暂无拓扑节点</strong><span>{mode === 'edit' ? '从左侧拖入应用或资源开始构建。' : '当前项目还没有应用或关联资源。'}</span></div>
      {:else}
        <div class="topology-legend"><span class="application-key"><i></i>应用</span><span class="runtime-key"><i></i>运行时资源</span><span class="resource-key"><i></i>其他资源</span><span class="external-key"><i></i>项目外对象</span></div>
        <div class="topology-scroll">
          <div class="topology-canvas" style={`height:${graphHeight}px;`}>
            <svg class="topology-edges" width="1280" height={graphHeight} aria-hidden="true">
              {#each displayEdges as edge (edge.id)}
                {@const faded = (selectedNode && !highlightedNodeIDs.has(edge.source_id) && !highlightedNodeIDs.has(edge.target_id)) || (selectedEdge && selectedEdge.id !== edge.id)}
                <path role="button" tabindex="0" aria-label={`${edgeLabel(edge)}关系`} class:dimmed={faded} class:selected={selectedEdgeID === edge.id} class={`edge-${edge.relation}`} d={edgePath(edge)} on:click|stopPropagation={() => { selectedEdgeID = edge.id; selectedNodeID = ''; onSelectionChange({ node: null, edge }); }} on:keydown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectedEdgeID = edge.id; selectedNodeID = ''; onSelectionChange({ node: null, edge }); } }} />
                <text class:dimmed={faded} x={(positions.get(edge.source_id)?.x ?? 0) + 131} y={(positions.get(edge.source_id)?.y ?? 0) + 20}>{edgeLabel(edge)}</text>
              {/each}
            </svg>
            {#each displayNodes as node (node.id)}
              {@const point = positions.get(node.id) ?? { x: 0, y: 0 }}
              {@const Icon = nodeIcon(node)}
              {@const faded = (selectedNode && !highlightedNodeIDs.has(node.id)) || (selectedEdge && selectedEdge.source_id !== node.id && selectedEdge.target_id !== node.id)}
              <article class="topology-node-card" class:application-node={node.kind === 'application'} class:runtime-node={isRuntime(node)} class:resource-node={node.kind === 'resource' && !isRuntime(node)} class:external-node={isExternal(node)} class:dimmed={faded} class:selected={selectedNodeID === node.id} class:dragging={nodeDrag?.id === node.id} style={`left:${point.x}px;top:${point.y}px;`} on:pointerdown={(event) => beginNodeDrag(event, node)}>
                {#if mode === 'edit' && node.kind !== 'annotation'}<button class="topology-anchor anchor-left" data-topology-anchor={node.id} type="button" aria-label={`連接到${node.name}`} on:pointerdown={(event) => beginConnection(event, node)}></button>{/if}
                <button class="topology-node-main" type="button" on:click={() => selectNode(node)}>
                  <span class="topology-node-icon">{#if node.kind === 'application'}<EntityBrandIcon kind="Application" fallback="lucide:AppWindow" size={13} />{:else if node.kind === 'resource'}<ResourceBrandIcon resource={{ kind: node.resource_kind ?? 'Host' }} size={13} />{:else}<Icon size={13} />{/if}</span>
                  <span class="topology-node-copy"><strong>{node.name}</strong><small>{nodeSubtitle(node)}</small></span>
                  {#if node.kind === 'application' && applicationAlertCounts[node.id]}<span class="topology-alert-indicator" data-tooltip={`${applicationAlertCounts[node.id]} 条关联告警`} aria-label={`${applicationAlertCounts[node.id]} 条关联告警`}><AlertTriangle size={13} />{applicationAlertCounts[node.id]}</span>{/if}
                  {#if isExternal(node)}<span class="external-mark">外部</span>{/if}
                </button>
                {#if mode === 'edit' && node.kind !== 'annotation'}<button class="topology-anchor anchor-right" data-topology-anchor={node.id} type="button" aria-label={`从${node.name}创建连线`} on:pointerdown={(event) => beginConnection(event, node)}></button>{/if}
              </article>
            {/each}
            {#if connectionDraft}
              {@const sourceNode = nodes.find((node) => node.id === (connectionDraft?.sourceID ?? ''))}
              {@const targetNode = nodes.find((node) => node.id === (connectionDraft?.targetID ?? ''))}
              <div class="topology-link-config"><div class="link-config-head"><Link2 size={15} /><strong>配置关系</strong><button class="icon-button" type="button" aria-label="取消" on:click={() => (connectionDraft = null)}><X size={14} /></button></div>
              <p>{sourceNode?.name} <ArrowRight size={13} /> {targetNode?.name}</p>
              {#if sourceNode?.kind === 'application' && targetNode?.kind === 'application'}<small>建立应用依赖关系，源应用依赖目标应用。</small>
              {:else if sourceNode?.kind === 'application' && targetNode?.kind === 'resource'}
                {#if isRuntime(targetNode)}<small class="topology-inline-warning">运行时绑定需要唯一定位实例，请通过应用向导完成预览校验。</small><button class="secondary" type="button" on:click={() => editRuntimeBinding(sourceNode.id)}><Pencil size={13} />打开应用向导</button>
                {:else}<label>资源类型<select bind:value={edgeKind}><option value="other">其他</option><option value="database">数据库</option><option value="messaging">消息</option><option value="cache">缓存</option><option value="configuration">配置</option><option value="storage">存储</option></select></label><label>对象信息<input bind:value={edgeBinding} placeholder="database=orders, schema=public" /></label>{/if}
              {:else if sourceNode?.kind === 'resource' && targetNode?.kind === 'resource'}<label>关系<select bind:value={resourceRelationKind}><option value="depends_on">依赖</option><option value="deployed_on">部署于</option><option value="contains">包含</option><option value="observed_by">由其观测</option><option value="exposes">暴露</option></select></label>
              {:else}<small class="topology-inline-warning">请从应用节点发起应用依赖或资源关联。</small>{/if}
              <button class="primary" type="button" disabled={busy || (sourceNode?.kind === 'application' && targetNode?.kind === 'resource' && isRuntime(targetNode)) || (sourceNode?.kind === 'application' && sourceNode.project_id !== project.id)} on:click={saveConnection}><Check size={14} />保存关系</button>
            </div>{/if}
            {#if connectionSource && connectionPointer}<svg class="topology-drag-line"><line x1="0" y1="0" x2={connectionPointer.x} y2={connectionPointer.y} /></svg>{/if}
          </div>
        </div>
      {/if}
      {#if selectedNode}<aside class="topology-detail-float" style={detailFloatStyle(selectedNode)}>
        <header><span><CircleHelp size={15} /><strong>节点详情</strong></span><button class="icon-button" type="button" aria-label="关闭详情" on:click={() => { selectedNodeID = ''; onSelectionChange({ node: null, edge: null }); }}><X size={14} /></button></header>
        <strong class="topology-detail-name">{selectedNode.name}</strong><small>{nodeSubtitle(selectedNode)}</small>
        {#if selectedNode.kind === 'application'}
          <dl><dt>应用状态</dt><dd>{String(selectedNode.metadata?.status ?? '未知')}</dd><dt>关键应用</dt><dd>{selectedNode.metadata?.critical ? '是' : '否'}</dd><dt>实例数量</dt><dd>{String(selectedNode.metadata?.instance_count ?? 0)}</dd><dt>所属项目</dt><dd>{selectedNode.project_name || project.name}</dd><dt>标签</dt><dd>{Object.entries((selectedNode.metadata?.labels ?? {}) as Record<string, string>).map(([key, value]) => value ? `${key}=${value}` : key).join('、') || '—'}</dd><dt>描述</dt><dd>{String(selectedNode.metadata?.description ?? '—') || '—'}</dd></dl>
          {#if mode === 'edit' && selectedNode.project_id === project.id}<button class="secondary topology-detail-action" type="button" on:click={() => onEditApplication(selectedNode as unknown as Application)}><Pencil size={13} />编辑应用配置</button>{/if}
        {:else if selectedNode.kind === 'resource'}
          <dl><dt>资源类型</dt><dd>{selectedNode.resource_kind}</dd><dt>类型</dt><dd>{String(selectedNode.metadata?.subtype ?? 'Direct')}</dd><dt>范围</dt><dd>{isExternal(selectedNode) ? '项目外' : '当前项目'}</dd><dt>关联边</dt><dd>{edges.filter((edge) => edge.source_id === selectedNode.id || edge.target_id === selectedNode.id).length}</dd></dl>
        {:else if selectedNode.kind === 'annotation'}<dl><dt>说明</dt><dd>{String(selectedNode.metadata?.description ?? '—')}</dd></dl>
        {:else}<dl><dt>项目标识</dt><dd>{String(selectedNode.metadata?.code ?? '—')}</dd><dt>关联边</dt><dd>{edges.filter((edge) => edge.source_id === selectedNode.id || edge.target_id === selectedNode.id).length}</dd></dl>{/if}
        {#if mode === 'edit' && ((selectedNode.kind === 'application' && selectedNode.project_id === project.id) || selectedNode.kind === 'annotation' || (selectedNode.kind === 'resource' && selectedNode.metadata?.topology_member))}<button class="danger-button topology-detail-action" type="button" on:click={() => (deletingNode = selectedNode)}><Trash2 size={13} />{selectedNode.kind === 'application' ? '删除应用' : '移除节点'}</button>{/if}
      </aside>{/if}
      {#if selectedEdge}<aside class="topology-detail-float edge-detail-float"><header><span><Link2 size={15} /><strong>关系详情</strong></span><button class="icon-button" type="button" aria-label="关闭详情" on:click={() => { selectedEdgeID = ''; onSelectionChange({ node: null, edge: null }); }}><X size={14} /></button></header><strong class="topology-detail-name">{nodes.find((node) => node.id === selectedEdge?.source_id)?.name} → {nodes.find((node) => node.id === selectedEdge?.target_id)?.name}</strong><small>{edgeLabel(selectedEdge)}</small><dl><dt>关系</dt><dd>{selectedEdge.relation}</dd>{#each Object.entries(selectedEdge.metadata ?? {}) as [key, value]}<dt>{key}</dt><dd>{typeof value === 'object' ? JSON.stringify(value) : String(value)}</dd>{/each}</dl>{#if mode === 'edit'}<button class="danger-button topology-detail-action" type="button" on:click={() => removeEdge(selectedEdge)} disabled={busy}><Trash2 size={13} />移除连线</button>{/if}</aside>{/if}
    </main>
  </div>
</section>

{#if deletingNode}<DeleteConfirmationDialog title={deletingNode.kind === 'application' ? `删除应用“${deletingNode.name}”？` : deletingNode.kind === 'resource' ? `从拓扑移除资源“${deletingNode.name}”？` : `删除说明块“${deletingNode.name}”？`} name={deletingNode.name} description={deletingNode.kind === 'application' ? '将删除该应用及其实例、资源绑定和应用依赖。' : deletingNode.kind === 'resource' ? '仅移除此项目拓扑中的资源引用，不会删除资源本身。' : '将从项目拓扑中删除此说明块。'} impact={deletingNode.kind === 'application' ? '此操作不可撤销。' : '此操作只影响当前项目拓扑。'} {busy} onCancel={() => (deletingNode = null)} onConfirm={confirmRemoveNode} />{/if}

<style>
  .topology-session { min-width: 0; }
  .topology-palette-item,.topology-detail-action,.topology-link-config .primary { display:inline-flex; align-items:center; gap:7px; }
  .topology-workspace { display:grid; grid-template-columns:minmax(0,1fr); min-height:420px; }
  .topology-workspace.editing { grid-template-columns:205px minmax(0,1fr); }
  .topology-workspace.editing.configuring .topology-palette { opacity:.55; pointer-events:none; }
  .topology-palette { display:flex; flex-direction:column; gap:5px; max-height:calc(100vh - 300px); min-height:0; overflow-y:auto; overflow-x:hidden; padding:12px; border-right:1px solid var(--theme-divider); background:var(--theme-bg-subtle); }
  .topology-palette-title { display:flex; justify-content:space-between; align-items:baseline; padding:2px 2px 8px; }
  .topology-palette-title strong { font-size:12px; }
  .topology-palette-title small { color:var(--theme-fg-muted); font-size:10px; }
  .topology-palette-section { display:grid; gap:5px; min-width:0; }
  .topology-palette-section > summary,.topology-palette-subsection > summary { display:flex; align-items:center; gap:7px; min-height:30px; padding:5px 4px; color:var(--theme-fg-strong); border-radius:4px; cursor:pointer; list-style:none; }
  .topology-palette-section > summary::-webkit-details-marker,.topology-palette-subsection > summary::-webkit-details-marker { display:none; }
  .topology-palette-section > summary::before,.topology-palette-subsection > summary::before { color:var(--theme-fg-muted); content:'▸'; font-size:11px; }
  .topology-palette-section[open] > summary::before,.topology-palette-subsection[open] > summary::before { content:'▾'; }
  .topology-palette-section > summary:hover,.topology-palette-subsection > summary:hover { background:var(--theme-bg-hover); }
  .topology-palette-subsection { display:grid; gap:4px; margin-left:8px; }
  .topology-palette-subsection > summary { min-height:26px; color:var(--theme-fg-muted); font-size:11px; }
  .topology-palette-subsection > summary small { margin-left:auto; color:var(--theme-fg-subtle); font-size:10px; }
  .topology-palette-empty { padding:3px 8px 5px 26px; color:var(--theme-fg-subtle); font-size:10px; }
  .topology-palette-group { padding:10px 3px 3px; color:var(--theme-fg-muted); font-size:10px; font-weight:650; }
  .topology-palette-item { width:100%; min-height:42px; padding:6px 8px; color:var(--theme-fg); text-align:left; background:var(--theme-bg-raised); border:1px solid var(--theme-border); border-radius:5px; cursor:grab; }
  .topology-palette-item:disabled { opacity:.48; cursor:not-allowed; }.topology-palette-item:disabled:hover { border-color:var(--theme-border); }
  .topology-palette-item > span { display:grid; flex:1; min-width:0; gap:2px; }
  .topology-palette-item strong,.topology-palette-item small { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .topology-palette-item strong { font-size:11px; }
  .topology-palette-item small { color:var(--theme-fg-muted); font-size:10px; }
  .topology-palette-item:hover { border-color:var(--theme-border-focus); }
  .topology-palette-item.application { color:var(--theme-accent); }
  .topology-palette-item.template { border-style:dashed; background:var(--theme-bg-subtle); }
  .topology-palette-item.resource { color:var(--theme-info); }
  .topology-palette-item.annotation { color:var(--theme-warning); }
  .topology-canvas-wrap { position:relative; min-width:0; min-height:420px; overflow:hidden; background:var(--theme-bg); }
  .topology-canvas-wrap.configuration-open::after { position:absolute; z-index:8; inset:0; display:grid; place-content:center; color:var(--theme-fg); background:color-mix(in srgb,var(--theme-bg) 62%,transparent); content:"组件配置中 · 完成右侧配置后继续"; font-size:12px; pointer-events:auto; }
  .topology-legend { display:flex; flex-wrap:wrap; align-items:center; gap:15px; padding:12px 15px 5px; color:var(--theme-fg-muted); font-size:10px; }
  .topology-legend span { display:inline-flex; align-items:center; gap:5px; }
  .topology-legend i { width:8px; height:8px; background:var(--theme-accent); border-radius:50%; }
  .topology-legend .runtime-key i { background:var(--theme-success); }
  .topology-legend .resource-key i { background:var(--theme-info); }
  .topology-legend .external-key i { background:var(--theme-warning); }
  .topology-scroll { max-width:100%; overflow:auto; }
  .topology-canvas { position:relative; width:1280px; min-height:370px; }
  .topology-column-label { position:absolute; top:8px; color:var(--theme-fg-muted); font-size:10px; font-weight:650; }
  .applications-label { left:36px; }.runtime-label { left:336px; }.resources-label { left:636px; }.external-label { left:936px; }
  .topology-edges { position:absolute; inset:0; overflow:visible; pointer-events:none; }
  .topology-edges path { fill:none; stroke:var(--theme-border-strong); stroke-width:1.6; pointer-events:stroke; cursor:pointer; transition:opacity .16s,stroke-width .16s; }
  .topology-edges path.edge-runs_on { stroke:var(--theme-success); }.topology-edges path.edge-uses { stroke:var(--theme-info); }.topology-edges path.edge-depends_on { stroke:var(--theme-accent); }.topology-edges path.selected { stroke-width:3; }
  .topology-edges text { fill:var(--theme-fg-muted); font-size:9px; pointer-events:none; }.topology-edges .dimmed,.topology-node-card.dimmed { opacity:.2; }
  .topology-node-card { position:absolute; z-index:1; display:flex; align-items:center; width:109px; height:44px; border:1px solid var(--theme-border); border-left:2px solid var(--theme-accent); border-radius:5px; background:var(--theme-bg-raised); box-shadow:var(--theme-shadow-soft); transition:opacity .16s,border-color .16s,box-shadow .16s; }
  .topology-workspace.editing .topology-node-card { cursor:grab; touch-action:none; }.topology-node-card.dragging { z-index:4; cursor:grabbing; }.topology-node-main { cursor:inherit; }
  .topology-node-card.runtime-node { border-left-color:var(--theme-success); }.topology-node-card.resource-node { border-left-color:var(--theme-info); }.topology-node-card.external-node { border-color:var(--theme-warning); border-left-color:var(--theme-warning); background:var(--theme-bg-warning-soft); }
  .topology-node-card.selected { border-color:var(--theme-border-focus); box-shadow:0 0 0 2px var(--theme-bg-accent-soft),var(--theme-shadow-soft); }
  .topology-node-main { display:flex; align-items:center; gap:5px; width:100%; min-width:0; height:100%; padding:4px 5px; color:var(--theme-fg); text-align:left; background:transparent; border:0; }
  .topology-node-icon { display:grid; flex:none; place-items:center; width:19px; height:19px; color:var(--theme-accent); background:var(--theme-bg-accent-soft); border-radius:4px; }
  .runtime-node .topology-node-icon { color:var(--theme-success); background:var(--theme-bg-success-soft); }.resource-node .topology-node-icon { color:var(--theme-info); background:color-mix(in srgb,var(--theme-info) 12%,var(--theme-bg-subtle)); }.external-node .topology-node-icon { color:var(--theme-warning); background:var(--theme-bg-warning-soft); }
  .topology-node-copy { display:grid; min-width:0; gap:1px; flex:1; }.topology-node-copy strong { overflow:hidden; font-size:9px; font-weight:650; text-overflow:ellipsis; white-space:nowrap; }.topology-node-copy small { overflow:hidden; color:var(--theme-fg-muted); font-size:8px; text-overflow:ellipsis; white-space:nowrap; }
  .external-mark { flex:none; padding:2px 3px; color:var(--theme-warning); background:var(--theme-bg-warning-soft); border-radius:3px; font-size:8px; }
  .topology-anchor { position:absolute; z-index:3; top:50%; width:11px; height:11px; padding:0; background:var(--theme-bg-raised); border:2px solid var(--theme-accent); border-radius:50%; transform:translateY(-50%); cursor:crosshair; }.anchor-left { left:-7px; }.anchor-right { right:-7px; }
  .topology-state { display:grid; place-items:center; gap:8px; min-height:360px; color:var(--theme-fg-muted); font-size:12px; }.topology-state strong { color:var(--theme-fg); }
  .topology-detail-float,.topology-link-config { position:absolute; z-index:5; top:14px; right:14px; display:grid; gap:8px; width:min(290px,calc(100% - 28px)); padding:12px; color:var(--theme-fg); background:var(--theme-bg-raised); border:1px solid var(--theme-border-strong); border-radius:6px; box-shadow:var(--theme-shadow-soft); }
  .topology-detail-float header,.link-config-head,.topology-detail-float header span { display:flex; align-items:center; justify-content:space-between; gap:8px; }.topology-detail-float header span { justify-content:flex-start; color:var(--theme-fg-muted); }
  .topology-detail-float header strong,.link-config-head strong { color:var(--theme-fg); font-size:12px; }.topology-detail-name { overflow:hidden; font-size:13px; text-overflow:ellipsis; white-space:nowrap; }.topology-detail-float > small { color:var(--theme-fg-muted); font-size:10px; }
  .topology-detail-float dl { display:grid; grid-template-columns:75px minmax(0,1fr); gap:6px 8px; margin:2px 0; font-size:10px; }.topology-detail-float dt { color:var(--theme-fg-muted); }.topology-detail-float dd { overflow-wrap:anywhere; margin:0; }
  .topology-detail-action { justify-content:center; width:100%; min-height:30px; font-size:11px; }.edge-detail-float { top:14px; }.topology-link-config { top:auto; right:18px; bottom:15px; width:310px; }.topology-link-config p { display:flex; align-items:center; gap:6px; margin:0; font-size:11px; }.topology-link-config small { color:var(--theme-fg-muted); font-size:10px; line-height:1.45; }.topology-link-config label { display:grid; gap:4px; color:var(--theme-fg-muted); font-size:10px; }.topology-link-config input,.topology-link-config select { width:100%; min-height:30px; padding:5px 7px; color:var(--theme-fg); background:var(--theme-bg); border:1px solid var(--theme-border-strong); border-radius:4px; }.topology-inline-warning { color:var(--theme-warning)!important; }.topology-link-config .primary { justify-content:center; min-height:31px; }
  .topology-drag-line { position:fixed; inset:0; z-index:10; width:100vw; height:100vh; overflow:visible; pointer-events:none; }.topology-drag-line line { stroke:var(--theme-accent); stroke-width:2; stroke-dasharray:5 4; }
  @media(max-width:780px) { .topology-workspace.editing { grid-template-columns:1fr; }.topology-palette { max-height:165px; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); border-right:0; border-bottom:1px solid var(--theme-divider); }.topology-palette-title,.topology-palette-group { grid-column:1/-1; } }
</style>
