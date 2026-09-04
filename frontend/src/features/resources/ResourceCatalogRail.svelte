<script lang="ts">
  import type { Resource } from '../../lib/api';

  export let resourceCategoryOptions: Record<string, string[]> = {};
  export let visibleResources: Resource[] = [];
  export let resourceCategory = '全部';
  export let resourceSubtype = '全部';
  export let expandedResourceCategory = '';
  export let resourceCategoryIcon: (category: string) => string;
  export let resourceCategoryFor: (resource: { kind: string; subtype?: string; config?: Record<string, unknown> }) => string;
  export let resourceSubtypeFor: (resource: { kind: string; subtype?: string; config?: Record<string, unknown> }) => string;
  export let onSelectCategory: (category: string, subtype?: string) => void;
</script>

<div class="resource-catalog-rail">
  <button class:active={resourceCategory === '全部'} class="catalog-root" type="button" on:click={() => onSelectCategory('全部')}><span class="catalog-icon">{resourceCategoryIcon('全部')}</span><span class="catalog-label">全部资源</span><span>{visibleResources.length}</span></button>
  {#each Object.entries(resourceCategoryOptions).filter(([name]) => name !== '全部') as [category, subtypes]}
    <div class="catalog-category">
      <button class:active={resourceCategory === category && resourceSubtype === '全部'} class="catalog-category-button" type="button" on:click={() => onSelectCategory(category)}><span class="catalog-name"><span class="catalog-icon">{resourceCategoryIcon(category)}</span>{category}</span><span>{visibleResources.filter((item) => resourceCategoryFor(item) === category).length}</span></button>
      {#if expandedResourceCategory === category}
        <div class="catalog-subtypes">
          {#each subtypes as subtype}
            <button class:active={resourceCategory === category && resourceSubtype === subtype} type="button" on:click={() => onSelectCategory(category, subtype)}><span class="catalog-name"><span class="catalog-icon subtype-icon">{resourceCategoryIcon(subtype)}</span>{subtype}</span><span>{visibleResources.filter((item) => resourceCategoryFor(item) === category && resourceSubtypeFor(item) === subtype).length}</span></button>
          {/each}
        </div>
      {/if}
    </div>
  {/each}
</div>
