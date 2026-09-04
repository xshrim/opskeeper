<script lang="ts">
  import { Plus } from 'lucide-svelte';

  export let resourceCategory = '全部';
  export let resourceSubtype = '全部';
  export let resourceCount = 0;
  export let resourceSearch = '';
  export let resourceStatusFilter = 'all';
  export let resourceLevelFilter = 'all';
  export let resourceAddMenuOpen = false;
  export let onToggleAddMenu: () => void;
</script>

<div class="resource-catalog-toolbar">
  <div class="resource-catalog-title">
    <h2>{resourceCategory === '全部' ? '全部' : resourceSubtype === '全部' ? resourceCategory : `${resourceCategory} · ${resourceSubtype}`}</h2>
    <small>{resourceCount} 个可见资源</small>
  </div>
  <div class="resource-catalog-filters">
    <input class="resource-search" bind:value={resourceSearch} placeholder="搜索名称、端点或标签" aria-label="搜索资源" />
    <select bind:value={resourceStatusFilter} aria-label="连接状态"><option value="all">全部状态</option><option value="active">正常</option><option value="disabled">已停用</option><option value="unknown">未知</option></select>
    <select bind:value={resourceLevelFilter} aria-label="资源级别"><option value="all">全部级别</option><option value="platform">平台级</option><option value="team">团队级</option><option value="project">项目级</option></select>
    <button class="primary resource-add-menu-trigger" type="button" on:click={onToggleAddMenu} aria-expanded={resourceAddMenuOpen}><Plus size={15} strokeWidth={2} aria-hidden="true" />添加资源</button>
  </div>
</div>
