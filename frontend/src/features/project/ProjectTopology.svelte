<script lang="ts">
  import {
    AppWindow, AlertTriangle, ArrowRight, Boxes, Check, CircleHelp, Cloud,
    Database, HardDrive, Link2, Maximize2, MousePointer2, Pencil,
    Plus, Server, StickyNote, Trash2, X, ZoomIn, ZoomOut
  } from 'lucide-svelte';
  import { onMount } from 'svelte';
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
  export let resourceScopeTypes: Record<string, string> = {};
  export let loading = false;
  export let mode: 'browse' | 'edit' = 'browse';
  export let configurationOpen = false;
  export let pendingComponent: ComponentIntent | null = null;
  export let onRefresh: () => void = () => {};
  type Point = { x: number; y: number };
  type ComponentIntent = { kind: 'application' | 'resource' | 'annotation'; id?: string; resourceKind?: string; runtimeKind?: string; position?: Point };
  export let onConfigureNode: (intent: ComponentIntent) => void = () => {};
  export let onEditApplication: (application: Application) => void = () => {};
  export let applicationAlertCounts: Record<string, number> = {};
  export let onSelectionChange: (selection: { node: ProjectTopology['nodes'][number] | null; edge: ProjectTopology['edges'][number] | null }) => void = () => {};
  export let onNotice: (message: string) => void = () => {};
  export let onError: (message: string) => void = () => {};

  type Node = ProjectTopology['nodes'][number];
  type Edge = ProjectTopology['edges'][number];
  type AnchorSide = 'top' | 'right' | 'bottom' | 'left';
  const NODE_WIDTH = 158;
  const NODE_HEIGHT = 60;
  const resourceKinds = Object.keys(resourceCategoryOptions).filter((kind) => kind !== '全部' && !['Host', 'Docker', 'Kubernetes'].includes(kind));
  const applicationRuntimes = [
    { kind: 'virtual_machine', label: '虚拟机应用' },
    { kind: 'containerized', label: '容器化应用' },
    { kind: 'cloud_native', label: '云原生应用' }
  ];

  let viewportElement: HTMLElement;
  let viewport = { width: 1, height: 420 };
  let zoom = 1;
  let pan = { x: 0, y: 0 };
  let positions = new Map<string, Point>();
  let topologySnapshot: ProjectTopology | null = null;
  let selectedNodeID = '';
  let selectedEdgeID = '';
  let nodeDrag: { id: string; origin: Point; start: Point; moved: boolean } | null = null;
  let suppressNodeClickID = '';
  let panDrag: { start: Point; origin: Point } | null = null;
  let connectionSource = '';
  let connectionPointer: Point | null = null;
  let connectionDraft: { sourceID: string; targetID: string } | null = null;
  let edgeKind = 'other';
  let edgeBinding = '';
  let resourceRelationKind = 'depends_on';
  let busy = false;
  let deletingNode: Node | null = null;

  $: nodes = topology?.nodes ?? [];
  $: edges = topology?.edges ?? [];
  $: displayNodes = nodes.filter((node) => node.kind !== 'project');
  $: if (topology !== topologySnapshot) {
    topologySnapshot = topology;
    positions = new Map(displayNodes.map((node, index) => [node.id, node.position ?? positions.get(node.id) ?? pointFor(index)]));
  }
  $: canvasWidth = Math.max(1, viewport.width / zoom);
  $: canvasHeight = Math.max(viewport.height / zoom, ...[...positions.values(), ...(pendingComponent?.position ? [pendingComponent.position] : [])].map((point) => point.y + NODE_HEIGHT + 32));
  $: pendingPoint = pendingComponent?.position ?? null;
  $: pendingTitle = pendingComponent?.kind === 'application'
    ? '新建应用'
    : pendingComponent?.kind === 'resource'
      ? '新建资源'
      : '新建实体';
  $: pendingType = pendingComponent?.kind === 'application'
    ? `应用 · ${applicationRuntimes.find((item) => item.kind === pendingComponent?.runtimeKind)?.label ?? '待配置'}`
    : pendingComponent?.kind === 'resource'
      ? pendingComponent.resourceKind ?? '资源'
      : '未纳管实体';
  $: selectedNode = nodes.find((node) => node.id === selectedNodeID) ?? null;
  $: selectedEdge = edges.find((edge) => edge.id === selectedEdgeID) ?? null;
  $: displayEdges = edges.filter((edge) => positions.has(edge.source_id) && positions.has(edge.target_id));
  $: highlightedNodeIDs = selectedNode ? linkedNodes(selectedNode.id) : new Set<string>();
  $: if (mode === 'browse') cancelConnection();

  onMount(() => {
    const observer = new ResizeObserver(([entry]) => {
      viewport = { width: entry.contentRect.width, height: Math.max(entry.contentRect.height, 420) };
    });
    observer.observe(viewportElement);
    return () => observer.disconnect();
  });

  function pointFor(index: number): Point {
    const columns = Math.max(2, Math.floor(Math.max(viewport.width, 500) / 210));
    return { x: 28 + (index % columns) * 190, y: 42 + Math.floor(index / columns) * 100 };
  }
  function isRuntime(node: Node) {
    return node.kind === 'resource' && ['Host', 'Docker', 'Kubernetes'].includes(node.resource_kind ?? '');
  }
  function isExternal(node: Node) {
    return node.kind === 'annotation' || (node.kind === 'application' && node.project_id !== project.id) || Boolean(node.metadata?.external);
  }
  function scopeLabel(node: Node) {
    if (node.kind === 'annotation') return '未纳管';
    if (!isExternal(node)) return '';
    const type = String(node.metadata?.scope_type ?? '');
    if (type === 'platform') return '平台';
    if (type === 'team') return '团队';
    return node.kind === 'application' ? '项目外' : '外部';
  }
  function nodeType(node: Node) {
    if (node.kind === 'application') return '应用';
    if (node.kind === 'annotation') return '实体';
    return isRuntime(node) ? `运行时 · ${node.resource_kind}` : node.resource_kind ?? '资源';
  }
  function nodeIcon(node: Node) {
    if (node.kind === 'annotation') return StickyNote;
    if (node.resource_kind === 'Host') return Server;
    if (node.resource_kind === 'Docker') return Boxes;
    if (node.resource_kind === 'Kubernetes') return Cloud;
    if (['PostgreSQL', 'MySQL', 'Oracle', 'OceanBase'].includes(node.resource_kind ?? '')) return Database;
    return HardDrive;
  }
  function edgeLabel(edge: Edge) {
    if (edge.relation === 'runs_on') return '运行于';
    if (edge.relation === 'uses') return String(edge.metadata?.kind ?? '资源关联');
    if (edge.relation === 'depends_on') return '依赖';
    if (edge.relation === 'external_dependency') return '外部依赖';
    if (edge.relation === 'contains') return '包含';
    return edge.relation.replaceAll('_', ' ');
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
    return source && target ? pathBetween(source, target) : '';
  }
  function anchorToward(source: Point, target: Point): AnchorSide {
    const dx = target.x + NODE_WIDTH / 2 - (source.x + NODE_WIDTH / 2);
    const dy = target.y + NODE_HEIGHT / 2 - (source.y + NODE_HEIGHT / 2);
    if (Math.abs(dx) >= Math.abs(dy)) return dx >= 0 ? 'right' : 'left';
    return dy >= 0 ? 'bottom' : 'top';
  }
  function oppositeAnchor(side: AnchorSide): AnchorSide {
    return side === 'top' ? 'bottom' : side === 'bottom' ? 'top' : side === 'left' ? 'right' : 'left';
  }
  function anchorPoint(point: Point, side: AnchorSide): Point {
    if (side === 'top') return { x: point.x + NODE_WIDTH / 2, y: point.y };
    if (side === 'bottom') return { x: point.x + NODE_WIDTH / 2, y: point.y + NODE_HEIGHT };
    if (side === 'left') return { x: point.x, y: point.y + NODE_HEIGHT / 2 };
    return { x: point.x + NODE_WIDTH, y: point.y + NODE_HEIGHT / 2 };
  }
  function anchorDirection(side: AnchorSide): Point {
    if (side === 'top') return { x: 0, y: -1 };
    if (side === 'bottom') return { x: 0, y: 1 };
    return { x: side === 'left' ? -1 : 1, y: 0 };
  }
  function curveBetween(source: Point, sourceSide: AnchorSide, target: Point, targetSide: AnchorSide) {
    const start = anchorPoint(source, sourceSide);
    const end = anchorPoint(target, targetSide);
    const distance = Math.max(36, Math.min(120, (Math.abs(end.x - start.x) + Math.abs(end.y - start.y)) * 0.38));
    const startDirection = anchorDirection(sourceSide);
    const endDirection = anchorDirection(targetSide);
    return `M ${start.x} ${start.y} C ${start.x + startDirection.x * distance} ${start.y + startDirection.y * distance}, ${end.x + endDirection.x * distance} ${end.y + endDirection.y * distance}, ${end.x} ${end.y}`;
  }
  function pathBetween(source: Point, target: Point) {
    const sourceSide = anchorToward(source, target);
    return curveBetween(source, sourceSide, target, oppositeAnchor(sourceSide));
  }
  function pathToPointer(source: Point, target: Point) {
    const sourceSide = anchorToward(source, target);
    const start = anchorPoint(source, sourceSide);
    const direction = anchorDirection(sourceSide);
    const distance = Math.max(36, Math.min(120, (Math.abs(target.x - start.x) + Math.abs(target.y - start.y)) * 0.38));
    return `M ${start.x} ${start.y} C ${start.x + direction.x * distance} ${start.y + direction.y * distance}, ${target.x} ${target.y}, ${target.x} ${target.y}`;
  }
  function viewportPoint(event: PointerEvent | DragEvent): Point {
    const bounds = viewportElement.getBoundingClientRect();
    return { x: (event.clientX - bounds.left - pan.x) / zoom, y: (event.clientY - bounds.top - pan.y) / zoom };
  }
  function clampPoint(point: Point): Point {
    return { x: Math.max(0, Math.min(canvasWidth - NODE_WIDTH, point.x)), y: Math.max(0, Math.min(canvasHeight - NODE_HEIGHT, point.y)) };
  }
  function selectNode(node: Node) {
    if (suppressNodeClickID === node.id) {
      suppressNodeClickID = '';
      return;
    }
    if (connectionSource && connectionSource !== node.id) {
      openConnection(connectionSource, node.id);
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
    event.preventDefault();
    nodeDrag = { id: node.id, origin: point, start: viewportPoint(event), moved: false };
  }
  function moveNode(event: PointerEvent) {
    if (!nodeDrag) return;
    const pointer = viewportPoint(event);
    const dx = pointer.x - nodeDrag.start.x;
    const dy = pointer.y - nodeDrag.start.y;
    if (!nodeDrag.moved && Math.hypot(dx, dy) < 2) return;
    nodeDrag.moved = true;
    suppressNodeClickID = nodeDrag.id;
    positions = new Map(positions).set(nodeDrag.id, clampPoint({ x: nodeDrag.origin.x + dx, y: nodeDrag.origin.y + dy }));
  }
  async function finishNodeDrag() {
    const drag = nodeDrag;
    nodeDrag = null;
    if (!drag?.moved) return;
    const point = positions.get(drag.id);
    if (!point) return;
    try {
      await api.setProjectTopologyPosition(project.id, drag.id, { x: Math.round(point.x), y: Math.round(point.y) });
    } catch (error) {
      positions = new Map(positions).set(drag.id, drag.origin);
      onError(error instanceof Error ? error.message : '保存节点位置失败');
    }
    window.setTimeout(() => {
      if (suppressNodeClickID === drag.id) suppressNodeClickID = '';
    }, 0);
  }
  function beginPan(event: PointerEvent) {
    if (event.button !== 0 || (event.target as HTMLElement).closest('.topology-node-card,.topology-link-config,.topology-detail-float,.topology-zoom-controls')) return;
    panDrag = { start: { x: event.clientX, y: event.clientY }, origin: pan };
  }
  function movePan(event: PointerEvent) {
    if (!panDrag) return;
    pan = { x: panDrag.origin.x + event.clientX - panDrag.start.x, y: panDrag.origin.y + event.clientY - panDrag.start.y };
  }
  function finishPan() { panDrag = null; }
  function adjustZoom(delta: number) {
    zoom = Math.max(0.55, Math.min(1.6, Number((zoom + delta).toFixed(2))));
  }
  function zoomCanvas(event: WheelEvent) {
    event.preventDefault();
    adjustZoom(event.deltaY < 0 ? 0.1 : -0.1);
  }
  function resetViewport() { zoom = 1; pan = { x: 0, y: 0 }; }
  function beginConnection(event: PointerEvent, node: Node) {
    if (mode !== 'edit') return;
    event.preventDefault();
    event.stopPropagation();
    connectionSource = node.id;
    connectionPointer = viewportPoint(event);
  }
  function moveConnection(event: PointerEvent) {
    if (connectionSource) connectionPointer = viewportPoint(event);
  }
  function finishConnection(event: PointerEvent) {
    if (!connectionSource) return;
    const target = (event.target as HTMLElement).closest<HTMLElement>('[data-topology-node]');
    const targetID = target?.dataset.topologyNode ?? '';
    const sourceID = connectionSource;
    cancelConnection();
    if (targetID && targetID !== sourceID) openConnection(sourceID, targetID);
  }
  function openConnection(sourceID: string, targetID: string) {
    const source = nodes.find((node) => node.id === sourceID);
    const target = nodes.find((node) => node.id === targetID);
    if (!source || !target || sourceID === targetID) return;
    if (source.kind === 'resource' && target.kind === 'application') [sourceID, targetID] = [targetID, sourceID];
    cancelConnection();
    connectionDraft = { sourceID, targetID };
    edgeKind = 'other';
    edgeBinding = '';
    resourceRelationKind = 'depends_on';
  }
  function cancelConnection() { connectionSource = ''; connectionPointer = null; }
  function dropKind(event: DragEvent) {
    event.preventDefault();
    if (configurationOpen || mode !== 'edit') return;
    const raw = event.dataTransfer?.getData('application/x-opskeeper-topology-component');
    if (!raw) return;
    try { onConfigureNode({ ...JSON.parse(raw), position: clampPoint(viewportPoint(event)) }); }
    catch { onError('无法识别拖入的拓扑组件'); }
  }
  function dragComponent(event: DragEvent, intent: ComponentIntent) {
    event.dataTransfer?.setData('application/x-opskeeper-topology-component', JSON.stringify(intent));
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'copy';
  }
  function connectionDraftStyle() {
    const target = positions.get(connectionDraft?.targetID ?? '');
    if (!target) return '';
    const left = target.x + NODE_WIDTH + 10 > canvasWidth - 300 ? Math.max(10, target.x - 300) : target.x + NODE_WIDTH + 10;
    const top = Math.max(8, Math.min(canvasHeight - 215, target.y));
    return `left:${left}px;top:${top}px;right:auto;bottom:auto;`;
  }
  function detailFloatStyle(node: Node) {
    const point = positions.get(node.id) ?? { x: 0, y: 0 };
    const left = point.x + NODE_WIDTH + 10 > canvasWidth - 290 ? Math.max(10, point.x - 300) : point.x + NODE_WIDTH + 10;
    return `left:${left}px;top:${Math.max(8, Math.min(canvasHeight - 250, point.y))}px;right:auto;bottom:auto;`;
  }
  function applicationPayload(application: Application, relations = application.relations, dependencies = application.dependencies) {
    return { name: application.name, code: application.code, description: application.description, icon: application.icon, runtime_kind: application.runtime_kind, external_uid: application.external_uid, labels: application.labels, critical: application.critical, technical_owner_ids: application.technical_owner_ids, operations_owner_ids: application.operations_owner_ids, relations: relations.map((item) => ({ target_application_id: item.target_application_id, relation_kind: item.relation_kind, status: item.status })), dependencies: dependencies.map((item) => ({ target_resource_id: item.target_resource_id, dependency_kind: item.dependency_kind, binding: item.binding, required: item.required, status: item.status })), instances: application.instances.map((item) => ({ name: item.name, runtime_kind: item.runtime_kind, target_resource_id: item.target_resource_id, selector: item.selector, log_binding: item.log_binding, status: item.status })) };
  }
  function parseBinding(value: string) {
    return Object.fromEntries(value.split(',').map((item) => item.trim()).filter(Boolean).map((item) => { const split = item.indexOf('='); return split < 0 ? [item, ''] : [item.slice(0, split).trim(), item.slice(split + 1).trim()]; }));
  }
  async function saveConnection() {
    if (!connectionDraft || busy) return;
    const source = nodes.find((node) => node.id === connectionDraft?.sourceID);
    const target = nodes.find((node) => node.id === connectionDraft?.targetID);
    if (!source || !target) return;
    busy = true;
    try {
      if (source.kind === 'annotation' || target.kind === 'annotation') {
        await api.createProjectTopologyEdge(project.id, { source_id: source.id, target_id: target.id, relation: 'external_dependency' });
      } else if (source.kind === 'application' && target.kind === 'application') {
        if (source.project_id !== project.id) throw new Error('只能从当前项目的应用建立依赖');
        const app = await api.application(project.id, source.id);
        await api.updateApplication(project.id, app.id, applicationPayload(app, [...app.relations, { target_application_id: target.id, relation_kind: 'depends_on', status: 'active' } as Application['relations'][number]]));
      } else if (source.kind === 'application' && target.kind === 'resource') {
        if (isRuntime(target)) throw new Error('运行时绑定请通过应用向导配置，以保留实例唯一定位校验');
        const app = await api.application(project.id, source.id);
        await api.updateApplication(project.id, app.id, applicationPayload(app, app.relations, [...app.dependencies, { target_resource_id: target.id, dependency_kind: edgeKind, binding: parseBinding(edgeBinding), required: false, status: 'active' } as Application['dependencies'][number]]));
      } else if (source.kind === 'resource' && target.kind === 'resource') {
        await api.createRelation(source.id, { target_resource_id: target.id, relation_type: resourceRelationKind, attributes: {}, discovery_source: 'manual', confidence: 1, confirmed: true });
      } else throw new Error('此节点组合暂不支持连线');
      connectionDraft = null;
      await onRefresh();
      onNotice('拓扑关系已保存');
    } catch (error) { onError(error instanceof Error ? error.message : '创建拓扑关系失败'); }
    finally { busy = false; }
  }
  async function removeEdge(edge: Edge) {
    if (busy) return;
    busy = true;
    try {
      if (edge.id.startsWith('topology:')) await api.deleteProjectTopologyEdge(project.id, edge.id.slice('topology:'.length));
      else if (edge.id.startsWith('resource_relation:')) await api.deleteRelation(edge.source_id, edge.id.slice('resource_relation:'.length));
      else if (edge.id.startsWith('instance:')) {
        const app = await api.application(project.id, edge.source_id);
        const instance = app.instances.find((item) => item.id === edge.id.slice('instance:'.length));
        if (!instance) throw new Error('实例绑定已不存在');
        await api.deleteApplicationInstance(project.id, app.id, instance.id);
      } else {
        const app = await api.application(project.id, edge.source_id);
        if (edge.id.startsWith('binding:')) await api.updateApplication(project.id, app.id, applicationPayload(app, app.relations, app.dependencies.filter((item) => item.id !== edge.id.slice('binding:'.length))));
        else if (edge.id.startsWith('relation:')) await api.updateApplication(project.id, app.id, applicationPayload(app, app.relations.filter((item) => item.id !== edge.id.slice('relation:'.length))));
      }
      selectedEdgeID = '';
      await onRefresh();
      onNotice('拓扑连线已移除');
    } catch (error) { onError(error instanceof Error ? error.message : '移除拓扑连线失败'); }
    finally { busy = false; }
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
    } catch (error) { onError(error instanceof Error ? error.message : '删除拓扑节点失败'); }
    finally { busy = false; }
  }
