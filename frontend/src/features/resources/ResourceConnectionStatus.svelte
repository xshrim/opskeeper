<script lang="ts">
  import type { ConnectionCheck, ConnectorCapability } from '../../lib/api';

  export let connectionCheck: ConnectionCheck | null = null;
  export let busy = false;
  export let connectionBusy = false;
  export let formatDate: (value: string) => string;
  export let capabilityName: (capability: ConnectorCapability) => string;
  export let onTestConnection: () => void;
</script>

<div class="connection-status">
  <div class="connection-summary">
    <span
      class:success={connectionCheck?.status === 'succeeded'}
      class:failed={connectionCheck?.status === 'failed'}
      class="connection-indicator"
      aria-hidden="true"
    ></span>
    <span>
      <strong>{connectionCheck?.status === 'succeeded' ? '连接正常' : connectionCheck?.status === 'failed' ? '连接失败' : '尚未测试'}</strong>
      <small>{connectionCheck ? `${connectionCheck.message} · ${connectionCheck.latency_ms} ms · ${formatDate(connectionCheck.checked_at)}` : '当前资源还没有连接测试记录'}</small>
    </span>
  </div>
  {#if connectionCheck?.capabilities.length}
    <div class="capability-list" aria-label="连接器能力">
      {#each connectionCheck.capabilities as capability}<span>{capabilityName(capability)}</span>{/each}
    </div>
  {/if}
  <button class="secondary connection-test-button" on:click={onTestConnection} disabled={busy || connectionBusy}>
    <span aria-hidden="true">↻</span>{connectionBusy ? '测试中' : '测试连接'}
  </button>
</div>
