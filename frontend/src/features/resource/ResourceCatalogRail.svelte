<script lang="ts">
  import { onMount } from 'svelte';
  import type { Resource } from '../../lib/api';
  import { Filter, X } from 'lucide-svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import {
    resourceCategoryFor,
    resourceCategoryIcon,
    resourceCategoryOptions,
    resourceSubtypeFor,
    resourceCatalogTagOptions,
    resourceCatalogTagsFor,
    resourceCatalogTagClass
  } from './resourceCatalog';

  export let resources: Resource[] = [];
  export let category = '全部';
  export let subtype = '全部';
  export let onSelect: (category: string, subtype?: string) => void = () => {};
  export let selectedCatalogTags: string[] = [];
  export let catalogQuery = '';
  export let onCatalogQuery: (value: string) => void = () => {};
  export let onToggleCatalogTag: (tag: string) => void = () => {};

  let expandedCategory = '';
  let filterOpen = false;
  let filterRoot: HTMLDivElement;

  function closeFilterOnOutside(event: PointerEvent) {
    if (filterOpen && filterRoot && !filterRoot.contains(event.target as Node)) filterOpen = false;
  }

  onMount(() => {
    window.addEventListener('pointerdown', closeFilterOnOutside);
    return () => window.removeEventListener('pointerdown', closeFilterOnOutside);
  });

  $: filteredCategories = Object.entries(resourceCategoryOptions).filter(([categoryName]) => {
    if (categoryName === '全部') return false;
    const tags = resourceCatalogTagsFor(categoryName);
    const query = catalogQuery.trim().toLowerCase();
    const hasQuery = Boolean(query);
    const hasTags = selectedCatalogTags.length > 0;
    const matchesQuery = [categoryName, ...tags].join(' ').toLowerCase().includes(query);
    const matchesTags = selectedCatalogTags.some((tag) => tags.includes(tag));
    if (!hasQuery && !hasTags) return true;
    return (hasQuery && matchesQuery) || (hasTags && matchesTags);
  });

  function select(categoryName: string, subtypeName = '全部') {
    expandedCategory = categoryName === '全部'
      ? ''
      : subtypeName === '全部' && expandedCategory === categoryName
        ? ''
        : categoryName;
    onSelect(categoryName, subtypeName);
  }
</script>

<section class="panel resource-list-panel">
  <div class="resource-catalog-rail">
    <button class:active={category === '全部'} class="catalog-root" type="button" on:click={() => select('全部')}>
      <span class="catalog-label">全部资源</span><span>{resources.length}</span>
    </button>
    <div class="resource-catalog-filter" bind:this={filterRoot}>
      <div class="resource-catalog-search-wrap">
        {#each selectedCatalogTags as tag}
          <button
            class={`resource-catalog-selected-tag resource-catalog-tag-${resourceCatalogTagClass(tag)}`}
            type="button"
            title={`移除筛选 ${tag}`}
            aria-label={`移除筛选 ${tag}`}
            on:click={() => onToggleCatalogTag(tag)}
          >{tag}<X size={11} aria-hidden="true" /></button>
        {/each}
        <input value={catalogQuery} on:input={(event) => onCatalogQuery((event.currentTarget as HTMLInputElement).value)} placeholder="搜索资源类型或标签" aria-label="搜索资源类型或标签" />
        <button class:active={filterOpen || selectedCatalogTags.length > 0} class="icon-button resource-catalog-filter-trigger" type="button" on:click={() => (filterOpen = !filterOpen)} title="按 Catalog 标签筛选" aria-label="按 Catalog 标签筛选" aria-expanded={filterOpen}>
          <Filter size={14} aria-hidden="true" />
        </button>
      </div>
      {#if filterOpen}
        <div class="resource-catalog-filter-popover" role="group" aria-label="Catalog 标签">
          {#each resourceCatalogTagOptions as tag}
            <button
              class:selected={selectedCatalogTags.includes(tag)}
              class="resource-catalog-filter-option"
              type="button"
              on:click={() => onToggleCatalogTag(tag)}
            ><span class={`resource-catalog-tag resource-catalog-tag-${resourceCatalogTagClass(tag)}`}>{tag}</span></button>
          {/each}
        </div>
      {/if}
    </div>
    {#each filteredCategories as [categoryName, subtypes]}
      <div class="catalog-category">
        <button class:active={category === categoryName && subtype === '全部'} class="catalog-category-button" type="button" on:click={() => select(categoryName)}>
          <span class="catalog-name"><span class="catalog-icon"><ResourceBrandIcon resource={{ kind: categoryName }} fallback={resourceCategoryIcon(categoryName)} size={20} /></span>{categoryName}{#each resourceCatalogTagsFor(categoryName) as tag}<em class={`resource-catalog-tag resource-catalog-tag-${resourceCatalogTagClass(tag)}`}>{tag}</em>{/each}</span>
          <span>{resources.filter((item) => resourceCategoryFor(item) === categoryName).length}</span>
        </button>
        {#if expandedCategory === categoryName}
          <div class="catalog-subtypes">
            {#each subtypes as subtypeName}
              <button class:active={category === categoryName && subtype === subtypeName} type="button" on:click={() => select(categoryName, subtypeName)}>
                <span class="catalog-name"><span class="catalog-icon subtype-icon"><ResourceBrandIcon resource={{ kind: categoryName, subtype: subtypeName }} fallback={resourceCategoryIcon(subtypeName)} size={15} /></span>{subtypeName}</span>
                <span>{resources.filter((item) => resourceCategoryFor(item) === categoryName && resourceSubtypeFor(item) === subtypeName).length}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/each}
  </div>
</section>
