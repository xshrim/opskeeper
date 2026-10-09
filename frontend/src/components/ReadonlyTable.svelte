<script context="module" lang="ts">
  export type ReadonlyTableColumn = {
    key: string;
    label: string;
    width?: string;
    className?: string;
  };
</script>

<script lang="ts">
  export let columns: ReadonlyTableColumn[] = [];
  export let rows: unknown[] = [];
  export let emptyText = '';
</script>

<div class="readonly-table-scroll">
  <table class="readonly-table">
    <colgroup>
      {#each columns as column (column.key)}
        <col style:width={column.width} />
      {/each}
    </colgroup>
    <thead>
      <tr>
        {#each columns as column (column.key)}
          <th class={column.className ?? ''} scope="col">{column.label}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each rows as row, index (`${index}`)}
        <tr>
          {#each columns as column (column.key)}
            <td class={column.className ?? ''}>
              <slot name="cell" {row} {column}>
                {String((row as Record<string, unknown>)[column.key] ?? '')}
              </slot>
            </td>
          {/each}
        </tr>
      {:else}
        {#if emptyText}<tr><td class="readonly-table-empty" colspan={Math.max(columns.length, 1)}>{emptyText}</td></tr>{/if}
      {/each}
    </tbody>
  </table>
</div>

<style>
  .readonly-table-scroll {
    min-width: 0;
    overflow-x: auto;
    border: 1px solid var(--theme-border);
    border-radius: 5px;
  }

  .readonly-table {
    width: 100%;
    border-collapse: separate;
    border-spacing: 0 1px;
    background: var(--theme-divider);
    color: var(--theme-fg);
    font-size: 11px;
  }

  th,
  td {
    padding: 5px 10px;
    text-align: left;
  }

  th {
    height: 28px;
    padding-top: 0;
    padding-bottom: 0;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    font-size: 10px;
    font-weight: 650;
    white-space: nowrap;
  }

  td {
    min-height: 30px;
    background: var(--theme-bg-raised);
    font-weight: 400;
  }

  .readonly-table-empty {
    color: var(--theme-fg-muted);
    text-align: center;
  }
</style>
