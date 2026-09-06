<script lang="ts">
  import { X } from 'lucide-svelte';
  import type { Relation, Resource, TopologyNode } from '../../lib/api';

  export let resource: Resource;
  export let resources: Resource[] = [];
  export let relations: Relation[] = [];
  export let topology: TopologyNode[] = [];
  export let target = '';
  export let relationType = 'depends_on';
  export let busy = false;
  export let relationBusy = false;
  export let onCreate: () => void = () => {};
  export let onDelete: (relation: Relation) => void = () => {};
</script>

<div class="relation-section">
  <div class="subheading"><h3>关系与拓扑</h3><span>{relations.length} 条关系 · {topology.length} 个节点</span></div>
  <form class="relation-form" on:submit|preventDefault={onCreate}>
    <select bind:value={target} required><option value="" disabled>选择目标资源</option>{#each resources.filter((item) => item.id !== resource.id) as candidate}<option value={candidate.id}>{candidate.name} · {candidate.kind}</option>{/each}</select>
    <select bind:value={relationType}><option value="depends_on">depends_on</option><option value="contains">contains</option><option value="deployed_on">deployed_on</option><option value="exposes">exposes</option><option value="uses_provider">uses_provider</option></select>
    <button class="secondary" disabled={busy || relationBusy}>建立关系</button>
  </form>
  {#if relations.length}<div class="relation-list">
    {#each relations as relation}
      {@const relatedId = relation.source_resource_id === resource.id ? relation.target_resource_id : relation.source_resource_id}
      <div class="relation-row"><span><strong>{relation.relation_type}</strong><small>{resources.find((item) => item.id === relatedId)?.name ?? '关联资源'}</small></span><button class="icon-button" type="button" data-tooltip="删除关系" aria-label="删除关系" disabled={busy || relationBusy} on:click={() => onDelete(relation)}><X size={14} aria-hidden="true" /></button></div>
    {/each}
  </div>{/if}
  {#if topology.length}<div class="topology-list">{#each topology as node}<span class="topology-node" style={`--depth: ${node.depth}`}>{node.depth} · {node.resource.name}</span>{/each}</div>{/if}
</div>
