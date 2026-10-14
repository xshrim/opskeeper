<script lang="ts">
  import Dialog from './Dialog.svelte';
  import MessageBanner from './MessageBanner.svelte';

  type DialogCoverage = 'page' | 'target' | 'none';
  type DialogOverflow = 'clip' | 'scroll' | 'expand';

  export let title = '';
  export let description = '';
  export let confirmLabel = '确认';
  export let busyLabel = '';
  export let busy = false;
  export let confirmDisabled = false;
  export let confirmVariant: 'primary' | 'danger' = 'primary';
  export let confirmForm = '';
  export let backLabel = '';
  export let backDisabled = false;
  export let dialogClass = '';
  export let contentInset = true;
  export let message = '';
  export let messageTone: 'success' | 'error' = 'error';
  // Modal dialogs require an explicit action to close; callers can opt into
  // backdrop dismissal for the few non-critical flows that need it.
  export let closeOnBackdrop = false;
  export let size: 'small' | 'medium' | 'large' | 'wide' = 'medium';
  export let coverage: DialogCoverage = 'page';
  export let inertTarget = '';
  export let positionTarget = '';
  export let widthTargetRatio = 0;
  export let overflowMode: DialogOverflow = 'scroll';
  export let autoHeight = true;
  export let onCancel: () => void = () => {};
  export let onBack: () => void | Promise<void> = () => {};
  export let onConfirm: () => void | Promise<void> = () => {};

  $: backdropClass = `modal-dialog-backdrop modal-dialog-coverage-${coverage}`;
  $: modal = coverage === 'page';

  function confirm() {
    if (!confirmForm) void onConfirm();
  }
</script>

<Dialog
  className={backdropClass}
  dialogClass={`modal-dialog modal-dialog-size-${size} ${dialogClass}`.trim()}
  label={title}
  {modal}
  inertTarget={coverage === 'target' ? inertTarget : ''}
  maskTarget={coverage === 'target' ? inertTarget : ''}
  {positionTarget}
  {widthTargetRatio}
  {overflowMode}
  {autoHeight}
  {closeOnBackdrop}
  onBackdrop={onCancel}
