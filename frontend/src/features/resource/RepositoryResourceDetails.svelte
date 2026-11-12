<script lang="ts">
  import { onMount } from 'svelte';
  import type { Resource } from '../../lib/api';
  import { appURL } from '../../lib/health';
  import BundleFileInput from '../../components/BundleFileInput.svelte';
  import RepositoryPreviewContent from './RepositoryPreviewContent.svelte';
  export let resource: Resource;
  let file: File | null = null;
  let busy = false;
  let message = '';
  let previewBusy = false;
  let previewMessage = '';
  let previewEntries: Array<{ path: string; type: string; size?: number }> = [];
  async function preview() {
    if (previewBusy) return;
    previewBusy = true;
    previewMessage = '';
    try {
      const response = await fetch(
        appURL(`api/v1/resources/${resource.id}/repository/tree`),
        { credentials: 'include' }
      );
      const data = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(
          data?.error?.message ?? data?.message ?? '仓库预览失败'
        );
      previewEntries = Array.isArray(data.entries) ? data.entries : [];
    } catch (error) {
      previewMessage = error instanceof Error ? error.message : '仓库预览失败';
      previewEntries = [];
    } finally {
      previewBusy = false;
    }
  }
  async function upload() {
    if (!file || busy) return;
    busy = true;
    message = '';
    try {
      const body = new FormData();
      body.append('bundle', file);
      const response = await fetch(
        appURL(`api/v1/resources/${resource.id}/bundle`),
        { method: 'POST', body, credentials: 'include' }
      );
      const data = await response.json().catch(() => ({}));
      if (!response.ok)
        throw new Error(data?.error?.message ?? data?.message ?? '上传失败');
      const branches = Array.isArray(data.branches) ? data.branches : [];
      message = `上传成功，已同步 ${branches.length} 个分支`;
      await preview();
    } catch (error) {
      message = error instanceof Error ? error.message : '上传失败';
    } finally {
      busy = false;
    }
  }
  onMount(() => void preview());
</script>

<section class="repository-preview">
  <div class="preview-heading">
    <div class="preview-title">
      <strong>仓库预览</strong><small>读取当前仓库默认分支的全部文件</small>
    </div>
    <div class="preview-actions">
      {#if String(resource.subtype).toLowerCase() === 'bundle'}
        <BundleFileInput
          bind:file
          className="preview-bundle-file-input"
          on:change={() => (message = '')}
        />
        <button
          class="primary"
          type="button"
          disabled={!file || busy || previewBusy}
          on:click={() => void upload()}>{busy ? '上传中…' : '上传'}</button
        >
      {/if}
      <button
        class="secondary"
        type="button"
        disabled={previewBusy || busy}
        on:click={() => void preview()}
        >{previewBusy ? '读取中…' : '刷新'}</button
      >
    </div>
  </div>
  {#if message}<small
      class:success={message.startsWith('上传成功')}
      class:repository-upload-error={!message.startsWith('上传成功')}
      class="repository-upload-message">{message}</small
    >{/if}
  {#if previewMessage}<small class="form-error">{previewMessage}</small>{/if}
  <RepositoryPreviewContent
    entries={previewEntries}
    busy={previewBusy}
    failed={Boolean(previewMessage)}
  />
</section>

<style>
  .preview-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    min-width: 0;
  }
  .preview-heading > div:not(.preview-actions) {
    min-width: 0;
  }
  .preview-actions {
    display: flex;
    flex: 0 0 auto;
    align-items: center;
    justify-content: flex-end;
    gap: 7px;
  }
  .preview-actions button {
    min-width: 76px;
    white-space: nowrap;
  }
  .preview-title {
    display: grid;
    flex: 1 1 auto;
    gap: 3px;
    min-width: 0;
  }
  :global(.preview-bundle-file-input) {
    width: 240px;
    max-width: min(340px, 42vw);
  }
  .repository-upload-message {
    font-size: 11px;
  }
  .repository-upload-message.success {
    color: var(--theme-success);
  }
  .repository-upload-message.repository-upload-error {
    color: var(--theme-danger);
  }
  .repository-preview {
    display: grid;
    gap: 8px;
    width: 100%;
    min-width: 0;
  }
  .preview-heading strong {
    color: var(--theme-fg-strong);
    font-size: 13px;
  }
  .preview-heading small {
    color: var(--theme-fg-muted);
    font-size: 11px;
  }
  @media (max-width: 560px) {
    .preview-heading {
      align-items: stretch;
      flex-direction: column;
    }
    .preview-actions {
      justify-content: flex-end;
    }
    :global(.preview-bundle-file-input) {
      max-width: 100%;
    }
  }
</style>
