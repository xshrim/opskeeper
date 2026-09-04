<script lang="ts">
  export let sidebarCompact = false;
  export let sidebarHoverMode = false;
</script>

<div class="app-shell" class:sidebar-compact={sidebarCompact} class:sidebar-hover-mode={sidebarHoverMode}>
  <slot />
</div>
