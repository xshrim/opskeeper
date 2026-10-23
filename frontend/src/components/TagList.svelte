<script lang="ts">
  import Tag from './Tag.svelte';

  export let tags: string[] = [];
  export let colorScheme: 'theme' | 'independent' = 'theme';
  export let distinguishKeyOnly = true;
  export let textMode: 'full' | 'ellipsis' | 'wrap' = 'ellipsis';
  export let tooltipOnTruncate = true;
  export let maxWidth = '100%';
  export let maxItems = 0;
  export let className = '';
  export let gap = '4px';
  export let wrap = true;
  export let emptyText = '';
  export let ariaLabel = '';
</script>

<span
  class={`tag-list ${wrap ? 'tag-list-wrap' : ''} ${className}`}
  style={`--tag-list-gap: ${gap}`}
  aria-label={ariaLabel || undefined}
>
  {#if tags.length}
    {#each (maxItems > 0 ? tags.slice(0, maxItems) : tags) as tag, index (index)}
      <Tag
        value={tag}
        {colorScheme}
        {distinguishKeyOnly}
        {textMode}
        {tooltipOnTruncate}
        {maxWidth}
      />
    {/each}
    {#if maxItems > 0 && tags.length > maxItems}
      <Tag value={`+${tags.length - maxItems}`} colorScheme={colorScheme} distinguishKeyOnly={false} textMode="full" overflow />
    {/if}
  {:else if emptyText}
    <span class="tag-list-empty">{emptyText}</span>
  {/if}
</span>

<style>
  .tag-list {
    display: flex;
    flex-wrap: nowrap;
    align-items: center;
    gap: var(--tag-list-gap);
    min-width: 0;
  }

  .tag-list-wrap {
    flex-wrap: wrap;
  }

  .tag-list-empty {
    color: var(--theme-fg-disabled);
    font-size: 10px;
  }
</style>
