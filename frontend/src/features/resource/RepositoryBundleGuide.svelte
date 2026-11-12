<script lang="ts">
  import { Check, ChevronDown, Copy } from 'lucide-svelte';
  import type { Resource } from '../../lib/api';

  export let resource: Resource | null = null;
  export let workflow = false;
  export let summaryLabel = 'Bundle 生成与上传向导';
  let copied = '';
  $: resourceID = resource?.id ?? '<资源 ID>';
  $: apiBase = 'https://opskeeper.example.com';
  $: apiUploadWithCookie = [
    'curl -fsS -b "$COOKIE_FILE" \\',
    "  -F 'bundle=@opskeeper.bundle' \\",
    `  "${apiBase}/api/v1/resources/${resourceID}/bundle"`
  ].join('\n');
  $: apiUploadWithBasicAuth = [
    'curl -fsS -u "用户名:密码" \\',
    "  -F 'bundle=@opskeeper.bundle' \\",
    `  "${apiBase}/api/v1/resources/${resourceID}/bundle"`
  ].join('\n');

  async function copyCommand(command: string, key: string) {
    await navigator.clipboard.writeText(command);
    copied = key;
    window.setTimeout(() => (copied = ''), 1600);
  }
</script>

<details
  class="repository-bundle-guide"
  class:compact={summaryLabel === '了解详情'}
