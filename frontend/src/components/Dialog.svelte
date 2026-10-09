<script lang="ts">
  export let className = '';
  export let dialogClass = '';
  export let labelledBy = '';
  export let label = '';
  export let closeOnBackdrop = true;
  export let onBackdrop: () => void = () => {};

  function handleBackdrop(event: MouseEvent) {
    if (closeOnBackdrop && event.currentTarget === event.target) onBackdrop();
  }
</script>

<div class={`dialog-backdrop ${className}`.trim()} role="presentation" on:click={handleBackdrop}>
  <dialog open class={`dialog ${dialogClass}`.trim()} aria-labelledby={labelledBy || undefined} aria-label={label || undefined}>
    <slot />
  </dialog>
</div>
