<script lang="ts">
  import ConnectionTest from '../../components/ConnectionTest.svelte';

  export let resourceName = '';
  export let subtype = 'Git';
  export let url = '';
  export let defaultBranch = 'main';
  export let credentialConfigured = false;
  export let scopeSummary = '';
  export let labelsConfigured = false;
  export let bundleConfigured = false;
  export let storageBackend = '';
  export let testBusy = false;
  export let testStatus = '';
  export let testMessage = '';
  export let testError = '';
  export let testLatency = 0;

  const tools = [
    'repository_branches',
    'repository_status',
    'repository_tree',
    'repository_file',
    'repository_search',
    'repository_metadata'
  ];
</script>

<div class="provider-summary docker-summary">
  <div>
    <span>Repository 资源</span><strong>{resourceName}</strong><small
      >{scopeSummary}</small
    >
  </div>
  <div>
    <span>接入方式</span><strong>{subtype}</strong>
    {#if subtype === 'Git'}<small
        >{url || '未设置仓库地址'} · 默认分支 {defaultBranch || 'main'}</small
      >
    {:else}<small>由平台启动配置指定存储后端</small>{/if}
  </div>
  {#if subtype === 'Git'}
    <div>
      <span>HTTPS 凭据</span><strong
        >{credentialConfigured ? '已配置' : '未配置'}</strong
      ><small
        >{credentialConfigured ? '凭据加密保存' : '公开仓库可不配置'}</small
      >
    </div>
  {:else}
    <div>
      <span>Bundle 状态</span><strong
        >{bundleConfigured ? '已上传并正在使用' : '待上传'}</strong
      ><small
        >{bundleConfigured
          ? '连接测试会从配置的存储后端读取'
          : '请上传 Bundle 后继续'}</small
      >
    </div>
    <div>
      <span>存储后端</span><strong
        >{storageBackend || '由平台启动配置决定'}</strong
      ><small>资源本身不单独选择存储方式</small>
    </div>
  {/if}
  <div>
    <span>资源属性</span><strong
      >{labelsConfigured ? '已配置标签' : '未配置标签'}</strong
    >
  </div>
  <div class="provider-test-summary">
    <span>连接核验</span>
    <ConnectionTest
      status={testError || testStatus === 'failed'
        ? 'failed'
        : testStatus === 'succeeded'
          ? 'succeeded'
          : ''}
      latencyMs={testLatency}
      message={testError || testMessage}
      busy={testBusy}
      disabled
      showTime={false}
      showTooltip={false}
    />
  </div>
  <div class="docker-tool-summary">
    <span>可用只读工具</span>
    <div>
      {#each tools as tool}<code>{tool}</code>{/each}
    </div>
  </div>
</div>
