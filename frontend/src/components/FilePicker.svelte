<script lang="ts">
  export let accept = '';
  export let label = '选择文件';
  export let buttonLabel = '选择文件';
  export let fileName = '';
  export let disabled = false;
  export let className = 'file-picker';
  export let inputClass = '';
  export let buttonClass = '';
  export let ariaLabel = '';
  export let inputAriaLabel = '';
  export let onFileChange: (event: Event) => void = () => {};
  let input: HTMLInputElement;
</script>

<span class={className}>
  {#if fileName}<small>{fileName}</small>{/if}
  <input class={inputClass} bind:this={input} type="file" {accept} {disabled} aria-label={inputAriaLabel || label} on:change={onFileChange} />
  <button class={buttonClass} type="button" {disabled} aria-label={ariaLabel || label} on:click|stopPropagation|preventDefault={() => input?.click()}>{buttonLabel}</button>
</span>