</script>

<svelte:window on:pointermove={(event) => { moveNode(event); movePan(event); moveConnection(event); }} on:pointerup={(event) => { finishConnection(event); void finishNodeDrag(); finishPan(); }} />

<section class="topology-session">
  <div class:editing={mode === 'edit'} class:configuring={configurationOpen} class="topology-workspace">
    {#if mode === 'edit'}
      <aside class="topology-palette">
        <div class="topology-palette-title"><strong>组件</strong><small>拖到画布</small></div>
        <button class="topology-palette-leaf" type="button" draggable="true" data-tooltip="拖入画布新增未纳管实体" on:dragstart={(event) => dragComponent(event, { kind: 'annotation' })}><span class="topology-palette-category-icon entity-tone"><StickyNote size={14} /></span><span>实体</span></button>
        <details class="topology-palette-section" open>
          <summary><span class="topology-palette-category-icon application-tone"><AppWindow size={15} /></span>应用</summary>
          {#each applicationRuntimes as runtime (runtime.kind)}
            {@const runtimeApps = applications.filter((application) => application.runtime_kind === runtime.kind)}
            <details class="topology-palette-subsection">
              <summary draggable="true" data-tooltip={`拖入画布新增${runtime.label}`} on:dragstart|stopPropagation={(event) => dragComponent(event, { kind: 'application', runtimeKind: runtime.kind })}><span class="topology-palette-category-icon application-tone">{#if runtime.kind === 'virtual_machine'}<Server size={14} />{:else}<ResourceBrandIcon resource={{ kind: runtime.kind === 'containerized' ? 'Docker' : 'Kubernetes' }} size={14} />{/if}</span><span>{runtime.label}</span><small>{runtimeApps.length}</small></summary>
              {#each runtimeApps as application (application.id)}
                {@const added = nodes.some((node) => node.kind === 'application' && node.id === application.id)}
                <button class="topology-palette-item application" disabled={added} draggable={!added} on:dragstart={(event) => dragComponent(event, { kind: 'application', id: application.id, runtimeKind: runtime.kind })}><span class="topology-palette-icon"><EntityBrandIcon kind="Application" fallback={application.icon || 'lucide:AppWindow'} size={16} /></span><span><strong>{application.name}</strong><small>{application.code}</small></span></button>
              {/each}
            </details>
          {/each}
        </details>
        <details class="topology-palette-section" open>
          <summary><span class="topology-palette-category-icon resource-tone"><HardDrive size={15} /></span>资源</summary>
          {#each resourceKinds as kind (kind)}
            {@const kindResources = resources.filter((resource) => resourceCategoryFor(resource) === kind)}
            <details class="topology-palette-subsection">
              <summary draggable="true" data-tooltip={`拖入画布新增${kind}资源`} on:dragstart|stopPropagation={(event) => dragComponent(event, { kind: 'resource', resourceKind: kind })}><span class="topology-palette-category-icon resource-tone"><ResourceBrandIcon resource={{ kind }} fallback={resourceCategoryIcon(kind)} size={14} /></span><span>{kind}</span><small>{kindResources.length}</small></summary>
              {#each kindResources as resource (resource.id)}
                {@const added = nodes.some((node) => node.kind === 'resource' && node.id === resource.id)}
                {@const scopeType = resourceScopeTypes[resource.id] || (resource.scope_id === project.scope.id ? 'project' : '')}
                {@const scopeLabel = scopeType === 'platform' ? '平台' : scopeType === 'team' ? '团队' : scopeType === 'project' ? '项目' : ''}
                <button class="topology-palette-item resource" disabled={added} draggable={!added} on:dragstart={(event) => dragComponent(event, { kind: 'resource', id: resource.id, resourceKind: kind })}><span class="topology-palette-icon"><ResourceBrandIcon {resource} fallback={resourceCategoryIcon(kind)} size={16} /></span><span><strong>{resource.name}</strong><small>{resource.subtype || kind}</small></span>{#if scopeLabel}<span class="scope-mark">{scopeLabel}</span>{/if}</button>
              {/each}
            </details>
          {/each}
        </details>
      </aside>
    {/if}

    <main bind:this={viewportElement} class="topology-canvas-wrap" on:pointerdown={beginPan} on:wheel={zoomCanvas} on:dragover|preventDefault on:drop={dropKind}>
      {#if loading}<div class="topology-state">正在加载项目拓扑…</div>
      {:else if !displayNodes.length && !pendingComponent}<div class="topology-state"><MousePointer2 size={18} /><span>{mode === 'edit' ? '拖入应用、资源或实体，开始构建项目拓扑' : '项目还没有拓扑节点'}</span></div>
      {:else}
        <div class="topology-legend"><span class="application-key"><i></i>应用</span><span class="runtime-key"><i></i>运行时</span><span class="resource-key"><i></i>资源</span><span class="entity-key"><i></i>实体</span></div>
        <div class="topology-zoom-controls" aria-label="画布缩放"><button class="icon-button" type="button" data-tooltip="缩小" on:click={() => adjustZoom(-0.1)}><ZoomOut size={14} /></button><span>{Math.round(zoom * 100)}%</span><button class="icon-button" type="button" data-tooltip="放大" on:click={() => adjustZoom(0.1)}><ZoomIn size={14} /></button><button class="icon-button" type="button" data-tooltip="重置视图" on:click={resetViewport}><Maximize2 size={14} /></button></div>
        <div class="topology-surface" style={`width:${canvasWidth}px;height:${canvasHeight}px;transform:translate(${pan.x}px,${pan.y}px) scale(${zoom});`}>
          <svg class="topology-edges" width={canvasWidth} height={canvasHeight} aria-hidden="true">
            {#each displayEdges as edge (edge.id)}
              {@const faded = (selectedNode && !highlightedNodeIDs.has(edge.source_id) && !highlightedNodeIDs.has(edge.target_id)) || (selectedEdge && selectedEdge.id !== edge.id)}
              <path role="button" tabindex="0" aria-label={`${edgeLabel(edge)}关系`} class:dimmed={faded} class:selected={selectedEdgeID === edge.id} class={`edge-${edge.relation}`} d={edgePath(edge)} on:click|stopPropagation={() => { selectedEdgeID = edge.id; selectedNodeID = ''; onSelectionChange({ node: null, edge }); }} on:keydown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectedEdgeID = edge.id; selectedNodeID = ''; onSelectionChange({ node: null, edge }); } }} />
            {/each}
            {#if connectionSource && connectionPointer}{@const source = positions.get(connectionSource)}{#if source}<path class="connection-preview" d={pathToPointer(source, connectionPointer)} />{/if}{/if}
          </svg>
          {#each displayNodes as node (node.id)}
            {@const point = positions.get(node.id) ?? { x: 0, y: 0 }}
            {@const Icon = nodeIcon(node)}
            {@const faded = (selectedNode && !highlightedNodeIDs.has(node.id)) || (selectedEdge && selectedEdge.source_id !== node.id && selectedEdge.target_id !== node.id)}
            <article data-topology-node={node.id} class="topology-node-card" class:application-node={node.kind === 'application'} class:resource-node={node.kind === 'resource' && !isRuntime(node)} class:runtime-node={isRuntime(node)} class:entity-node={node.kind === 'annotation'} class:dimmed={faded} class:selected={selectedNodeID === node.id} class:dragging={nodeDrag?.id === node.id} style={`left:${point.x}px;top:${point.y}px;`} on:pointerdown={(event) => beginNodeDrag(event, node)}>
              <button class="topology-node-main" type="button" on:click={() => selectNode(node)}><span class="topology-node-icon">{#if node.kind === 'application'}<EntityBrandIcon kind="Application" fallback="lucide:AppWindow" size={15} />{:else if node.kind === 'resource'}<ResourceBrandIcon resource={{ kind: node.resource_kind ?? 'Host' }} size={15} />{:else}<Icon size={15} />{/if}</span><span class="topology-node-copy"><strong>{node.name}</strong><small>{nodeType(node)}</small></span>{#if scopeLabel(node)}<span class="scope-mark">{scopeLabel(node)}</span>{/if}{#if node.kind === 'application' && applicationAlertCounts[node.id]}<span class="topology-alert-indicator" data-tooltip={`${applicationAlertCounts[node.id]} 条关联告警`}><AlertTriangle size={12} />{applicationAlertCounts[node.id]}</span>{/if}</button>
              {#if mode === 'edit'}
                {#each ['top', 'right', 'bottom', 'left'] as side}
                  <button class={`topology-anchor anchor-${side}`} data-topology-anchor={node.id} type="button" aria-label={`从${node.name}${side === 'top' ? '上方' : side === 'right' ? '右侧' : side === 'bottom' ? '下方' : '左侧'}连线`} on:pointerdown|preventDefault|stopPropagation={(event) => beginConnection(event, node)} on:click|stopPropagation><span></span></button>
                {/each}
              {/if}
            </article>
          {/each}
          {#if pendingComponent && pendingPoint}
            {@const PendingIcon = pendingComponent.kind === 'application' ? AppWindow : pendingComponent.kind === 'resource' ? HardDrive : StickyNote}
            <article class="topology-node-card pending-node" class:application-node={pendingComponent.kind === 'application'} class:resource-node={pendingComponent.kind === 'resource'} class:entity-node={pendingComponent.kind === 'annotation'} aria-label={`${pendingTitle}，正在配置`} style={`left:${pendingPoint.x}px;top:${pendingPoint.y}px;`}>
              <div class="topology-node-main"><span class="topology-node-icon"><PendingIcon size={15} /></span><span class="topology-node-copy"><strong>{pendingTitle}</strong><small>{pendingType}</small></span></div>
            </article>
          {/if}
          {#if connectionDraft}
            {@const sourceNode = nodes.find((node) => node.id === connectionDraft?.sourceID)}
            {@const targetNode = nodes.find((node) => node.id === connectionDraft?.targetID)}
            <div class="topology-link-config" style={connectionDraftStyle()}><div class="link-config-head"><Link2 size={15} /><strong>建立关系</strong><button class="icon-button" type="button" aria-label="取消" on:click={() => (connectionDraft = null)}><X size={14} /></button></div><p>{sourceNode?.name} <ArrowRight size={13} /> {targetNode?.name}</p>
            {#if sourceNode?.kind === 'annotation' || targetNode?.kind === 'annotation'}<small>记录当前项目与实体之间的外部依赖。</small>
            {:else if sourceNode?.kind === 'application' && targetNode?.kind === 'application'}<small>源应用依赖目标应用。</small>
            {:else if sourceNode?.kind === 'application' && targetNode?.kind === 'resource'}{#if isRuntime(targetNode)}<small class="topology-inline-warning">运行时绑定需要唯一定位实例，请通过应用向导完成预览校验。</small>{:else}<label>资源类型<select bind:value={edgeKind}><option value="other">其他</option><option value="database">数据库</option><option value="messaging">消息</option><option value="cache">缓存</option><option value="configuration">配置</option><option value="storage">存储</option></select></label><label>对象信息<input bind:value={edgeBinding} placeholder="database=orders, schema=public" /></label>{/if}
            {:else if sourceNode?.kind === 'resource' && targetNode?.kind === 'resource'}<label>关系<select bind:value={resourceRelationKind}><option value="depends_on">依赖</option><option value="deployed_on">部署于</option><option value="contains">包含</option><option value="observed_by">由其观测</option><option value="exposes">暴露</option></select></label>{/if}
            <button class="primary" type="button" disabled={busy || (sourceNode?.kind === 'application' && targetNode?.kind === 'resource' && isRuntime(targetNode))} on:click={saveConnection}><Check size={14} />保存关系</button></div>
          {/if}
          {#if selectedNode}<aside class="topology-detail-float" style={detailFloatStyle(selectedNode)}><header><span><CircleHelp size={15} /><strong>节点详情</strong></span><button class="icon-button" type="button" aria-label="关闭详情" on:click={() => { selectedNodeID = ''; onSelectionChange({ node: null, edge: null }); }}><X size={14} /></button></header><strong class="topology-detail-name">{selectedNode.name}</strong><small>{nodeType(selectedNode)}</small>{#if selectedNode.kind === 'annotation'}<dl><dt>说明</dt><dd>{String(selectedNode.metadata?.description ?? '—')}</dd></dl>{:else if selectedNode.kind === 'application'}<dl><dt>状态</dt><dd>{String(selectedNode.metadata?.status ?? '未知')}</dd><dt>实例数量</dt><dd>{String(selectedNode.metadata?.instance_count ?? 0)}</dd><dt>所属项目</dt><dd>{selectedNode.project_name || project.name}</dd></dl>{:else}<dl><dt>资源类型</dt><dd>{selectedNode.resource_kind}</dd><dt>连接方式</dt><dd>{String(selectedNode.metadata?.subtype ?? 'Direct')}</dd><dt>范围</dt><dd>{scopeLabel(selectedNode) || '项目'}</dd></dl>{/if}{#if mode === 'edit' && ((selectedNode.kind === 'application' && selectedNode.project_id === project.id) || selectedNode.kind === 'annotation' || (selectedNode.kind === 'resource' && selectedNode.metadata?.topology_member))}<div class="topology-detail-actions">{#if selectedNode.kind === 'application' && selectedNode.project_id === project.id}<button class="secondary topology-detail-action" type="button" on:click={() => onEditApplication(selectedNode as unknown as Application)}><Pencil size={13} />编辑</button>{/if}<button class="danger-button topology-detail-action" type="button" on:click={() => (deletingNode = selectedNode)}><Trash2 size={13} />移除</button></div>{/if}</aside>{/if}
          {#if selectedEdge}<aside class="topology-detail-float topology-edge-detail"><header><span><Link2 size={15} /><strong>关系详情</strong></span><button class="icon-button" type="button" aria-label="关闭详情" on:click={() => { selectedEdgeID = ''; onSelectionChange({ node: null, edge: null }); }}><X size={14} /></button></header><strong class="topology-detail-name">{nodes.find((node) => node.id === selectedEdge?.source_id)?.name} → {nodes.find((node) => node.id === selectedEdge?.target_id)?.name}</strong><small>{edgeLabel(selectedEdge)}</small>{#if mode === 'edit'}<button class="danger-button topology-detail-action" type="button" on:click={() => removeEdge(selectedEdge)} disabled={busy}><Trash2 size={13} />移除连线</button>{/if}</aside>{/if}
        </div>
      {/if}
    </main>
  </div>
</section>

{#if deletingNode}<DeleteConfirmationDialog title={deletingNode.kind === 'application' ? `删除应用“${deletingNode.name}”？` : deletingNode.kind === 'resource' ? `从拓扑移除资源“${deletingNode.name}”？` : `删除实体“${deletingNode.name}”？`} name={deletingNode.name} description={deletingNode.kind === 'application' ? '将删除该应用及其实例、资源关联和应用依赖。' : deletingNode.kind === 'resource' ? '仅移除此项目拓扑中的资源引用，不会删除资源本身。' : '将删除此未纳管实体及其拓扑关系。'} impact={deletingNode.kind === 'application' ? '此操作不可撤销。' : '此操作只影响当前项目拓扑。'} {busy} onCancel={() => (deletingNode = null)} onConfirm={confirmRemoveNode} />{/if}

<style>
  .topology-session { min-width:0; height:100%; }
  .topology-workspace { display:grid; grid-template-columns:minmax(0,1fr); min-height:520px; height:calc(100vh - 300px); max-height:760px; overflow:hidden; }
  .topology-workspace.editing { grid-template-columns:205px minmax(0,1fr); }
  .topology-workspace.configuring .topology-palette { opacity:.5; pointer-events:none; }
  .topology-palette { display:flex; flex-direction:column; gap:6px; min-height:0; overflow-y:auto; overflow-x:hidden; padding:12px; border-right:1px solid var(--theme-divider); background:var(--theme-bg-subtle); }
  .topology-palette-title { display:flex; justify-content:space-between; align-items:baseline; padding:2px 2px 8px; }.topology-palette-title strong { font-size:12px; }.topology-palette-title small,.topology-palette-empty { color:var(--theme-fg-muted); font-size:10px; }
  .topology-palette-section,.topology-palette-subsection { display:grid; gap:5px; min-width:0; }.topology-palette-subsection { margin-left:8px; }
  .topology-palette-section > summary,.topology-palette-subsection > summary { display:flex; align-items:center; gap:7px; min-height:28px; padding:4px; color:var(--theme-fg-strong); border-radius:4px; cursor:pointer; list-style:none; }.topology-palette-section > summary::-webkit-details-marker,.topology-palette-subsection > summary::-webkit-details-marker { display:none; }.topology-palette-section > summary::before { flex:none; width:10px; color:var(--theme-fg-muted); content:'▸'; font-size:11px; }.topology-palette-section[open] > summary::before { content:'▾'; }.topology-palette-subsection > summary { color:var(--theme-fg-muted); font-size:11px; }.topology-palette-subsection > summary[draggable="true"] { cursor:grab; }.topology-palette-subsection > summary[draggable="true"]:active { cursor:grabbing; }.topology-palette-subsection > summary small { margin-left:auto; color:var(--theme-fg-subtle); }
  .topology-palette-category-icon,.topology-palette-icon { display:grid; flex:none; place-items:center; width:22px; height:22px; color:var(--topology-tone,var(--theme-fg-muted)); background:transparent; border:0; }.topology-palette-category-icon { width:16px; height:18px; }.topology-palette-category-icon.application-tone,.topology-palette-item.application { --topology-tone:var(--theme-application); }.topology-palette-category-icon.resource-tone,.topology-palette-item.resource { --topology-tone:var(--theme-resource); }.topology-palette-category-icon.entity-tone,.topology-palette-item.entity { --topology-tone:var(--theme-warning); }.topology-palette-item { --topology-tone:var(--theme-accent); display:inline-flex; align-items:center; gap:7px; width:100%; min-height:40px; padding:6px 8px; color:var(--theme-fg); text-align:left; background:var(--theme-bg-raised); border:1px solid var(--theme-border); border-left:3px solid var(--topology-tone); border-radius:5px; cursor:grab; }.topology-palette-item > span:not(.topology-palette-icon) { display:grid; flex:1; min-width:0; gap:2px; }.topology-palette-item strong,.topology-palette-item small { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.topology-palette-item strong { font-size:11px; }.topology-palette-item small { color:var(--theme-fg-muted); font-size:10px; }.topology-palette-item:disabled { opacity:.45; cursor:not-allowed; }.topology-palette-item.application { --topology-tone:var(--theme-application); }.topology-palette-item.resource { --topology-tone:var(--theme-resource); }.topology-palette-item.entity { --topology-tone:var(--theme-warning); }
  :global(.topology-palette .entity-icon),:global(.topology-node-icon .entity-icon) { width:auto; height:auto; color:var(--topology-tone,var(--theme-fg-muted)); background:transparent; border:0; border-radius:0; }
  .topology-canvas-wrap { position:relative; min-width:0; min-height:0; overflow:hidden; background:var(--theme-bg); touch-action:none; }
  .topology-legend { position:absolute; z-index:5; top:12px; left:14px; display:flex; flex-wrap:wrap; gap:12px; color:var(--theme-fg-muted); font-size:10px; pointer-events:none; }.topology-legend span { display:inline-flex; align-items:center; gap:5px; }.topology-legend i { width:7px; height:7px; border-radius:50%; }.topology-legend .application-key i { background:var(--theme-application); }.topology-legend .runtime-key i { background:var(--theme-success); }.topology-legend .resource-key i { background:var(--theme-resource); }.topology-legend .entity-key i { background:var(--theme-warning); }
  .topology-zoom-controls { position:absolute; z-index:6; right:12px; bottom:12px; display:flex; align-items:center; gap:4px; padding:4px; background:var(--theme-bg-raised); border:1px solid var(--theme-border); border-radius:5px; box-shadow:var(--theme-shadow-soft); }.topology-zoom-controls span { min-width:38px; color:var(--theme-fg-muted); font-size:10px; text-align:center; }
  .topology-surface { position:absolute; inset:0 auto auto 0; transform-origin:0 0; }.topology-edges { position:absolute; inset:0; overflow:visible; pointer-events:none; }.topology-edges path { fill:none; stroke:var(--theme-border-strong); stroke-width:1.8; pointer-events:stroke; cursor:pointer; transition:opacity .16s,stroke-width .16s; }.topology-edges path.edge-runs_on { stroke:var(--theme-success); }.topology-edges path.edge-uses { stroke:var(--theme-info); }.topology-edges path.edge-depends_on { stroke:var(--theme-accent); }.topology-edges path.edge-external_dependency { stroke:var(--theme-warning); stroke-dasharray:5 4; }.topology-edges path.selected { stroke-width:3; }.topology-edges .dimmed,.topology-node-card.dimmed { opacity:.2; }.topology-edges .connection-preview { stroke:var(--theme-accent); stroke-width:2; stroke-dasharray:5 4; pointer-events:none; }
  .topology-node-card { --topology-tone:var(--theme-accent); --topology-soft:var(--theme-bg-accent-soft); position:absolute; z-index:1; display:flex; align-items:center; width:158px; height:60px; border:1px solid color-mix(in srgb,var(--topology-tone) 38%,var(--theme-border)); border-left:3px solid var(--topology-tone); border-radius:5px; background:color-mix(in srgb,var(--topology-soft) 72%,var(--theme-bg-raised)); box-shadow:var(--theme-shadow-soft); }.topology-workspace.editing .topology-node-card { cursor:grab; }.topology-node-card.dragging { z-index:4; cursor:grabbing; }.topology-node-card.application-node { --topology-tone:var(--theme-application); --topology-soft:color-mix(in srgb,var(--theme-application) 14%,var(--theme-bg-raised)); }.topology-node-card.runtime-node { --topology-tone:var(--theme-success); --topology-soft:color-mix(in srgb,var(--theme-success) 14%,var(--theme-bg-subtle)); }.topology-node-card.resource-node { --topology-tone:var(--theme-resource); --topology-soft:color-mix(in srgb,var(--theme-resource) 14%,var(--theme-bg-subtle)); }.topology-node-card.entity-node { --topology-tone:var(--theme-warning); --topology-soft:var(--theme-bg-warning-soft); }.topology-node-card.pending-node { border:1px dashed color-mix(in srgb,var(--topology-tone) 65%,var(--theme-border)); border-left:3px dashed var(--topology-tone); opacity:.62; box-shadow:none; pointer-events:none; }.topology-node-card.selected { border-color:var(--theme-border-focus); box-shadow:0 0 0 2px color-mix(in srgb,var(--topology-tone) 22%,transparent),var(--theme-shadow-soft); }
  .topology-node-main { display:flex; align-items:center; gap:6px; width:100%; min-width:0; height:100%; padding:7px; color:var(--theme-fg); text-align:left; background:transparent; border:0; cursor:inherit; }.topology-node-icon { display:grid; flex:none; place-items:center; width:25px; height:25px; color:var(--topology-tone); }.topology-node-copy { display:grid; min-width:0; gap:3px; flex:1; }.topology-node-copy strong,.topology-node-copy small { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.topology-node-copy strong { font-size:11px; }.topology-node-copy small { color:var(--theme-fg-muted); font-size:9px; }.scope-mark { flex:none; padding:2px 4px; color:var(--theme-info); background:color-mix(in srgb,var(--theme-info) 14%,var(--theme-bg-raised)); border-radius:3px; font-size:8px; }.topology-alert-indicator { display:inline-flex; align-items:center; gap:2px; color:var(--theme-warning); font-size:9px; }
  .topology-anchor { position:absolute; z-index:3; display:grid; place-items:center; width:16px; height:16px; padding:0; background:transparent; border:0; cursor:crosshair; opacity:0; }.topology-anchor span { width:8px; height:8px; background:var(--topology-tone); border:1px solid var(--theme-bg-raised); border-radius:50%; box-shadow:0 0 0 1px color-mix(in srgb,var(--topology-tone) 60%,transparent); }.anchor-top { top:-8px; left:50%; transform:translateX(-50%); }.anchor-right { top:50%; right:-8px; transform:translateY(-50%); }.anchor-bottom { bottom:-8px; left:50%; transform:translateX(-50%); }.anchor-left { top:50%; left:-8px; transform:translateY(-50%); }.topology-node-card:hover .topology-anchor,.topology-anchor:focus { opacity:1; }
  .topology-state { display:flex; align-items:center; justify-content:center; gap:8px; min-height:360px; color:var(--theme-fg-muted); font-size:12px; text-align:center; }
  .topology-detail-float,.topology-link-config { position:absolute; z-index:8; display:grid; gap:8px; width:280px; padding:12px; color:var(--theme-fg); background:var(--theme-bg-raised); border:1px solid var(--theme-border-strong); border-radius:6px; box-shadow:var(--theme-shadow-soft); }.topology-detail-float header,.link-config-head,.topology-detail-float header span { display:flex; align-items:center; justify-content:space-between; gap:8px; }.topology-detail-float header span { justify-content:flex-start; color:var(--theme-fg-muted); }.topology-detail-float header strong,.link-config-head strong { color:var(--theme-fg); font-size:12px; }.topology-detail-name { overflow:hidden; font-size:13px; text-overflow:ellipsis; white-space:nowrap; }.topology-detail-float > small,.topology-link-config small { color:var(--theme-fg-muted); font-size:10px; line-height:1.45; }.topology-detail-float dl { display:grid; grid-template-columns:70px minmax(0,1fr); gap:6px 8px; margin:2px 0; font-size:10px; }.topology-detail-float dt { color:var(--theme-fg-muted); }.topology-detail-float dd { overflow-wrap:anywhere; margin:0; }.topology-detail-actions { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:8px; }.topology-detail-action,.topology-link-config .primary { display:inline-flex; align-items:center; justify-content:center; gap:7px; width:100%; min-height:30px; font-size:11px; }.topology-link-config p { display:flex; align-items:center; gap:6px; margin:0; font-size:11px; }.topology-link-config label { display:grid; gap:4px; color:var(--theme-fg-muted); font-size:10px; }.topology-link-config input,.topology-link-config select { width:100%; min-height:30px; padding:5px 7px; color:var(--theme-fg); background:var(--theme-bg); border:1px solid var(--theme-border-strong); border-radius:4px; }.topology-inline-warning { color:var(--theme-warning)!important; }.topology-edge-detail { right:14px; bottom:14px; left:auto; top:auto; }
  .topology-palette-leaf { display:flex; align-items:center; gap:7px; min-width:0; min-height:28px; margin-left:8px; padding:4px; color:var(--theme-fg-muted); text-align:left; background:transparent; border:0; font-size:11px; cursor:grab; }.topology-palette-leaf:active { cursor:grabbing; }
  @media(max-width:780px) { .topology-workspace.editing { grid-template-columns:1fr; height:auto; max-height:none; }.topology-palette { max-height:165px; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); border-right:0; border-bottom:1px solid var(--theme-divider); }.topology-palette-title { grid-column:1/-1; }.topology-canvas-wrap { min-height:520px; } }
</style>
