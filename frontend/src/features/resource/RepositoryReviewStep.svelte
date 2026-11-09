<script lang="ts">
  export let resourceName = ''; export let subtype = 'Git'; export let url = ''; export let credentialConfigured = false;
</script>
<div class="stack-form"><p><strong>资源名称：</strong>{resourceName}</p><p><strong>接入方式：</strong>{subtype}</p>{#if subtype === 'Git'}<p><strong>仓库：</strong>{url}</p><p><strong>HTTPS 凭据：</strong>{credentialConfigured ? '已配置' : '未配置（公开仓库可留空）'}</p>{:else}<p><strong>存储：</strong>由平台启动配置统一决定</p>{/if}</div>
