<script lang="ts">
  import { onMount } from 'svelte';

  export let status: 'succeeded' | 'failed' | '' = '';
  export let latencyMs = 0;
  export let message = '';
  export let checkedAt = '';
  export let busy = false;
  export let disabled = false;
  export let showTime = true;
  export let showTooltip = true;
  export let tooltipText = '';
  export let onTest: () => void = () => {};

  let now = Date.now();

  onMount(() => {
    const timer = window.setInterval(() => (now = Date.now()), 60_000);
    return () => window.clearInterval(timer);
  });

  $: detail = (message || '连接测试失败').replace(/\s+/g, ' ').trim();
  $: latency = latencyMs >= 1000 ? `${(latencyMs / 1000).toFixed(1)}s` : `${latencyMs}ms`;
  $: label = busy ? '测试中' : status === 'succeeded' ? `正常·${latency}` : status === 'failed' ? `异常·${detail}` : '未测试';
  $: tooltip = status === 'failed' ? `异常·${detail}` : tooltipText || '点击进行连接测试';
  $: checkedTime = checkedAt ? relativeTime(checkedAt, now) : '尚未测试';

  function activate() {
    if (!disabled && !busy) onTest();
  }

  function relativeTime(value: string, timestamp: number) {
    const checked = Date.parse(value);
    if (!Number.isFinite(checked)) return '尚未测试';
    const elapsed = Math.max(timestamp - checked, 0);
    const minute = 60_000;
    const hour = 60 * minute;
    const day = 24 * hour;
    const week = 7 * day;
    const month = 30 * day;
    const year = 365 * day;
    const format = (amount: number, unit: string) => `${amount.toFixed(1).padStart(4, '0')} ${unit}`;
    if (elapsed < minute) return format(elapsed / 1000, '秒');
    if (elapsed < hour) return format(elapsed / minute, '分');
    if (elapsed < day) return format(elapsed / hour, '时');
    if (elapsed < week) return format(elapsed / day, '天');
    if (elapsed < month) return format(elapsed / week, '周');
    if (elapsed < year) return format(elapsed / month, '月');
    return format(elapsed / year, '年');
  }
</script>

<span
  class="connection-test"
  class:succeeded={status === 'succeeded'}
  class:failed={status === 'failed'}
  class:testing={busy}
  role="button"
  tabindex={disabled || busy ? -1 : 0}
  aria-disabled={disabled || busy}
  aria-label={label}
  data-tooltip={showTooltip ? (busy ? '连接测试中' : tooltip) : undefined}
  on:click={(event) => { event.stopPropagation(); activate(); }}
  on:keydown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); event.stopPropagation(); activate(); } }}
>
  <span class="connection-test-label" class:scroll={status === 'failed'}>{label}</span>
  {#if showTime}<small>{checkedTime}</small>{/if}
</span>

<style>
  .connection-test {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 2px;
    width: 80px;
    min-width: 80px;
    height: 42px;
    min-height: 42px;
    box-sizing: border-box;
    overflow: hidden;
    color: var(--theme-fg-muted);
    text-align: center;
    cursor: pointer;
  }
  .connection-test-label {
    display: block;
    width: 100%;
    box-sizing: border-box;
    padding: 2px 4px;
    overflow: hidden;
    color: inherit;
    font-size: 10px;
    line-height: 1.25;
    text-align: center;
    text-overflow: clip;
    white-space: nowrap;
    background: var(--theme-bg-subtle);
    border: 1px solid transparent;
    border-radius: 8px;
  }
  .connection-test-label.scroll { width: max-content; animation: connection-test-scroll 7s linear infinite alternate; }
  .connection-test small {
    display: block;
    overflow: hidden;
    color: var(--theme-fg-muted);
    font-size: 9px;
    line-height: 1.2;
    text-align: center;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .connection-test.succeeded { color: var(--theme-success); }
  .connection-test.failed { color: var(--theme-danger); }
  .connection-test.testing { color: var(--theme-accent); cursor: progress; }
  .connection-test:hover:not([aria-disabled='true']) .connection-test-label,
  .connection-test:focus-visible .connection-test-label {
    border-color: var(--theme-border-focus);
  }
  .connection-test:focus-visible {
    outline: none;
  }
  .connection-test:focus-visible { box-shadow: var(--theme-focus); }
  .connection-test[aria-disabled='true'] { cursor: not-allowed; opacity: .72; }
  @keyframes connection-test-scroll {
    from { transform: translateX(0); }
    to { transform: translateX(calc(70px - 100%)); }
  }
</style>
