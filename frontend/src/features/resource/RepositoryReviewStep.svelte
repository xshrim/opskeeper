<script lang="ts">
  export let resourceName = ''; export let subtype = 'Git'; export let url = ''; export let storageBackend = 'local'; export let s3Bucket = '';
</script>
<div class="stack-form"><p><strong>资源名称：</strong>{resourceName}</p><p><strong>接入方式：</strong>{subtype}</p>{#if subtype === 'Git'}<p><strong>仓库：</strong>{url}</p>{:else}<p><strong>存储：</strong>{storageBackend === 's3' ? `S3 · ${s3Bucket}` : storageBackend === 'postgres' ? 'PostgreSQL' : '本地磁盘'}</p>{/if}</div>
