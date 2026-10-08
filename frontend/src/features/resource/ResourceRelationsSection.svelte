<script lang="ts">
  import { X } from 'lucide-svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
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
  $: targetOptions = resources.filter((item) => item.id !== resource.id).map((item) => ({ value: item.id, label: `${item.name} · ${item.kind}` }));
  const relationOptions = [
    { value: 'depends_on', label: 'depends_on' },
    { value: 'contains', label: 'contains' },
    { value: 'deployed_on', label: 'deployed_on' },
    { value: 'exposes', label: 'exposes' },
    { value: 'uses_provider', label: 'uses_provider' }
  ];
</script>

<div class="relation-section">
  <div class="subheading"><h3>关系与拓扑</h3><span>{relations.length} 条关系 · {topology.length} 个节点</span></div>
  <form class="relation-form" on:submit|preventDefault={onCreate}>
    <FormField label="目标资源" required><DropdownSelect bind:value={target} options={targetOptions} placeholder="选择目标资源" ariaLabel="目标资源" /></FormField>
    <DropdownSelect bind:value={relationType} options={relationOptions} ariaLabel="关系类型" />
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