>
  <summary>
    {#if summaryLabel === '了解详情'}
      <strong class="detail-link">{summaryLabel}</strong>
    {:else}
      <span
        ><strong>{summaryLabel}</strong><small
          >生成、校验，并通过页面或 API 上传</small
        ></span
      >
    {/if}
    {#if summaryLabel !== '了解详情'}<ChevronDown
        size={17}
        aria-hidden="true"
      />{/if}
  </summary>
  <div class="guide-content">
    <section class="guide-step">
      <div class="step-number" aria-hidden="true">1</div>
      <div class="step-content">
        <h3>打包</h3>
        <p>在目标 Git 仓库的工作副本根目录执行，命令会包含所有分支和标签。</p>
        <pre><code
            >git bundle create opskeeper.bundle --all
git bundle verify opskeeper.bundle</code
          ></pre>
      </div>
    </section>
    <section class="guide-step">
      <div class="step-number" aria-hidden="true">2</div>
      <div class="step-content">
        <h3>上传</h3>
        <div class="upload-methods">
          <section class="upload-method">
            <h4><span>A.</span> 页面上传</h4>
            <ol>
              <li>
                在页面上方的 Bundle 上传区域，选择生成的 <code
                  >opskeeper.bundle</code
                > 文件。
              </li>
              <li>
                {workflow
                  ? '点击“下一步”，平台会在创建资源后上传、校验并打开仓库预览。'
                  : '点击“上传并更新分支”，等待页面显示上传成功和已同步分支。'}
              </li>
            </ol>
          </section>
          <section class="upload-method">
            <h4><span>B.</span> API 上传</h4>
            <p>
              {#if workflow}先完成当前资源创建，再将命令中的 OpsKeeper
                地址和资源 ID 替换为实际值。{:else}将命令中的 OpsKeeper
                地址替换为实际部署地址（若配置了基础路径，也要包含基础路径）。{/if}账号需要具备此资源的更新权限。
            </p>
            <div class="command-heading">
              <strong>已获取 Cookie</strong><button
                type="button"
                class="icon-button"
                aria-label="复制 Cookie 上传命令"
                title="复制 Cookie 上传命令"
                on:click={() => void copyCommand(apiUploadWithCookie, 'cookie')}
                >{#if copied === 'cookie'}<Check
                    size={16}
                    aria-hidden="true"
                  />{:else}<Copy size={16} aria-hidden="true" />{/if}</button
              >
            </div>
            <pre><code>{apiUploadWithCookie}</code></pre>
            <div class="command-heading">
              <strong>Basic 认证</strong><button
                type="button"
                class="icon-button"
                aria-label="复制 Basic 认证上传命令"
                title="复制 Basic 认证上传命令"
                on:click={() =>
                  void copyCommand(apiUploadWithBasicAuth, 'basic')}
                >{#if copied === 'basic'}<Check
                    size={16}
                    aria-hidden="true"
                  />{:else}<Copy size={16} aria-hidden="true" />{/if}</button
              >
            </div>
            <pre><code>{apiUploadWithBasicAuth}</code></pre>
          </section>
        </div>
      </div>
    </section>
  </div>
</details>

<style>
  .repository-bundle-guide {
    border-top: 1px solid var(--theme-divider);
    border-bottom: 1px solid var(--theme-divider);
  }
  .repository-bundle-guide summary {
    display: flex;
    min-height: 54px;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    cursor: pointer;
    list-style: none;
  }
  .repository-bundle-guide summary::-webkit-details-marker {
    display: none;
  }
  .repository-bundle-guide.compact {
    display: contents;
    border: 0;
  }
  .repository-bundle-guide.compact summary {
    display: inline-flex;
    min-height: 0;
  }
  .repository-bundle-guide.compact .guide-content {
    flex-basis: 100%;
    padding-top: 8px;
  }
  .repository-bundle-guide summary span {
    display: grid;
    gap: 3px;
  }
  .repository-bundle-guide summary strong {
    color: var(--theme-fg-strong);
    font-size: 13px;
  }
  .repository-bundle-guide summary .detail-link {
    color: var(--theme-action);
    font-size: 12px;
    line-height: 1.55;
    text-decoration: underline;
    text-underline-offset: 3px;
    font-weight: 500;
  }
  .repository-bundle-guide summary small,
  .repository-bundle-guide p,
  .repository-bundle-guide li {
    color: var(--theme-fg-muted);
    font-size: 12px;
    line-height: 1.55;
  }
  .repository-bundle-guide summary :global(svg) {
    flex: 0 0 auto;
    transition: transform 150ms ease;
  }
  .repository-bundle-guide[open] summary :global(svg) {
    transform: rotate(180deg);
  }
  .guide-content {
    display: grid;
    gap: 12px;
    padding: 6px 0 12px;
  }
  .guide-step {
    display: grid;
    grid-template-columns: 26px minmax(0, 1fr);
    gap: 10px;
    min-width: 0;
  }
  .step-number {
    display: grid;
    width: 24px;
    height: 24px;
    place-items: center;
    border: 1px solid var(--theme-border-focus);
    border-radius: 4px;
    background: var(--theme-bg-accent-soft);
    color: var(--theme-action);
    font-size: 12px;
    font-weight: 700;
  }
  .step-content {
    min-width: 0;
  }
  .repository-bundle-guide h3 {
    margin: 0 0 4px;
    color: var(--theme-fg-strong);
    font-size: 12px;
  }
  .repository-bundle-guide h4 {
    margin: 0 0 4px;
    color: var(--theme-fg-strong);
    font-size: 12px;
  }
  .upload-methods {
    display: grid;
    gap: 10px;
    margin-top: 6px;
    padding-left: 13px;
    border-left: 1px solid var(--theme-divider);
  }
  .upload-method {
    display: grid;
    gap: 4px;
  }
  .upload-method h4 {
    display: flex;
    align-items: center;
    gap: 7px;
  }
  .upload-method h4 span {
    color: var(--theme-action);
    font-variant-numeric: tabular-nums;
  }
  .repository-bundle-guide p {
    margin: 0 0 5px;
  }
  .repository-bundle-guide ol {
    margin: 0;
    padding-left: 20px;
  }
  .repository-bundle-guide li + li {
    margin-top: 4px;
  }
  .repository-bundle-guide pre {
    overflow: auto;
    margin: 5px 0 0;
    padding: 9px 10px;
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    background: var(--theme-bg-sunken);
    color: var(--theme-fg-strong);
    font:
      11px/1.55 ui-monospace,
      SFMono-Regular,
      Menlo,
      monospace;
    white-space: pre;
  }
  .command-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-top: 4px;
  }
  .command-heading strong {
    color: var(--theme-fg-strong);
    font-size: 12px;
  }
  .icon-button {
    display: grid;
    width: 30px;
    height: 30px;
    flex: 0 0 auto;
    place-items: center;
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    background: var(--theme-bg-raised);
    color: var(--theme-fg-muted);
    cursor: pointer;
  }
  .icon-button:hover {
    color: var(--theme-fg-strong);
  }
</style>
