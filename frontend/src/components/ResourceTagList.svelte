<script lang="ts">
  import { onMount, tick } from 'svelte';

  export let tags: string[] = [];
  export let emptyText = '未设置标签';

  let container: HTMLSpanElement;
  let measurement: HTMLSpanElement;
  let visibleCount = tags.length;
  let resizeObserver: ResizeObserver | undefined;

  async function recalculate() {
    await tick();
    if (!container || !measurement || !tags.length) {
      visibleCount = tags.length;
      return;
    }
    const available = container.clientWidth;
    const items = Array.from(measurement.querySelectorAll<HTMLElement>('[data-tag-measure]'));
    const overflow = measurement.querySelector<HTMLElement>('[data-tag-overflow]');
    if (!available || items.length !== tags.length || !overflow) return;
    const gap = 4;
    const overflowWidth = (count: number) => {
      overflow.textContent = `+${count}`;
      return overflow.offsetWidth;
    };
    let used = 0;
    let count = 0;
    for (let index = 0; index < items.length; index += 1) {
      const remaining = items.length - index - 1;
      const nextWidth = items[index].offsetWidth;
      const nextUsed = used + (count ? gap : 0) + nextWidth;
      const withOverflow = remaining ? nextUsed + gap + overflowWidth(remaining) : nextUsed;
      if (withOverflow > available) break;
      used = nextUsed;
      count += 1;
    }
    visibleCount = count || 0;
  }

  $: if (tags) void recalculate();

  onMount(() => {
    resizeObserver = new ResizeObserver(() => void recalculate());
    if (container) resizeObserver.observe(container);
    return () => resizeObserver?.disconnect();
  });
</script>

<span class="resource-tags-group">
  {#if tags.length}
    <span bind:this={container} class="resource-tags" aria-label="资源标签">
      {#each tags.slice(0, visibleCount) as tag (tag)}
        <span class="resource-tag" title={tag}>{tag}</span>
      {/each}
      {#if visibleCount < tags.length}<span class="resource-tag resource-tag-overflow">+{tags.length - visibleCount}</span>{/if}
    </span>
    <span bind:this={measurement} class="resource-tags-measure" aria-hidden="true">
      {#each tags as tag (tag)}<span class="resource-tag" data-tag-measure>{tag}</span>{/each}
      <span class="resource-tag resource-tag-overflow" data-tag-overflow>+0</span>
    </span>
  {:else}
    <span class="resource-tags resource-tags-empty-state"><small class="resource-tags-empty">{emptyText}</small></span>
  {/if}
</span>
