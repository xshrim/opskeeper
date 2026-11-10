<script lang="ts">
  export let entries: Array<{ path: string; type: string; size?: number }> = [];
  export let busy = false;
  export let message = '';
</script>

<section class="repository-preview-step">
  <div class="preview-intro">
    <strong>仓库预览</strong>
    <small>{busy ? '正在读取仓库内容…' : '已读取默认分支的前 200 个文件。确认目录内容无误后继续保存。'}</small>
  </div>
  {#if message}<p class="form-error" role="alert">{message}</p>{/if}
  {#if entries.length}
      <ul aria-label="仓库目录树">
      {#each entries as entry}<li><code>{entry.path}</code>{#if entry.size !== undefined}<small>{entry.size} B</small>{/if}</li>{/each}
    </ul>
  {:else if !busy && !message}
    <p class="muted">仓库没有可显示的文件，或默认分支为空。</p>
  {/if}
</section>

<style>
  .repository-preview-step { display: grid; gap: 12px; }
  .preview-intro { display: grid; gap: 4px; }
  .preview-intro strong { color: var(--theme-fg-strong); font-size: 14px; }
  .preview-intro small { color: var(--theme-fg-muted); font-size: 12px; }
  ul { display: grid; gap: 4px; max-height: 360px; overflow: auto; margin: 0; padding: 12px; border: 1px solid var(--theme-border); border-radius: 4px; background: var(--theme-bg-sunken); list-style: none; }
  li { display: flex; justify-content: space-between; gap: 12px; color: var(--theme-fg-strong); font-size: 12px; }
  li small { color: var(--theme-fg-muted); }
</style>
