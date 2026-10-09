<script lang="ts">
  import { ChevronDown, ChevronRight } from 'lucide-svelte';
  import type { ReadonlyTableColumn } from './ReadonlyTable.svelte';

  export let columns: ReadonlyTableColumn[] = [];
  export let rows: any[] = [];
  export let rowKey: (row: any) => string = (row) => String(row.id);
  export let expandable = false;
  export let selectable = false;
  export let selectedIds: string[] = [];
  export let canSelectRow: (row: any) => boolean = () => true;
  export let onSelectionChange: (ids: string[]) => void = () => {};
  export let onRowToggle: (row: any, expanded: boolean) => void = () => {};
  export let filterable = false;
  export let filterValue = '';
  export let filterPlaceholder = '筛选';
  export let filterLabel = '';
  export let filterFn: (row: any, query: string) => boolean = (row, query) =>
    JSON.stringify(row).toLowerCase().includes(query.toLowerCase());
  export let onFilterChange: (value: string) => void = () => {};
  export let rowClass: (row: any) => string = () => '';
  export let className = '';
  export let togglePlacement: 'column' | 'cell' = 'column';
  export let emptyText = '';

  let expandedIds = new Set<string>();
  let selectAllInput: HTMLInputElement;

  $: filteredRows = filterable && filterValue.trim()
    ? rows.filter((row) => filterFn(row, filterValue.trim()))
    : rows;
  $: selectableRows = selectable ? filteredRows.filter(canSelectRow) : [];
  $: allSelected = selectableRows.length > 0 && selectableRows.every((row) => selectedIds.includes(rowKey(row)));
  $: someSelected = selectableRows.some((row) => selectedIds.includes(rowKey(row)));
  $: if (selectAllInput) selectAllInput.indeterminate = someSelected && !allSelected;

  function toggleExpanded(row: any) {
    const id = rowKey(row);
    const next = new Set(expandedIds);
    const expanded = !next.has(id);
    if (expanded) next.add(id);
    else next.delete(id);
    expandedIds = next;
    onRowToggle(row, expanded);
  }

  function setRowSelected(row: any, checked: boolean) {
    const id = rowKey(row);
    const next = checked
      ? [...new Set([...selectedIds, id])]
      : selectedIds.filter((selectedId) => selectedId !== id);
    selectedIds = next;
    onSelectionChange(next);
  }

  function setAllSelected(checked: boolean) {
    const ids = checked ? selectableRows.map(rowKey) : [];
    selectedIds = ids;
    onSelectionChange(ids);
  }
</script>

