<script lang="ts">
  import { onDestroy, tick } from 'svelte';

  let target: HTMLElement | null = null;
  let text = '';
  let visible = false;
  let left = 0;
  let top = 0;
  let pointerPosition: { x: number; y: number } | null = null;
  let placement: 'top' | 'bottom' = 'bottom';
  let tooltipElement: HTMLDivElement;

  function hide() {
    if (tooltipElement?.matches(':popover-open')) tooltipElement.hidePopover();
    visible = false;
    target = null;
    pointerPosition = null;
  }
  function position(pointer?: { x: number; y: number }) {
    if (!target) return;
    if (pointer) {
      pointerPosition = pointer;
      left = pointer.x;
      top = pointer.y;
      return;
    }
    if (pointerPosition) {
      left = pointerPosition.x;
      top = pointerPosition.y;
      return;
    }
    const rect = target.getBoundingClientRect();
    placement = 'bottom';
    left = rect.left;
    top = rect.bottom + 8;
  }
  async function show(node: HTMLElement, pointer?: { x: number; y: number }) {
    const value = node.dataset.tooltip?.trim();
    if (!value || node.matches(':disabled')) return hide();
    target = node;
    text = value;
    position(pointer);
    visible = true;
    await tick();
    if (target === node && tooltipElement && !tooltipElement.matches(':popover-open')) {
      tooltipElement.showPopover();
    }
  }
  function tooltipTarget(event: Event) {
    const node = event.target instanceof Element ? event.target.closest<HTMLElement>('[data-tooltip]') : null;
    return node && document.contains(node) ? node : null;
  }
  function onPointerOver(event: PointerEvent) {
    const node = tooltipTarget(event);
    if (node) show(node, { x: event.clientX, y: event.clientY });
  }
  function onPointerMove(event: PointerEvent) {
    const node = tooltipTarget(event);
    if (node && node === target) position({ x: event.clientX, y: event.clientY });
  }
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
  window.addEventListener('pointermove', onPointerMove);
  window.addEventListener('pointerout', onPointerOut);
  window.addEventListener('focusin', onFocusIn);
  window.addEventListener('focusout', onFocusOut);
  window.addEventListener('scroll', onViewportChange, true);
  window.addEventListener('resize', onViewportChange);
  onDestroy(() => {
    window.removeEventListener('pointerover', onPointerOver);
    window.removeEventListener('pointermove', onPointerMove);
    window.removeEventListener('pointerout', onPointerOut);
    window.removeEventListener('focusin', onFocusIn);
    window.removeEventListener('focusout', onFocusOut);
    window.removeEventListener('scroll', onViewportChange, true);
    window.removeEventListener('resize', onViewportChange);
  });
</script>

{#if visible}
  <div bind:this={tooltipElement} popover="manual" class:top={placement === 'top'} class="tooltip-portal" role="tooltip" style={`left:${left}px;top:${top}px`}>{text}</div>
{/if}

<style>
  .tooltip-portal { position: fixed; inset: auto; z-index: 3000; width: max-content; max-width: min(420px, calc(100vw - 24px)); margin: 0; padding: 6px 9px; color: var(--theme-fg-strong); background: var(--theme-bg-raised); border: 1px solid var(--theme-border); border-radius: 5px; box-shadow: var(--theme-shadow-soft); font-size: 11px; font-weight: 500; line-height: 1.35; pointer-events: none; transform: none; white-space: pre-wrap; overflow-wrap: normal; word-break: normal; }
  .tooltip-portal.top { transform: none; }
  :global(:root[data-theme='dark']) .tooltip-portal { color: var(--theme-teal); border-color: var(--theme-border-focus); }
</style>
