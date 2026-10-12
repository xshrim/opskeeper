<script lang="ts">
  import { onDestroy } from 'svelte';

  let target: HTMLElement | null = null;
  let text = '';
  let visible = false;
  let left = 0;
  let top = 0;
  let placement: 'top' | 'bottom' = 'bottom';

  function hide() { visible = false; target = null; }
  function position() {
    if (!target) return;
    const rect = target.getBoundingClientRect();
    placement = rect.bottom + 38 > window.innerHeight && rect.top > 38 ? 'top' : 'bottom';
    left = Math.min(Math.max(rect.left + rect.width / 2, 12), window.innerWidth - 12);
    top = placement === 'top' ? rect.top - 8 : rect.bottom + 8;
  }
  function show(node: HTMLElement) {
    const value = node.dataset.tooltip?.trim();
    if (!value || node.matches(':disabled')) return hide();
    target = node;
    text = value;
    position();
    visible = true;
  }
  function tooltipTarget(event: Event) {
    const node = event.target instanceof Element ? event.target.closest<HTMLElement>('[data-tooltip]') : null;
    return node && document.contains(node) ? node : null;
  }
  function onPointerOver(event: PointerEvent) { const node = tooltipTarget(event); if (node) show(node); }
  function onPointerOut(event: PointerEvent) {
    const node = tooltipTarget(event);
    if (node && node === target && !node.contains(event.relatedTarget as Node | null)) hide();
  }
  function onFocusIn(event: FocusEvent) { const node = tooltipTarget(event); if (node) show(node); }
  function onFocusOut(event: FocusEvent) {
    const node = tooltipTarget(event);
    if (node && node === target && !node.contains(event.relatedTarget as Node | null)) hide();
  }
  function onViewportChange() { if (visible) position(); }

  window.addEventListener('pointerover', onPointerOver);
  window.addEventListener('pointerout', onPointerOut);
  window.addEventListener('focusin', onFocusIn);
  window.addEventListener('focusout', onFocusOut);
  window.addEventListener('scroll', onViewportChange, true);
  window.addEventListener('resize', onViewportChange);
  onDestroy(() => {
    window.removeEventListener('pointerover', onPointerOver);
    window.removeEventListener('pointerout', onPointerOut);
    window.removeEventListener('focusin', onFocusIn);
    window.removeEventListener('focusout', onFocusOut);
    window.removeEventListener('scroll', onViewportChange, true);
    window.removeEventListener('resize', onViewportChange);
  });
</script>

{#if visible}
  <div class:top={placement === 'top'} class="tooltip-portal" role="tooltip" style={`left:${left}px;top:${top}px`}>{text}</div>
{/if}

<style>
  .tooltip-portal { position: fixed; z-index: 3000; width: max-content; max-width: min(240px, calc(100vw - 24px)); padding: 6px 9px; color: var(--theme-fg-strong); background: var(--theme-bg-raised); border: 1px solid var(--theme-border); border-radius: 5px; box-shadow: var(--theme-shadow-soft); font-size: 11px; font-weight: 500; line-height: 1.35; pointer-events: none; transform: translate(-50%, 0); }
  .tooltip-portal.top { transform: translate(-50%, -100%); }
  :global(:root[data-theme='dark']) .tooltip-portal { color: var(--theme-teal); border-color: var(--theme-border-focus); }
</style>
