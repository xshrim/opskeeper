<script lang="ts">
  import { onMount } from 'svelte';

  export let className = '';
  export let dialogClass = '';
  export let labelledBy = '';
  export let label = '';
  export let closeOnBackdrop = true;
  export let modal = true;
  export let inertTarget = '';
  export let maskTarget = '';
  export let positionTarget = '';
  export let widthTargetRatio = 0;
  export let overflowMode: 'clip' | 'scroll' | 'expand' = 'scroll';
  export let autoHeight = false;
  export let initialCenterRatio = 0.42;
  export let onBackdrop: () => void = () => {};

  let dialogElement: HTMLDialogElement;
  let targetMaskStyle = '';
  let positionInitialized = false;
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

  function updateTargetMask() {
    const target = maskTarget
      ? document.querySelector<HTMLElement>(maskTarget)
      : null;
    if (!target) {
      targetMaskStyle = '';
      return;
    }
    const rect = target.getBoundingClientRect();
    targetMaskStyle = `left:${rect.left}px;top:${rect.top}px;width:${rect.width}px;height:${rect.height}px`;
  }

  function clamp(value: number, min: number, max: number) {
    return Math.min(Math.max(value, min), Math.max(min, max));
  }

  function setGeometry(
    left: number,
    top: number,
    width: number,
    height: number,
    lockWidth = true,
    lockHeight = true,
    preservePosition = false
  ) {
    const maxWidth = Math.max(minWidth, window.innerWidth - viewportMargin * 2);
    const maxHeight = Math.max(minHeight, window.innerHeight - viewportMargin * 2);
    width = clamp(width, minWidth, maxWidth);
    height = clamp(height, minHeight, maxHeight);
    if (!preservePosition) {
      left = clamp(left, viewportMargin, window.innerWidth - width - viewportMargin);
      top = clamp(top, viewportMargin, window.innerHeight - height - viewportMargin);
    }
    dialogElement.style.left = `${left}px`;
    dialogElement.style.top = `${top}px`;
    dialogElement.style.width = lockWidth ? `${width}px` : '';
    dialogElement.style.height = lockHeight ? `${height}px` : 'auto';
    dialogElement.style.transform = 'none';
  }

  function positionDialog(preservePosition = false) {
    preservePosition = preservePosition && positionInitialized;
    updateTargetMask();
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
    if (overflowMode === 'expand' || autoHeight) {
      if (overflowMode === 'expand' && !(targetRect && widthTargetRatio > 0)) {
        dialogElement.style.width = '';
      }
      dialogElement.style.height = 'auto';
    }
    if (!targetRect && !autoHeight && overflowMode !== 'expand') return;
    const expandedRect = dialogElement.getBoundingClientRect();
    const left = preservePosition
      ? currentRect.left
      : targetRect
        ? targetRect.left + (targetRect.width - expandedRect.width) / 2
        : (window.innerWidth - expandedRect.width) / 2;
    const top = preservePosition
      ? currentRect.top
      : window.innerHeight * initialCenterRatio - expandedRect.height / 2;
    setGeometry(
      left,
      top,
      expandedRect.width,
      expandedRect.height,
      overflowMode !== 'expand' || Boolean(targetRect && widthTargetRatio > 0),
      overflowMode !== 'expand' && !autoHeight,
      preservePosition && autoHeight
    );
    positionInitialized = true;
  }

  function handlePointerDown(event: PointerEvent) {
    const target = event.target as HTMLElement | null;
    if (!target || target.closest('.dialog-resize-handle')) return;
    const header = target.closest('.modal-dialog-header');
    if (!header || target.closest('button, a, input, select, textarea, .message-banner, [data-dialog-no-drag]')) return;
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
    if (!closeOnBackdrop || event.currentTarget !== event.target) return;
    if (event.currentTarget === dialogElement) {
      const rect = dialogElement.getBoundingClientRect();
      if (
        event.clientX >= rect.left && event.clientX <= rect.right &&
        event.clientY >= rect.top && event.clientY <= rect.bottom
      ) return;
    }
    onBackdrop();
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

    const frame = requestAnimationFrame(() => positionDialog());
    const resizeObserver = autoHeight
      ? new ResizeObserver(() => requestAnimationFrame(() => positionDialog(true)))
      : null;
    resizeObserver?.observe(dialogElement);
    const handleWindowResize = () => positionDialog();
    window.addEventListener('resize', handleWindowResize);
    window.addEventListener('scroll', updateTargetMask, true);
    return () => {
      cancelAnimationFrame(frame);
      resizeObserver?.disconnect();
      window.removeEventListener('resize', handleWindowResize);
      window.removeEventListener('scroll', updateTargetMask, true);
      inertElements.forEach((element, index) => {
        element.inert = previousInert[index];
      });
      if (dialogElement.open) dialogElement.close();
    };
  });
</script>

<div class={`dialog-backdrop ${className}`.trim()} role="presentation" on:click={handleBackdrop}>
  {#if maskTarget && targetMaskStyle}
    <div class="dialog-target-mask" style={targetMaskStyle} aria-hidden="true" on:click={handleBackdrop}></div>
  {/if}
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
    width: 22px;
    height: 22px;
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
    width: 9px;
    height: 9px;
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
  .dialog-target-mask {
    position: fixed;
    z-index: 0;
    background: var(--theme-overlay);
    pointer-events: auto;
  }
</style>
