<script lang="ts">
  import { ChevronDown, X } from 'lucide-svelte';
  import { createEventDispatcher, tick } from 'svelte';
  import IconValue from './IconValue.svelte';
  import SearchInput from './SearchInput.svelte';

  type DropdownOption = {
    value: string;
    label: string;
    icon?: string;
    description?: string;
    disabled?: boolean;
  };

  export let options: DropdownOption[] = [];
  export let value: string | string[] = '';
  export let placeholder = '请选择';
  export let ariaLabel = '选择';
  export let disabled = false;
  export let invalid = false;
  export let invalidMode: 'border' | 'bubble' = 'border';
  export let invalidMessage = '请选择一项';
  export let searchable = false;
  export let showIcons = false;
  export let allowCustom = false;
  export let allowAll = false;
  export let allLabel = '全部';
  export let allValue = '__all__';
  export let multiple = false;
  export let variant: 'standard' | 'underline' | 'pill' | 'soft' = 'standard';
  export let menuClass = '';

  const dispatch = createEventDispatcher<{ change: string | string[] }>();

  let open = false;
  let query = '';
  let root: HTMLDivElement;
  let customValue = '';
  let menuStyle = '';

  $: selectedValues = multiple ? (Array.isArray(value) ? value : value ? [value] : []) : value ? [value] : [];
  $: selectedOptions = options.filter((option) => selectedValues.includes(option.value));
  $: selected = Array.isArray(value) ? selectedOptions[0] : options.find((option) => option.value === value) ?? null;
  $: displayValue = selectedValues.includes(allValue)
    ? allLabel
    : multiple
      ? selectedOptions.map((option) => option.label).join('、')
      : selected?.label ?? (allowCustom && typeof value === 'string' && value ? value : '');
  $: normalizedQuery = query.trim().toLocaleLowerCase();
  $: filteredOptions = normalizedQuery
    ? options.filter((option) => `${option.label} ${option.value} ${option.description ?? ''}`.toLocaleLowerCase().includes(normalizedQuery))
    : options;

  function emit(next: string) {
    if (multiple) {
      const current = Array.isArray(value) ? value : [];
      value = current.includes(next)
        ? current.filter((item) => item !== next)
        : [...current.filter((item) => item !== allValue), next];
      dispatch('change', value);
      return;
    }
    value = next;
    customValue = next;
    dispatch('change', value);
  }

  function select(next: string) {
    if (multiple && next === allValue) {
      value = selectedValues.includes(allValue) ? [] : [allValue];
      dispatch('change', value);
      query = '';
      return;
    }
    emit(next);
    if (!multiple) {
      open = false;
      menuStyle = '';
    }
    query = '';
  }

  function toggle() {
    if (disabled) return;
    open = !open;
    if (open) {
      query = '';
      positionMenu();
    } else {
      menuStyle = '';
    }
  }

  async function positionMenu() {
    await tick();
    if (!open || !root) return;
    const rect = root.getBoundingClientRect();
    const padding = 8;
    const gap = 5;
    const availableHeight = Math.max(0, window.innerHeight - rect.bottom - gap - padding);
    menuStyle = `top: ${Math.round(rect.bottom + gap)}px; left: ${Math.round(rect.left)}px; width: ${Math.round(rect.width)}px; max-height: ${Math.round(availableHeight)}px;`;
  }

  function handleViewportChange() {
    if (open) positionMenu();
  }

  function close(event: MouseEvent) {
    if (root && !root.contains(event.target as Node)) {
      open = false;
      menuStyle = '';
    }
  }

  function handleCustomInput(event: Event) {
    customValue = (event.currentTarget as HTMLInputElement).value;
    emit(customValue);
  }

  function clearCustom() {
    value = '';
    customValue = '';
    dispatch('change', value);
  }
</script>

<svelte:window on:click={close} on:resize={handleViewportChange} on:scroll={handleViewportChange} />

