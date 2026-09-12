<script lang="ts">
  import type { Resource } from '../../lib/api';
  import { appURL } from '../../lib/health';
  export let resource: Resource;
  let file: File | null = null;
  let busy = false;
  let message = '';
  let branches: string[] = [];
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
    } catch (error) { message = error instanceof Error ? error.message : '上传失败'; }
    finally { busy = false; }
  }
</script>
<section class="resource-detail stack-form">
  <p><strong>接入方式：</strong>{resource.subtype ?? 'Git'}</p>
  {#if String(resource.subtype).toLowerCase() === 'bundle'}
    <label>上传 Git Bundle<input type="file" accept=".bundle,application/octet-stream" on:change={(event) => (file = (event.currentTarget as HTMLInputElement).files?.[0] ?? null)} /></label>
    <button class="primary" type="button" disabled={!file || busy} on:click={() => void upload()}>{busy ? '上传中…' : '上传并更新分支'}</button>
    {#if message}<small>{message}</small>{/if}
    {#if branches.length}<small>当前分支：{branches.join('、')}</small>{/if}
  {:else}
    <p class="muted">Git 仓库按需从远端读取，工具支持分支、状态、目录、文件和搜索。</p>
  {/if}
</section>
