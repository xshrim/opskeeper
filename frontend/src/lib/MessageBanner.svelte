<script lang="ts">
  import { CircleAlert, CircleCheck, Info, TriangleAlert } from 'lucide-svelte';

  export let message: string;
  export let tone: 'success' | 'error' | 'warning' | 'info' = 'info';

  const icons = {
    success: CircleCheck,
    error: CircleAlert,
    warning: TriangleAlert,
    info: Info
  };
  $: Icon = icons[tone];
</script>

<div
  class={`message-banner ${tone}`}
  role={tone === 'error' ? 'alert' : 'status'}
  aria-live="polite"
>
  <svelte:component this={Icon} size={15} strokeWidth={2} aria-hidden="true" />
  <span>{message}</span>
</div>
