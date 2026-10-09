<script lang="ts">
  import SectionHeading from './SectionHeading.svelte';

  export let title = '';
  export let eyebrow = '';
  export let description = '';
  export let titleTag: 'h2' | 'h3' = 'h2';
  export let titleId = '';
</script>

<SectionHeading {title} {eyebrow} {description} {titleTag} {titleId} className="panel-heading">
  <slot />
  <slot name="actions" slot="actions" />
</SectionHeading>