<div class={`expandable-table-container ${className}`}>
  {#if filterable}
    <label class="expandable-table-filter">
      {#if filterLabel}<span>{filterLabel}</span>{/if}
      <input
        type="search"
        value={filterValue}
        placeholder={filterPlaceholder}
        on:input={(event) => {
          filterValue = event.currentTarget.value;
          onFilterChange(filterValue);
        }}
      />
    </label>
  {/if}
  <div class="expandable-table-scroll">
  <table class="expandable-table">
    <colgroup>
      {#if expandable && togglePlacement === 'column'}<col class="expandable-table-toggle-column" />{/if}
      {#if selectable}<col class="expandable-table-select-column" />{/if}
      {#each columns as column (column.key)}
        <col style:width={column.width} />
      {/each}
    </colgroup>
    <thead>
      <tr>
        {#if expandable && togglePlacement === 'column'}<th class="expandable-table-toggle-cell" aria-label="展开"></th>{/if}
        {#if selectable}
          <th class="expandable-table-select-cell">
            <input
              bind:this={selectAllInput}
              type="checkbox"
              aria-label="选择全部可管理行"
              checked={allSelected}
              disabled={selectableRows.length === 0}
              on:change={(event) => setAllSelected(event.currentTarget.checked)}
            />
          </th>
        {/if}
        {#each columns as column (column.key)}
          <th class={column.className ?? ''} scope="col">{column.label}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each filteredRows as row (rowKey(row))}
        {@const id = rowKey(row)}
        {@const expanded = expandedIds.has(id)}
        <tr class:selected={selectedIds.includes(id)} class={rowClass(row)} on:click={(event) => {
          const target = event.target as HTMLElement;
          if (expandable && !(target.closest('button, input, a, select, textarea'))) toggleExpanded(row);
        }}>
          {#if expandable && togglePlacement === 'column'}
            <td class="expandable-table-toggle-cell">
              <button
                type="button"
                class="expandable-table-toggle"
                aria-label={`${expanded ? '折叠' : '展开'} ${row.name ?? id}`}
                aria-expanded={expanded}
                on:click={() => toggleExpanded(row)}
              >
                {#if expanded}<ChevronDown size={15} aria-hidden="true" />{:else}<ChevronRight size={15} aria-hidden="true" />{/if}
              </button>
            </td>
          {/if}
          {#if selectable}
            <td class="expandable-table-select-cell">
              <input
                type="checkbox"
                aria-label={`选择 ${row.name ?? row.display_name ?? id}`}
                checked={selectedIds.includes(id)}
                disabled={!canSelectRow(row)}
                on:change={(event) => setRowSelected(row, event.currentTarget.checked)}
              />
            </td>
          {/if}
          {#each columns as column (column.key)}
            <td class={column.className ?? ''}>
              <slot name="cell" {row} {column} {expanded} toggle={() => toggleExpanded(row)}>
                {String(row[column.key] ?? '')}
              </slot>
            </td>
          {/each}
        </tr>
        {#if expandable && expanded}
          <tr class="expandable-table-details-row">
            <td colspan={columns.length + Number(selectable) + Number(expandable && togglePlacement === 'column')}>
              <slot name="details" {row} />
            </td>
          </tr>
        {/if}
      {:else}
        {#if emptyText}<tr><td class="expandable-table-empty" colspan={Math.max(columns.length + Number(selectable) + Number(expandable && togglePlacement === 'column'), 1)}>{emptyText}</td></tr>{/if}
      {/each}
    </tbody>
  </table>
  </div>
</div>

<style>
  .expandable-table-scroll {
    min-width: 0;
    overflow-x: auto;
  }

  .expandable-table-container {
    min-width: 0;
  }

  .expandable-table-filter {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
    color: var(--theme-fg-muted);
    font-size: 11px;
  }

  .expandable-table-filter input {
    min-width: 0;
    width: min(260px, 100%);
    height: 30px;
    padding: 0 9px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    outline: none;
  }

  .expandable-table-filter input:focus {
    border-color: var(--theme-border-focus);
    outline: 2px solid var(--theme-border-focus);
    outline-offset: -2px;
  }

  .expandable-table {
    width: 100%;
    border-collapse: separate;
    border-spacing: 0;
    color: var(--theme-fg);
    font-size: 11px;
  }

  .expandable-table th,
  .expandable-table td {
    padding: 8px 12px;
    text-align: left;
    vertical-align: middle;
  }

  .expandable-table thead th {
    height: 36px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border-top: 1px solid var(--theme-divider);
    border-bottom: 1px solid var(--theme-divider);
    font-size: 10px;
    font-weight: 650;
    white-space: nowrap;
  }

  .expandable-table tbody > tr:not(.expandable-table-details-row) > td {
    min-height: 56px;
    border-bottom: 1px solid var(--theme-divider);
  }

  .expandable-table tbody > tr:not(.expandable-table-details-row):hover > td,
  .expandable-table tbody > tr.selected > td {
    background: var(--theme-bg-hover);
  }

  .expandable-table-toggle-column,
  .expandable-table-toggle-cell {
    width: 26px;
  }

  .expandable-table-select-column,
  .expandable-table-select-cell {
    width: 32px;
    text-align: center !important;
  }

  .expandable-table-toggle {
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    padding: 0;
    color: var(--theme-fg-muted);
    background: transparent;
    border: 0;
    border-radius: 4px;
    cursor: pointer;
  }

  .expandable-table-toggle:hover {
    color: var(--theme-fg);
    background: var(--theme-bg-hover);
  }

  .expandable-table-toggle:focus-visible {
    outline: 2px solid var(--theme-border-focus);
    outline-offset: 1px;
  }

  .expandable-table-details-row > td {
    padding: 0;
    background: var(--theme-bg-raised);
    border-bottom: 1px solid var(--theme-divider);
  }

  .expandable-table-empty {
    color: var(--theme-fg-muted);
    text-align: center !important;
  }
</style>
