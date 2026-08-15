<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchHealth, toStatusRows, type HealthReport } from './lib/health';

  let report: HealthReport | null = null;
  let requestFailed = false;

  onMount(() => {
    const controller = new AbortController();
    const load = async () => {
      try {
        report = await fetchHealth(controller.signal);
        requestFailed = false;
      } catch (error) {
        if (!(error instanceof DOMException && error.name === 'AbortError')) {
          requestFailed = true;
        }
      }
    };

    void load();
    const interval = window.setInterval(load, 10_000);
    return () => {
      controller.abort();
      window.clearInterval(interval);
    };
  });

  $: rows = toStatusRows(report);
</script>

<svelte:head>
  <meta name="description" content="OpsKeeper platform control plane status" />
</svelte:head>

<div class="app-shell">
  <aside class="sidebar">
    <div class="brand">
      <span class="brand-mark" aria-hidden="true">O</span>
      <span>OpsKeeper</span>
    </div>
    <nav aria-label="Primary navigation">
      <a class="nav-item active" href="./" aria-current="page">Overview</a>
    </nav>
    <div class="environment">Development</div>
  </aside>

  <main>
    <header class="topbar">
      <div>
        <p class="breadcrumb">Platform</p>
        <h1>System status</h1>
      </div>
      <span class:healthy={report?.status === 'ready'} class="overall-status">
        <span class="status-dot" aria-hidden="true"></span>
        {report?.status === 'ready'
          ? 'Ready'
          : requestFailed
            ? 'Unavailable'
            : 'Checking'}
      </span>
    </header>

    <section class="status-panel" aria-labelledby="control-plane-heading">
      <div class="section-heading">
        <div>
          <h2 id="control-plane-heading">Control plane</h2>
          <p>
            Last checked {report
              ? new Date(report.timestamp).toLocaleTimeString()
              : 'not yet'}
          </p>
        </div>
        <span class="version">{report?.version ?? 'dev'}</span>
      </div>

      <div
        class="status-table"
        role="table"
        aria-label="Control plane dependencies"
      >
        <div class="table-header" role="row">
          <span role="columnheader">Service</span>
          <span role="columnheader">Status</span>
          <span role="columnheader">Latency</span>
        </div>
        {#each rows as row}
          <div class="table-row" role="row">
            <span role="cell" class="service-name">{row.name}</span>
            <span role="cell" class="service-status {row.status}">
              <span class="status-dot" aria-hidden="true"></span>
              {row.status === 'up'
                ? 'Operational'
                : row.status === 'down'
                  ? 'Unavailable'
                  : 'Checking'}
            </span>
            <span role="cell" class="latency">{row.latency ?? '—'}</span>
          </div>
        {/each}
      </div>
    </section>
  </main>
</div>
