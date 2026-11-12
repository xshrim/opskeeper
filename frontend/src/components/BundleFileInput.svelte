<script lang="ts">
  import { Upload } from 'lucide-svelte';
  import { createEventDispatcher } from 'svelte';

  export let file: File | null = null;
  export let accept = '.bundle,application/octet-stream';
  export let ariaLabel = '选择git bundle文件上传';
  export let className = '';
  const dispatch = createEventDispatcher<{ change: File | null }>();

  function selectFile(event: Event) {
    file = (event.currentTarget as HTMLInputElement).files?.[0] ?? null;
    dispatch('change', file);
  }
</script>

<span class={`bundle-file-input ${className}`.trim()}>
  <Upload class="upload-icon" size={16} aria-hidden="true" />
  <span class="file-copy" title={file?.name ?? ariaLabel}
    >{file?.name ?? ariaLabel}</span
  >
  <span class="choose-button">选择文件</span>
  <input type="file" {accept} aria-label={ariaLabel} on:change={selectFile} />
</span>

<style>
  .bundle-file-input {
    position: relative;
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-width: 0;
    min-height: 36px;
    padding: 4px 5px 4px 10px;
    overflow: hidden;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border-strong);
    border-radius: 5px;
    box-sizing: border-box;
    cursor: pointer;
    transition:
      border-color 120ms ease,
      box-shadow 120ms ease,
      background 120ms ease;
  }
  .bundle-file-input:hover {
    background: var(--theme-bg-raised);
    border-color: var(--theme-border-focus);
  }
  .bundle-file-input:focus-within {
    border-color: var(--theme-border-focus);
    box-shadow: 0 0 0 3px var(--theme-focus);
  }
  .upload-icon {
    flex: 0 0 auto;
    color: var(--theme-accent);
  }
  .file-copy {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .choose-button {
    flex: 0 0 auto;
    padding: 5px 8px;
    color: var(--theme-fg-strong);
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    font-size: 11px;
    font-weight: 600;
    line-height: 1;
  }
  .bundle-file-input input {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    opacity: 0;
    cursor: pointer;
  }
</style>
