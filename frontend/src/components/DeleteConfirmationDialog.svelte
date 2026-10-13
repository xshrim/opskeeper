<script lang="ts">
  import ModalDialog from './ModalDialog.svelte';
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

<ModalDialog {title} description="输入对象名称以确认删除。" size="small" dialogClass="confirm-dialog" confirmLabel="确认删除" busyLabel="正在删除" confirmVariant="danger" confirmDisabled={!confirmed} {busy} closeOnBackdrop={false} {onCancel} {onConfirm}>
  <p class="confirm-copy">{description}</p>
  {#if impact}<p class="confirm-impact">{impact}</p>{/if}
  <label class="delete-confirmation-field">输入 <strong>{name}</strong> 确认删除<TextInput bind:value={confirmation} ariaLabel={`输入 ${name} 确认删除`} autocomplete="off" /></label>
</ModalDialog>

<style>
  .confirm-copy, .confirm-impact { margin: 0; color: var(--theme-fg-muted); font-size: 12px; line-height: 1.6; }
  .confirm-impact { margin-top: 16px; }
  .delete-confirmation-field { display: grid; gap: 7px; margin-top: 16px; color: var(--theme-fg-muted); font-size: 12px; }
  .delete-confirmation-field strong { color: var(--theme-fg-strong); }
</style>
