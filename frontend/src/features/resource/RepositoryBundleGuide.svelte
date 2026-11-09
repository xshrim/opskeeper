<script lang="ts">
  import { Check, ChevronDown, Copy } from 'lucide-svelte';
  import type { Resource } from '../../lib/api';

  export let resource: Resource;
  let copied = false;
  $: uploadScript = [
    'set -euo pipefail',
    '',
    '# Run this in the root of the Git working copy.',
    'git bundle create opskeeper.bundle --all',
    'git bundle verify opskeeper.bundle',
    '',
    '# Replace this with your OpsKeeper address; include its configured base path.',
    'OPSK_BASE_URL="https://opskeeper.example.com"',
    `RESOURCE_ID="${resource.id}"`,
    'COOKIE_FILE="$(mktemp)"',
    'chmod 600 "$COOKIE_FILE"',
    'trap \'rm -f "$COOKIE_FILE"; unset OPSK_PASSWORD\' EXIT',
    '',
    'read -r -p "OpsKeeper username: " OPSK_USERNAME',
    'read -r -s -p "OpsKeeper password: " OPSK_PASSWORD',
    "printf '\\n'",
    '',
    'curl -fsS -c "$COOKIE_FILE" \\',
    "  -H 'Content-Type: application/json' \\",
    '  --data "$(jq -n --arg identifier "$OPSK_USERNAME" --arg password "$OPSK_PASSWORD" \'{identifier:$identifier,password:$password}\')" \\',
    '  "$OPSK_BASE_URL/api/v1/auth/login"',
    '',
    'curl -fsS -b "$COOKIE_FILE" \\',
    "  -F 'bundle=@opskeeper.bundle' \\",
    '  "$OPSK_BASE_URL/api/v1/resources/$RESOURCE_ID/bundle"'
  ].join('\n');

  async function copyScript() {
    await navigator.clipboard.writeText(uploadScript);
    copied = true;
    window.setTimeout(() => (copied = false), 1600);
  }
</script>

<details class="repository-bundle-guide">
  <summary>
    <span
      ><strong>Bundle 生成与上传向导</strong><small
        >生成、校验，并通过页面或 API 上传</small
      ></span
    >
    <ChevronDown size={17} aria-hidden="true" />
  </summary>
  <div class="guide-content">
    <section>
      <h3>生成 Bundle</h3>
      <p>在目标 Git 仓库的工作副本根目录执行，命令会包含所有分支和标签。</p>
      <pre><code
          >git bundle create opskeeper.bundle --all
git bundle verify opskeeper.bundle</code
        ></pre>
    </section>
    <section>
      <h3>通过当前页面上传</h3>
      <ol>
        <li>
          返回页面上方的 Bundle 上传区域，选择生成的 <code
            >opskeeper.bundle</code
          > 文件。
        </li>
        <li>点击“上传并更新分支”，等待页面显示上传成功和已同步分支。</li>
      </ol>
      <p>Bundle 的实际存储后端由 OpsKeeper 启动时的环境变量统一决定，新增或编辑资源时无需选择存储方式。</p>
    </section>
    <section>
      <div class="api-heading">
        <div>
          <h3>通过 API 上传</h3>
          <p>
            需要安装 <code>git</code>、<code>curl</code> 和
            <code>jq</code>。登录账号必须有此资源的更新权限。
          </p>
        </div>
        <button
          type="button"
          class="icon-button"
          aria-label="复制 API 上传命令"
          title="复制 API 上传命令"
          on:click={() => void copyScript()}
        >
          {#if copied}<Check size={16} aria-hidden="true" />{:else}<Copy
              size={16}
              aria-hidden="true"
            />{/if}
        </button>
      </div>
      <p>
        先将脚本中的 OpsKeeper
        地址替换为实际部署地址（若配置了基础路径，也要包含基础路径），再在 Git
        工作副本根目录运行：
      </p>
      <pre><code>{uploadScript}</code></pre>
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
  .repository-bundle-guide summary span {
    display: grid;
    gap: 3px;
  }
  .repository-bundle-guide summary strong {
    color: var(--theme-fg-strong);
    font-size: 13px;
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
    gap: 18px;
    padding: 4px 0 18px;
  }
  .guide-content section {
    min-width: 0;
  }
  .repository-bundle-guide h3 {
    margin: 0 0 6px;
    color: var(--theme-fg-strong);
    font-size: 12px;
  }
  .repository-bundle-guide p {
    margin: 0 0 8px;
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
    margin: 8px 0 0;
    padding: 12px;
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
  .api-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }
  .api-heading > div {
    min-width: 0;
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
