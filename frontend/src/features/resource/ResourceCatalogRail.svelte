<script lang="ts">
  import type { Resource } from '../../lib/api';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import {
    resourceCategoryFor,
    resourceCategoryIcon,
    resourceCategoryOptions,
    resourceSubtypeFor
  } from './resourceCatalog';

  export let resources: Resource[] = [];
  export let category = '全部';
  export let subtype = '全部';
  export let onSelect: (category: string, subtype?: string) => void = () => {};

  let expandedCategory = '';

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
    {#each Object.entries(resourceCategoryOptions).filter(([name]) => name !== '全部') as [categoryName, subtypes]}
      <div class="catalog-category">
        <button class:active={category === categoryName && subtype === '全部'} class="catalog-category-button" type="button" on:click={() => select(categoryName)}>
          <span class="catalog-name"><span class="catalog-icon"><ResourceBrandIcon resource={{ kind: categoryName }} fallback={resourceCategoryIcon(categoryName)} size={20} /></span>{categoryName}</span>
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
