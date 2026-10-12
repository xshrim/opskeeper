<script lang="ts">
  import Dialog from './Dialog.svelte';
  import TextInput from './TextInput.svelte';

  export let title = '确认删除';
  export let name = '';
  export let description = '';
  export let impact = '';
  export let busy = false;
  export let onCancel: () => void = () => {};
  export let onConfirm: () => void | Promise<void> = () => {};
  let confirmation = '';

  $: confirmed = confirmation === name;
</script>

<Dialog dialogClass="confirm-dialog" labelledBy="delete-confirmation-title" closeOnBackdrop={false}>
  <div class="dialog-heading">
    <div><p class="eyebrow">DESTRUCTIVE ACTION</p><h2 id="delete-confirmation-title">{title}</h2></div>
  </div>
  <p class="confirm-copy">{description}</p>
  {#if impact}<p class="form-help">{impact}</p>{/if}
  <label class="delete-confirmation-field">输入 <strong>{name}</strong> 确认删除<TextInput bind:value={confirmation} ariaLabel={`输入 ${name} 确认删除`} autocomplete="off" /></label>
  <div class="dialog-actions">
    <button class="secondary" type="button" disabled={busy} on:click={onCancel}>取消</button>
    <button class="danger-button" type="button" disabled={busy || !confirmed} on:click={onConfirm}>{busy ? '正在删除' : '确认删除'}</button>
  </div>
</Dialog>

<style>
  .delete-confirmation-field { display: grid; gap: 7px; margin-top: 16px; color: var(--theme-fg-muted); font-size: 12px; }
  .delete-confirmation-field strong { color: var(--theme-fg-strong); }
  .dialog-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 18px; }
</style>
