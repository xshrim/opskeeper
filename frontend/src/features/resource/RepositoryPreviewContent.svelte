<script lang="ts">
  import { ChevronDown, ChevronRight, File, Folder, List, ListTree } from 'lucide-svelte';
  import {
    buildRepositoryPreviewTree,
    visibleRepositoryPreviewNodes,
    type RepositoryPreviewEntry
  } from './repositoryPreview';

  export let entries: RepositoryPreviewEntry[] = [];
  export let busy = false;
  export let failed = false;

  let mode: 'list' | 'tree' = 'list';
  let expandedPaths = new Set<string>();
  $: tree = buildRepositoryPreviewTree(entries);
  $: flattenedTree = visibleRepositoryPreviewNodes(tree, expandedPaths);

  function toggle(path: string) {
    const next = new Set(expandedPaths);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    expandedPaths = next;
  }
</script>

<div class="repository-preview-content">
  <div class="preview-panel">
    <div class="preview-mode segmented-control" role="group" aria-label="仓库预览模式">
      <button class:active={mode === 'list'} type="button" aria-pressed={mode === 'list'} on:click={() => (mode = 'list')}><List size={14} aria-hidden="true" />列表</button>
      <button class:active={mode === 'tree'} type="button" aria-pressed={mode === 'tree'} on:click={() => (mode = 'tree')}><ListTree size={14} aria-hidden="true" />目录树</button>
    </div>
    {#if entries.length}
    {#if mode === 'list'}
      <ul class="repository-file-list" aria-label="仓库文件列表">
        {#each entries as entry (entry.path)}<li><code>{entry.path}</code>{#if entry.size !== undefined}<small>{entry.size} B</small>{/if}</li>{/each}
      </ul>
    {:else}
      <ul class="repository-file-tree" aria-label="仓库目录树">
        {#each flattenedTree as node (node.path)}
          <li style:--depth={node.depth}>
            {#if node.children.length}
              <button class="tree-toggle" type="button" aria-expanded={expandedPaths.has(node.path)} aria-label={`${expandedPaths.has(node.path) ? '折叠' : '展开'} ${node.name}`} on:click={() => toggle(node.path)}>
                {#if expandedPaths.has(node.path)}<ChevronDown size={14} aria-hidden="true" />{:else}<ChevronRight size={14} aria-hidden="true" />{/if}
                <Folder size={14} aria-hidden="true" /><span>{node.name}</span>
              </button>
            {:else}
              <span class="tree-file"><File size={14} aria-hidden="true" /><code>{node.name}</code></span>
            {/if}
            {#if node.size !== undefined}<small>{node.size} B</small>{/if}
          </li>
        {/each}
      </ul>
    {/if}
    {:else if busy}
      <p class="preview-empty muted">正在读取仓库内容…</p>
    {:else if failed}
      <p class="preview-empty muted">读取失败，请根据标题栏提示处理后重试。</p>
    {:else}
      <p class="preview-empty muted">仓库没有可显示的文件，或默认分支为空。</p>
    {/if}
  </div>
</div>

<style>
  .repository-preview-content { min-width: 0; }
  .preview-panel { position: relative; min-width: 0; }
  .preview-mode { position: absolute; z-index: 1; top: 7px; right: 8px; margin: 0; background: var(--theme-bg-raised); }
  .preview-mode button { min-height: 28px; gap: 5px; padding: 0 8px; }
  .repository-file-list,
  .repository-file-tree { display: grid; gap: 4px; max-height: 360px; overflow: auto; margin: 0; padding: 46px 12px 10px; border: 1px solid var(--theme-border); border-radius: 4px; background: var(--theme-bg-sunken); list-style: none; }
  .preview-empty { min-height: 52px; margin: 0; padding: 48px 12px 12px; border: 1px solid var(--theme-border); border-radius: 4px; background: var(--theme-bg-sunken); }
  .repository-file-list li,
  .repository-file-tree li { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 22px; color: var(--theme-fg-strong); font-size: 12px; }
  .repository-file-list small,
  .repository-file-tree small { color: var(--theme-fg-muted); font-size: 11px; }
  .repository-file-list code,
  .tree-file code { overflow-wrap: anywhere; }
  .repository-file-tree li { padding-left: calc(var(--depth) * 18px); }
  .tree-toggle,
  .tree-file { display: inline-flex; align-items: center; gap: 5px; min-width: 0; color: var(--theme-fg-strong); }
  .tree-toggle { padding: 0; background: none; border: 0; text-align: left; }
  .tree-toggle span { overflow-wrap: anywhere; }
  .tree-file { padding-left: 19px; }
  .repository-file-tree :global(svg) { flex: 0 0 auto; color: var(--theme-fg-muted); }
</style>
