<script lang="ts">
  import ConnectionTest from '../../components/ConnectionTest.svelte';
  import type { ConnectionCheck, ConnectorCapability } from '../../lib/api';

  export let check: ConnectionCheck | null = null;
  export let busy = false;
  export let connectionBusy = false;
  export let capabilityName: (capability: ConnectorCapability) => string;
  export let onTest: () => void = () => {};
</script>

<div class="connection-status">
  <ConnectionTest
    status={check?.status ?? ''}
    latencyMs={check?.latency_ms ?? 0}
    message={check?.message ?? ''}
    checkedAt={check?.checked_at ?? ''}
    busy={connectionBusy}
    disabled={busy}
    {onTest}
  />
  {#if check?.capabilities.length}<div
      class="capability-list"
      aria-label="连接器能力"
    >
      {#each check.capabilities as capability}<span
          >{capabilityName(capability)}</span
        >{/each}
    </div>{/if}
</div>
