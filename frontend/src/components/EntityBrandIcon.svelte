<script lang="ts">
  import ResourceBrandIcon from './ResourceBrandIcon.svelte';

  export let kind = 'Resource';
  export let subtype = '';
  export let fallback = 'lucide:Circle';
  export let size = 18;
  export let className = '';
</script>

<span class={`entity-icon resource-icon ${className}`.trim()}>
  <ResourceBrandIcon resource={{ kind, subtype }} {fallback} {size} />
</span>
