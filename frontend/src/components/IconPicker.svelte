<script lang="ts">
  import { onMount } from 'svelte';
  import { Search, Upload, X } from 'lucide-svelte';
  import { icons as lucideIcons } from 'lucide-svelte';
  import IconValue from './IconValue.svelte';
  import { iconifyIconEntries } from '../lib/iconifyIcons';

  type IconCategory = 'lucide' | 'iconify' | 'image';

  export let value = 'lucide:FolderKanban';
  export let onSelect: (icon: string) => void = () => {};
  export let ariaLabel = '选择图标';

  let open = false;
  let query = '';
  let visibleCount = 160;
  let category: IconCategory = 'lucide';
  let root: HTMLDivElement;
  let trigger: HTMLButtonElement;
  let fileInput: HTMLInputElement;
  let popoverStyle = '';

  const iconNames = Object.keys(lucideIcons).filter((name) => name !== 'default');
  const imageTypes: Record<string, string> = {
    avif: 'image/avif',
    gif: 'image/gif',
    jpeg: 'image/jpeg',
    jpg: 'image/jpeg',
    png: 'image/png',
    svg: 'image/svg+xml',
    webp: 'image/webp'
  };

  $: normalizedQuery = query.trim().toLowerCase();
  $: filteredIcons = iconNames.filter((name) =>
    !normalizedQuery || name.toLowerCase().includes(normalizedQuery)
  );
  $: filteredIconifyIcons = iconifyIconEntries.filter((entry) => {
    if (!normalizedQuery) return true;
    return [entry.label, ...entry.aliases, entry.id]
      .join(' ')
      .toLowerCase()
      .includes(normalizedQuery);
  });
  $: visibleIcons = filteredIcons.slice(0, visibleCount);
  $: visibleIconifyIcons = filteredIconifyIcons.slice(0, visibleCount);

  function isImage(icon: string) {
    return icon.startsWith('data:image/');
  }

  function categoryForValue(icon: string): IconCategory {
    if (isImage(icon)) return 'image';
    if (icon.startsWith('iconify:')) return 'iconify';
    return 'lucide';
  }

  function lucideValue(name: string) {
    return `lucide:${name}`;
  }

  function isSelected(candidate: string) {
    return value === candidate;
  }

  function select(icon: string) {
    onSelect(icon);
    open = false;
    query = '';
  }

  function place() {
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const margin = 12;
    const width = Math.min(360, window.innerWidth - margin * 2);
    const left = Math.max(margin, Math.min(rect.left, window.innerWidth - width - margin));
    const below = window.innerHeight - rect.bottom - margin;
    const above = rect.top - margin;
    const dropUp = below < 260 && above > below;
    const height = Math.max(0, Math.min(300, (dropUp ? above : below) - 6));
    const vertical = dropUp
      ? `top:auto;bottom:${Math.max(margin, window.innerHeight - rect.top + 6)}px`
      : `top:${rect.bottom + 6}px;bottom:auto`;
    popoverStyle = `${vertical};left:${left}px;width:${width}px;max-height:${height}px`;
  }

  function toggleOpen() {
    open = !open;
    if (open) {
      category = categoryForValue(value);
      visibleCount = 160;
      place();
    }
  }

  function updateQuery(event: Event) {
    query = (event.currentTarget as HTMLInputElement).value;
    visibleCount = 160;
  }

  function changeCategory(next: IconCategory) {
    category = next;
    query = '';
    visibleCount = 160;
  }

  function loadMore(event: Event) {
    const element = event.currentTarget as HTMLDivElement;
    if (element.scrollTop + element.clientHeight >= element.scrollHeight - 48) {
      const total = category === 'iconify' ? filteredIconifyIcons.length : filteredIcons.length;
      visibleCount = Math.min(visibleCount + 160, total);
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') open = false;
  }

  function importFile(event: Event) {
    const file = (event.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    const extension = file.name.split('.').pop()?.toLowerCase() ?? '';
    const mime = file.type.startsWith('image/') ? file.type : imageTypes[extension];
    if (!mime) return;
    const reader = new FileReader();
    reader.onload = () => {
      const result = String(reader.result ?? '');
      if (!result) return;
      const dataURL = result.startsWith('data:image/')
        ? result
        : result.replace(/^data:[^;,]+/, `data:${mime}`);
      select(dataURL);
    };
    reader.onerror = () => { open = false; };
    reader.readAsDataURL(file);
    (event.currentTarget as HTMLInputElement).value = '';
  }

  onMount(() => {
    const closeOnOutside = (event: PointerEvent) => {
      if (open && root && !root.contains(event.target as Node)) open = false;
    };
    window.addEventListener('pointerdown', closeOnOutside);
    window.addEventListener('keydown', handleKeydown);
    window.addEventListener('scroll', place, true);
    window.addEventListener('resize', place);
    return () => {
      window.removeEventListener('pointerdown', closeOnOutside);
      window.removeEventListener('keydown', handleKeydown);
      window.removeEventListener('scroll', place, true);
      window.removeEventListener('resize', place);
    };
  });
</script>

<div class="icon-picker" bind:this={root}>
  <button bind:this={trigger} class="icon-picker-trigger" type="button" aria-label={ariaLabel} title={ariaLabel} aria-expanded={open} on:click|stopPropagation={toggleOpen}>
    <IconValue {value} size={18} />
  </button>
  {#if open}
    <div class="icon-picker-popover" role="dialog" aria-label="选择图标" tabindex="-1" style={popoverStyle} on:click|stopPropagation on:keydown|stopPropagation>
      <div class="icon-picker-tabs" role="tablist" aria-label="图标类型">
        <button type="button" class:active={category === 'lucide'} role="tab" aria-selected={category === 'lucide'} on:click={() => changeCategory('lucide')}>通用图标</button>
        <button type="button" class:active={category === 'iconify'} role="tab" aria-selected={category === 'iconify'} on:click={() => changeCategory('iconify')}>引入图标</button>
        <button type="button" class:active={category === 'image'} role="tab" aria-selected={category === 'image'} on:click={() => changeCategory('image')}>本地图片</button>
      </div>
      <div class="icon-picker-toolbar">
        {#if category !== 'image'}
          <label><Search size={14} aria-hidden="true" /><input value={query} on:input={updateQuery} placeholder="搜索图标或品牌" aria-label="搜索图标或品牌" /></label>
        {:else}
          <span class="icon-picker-toolbar-hint">选择本地图片作为图标</span>
        {/if}
        <input bind:this={fileInput} type="file" accept="image/*" hidden on:change={importFile} />
        {#if category === 'image'}
          <button type="button" class="icon-picker-upload" title="上传本地图标" aria-label="上传本地图标" on:click={() => fileInput?.click()}><Upload size={14} /></button>
        {/if}
        <button type="button" class="icon-picker-close" title="关闭" aria-label="关闭" on:click={() => (open = false)}><X size={14} /></button>
      </div>
      {#if category === 'image'}
        <div class="icon-picker-image-panel">
          {#if isImage(value)}
            <IconValue {value} size={56} />
            <span>当前本地图标</span>
          {:else}
            <small>还没有选择本地图标</small>
          {/if}
        </div>
      {:else if category === 'iconify'}
        <div class="icon-picker-grid" aria-label="引入图标列表" on:scroll={loadMore}>
          {#each visibleIconifyIcons as entry}
            <button type="button" title={entry.label} aria-label={`选择图标 ${entry.label}`} class:active={value === entry.id} on:click={() => select(entry.id)}><IconValue value={entry.id} size={17} /></button>
          {:else}
            <small class="icon-picker-empty">没有匹配的引入图标</small>
          {/each}
        </div>
      {:else}
        <div class="icon-picker-grid" aria-label="Lucide 图标列表" on:scroll={loadMore}>
          {#each visibleIcons as name}
            {@const Icon: any = lucideIcons[name as keyof typeof lucideIcons]}
            <button type="button" title={name} aria-label={`选择图标 ${name}`} class:active={isSelected(lucideValue(name))} on:click={() => select(lucideValue(name))}><svelte:component this={Icon} size={17} strokeWidth={1.8} /></button>
          {:else}
            <small class="icon-picker-empty">没有匹配的图标</small>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .icon-picker { position: relative; display: inline-flex; }
  .icon-picker-trigger { display: grid; place-items: center; width: 38px; height: 38px; padding: 0; color: var(--theme-fg-muted); background: var(--theme-bg-surface); border: 1px solid var(--theme-border); border-radius: 6px; cursor: pointer; }
  .icon-picker-trigger:hover { color: var(--color-primary); border-color: var(--color-primary); }
  .icon-picker-trigger :global(img) { width: 22px; height: 22px; }
  .icon-picker-popover { position: fixed; z-index: 90; display: flex; flex-direction: column; padding: 8px; background: var(--theme-bg-surface); border: 1px solid var(--theme-border-strong); border-radius: 6px; box-shadow: var(--theme-shadow); }
  .icon-picker-tabs { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 3px; margin-bottom: 7px; padding: 3px; background: var(--theme-bg-subtle); border-radius: 5px; }
  .icon-picker-tabs button { min-width: 0; height: 28px; padding: 0 5px; overflow: hidden; color: var(--theme-fg-muted); background: transparent; border: 0; border-radius: 4px; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }
  .icon-picker-tabs button:hover, .icon-picker-tabs button.active { color: var(--theme-fg); background: var(--theme-bg-surface); }
  .icon-picker-toolbar { display: flex; gap: 5px; margin-bottom: 7px; min-height: 30px; }
  .icon-picker-toolbar label { display: flex; align-items: center; flex: 1; gap: 6px; min-width: 0; padding: 0 7px; border: 1px solid var(--theme-border); border-radius: 4px; color: var(--theme-fg-muted); }
  .icon-picker-toolbar input { min-width: 0; width: 100%; height: 28px; min-height: 0; padding: 0; border: 0; outline: 0; background: transparent; color: var(--theme-fg); }
  .icon-picker-toolbar-hint { display: flex; align-items: center; flex: 1; padding: 0 7px; color: var(--theme-fg-muted); font-size: 12px; }
  .icon-picker-toolbar button { display: grid; place-items: center; width: 30px; height: 30px; padding: 0; color: var(--theme-fg-muted); background: transparent; border: 1px solid var(--theme-border); border-radius: 4px; cursor: pointer; }
  .icon-picker-toolbar button:hover { color: var(--color-primary); border-color: var(--color-primary); }
  .icon-picker-grid { display: grid; flex: 1 1 auto; min-height: 0; grid-template-columns: repeat(8, minmax(0, 1fr)); gap: 3px; max-height: 230px; overflow: auto; }
  .icon-picker-grid button { display: grid; place-items: center; width: 100%; aspect-ratio: 1; padding: 0; color: var(--theme-fg-muted); background: transparent; border: 1px solid transparent; border-radius: 4px; cursor: pointer; }
  .icon-picker-grid button:hover, .icon-picker-grid button.active { color: var(--color-primary); background: var(--theme-bg-hover); border-color: var(--color-primary); }
  .icon-picker-image-panel { display: grid; place-items: center; gap: 10px; min-height: 150px; color: var(--theme-fg-muted); }
  .icon-picker-image-panel :global(img) { width: 56px; height: 56px; }
  .icon-picker-empty { grid-column: 1 / -1; padding: 18px; color: var(--theme-fg-subtle); text-align: center; }
</style>
