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
  export let dialogClass = '';
  export let contentInset = true;
  export let message = '';
  export let messageTone: 'success' | 'error' = 'error';
  export let closeOnBackdrop = true;
  export let size: 'small' | 'medium' | 'large' | 'wide' = 'medium';
  export let coverage: DialogCoverage = 'page';
  export let inertTarget = '';
  export let positionTarget = '';
  export let widthTargetRatio = 0;
  export let overflowMode: DialogOverflow = 'scroll';
  export let onCancel: () => void = () => {};
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
  {positionTarget}
  {widthTargetRatio}
  {overflowMode}
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
    height: 56px;
    min-height: 56px;
    padding: 6px 20px;
    border-bottom: 1px solid var(--theme-divider);
    cursor: move;
    touch-action: none;
    user-select: none;
  }
  .modal-dialog-title { min-width: 0; }
  .modal-dialog-title h2 { margin: 0; color: var(--theme-fg-strong); font-size: 17px; }
  .modal-dialog-title p { margin: 4px 0 0; color: var(--theme-fg-muted); font-size: 11px; line-height: 1.4; }
  .modal-dialog-header :global(.message-banner) {
    position: absolute;
    top: 50%;
    left: 50%;
    z-index: 1;
    width: min(420px, calc(100% - 250px));
    transform: translate(-50%, -50%);
  }
  .modal-dialog-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }
  .modal-dialog-actions :global(button) { cursor: pointer; }
  .modal-dialog-content { flex: 1 1 auto; min-width: 0; min-height: 0; padding: 22px 24px; }
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
  @media (max-width: 720px) {
    .modal-dialog-header { padding-right: 16px; padding-left: 16px; }
    .modal-dialog-content { padding: 18px 16px; }
    .modal-dialog-content.modal-dialog-content-flush { padding: 0; }
    .modal-dialog-header { align-items: flex-start; }
    .modal-dialog-header :global(.message-banner) { width: calc(100% - 32px); top: calc(100% + 8px); }
    :global(.dialog.modal-dialog) { width: min(100%, calc(100vw - 40px)); max-height: calc(100vh - 40px); border-radius: 7px; }
  }
</style>
