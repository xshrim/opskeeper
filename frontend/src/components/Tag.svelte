<script lang="ts">
  export let value = '';
  export let colorScheme: 'theme' | 'independent' = 'theme';
  export let distinguishKeyOnly = true;
  export let textMode: 'full' | 'ellipsis' | 'wrap' = 'ellipsis';
  export let tooltipOnTruncate = true;
  export let maxWidth = '';
  export let className = '';
  export let overflow = false;
  export let measure = false;

  $: separator = value.search(/[=:]/);
  $: hasValue = separator > 0 && separator < value.length - 1;
</script>

<span
  class={`tag tag-${colorScheme} tag-text-${textMode} ${className}`}
  class:tag-has-value={hasValue && distinguishKeyOnly}
  class:tag-key-only={!hasValue && distinguishKeyOnly}
  class:tag-overflow={overflow}
  data-tag-measure={measure ? '' : undefined}
  style={`--tag-max-width: ${maxWidth || '100%'}`}
  title={textMode === 'ellipsis' && tooltipOnTruncate ? value : undefined}
  aria-label={value}>{value}</span
>

<style>
  .tag {
    display: inline-flex;
    align-items: center;
    flex: 0 0 auto;
    min-height: 24px;
    max-width: var(--tag-max-width);
    box-sizing: border-box;
    padding: 0 7px;
    border: 1px solid transparent;
    border-radius: 4px;
    font-size: 10px;
    line-height: 1.2;
  }

  .tag.tag-theme {
    color: var(--theme-fg-strong);
    background: var(--theme-bg-accent-soft);
    border-color: var(--theme-border-focus);
  }

  .tag.tag-theme.tag-key-only {
    color: var(--theme-fg-strong);
    background: var(--theme-bg-subtle);
    border-color: var(--theme-border);
  }

  .tag.tag-independent.tag-has-value {
    color: var(--theme-fg-strong);
    background: color-mix(in srgb, var(--theme-info) 12%, var(--theme-bg-subtle));
    border-color: color-mix(in srgb, var(--theme-info) 30%, var(--theme-border));
  }

  .tag.tag-independent.tag-key-only {
    color: var(--theme-fg-strong);
    background: color-mix(in srgb, var(--theme-success) 10%, var(--theme-bg-subtle));
    border-color: color-mix(in srgb, var(--theme-success) 28%, var(--theme-border));
  }

  .tag-text-full {
    overflow: visible;
    text-overflow: clip;
    white-space: nowrap;
  }

  .tag-text-ellipsis {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tag-text-wrap {
    max-width: 100%;
    overflow-wrap: anywhere;
    white-space: normal;
  }

  .tag.tag-overflow {
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border-color: var(--theme-border);
  }
</style>