<div class={`dropdown-select variant-${variant}`} class:open class:disabled class:invalid class:bubble={invalid && invalidMode === 'bubble'} data-error={invalid && invalidMode === 'bubble' ? invalidMessage : undefined} bind:this={root}>
  <div class="dropdown-select-trigger" class:editable={allowCustom}>
    {#if allowCustom}
      <input
        class="dropdown-select-input"
        value={customValue || displayValue}
        {placeholder}
        {disabled}
        aria-label={ariaLabel}
        on:input={handleCustomInput}
      />
      {#if customValue}<button class="dropdown-select-clear" type="button" aria-label="清空" on:click|stopPropagation={clearCustom}><X size={13} /></button>{/if}
    {:else}
      <button
        class="dropdown-select-button"
        type="button"
        {disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
        on:click|stopPropagation={toggle}
      >
        {#if showIcons && selected?.icon}<span class="dropdown-select-icon"><IconValue value={selected.icon} size={14} /></span>{/if}
        <span class:placeholder={!displayValue} class="dropdown-select-value">{displayValue || placeholder}</span>
      </button>
    {/if}
    <button class="dropdown-select-chevron" type="button" aria-label={open ? '收起选项' : '展开选项'} {disabled} on:click|stopPropagation={toggle}><ChevronDown size={14} /></button>
  </div>

  {#if open}
    <div class={`dropdown-select-menu ${menuClass}`.trim()} style={menuStyle} role="listbox" tabindex="-1" aria-label={ariaLabel}>
      {#if searchable}<SearchInput bind:value={query} width="100%" height="30px" placeholder="搜索" ariaLabel={`搜索${ariaLabel}`} />{/if}
      <div class="dropdown-select-options">
        {#if allowAll}<button class="dropdown-select-option all-option" type="button" role="option" aria-selected={selectedValues.includes(allValue)} on:click={() => select(allValue)}>{#if showIcons}<span class="dropdown-select-icon"><span aria-hidden="true">✦</span></span>{/if}<span>{allLabel}</span></button>{/if}
        {#each filteredOptions as option}
          <button class:selected={(multiple ? selectedValues.includes(option.value) : option.value === value)} class="dropdown-select-option" type="button" role="option" aria-selected={multiple ? selectedValues.includes(option.value) : option.value === value} disabled={option.disabled} on:click={() => select(option.value)}>
            {#if showIcons && option.icon}<span class="dropdown-select-icon"><IconValue value={option.icon} size={14} /></span>{/if}
            <span class="dropdown-select-option-copy"><strong>{option.label}</strong>{#if option.description}<small>{option.description}</small>{/if}</span>
          </button>
        {:else}<span class="dropdown-select-empty">没有匹配项</span>{/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .dropdown-select { position: relative; min-width: 0; width: 100%; color: var(--theme-fg); }
  .dropdown-select-trigger { display: flex; align-items: center; min-height: 36px; width: 100%; overflow: hidden; color: var(--theme-fg); background: var(--theme-bg-input); border: 1px solid var(--theme-border-strong); border-radius: 4px; }
  .dropdown-select-trigger:focus-within, .dropdown-select-trigger:hover, .dropdown-select.open .dropdown-select-trigger { border-color: var(--theme-border-focus); }
  .dropdown-select.invalid .dropdown-select-trigger { border-color: var(--theme-danger); box-shadow: 0 0 0 2px var(--theme-bg-danger-soft); }
  .dropdown-select.invalid.bubble .dropdown-select-trigger { border-color: var(--theme-border-strong); box-shadow: none; }
  .dropdown-select.bubble::after { position: absolute; z-index: 30; bottom: calc(100% + 7px); left: 0; max-width: min(280px, 90vw); padding: 6px 9px; color: var(--theme-danger); background: var(--theme-bg-danger-soft); border-radius: 4px; box-shadow: var(--theme-shadow-soft); content: attr(data-error); font-size: 11px; line-height: 1.35; white-space: normal; }
  .dropdown-select.bubble::before { position: absolute; z-index: 30; bottom: calc(100% + 1px); left: 12px; width: 0; height: 0; border: 6px solid transparent; border-top-color: var(--theme-bg-danger-soft); content: ''; }
  .dropdown-select-button { display: flex; align-items: center; gap: 7px; min-width: 0; flex: 1; height: 34px; padding: 0 9px; color: inherit; background: transparent; border: 0; text-align: left; }
  .dropdown-select-value { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }
  .dropdown-select-value.placeholder, .dropdown-select-input::placeholder, .dropdown-select-empty { color: var(--theme-fg-muted); }
  .dropdown-select-input { min-width: 0; flex: 1; height: 34px; padding: 0 9px; color: inherit; background: transparent; border: 0; outline: 0; font-size: 12px; }
  .dropdown-select-chevron, .dropdown-select-clear { display: grid; place-items: center; flex: 0 0 30px; height: 34px; padding: 0; color: var(--theme-fg-muted); background: transparent; border: 0; }
  .dropdown-select.open .dropdown-select-chevron { color: var(--theme-fg); transform: rotate(180deg); }
  .dropdown-select.disabled { opacity: .6; pointer-events: none; }
  .dropdown-select-menu { position: fixed; z-index: 1000; padding: 5px; overflow: auto; background: var(--theme-bg-raised); border: 1px solid var(--theme-border-strong); border-radius: 5px; box-shadow: var(--theme-shadow); }
  .dropdown-select-menu :global(.search-input) { margin-bottom: 5px; border-radius: 4px; }
  .dropdown-select-options { max-height: 240px; overflow: auto; }
  .dropdown-select-option { display: flex; align-items: center; gap: 7px; width: 100%; min-height: 32px; padding: 5px 7px; color: var(--theme-fg); background: transparent; border: 0; border-radius: 3px; text-align: left; }
  .dropdown-select-option:hover, .dropdown-select-option:focus-visible, .dropdown-select-option.selected { color: var(--theme-fg-strong); background: var(--theme-bg-selected); outline: 0; }
  .dropdown-select-option:disabled { opacity: .5; cursor: not-allowed; }
  .dropdown-select-option.all-option { color: var(--theme-accent); border-bottom: 1px solid var(--theme-divider); border-radius: 0; }
  .dropdown-select-icon { display: grid; place-items: center; width: 20px; height: 20px; flex: 0 0 20px; color: var(--theme-fg-muted); }
  .dropdown-select-option-copy { display: grid; min-width: 0; gap: 1px; }
  .dropdown-select-option-copy strong { overflow: hidden; font-size: 12px; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
  .dropdown-select-option-copy small { overflow: hidden; color: var(--theme-fg-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
  .dropdown-select-empty { display: block; padding: 9px 7px; font-size: 12px; }
  .dropdown-select.variant-underline .dropdown-select-trigger { border-width: 0 0 1px; border-radius: 0; background: transparent; }
  .dropdown-select.variant-pill .dropdown-select-trigger { border-radius: 18px; padding-left: 2px; }
  .dropdown-select.variant-pill .dropdown-select-button { padding-left: 12px; }
  .dropdown-select.variant-soft .dropdown-select-trigger { background: var(--theme-bg-selected); border-color: transparent; }
</style>
