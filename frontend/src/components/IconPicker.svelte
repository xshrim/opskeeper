<script lang="ts">
  import { onMount } from 'svelte';
  import { Search, Upload, X } from 'lucide-svelte';
  import { icons as lucideIcons } from 'lucide-svelte';

  export let value = 'FolderKanban';
  export let onSelect: (icon: string) => void = () => {};
  export let ariaLabel = '选择图标';

  let open = false;
  let query = '';
  let visibleCount = 160;
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
  $: filteredIcons = iconNames.filter((name) =>
    !query.trim() || name.toLowerCase().includes(query.trim().toLowerCase())
  );
  $: visibleIcons = filteredIcons.slice(0, visibleCount);

  function isImage(value: string) {
    return value.startsWith('data:image/');
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
    const dropUp = below < 220 && above > below;
    const height = Math.max(0, Math.min(230, (dropUp ? above : below) - 6));
    const vertical = dropUp
      ? `top:auto;bottom:${Math.max(margin, window.innerHeight - rect.top + 6)}px`
      : `top:${rect.bottom + 6}px;bottom:auto`;
    popoverStyle = `${vertical};left:${left}px;width:${width}px;max-height:${height}px`;
  }

  function toggleOpen() {
    open = !open;
    if (open) place();
  }

  function updateQuery(event: Event) {
    query = (event.currentTarget as HTMLInputElement).value;
    visibleCount = 160;
  }

  function loadMore(event: Event) {
    const element = event.currentTarget as HTMLDivElement;
    if (element.scrollTop + element.clientHeight >= element.scrollHeight - 48) {
      visibleCount = Math.min(visibleCount + 160, filteredIcons.length);
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
      // Keep the file content as a data URL. Do not wait for an image decode or
      // canvas conversion: unsupported browser formats would otherwise leave
      // the picker apparently unchanged.
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
    {#if isImage(value)}
      <img src={value} alt="" />
    {:else if lucideIcons[value as keyof typeof lucideIcons]}
      {@const SelectedIcon: any = lucideIcons[value as keyof typeof lucideIcons]}
      <svelte:component this={SelectedIcon} size={18} strokeWidth={1.8} aria-hidden="true" />
    {:else}
      <span aria-hidden="true">◇</span>
    {/if}
  </button>
  {#if open}
    <div class="icon-picker-popover" role="dialog" aria-label="选择图标" tabindex="-1" style={popoverStyle} on:click|stopPropagation on:keydown|stopPropagation>
      <div class="icon-picker-toolbar">
        <label><Search size={14} aria-hidden="true" /><input value={query} on:input={updateQuery} placeholder="搜索图标" aria-label="搜索图标" /></label>
        <input bind:this={fileInput} type="file" accept="image/*" hidden on:change={importFile} />
        <button type="button" class="icon-picker-upload" title="上传本地图标" aria-label="上传本地图标" on:click={() => fileInput?.click()}><Upload size={14} /></button>
        <button type="button" class="icon-picker-close" title="关闭" aria-label="关闭" on:click={() => (open = false)}><X size={14} /></button>
      </div>
      <div class="icon-picker-grid" aria-label="Lucide 图标列表" on:scroll={loadMore}>
        {#each visibleIcons as name}
          {@const Icon: any = lucideIcons[name as keyof typeof lucideIcons]}
          <button type="button" title={name} aria-label={`选择图标 ${name}`} class:active={value === name} on:click={() => select(name)}><svelte:component this={Icon} size={17} strokeWidth={1.8} /></button>
        {:else}
          <small class="icon-picker-empty">没有匹配的图标</small>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .icon-picker { position: relative; display: inline-flex; }
  .icon-picker-trigger { display: grid; place-items: center; width: 38px; height: 38px; padding: 0; color: var(--theme-fg-muted); background: var(--theme-bg-surface); border: 1px solid var(--theme-border); border-radius: 6px; cursor: pointer; }
  .icon-picker-trigger:hover { color: var(--color-primary); border-color: var(--color-primary); }
  .icon-picker-trigger img { width: 22px; height: 22px; object-fit: contain; border-radius: 3px; }
  .icon-picker-popover { position: fixed; z-index: 90; display: flex; flex-direction: column; padding: 8px; background: var(--theme-bg-surface); border: 1px solid var(--theme-border-strong); border-radius: 6px; box-shadow: 0 12px 28px rgba(15, 23, 42, .18); }
  .icon-picker-toolbar { display: flex; gap: 5px; margin-bottom: 7px; }
  .icon-picker-toolbar label { display: flex; align-items: center; flex: 1; gap: 6px; min-width: 0; padding: 0 7px; border: 1px solid var(--theme-border); border-radius: 4px; color: var(--theme-fg-muted); }
  .icon-picker-toolbar input { min-width: 0; width: 100%; height: 28px; min-height: 0; padding: 0; border: 0; outline: 0; background: transparent; color: var(--theme-fg); }
  .icon-picker-toolbar button { display: grid; place-items: center; width: 30px; height: 30px; padding: 0; color: var(--theme-fg-muted); background: transparent; border: 1px solid var(--theme-border); border-radius: 4px; cursor: pointer; }
  .icon-picker-toolbar button:hover { color: var(--color-primary); border-color: var(--color-primary); }
  .icon-picker-grid { display: grid; flex: 1 1 auto; min-height: 0; grid-template-columns: repeat(8, minmax(0, 1fr)); gap: 3px; max-height: 230px; overflow: auto; }
  .icon-picker-grid button { display: grid; place-items: center; width: 100%; aspect-ratio: 1; padding: 0; color: var(--theme-fg-muted); background: transparent; border: 1px solid transparent; border-radius: 4px; cursor: pointer; }
  .icon-picker-grid button:hover, .icon-picker-grid button.active { color: var(--color-primary); background: var(--theme-bg-hover); border-color: var(--color-primary); }
  .icon-picker-empty { grid-column: 1 / -1; padding: 18px; color: var(--theme-fg-subtle); text-align: center; }
</style>
