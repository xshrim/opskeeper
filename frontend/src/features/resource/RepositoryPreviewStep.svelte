<script lang="ts">
  import RepositoryPreviewContent from './RepositoryPreviewContent.svelte';
  export let entries: Array<{ path: string; type: string; size?: number }> = [];
  export let busy = false;
  export let failed = false;
</script>

<section class="repository-preview-step">
  <div class="preview-intro">
    <strong>仓库预览</strong>
    <small
      >{busy
        ? '正在读取仓库内容…'
        : failed
          ? '读取失败，请根据标题栏提示处理后重试。'
          : '已读取默认分支的全部文件。确认目录内容无误后继续保存。'}</small
    >
  </div>
  <RepositoryPreviewContent {entries} {busy} {failed} />
</section>

<style>
  .repository-preview-step {
    display: grid;
    gap: 12px;
  }
  .preview-intro {
    display: grid;
    gap: 4px;
  }
  .preview-intro strong {
    color: var(--theme-fg-strong);
    font-size: 14px;
  }
  .preview-intro small {
    color: var(--theme-fg-muted);
    font-size: 12px;
  }
</style>
