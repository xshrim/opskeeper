<script lang="ts">
  import { afterUpdate, onMount } from 'svelte';
  import Tag from './Tag.svelte';

  export let tags: string[] = [];
  export let colorScheme: 'theme' | 'independent' = 'theme';
  export let distinguishKeyOnly = true;
  export let textMode: 'full' | 'ellipsis' | 'wrap' = 'ellipsis';
  export let tooltipOnTruncate = true;
  export let maxWidth = '100%';
  export let maxItems = 0;
  export let fit = false;
  export let className = '';
  export let gap = '4px';
  export let wrap = true;
  export let emptyText = '';
  export let ariaLabel = '';

  let listElement: HTMLElement;
  let measureElement: HTMLElement;
  let visibleCount = tags.length;
  let resizeObserver: ResizeObserver | undefined;

  $: maxVisibleCount = maxItems > 0 ? Math.min(maxItems, tags.length) : tags.length;
  $: if (!fit) visibleCount = maxVisibleCount;
  $: displayCount = fit ? Math.min(visibleCount, maxVisibleCount) : maxVisibleCount;

  function recalculateVisibleCount() {
    if (!fit || !listElement || !measureElement || !tags.length) {
      if (fit && visibleCount !== 0 && !tags.length) visibleCount = 0;
      return;
    }
    const available = listElement.clientWidth;
    if (!available) return;
    const measuredTags = Array.from(
      measureElement.querySelectorAll<HTMLElement>('[data-tag-measure="tag"]')
    );
    const overflowWidths = new Map(
      Array.from(measureElement.querySelectorAll<HTMLElement>('[data-overflow-count]')).map(
        (element) => [Number(element.dataset.overflowCount), element.offsetWidth]
      )
    );
    const gap = Number.parseFloat(getComputedStyle(listElement).gap) || 0;
    let nextCount = maxVisibleCount;
    for (let count = maxVisibleCount; count >= 0; count -= 1) {
      const tagWidth = measuredTags.slice(0, count).reduce((sum, element) => sum + element.offsetWidth, 0);
      const tagGap = gap * Math.max(0, count - 1);
      const hiddenCount = tags.length - count;
      const overflowWidth = hiddenCount ? (overflowWidths.get(hiddenCount) ?? 0) : 0;
      const overflowGap = hiddenCount && count ? gap : 0;
      if (tagWidth + tagGap + overflowWidth + overflowGap <= available) {
        nextCount = count;
        break;
      }
    }
    if (nextCount !== visibleCount) visibleCount = nextCount;
  }

  onMount(() => {
    if (!fit || !listElement) return;
    resizeObserver = new ResizeObserver(recalculateVisibleCount);
    resizeObserver.observe(listElement);
    recalculateVisibleCount();
    return () => resizeObserver?.disconnect();
  });

  afterUpdate(() => {
    if (fit) recalculateVisibleCount();
  });
</script>

<span
  class={`tag-list ${wrap ? 'tag-list-wrap' : ''} ${className}`}
  style={`--tag-list-gap: ${gap}`}
  aria-label={ariaLabel || undefined}
  bind:this={listElement}
>
  {#if tags.length}
    {#each tags.slice(0, displayCount) as tag, index (index)}
      <Tag
        value={tag}
        {colorScheme}
        {distinguishKeyOnly}
        {textMode}
        {tooltipOnTruncate}
        {maxWidth}
      />
    {/each}
    {#if tags.length > displayCount}
      <Tag value={`+${tags.length - displayCount}`} colorScheme={colorScheme} distinguishKeyOnly={false} textMode="full" overflow />
    {/if}
  {:else if emptyText}
    <span class="tag-list-empty">{emptyText}</span>
  {/if}
</span>
{#if fit}
  <span class="tag-list-measure" aria-hidden="true" bind:this={measureElement}>
    {#each tags as tag}
      <span data-tag-measure="tag"><Tag value={tag} {colorScheme} {distinguishKeyOnly} textMode="full" tooltipOnTruncate={false} {maxWidth} /></span>
    {/each}
    {#each Array(tags.length) as _, index}
      <span data-overflow-count={index + 1}><Tag value={`+${index + 1}`} colorScheme={colorScheme} distinguishKeyOnly={false} textMode="full" overflow /></span>
    {/each}
  </span>
{/if}

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

  .tag-list-measure {
    position: absolute;
    width: max-content;
    height: 0;
    overflow: hidden;
    visibility: hidden;
    pointer-events: none;
  }

  .tag-list-measure > span {
    display: inline-flex;
  }
</style>