>
  <header class="modal-dialog-header">
    <div class="modal-dialog-title">
      <h2>{title}</h2>
      {#if description}<p>{description}</p>{/if}
    </div>
    {#if message}<MessageBanner {message} tone={messageTone} />{/if}
    <div class="modal-dialog-actions" data-dialog-no-drag>
      <button class="secondary" type="button" disabled={busy} on:click={onCancel}>取消</button>
      <slot name="actions" />
      {#if backLabel}<button class="secondary" type="button" disabled={busy || backDisabled} on:click={() => void onBack()}>{backLabel}</button>{/if}
      <button class={confirmVariant === 'danger' ? 'danger-button' : 'primary'} type={confirmForm ? 'submit' : 'button'} form={confirmForm || undefined} disabled={busy || confirmDisabled} on:click={confirm}>{busy && busyLabel ? busyLabel : confirmLabel}</button>
    </div>
  </header>
  <div class={`modal-dialog-content modal-dialog-content-overflow-${overflowMode}`} class:modal-dialog-content-flush={!contentInset}><slot /></div>
</Dialog>

<style>
  :global(.dialog.modal-dialog) {
    position: fixed;
    display: flex;
    flex-direction: column;
    padding: 0;
    overflow: hidden;
    box-sizing: border-box;
    inset: auto;
    left: 50%;
    top: 50%;
    margin: 0;
    transform: translate(-50%, -50%);
  }
  .modal-dialog-header {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    box-sizing: border-box;
    height: 60px;
    min-height: 60px;
    padding: 12px 15px;
    border-bottom: 1px solid var(--theme-divider);
    cursor: move;
    touch-action: none;
    user-select: none;
  }
  .modal-dialog-title { min-width: 0; }
  .modal-dialog-title h2 {
    margin: 0;
    color: var(--theme-fg-strong);
    font-size: 15px;
    line-height: 1.25;
  }
  .modal-dialog-title p {
    display: block;
    margin: 4px 0 0;
    color: var(--theme-fg-muted);
    font-size: 11px;
  }
  .modal-dialog-header :global(.message-banner) {
    position: absolute;
    top: 50%;
    left: 50%;
    z-index: 1;
    width: min(420px, calc(100% - 160px));
    transform: translate(-50%, -50%);
  }
  .modal-dialog-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 7px; }
  .modal-dialog-actions :global(button) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    min-height: 37px;
    padding: 0 14px;
    border: 1px solid var(--theme-border);
    border-radius: 5px;
    font-size: 12px;
    font-weight: 700;
  }
  .modal-dialog-actions :global(button.primary),
  .modal-dialog-actions :global(button.secondary) {
    min-height: 37px;
    padding: 0 14px;
    font-size: 12px;
    font-weight: 700;
  }
  .modal-dialog-actions :global(button.primary) {
    color: var(--theme-accent-contrast);
    background: var(--color-primary);
    border-color: var(--color-primary);
  }
  .modal-dialog-actions :global(button.secondary) {
    color: var(--theme-fg);
    background: var(--theme-bg-surface);
  }
  .modal-dialog-actions :global(button.danger-button) {
    color: #fff;
    background: #b84b38;
    border-color: transparent;
  }
  .modal-dialog-actions :global(button) { cursor: pointer; }
  .modal-dialog-content { flex: 1 1 auto; min-width: 0; min-height: 0; padding: 11px 15px; }
  .modal-dialog-content :global(.stack-form) { padding: 0; }
  .modal-dialog-content-overflow-clip { overflow: hidden; }
  .modal-dialog-content-overflow-scroll { overflow: auto; }
  .modal-dialog-content-overflow-expand { overflow: auto; }
  .modal-dialog-content.modal-dialog-content-flush { padding: 0; }
  :global(.dialog.modal-dialog) { max-height: min(760px, calc(100vh - 40px)); border-radius: 7px; }
  :global(.dialog.modal-dialog-size-small) { width: min(480px, calc(100vw - 40px)); }
  :global(.dialog.modal-dialog-size-medium) { width: min(680px, calc(100vw - 40px)); }
  :global(.dialog.modal-dialog-size-large) { width: min(900px, calc(100vw - 40px)); }
  :global(.dialog.modal-dialog-size-wide) { width: min(1100px, calc(100vw - 40px)); }
  :global(.dialog.modal-dialog.application-wizard-dialog) { width: min(980px, calc(100vw - 40px)); }
  :global(.dialog.modal-dialog.wide-dialog) { width: min(720px, calc(100vw - 40px)); }
  :global(.dialog.dialog-overflow-expand) {
    width: max-content;
    max-width: calc(100vw - 40px);
    max-height: calc(100vh - 40px);
  }
  :global(.dialog-backdrop.modal-dialog-backdrop) {
    position: fixed;
    inset: 0;
    z-index: 30;
    display: block;
    padding: 0;
    background: var(--theme-overlay);
  }
  :global(.dialog-backdrop.modal-dialog-coverage-target),
  :global(.dialog-backdrop.modal-dialog-coverage-none) {
    pointer-events: none;
    background: transparent;
  }
  :global(.dialog-backdrop.modal-dialog-coverage-target .dialog),
  :global(.dialog-backdrop.modal-dialog-coverage-none .dialog) { pointer-events: auto; }
  :global(.dialog-backdrop.modal-dialog-coverage-target .dialog) { z-index: 1; }
  @media (max-width: 720px) {
    .modal-dialog-header { padding-right: 16px; padding-left: 16px; }
    .modal-dialog-content { padding: 9px 16px; }
    .modal-dialog-content.modal-dialog-content-flush { padding: 0; }
    .modal-dialog-header { align-items: flex-start; }
    .modal-dialog-header :global(.message-banner) { width: calc(100% - 32px); top: calc(100% + 8px); }
    :global(.dialog.modal-dialog) { width: min(100%, calc(100vw - 40px)); max-height: calc(100vh - 40px); border-radius: 7px; }
  }
</style>
