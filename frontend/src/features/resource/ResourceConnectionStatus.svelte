<script lang="ts">
  import type { ConnectionCheck, ConnectorCapability } from '../../lib/api';

  export let check: ConnectionCheck | null = null;
  export let busy = false;
  export let connectionBusy = false;
  export let formatDate: (value: string) => string;
  export let capabilityName: (capability: ConnectorCapability) => string;
  export let onTest: () => void = () => {};
</script>

<div class="connection-status">
  <div class="connection-summary">
    <span class:success={check?.status === 'succeeded'} class:failed={check?.status === 'failed'} class="connection-indicator" aria-hidden="true"></span>
    <span><strong>{check?.status === 'succeeded' ? '连接正常' : check?.status === 'failed' ? '连接失败' : '尚未测试'}</strong><small>{check ? `${check.message} · ${check.latency_ms} ms · ${formatDate(check.checked_at)}` : '当前资源还没有连接测试记录'}</small></span>
  </div>
  {#if check?.capabilities.length}<div class="capability-list" aria-label="连接器能力">{#each check.capabilities as capability}<span>{capabilityName(capability)}</span>{/each}</div>{/if}
  <button class="secondary connection-test-button" type="button" on:click={onTest} disabled={busy || connectionBusy}><span aria-hidden="true">↻</span>{connectionBusy ? '测试中' : '测试连接'}</button>
</div>
