<script lang="ts">
  import { onMount } from 'svelte';

  export let className = '';
  export let dialogClass = '';
  export let labelledBy = '';
  export let label = '';
  export let closeOnBackdrop = true;
  export let modal = true;
  export let inertTarget = '';
  export let positionTarget = '';
  export let widthTargetRatio = 0;
  export let overflowMode: 'clip' | 'scroll' | 'expand' = 'scroll';
  export let onBackdrop: () => void = () => {};

  let dialogElement: HTMLDialogElement;
  let interaction:
    | { kind: 'drag'; pointerId: number; offsetX: number; offsetY: number }
    | {
        kind: 'resize';
        pointerId: number;
        edge: 'left' | 'right';
        rect: DOMRect;
      }
    | null = null;

  const viewportMargin = 20;
  const minWidth = 320;
  const minHeight = 220;

  function clamp(value: number, min: number, max: number) {
    return Math.min(Math.max(value, min), Math.max(min, max));
  }

  function setGeometry(
    left: number,
    top: number,
    width: number,
    height: number,
    lockWidth = true,
    lockHeight = true
  ) {
    const maxWidth = Math.max(minWidth, window.innerWidth - viewportMargin * 2);
    const maxHeight = Math.max(minHeight, window.innerHeight - viewportMargin * 2);
    width = clamp(width, minWidth, maxWidth);
    height = clamp(height, minHeight, maxHeight);
    left = clamp(left, viewportMargin, window.innerWidth - width - viewportMargin);
    top = clamp(top, viewportMargin, window.innerHeight - height - viewportMargin);
    dialogElement.style.left = `${left}px`;
    dialogElement.style.top = `${top}px`;
    dialogElement.style.width = lockWidth ? `${width}px` : '';
    dialogElement.style.height = lockHeight ? `${height}px` : 'auto';
    dialogElement.style.transform = 'none';
  }

  function positionDialog() {
    const target = positionTarget
      ? document.querySelector<HTMLElement>(positionTarget)
      : null;
    const targetRect = target?.getBoundingClientRect();
    const currentRect = dialogElement.getBoundingClientRect();
    let width = currentRect.width;
    if (targetRect && widthTargetRatio > 0) {
      width = clamp(
        targetRect.width * widthTargetRatio,
        minWidth,
        window.innerWidth - viewportMargin * 2
      );
      dialogElement.style.width = `${width}px`;
    }
    const measuredRect = dialogElement.getBoundingClientRect();
    if (!targetRect) return;
    const left = targetRect.left + (targetRect.width - measuredRect.width) / 2;
    const top = (window.innerHeight - measuredRect.height) / 2;
    setGeometry(
      left,
      top,
      measuredRect.width,
      measuredRect.height,
      overflowMode !== 'expand' || Boolean(positionTarget && widthTargetRatio > 0),
      overflowMode !== 'expand'
    );
  }

  function handlePointerDown(event: PointerEvent) {
    const target = event.target as HTMLElement | null;
    if (!target || target.closest('.dialog-resize-handle')) return;
    const header = target.closest('.modal-dialog-header');
    if (!header || target.closest('button, a, input, select, textarea, [data-dialog-no-drag]')) return;
    const rect = dialogElement.getBoundingClientRect();
    interaction = {
      kind: 'drag',
      pointerId: event.pointerId,
      offsetX: event.clientX - rect.left,
      offsetY: event.clientY - rect.top
    };
    dialogElement.setPointerCapture?.(event.pointerId);
    event.preventDefault();
  }

  function handleResizePointerDown(event: PointerEvent, edge: 'left' | 'right') {
    const rect = dialogElement.getBoundingClientRect();
    interaction = { kind: 'resize', pointerId: event.pointerId, edge, rect };
    dialogElement.setPointerCapture?.(event.pointerId);
    event.preventDefault();
    event.stopPropagation();
  }

  function handlePointerMove(event: PointerEvent) {
    if (!interaction || interaction.pointerId !== event.pointerId) return;
    if (interaction.kind === 'drag') {
      setGeometry(
        event.clientX - interaction.offsetX,
        event.clientY - interaction.offsetY,
        dialogElement.getBoundingClientRect().width,
        dialogElement.getBoundingClientRect().height
      );
      return;
    }
    const { rect, edge } = interaction;
    const width = edge === 'left' ? rect.right - event.clientX : event.clientX - rect.left;
    const left = edge === 'left' ? rect.right - width : rect.left;
    const height = event.clientY - rect.top;
    setGeometry(left, rect.top, width, height);
  }

  function handlePointerUp(event: PointerEvent) {
    if (interaction?.pointerId === event.pointerId) {
      dialogElement.releasePointerCapture?.(event.pointerId);
      interaction = null;
    }
  }

  function handleCancel(event: Event) {
    event.preventDefault();
    if (closeOnBackdrop) onBackdrop();
  }

  function handleBackdrop(event: MouseEvent) {
    if (closeOnBackdrop && event.currentTarget === event.target) onBackdrop();
  }

  onMount(() => {
    if (modal) dialogElement.showModal();
    else dialogElement.show();

    const inertElements = inertTarget
      ? Array.from(document.querySelectorAll<HTMLElement>(inertTarget))
      : [];
    const previousInert = inertElements.map((element) => element.inert);
    inertElements.forEach((element) => {
      element.inert = true;
    });

    const frame = requestAnimationFrame(positionDialog);
    window.addEventListener('resize', positionDialog);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('resize', positionDialog);
      inertElements.forEach((element, index) => {
        element.inert = previousInert[index];
      });
      if (dialogElement.open) dialogElement.close();
    };
  });
</script>

<div class={`dialog-backdrop ${className}`.trim()} role="presentation" on:click={handleBackdrop}>
  <dialog
    bind:this={dialogElement}
    aria-modal={modal ? 'true' : undefined}
    class={`dialog ${dialogClass} dialog-overflow-${overflowMode}`.trim()}
    aria-labelledby={labelledBy || undefined}
    aria-label={label || undefined}
    on:click={handleBackdrop}
    on:cancel={handleCancel}
    on:pointerdown={handlePointerDown}
    on:pointermove={handlePointerMove}
    on:pointerup={handlePointerUp}
    on:pointercancel={handlePointerUp}
  >
    <slot />
    <button
      class="dialog-resize-handle dialog-resize-handle-left"
      type="button"
      aria-label="调整弹窗大小"
      on:pointerdown={(event) => handleResizePointerDown(event, 'left')}
    ></button>
    <button
      class="dialog-resize-handle dialog-resize-handle-right"
      type="button"
      aria-label="调整弹窗大小"
      on:pointerdown={(event) => handleResizePointerDown(event, 'right')}
    ></button>
  </dialog>
</div>

<style>
  .dialog-resize-handle {
    position: absolute;
    bottom: 0;
    z-index: 3;
    width: 18px;
    height: 18px;
    padding: 0;
    background: transparent;
    border: 0;
    cursor: nwse-resize;
    touch-action: none;
  }
  .dialog-resize-handle-left { left: 0; cursor: nesw-resize; }
  .dialog-resize-handle-right { right: 0; }
  .dialog-resize-handle::after {
    position: absolute;
    right: 4px;
    bottom: 4px;
    width: 7px;
    height: 7px;
    border-right: 1px solid var(--theme-fg-subtle);
    border-bottom: 1px solid var(--theme-fg-subtle);
    content: '';
    opacity: 0.65;
  }
  .dialog-resize-handle-left::after {
    right: auto;
    left: 4px;
    transform: scaleX(-1);
  }
</style>
