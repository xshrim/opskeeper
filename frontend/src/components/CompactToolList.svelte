<script context="module" lang="ts">
  export type CompactTool = string | string[] | { name: string; description?: string };
</script>
<script lang="ts">
  export let tools: CompactTool[] = [];

  function toolParts(tool: CompactTool): [string, string] {
    if (Array.isArray(tool)) return [tool[0] ?? '', tool[1] ?? '只读诊断工具'];
    if (typeof tool === 'string') return [tool, '只读诊断工具'];
    return [tool.name, tool.description || '只读诊断工具'];
  }
</script>

<div class="compact-tool-list" aria-label="工具列表">
  {#each tools as tool}
    {@const [name, description] = toolParts(tool)}
    <div class="compact-tool-item">
      <code>{name}</code>
      <span>{description}</span>
    </div>
  {:else}
    <span class="compact-tool-empty">暂无工具</span>
  {/each}
</div>
