<script lang="ts">
  import { onMount } from 'svelte';
  import type { Resource } from '../../lib/api';
  import { appURL } from '../../lib/health';
  import FilePicker from '../../components/FilePicker.svelte';
  export let resource: Resource;
  let file: File | null = null;
  let busy = false;
  let message = '';
  let branches: string[] = [];
  let previewBusy = false;
  let previewMessage = '';
  let previewEntries: Array<{ path: string; type: string; size?: number }> = [];
  async function preview() {
    if (previewBusy) return;
    previewBusy = true;
    previewMessage = '';
    try {
      const response = await fetch(appURL(`api/v1/resources/${resource.id}/repository/tree`), { credentials: 'include' });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data?.error?.message ?? data?.message ?? '仓库预览失败');
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
    busy = true; message = '';
    try {
      const body = new FormData(); body.append('bundle', file);
      const response = await fetch(appURL(`api/v1/resources/${resource.id}/bundle`), { method: 'POST', body, credentials: 'include' });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data?.error?.message ?? data?.message ?? '上传失败');
      branches = Array.isArray(data.branches) ? data.branches : [];
      message = `上传成功，已同步 ${branches.length} 个分支`;
      await preview();
    } catch (error) { message = error instanceof Error ? error.message : '上传失败'; }
    finally { busy = false; }
  }
  onMount(() => {
    if (String(resource.subtype).toLowerCase() !== 'bundle') void preview();
  });
</script>
<section class="resource-detail stack-form">
  <p><strong>接入方式：</strong>{resource.subtype ?? 'Git'}</p>
  {#if String(resource.subtype).toLowerCase() === 'bundle'}
    <FilePicker label="上传 Git Bundle" buttonLabel={file?.name || '选择 Git Bundle'} accept=".bundle,application/octet-stream" ariaLabel="选择 Git Bundle" onFileChange={(event) => (file = (event.currentTarget as HTMLInputElement).files?.[0] ?? null)} />
    <button class="primary" type="button" disabled={!file || busy} on:click={() => void upload()}>{busy ? '上传中…' : '上传并更新分支'}</button>
    {#if message}<small>{message}</small>{/if}
    {#if branches.length}<small>当前分支：{branches.join('、')}</small>{/if}
  {:else}
    <p class="muted">Git 仓库按需从远端读取，工具支持分支、状态、目录、文件和搜索。</p>
  {/if}
  <section class="repository-preview">
    <div class="preview-heading"><div><strong>仓库预览</strong><small>读取当前仓库默认分支的前 200 个文件</small></div><button class="secondary" type="button" disabled={previewBusy} on:click={() => void preview()}>{previewBusy ? '读取中…' : '刷新预览'}</button></div>
    {#if previewMessage}<small class="form-error">{previewMessage}</small>{/if}
    {#if previewEntries.length}
      <ul class="repository-tree" aria-label="仓库目录树">
        {#each previewEntries as entry}<li><code>{entry.path}</code>{#if entry.size !== undefined}<small>{entry.size} B</small>{/if}</li>{/each}
      </ul>
    {:else if !previewBusy && !previewMessage}<small class="muted">点击刷新预览查看目录树。</small>{/if}
  </section>
</section>

<style>
  .repository-preview { display: grid; gap: 8px; border-top: 1px solid var(--theme-divider); padding-top: 14px; }
  .preview-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .preview-heading > div { display: grid; gap: 3px; }
  .preview-heading strong { color: var(--theme-fg-strong); font-size: 13px; }
  .preview-heading small, .repository-tree small { color: var(--theme-fg-muted); font-size: 11px; }
  .repository-tree { display: grid; gap: 4px; max-height: 260px; overflow: auto; margin: 0; padding: 10px 12px; border: 1px solid var(--theme-border); border-radius: 4px; background: var(--theme-bg-sunken); list-style: none; }
  .repository-tree li { display: flex; justify-content: space-between; gap: 12px; color: var(--theme-fg-strong); font-size: 12px; }
</style>
