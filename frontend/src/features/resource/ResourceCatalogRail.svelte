<script lang="ts">
  import type { Resource } from '../../lib/api';
  import { Filter } from 'lucide-svelte';
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

  function categoryMatches(categoryName: string) {
    const tags = resourceCatalogTagsFor(categoryName);
    if (selectedCatalogTags.length && !selectedCatalogTags.some((tag) => tags.includes(tag))) return false;
    const query = catalogQuery.trim().toLowerCase();
    return !query || [categoryName, ...tags].join(' ').toLowerCase().includes(query);
  }

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
      <span class="catalog-icon"><ResourceBrandIcon resource={{ kind: 'Unknown' }} fallback={resourceCategoryIcon('全部')} size={20} /></span><span class="catalog-label">全部资源</span><span>{resources.length}</span>
    </button>
    <div class="resource-catalog-filter">
      <div class="resource-catalog-search-wrap">
        <input value={catalogQuery} on:input={(event) => onCatalogQuery((event.currentTarget as HTMLInputElement).value)} placeholder="搜索资源类型或标签" aria-label="搜索资源类型或标签" />
        <button class:active={filterOpen || selectedCatalogTags.length > 0} class="icon-button resource-catalog-filter-trigger" type="button" on:click={() => (filterOpen = !filterOpen)} title="按 Catalog 标签筛选" aria-label="按 Catalog 标签筛选" aria-expanded={filterOpen}>
          <Filter size={15} aria-hidden="true" />{#if selectedCatalogTags.length}<em>{selectedCatalogTags.length}</em>{/if}
        </button>
      </div>
      {#if filterOpen}
        <div class="resource-catalog-filter-popover" role="group" aria-label="Catalog 标签">
          {#each resourceCatalogTagOptions as tag}
            <label><input type="checkbox" checked={selectedCatalogTags.includes(tag)} on:change={() => onToggleCatalogTag(tag)} /> <span class={`resource-catalog-tag resource-catalog-tag-${resourceCatalogTagClass(tag)}`}>{tag}</span></label>
          {/each}
        </div>
      {/if}
    </div>
    {#each Object.entries(resourceCategoryOptions).filter(([name]) => name !== '全部' && categoryMatches(name)) as [categoryName, subtypes]}
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
