<script lang="ts">
  export let subtype = 'Git';
  export let url = '';
  export let defaultBranch = 'main';
  export let storageBackend = 'local';
  export let localRoot = '';
  export let s3Endpoint = '';
  export let s3Bucket = '';
  export let s3Prefix = 'repositories';
  export let configurationAttempted = false;
  $: issues = subtype === 'Git'
    ? (!url.trim() ? ['Git 仓库 URL'] : [])
    : storageBackend === 's3'
      ? (!s3Endpoint.trim() || !s3Bucket.trim() ? ['S3 Endpoint 和 Bucket'] : [])
      : [];
  $: valid = issues.length === 0;
</script>
<div class="stack-form">
  {#if subtype === 'Git'}
    <label><span>Git 仓库 URL</span><input bind:value={url} placeholder="https://git.example.com/team/repo.git" /></label>
    <label><span>默认分支</span><input bind:value={defaultBranch} placeholder="main" /></label>
    <p class="muted">Git 仓库按需实时读取；认证凭据可在保存后关联资源凭据。</p>
  {:else}
    <label><span>Bundle 存储后端</span><select bind:value={storageBackend}><option value="local">本地磁盘</option><option value="s3">S3-compatible</option></select></label>
    {#if storageBackend === 'local'}
      <label><span>本地目录（可选）</span><input bind:value={localRoot} placeholder="由服务端默认目录管理" /></label>
    {:else}
      <label><span>S3 Endpoint</span><input bind:value={s3Endpoint} placeholder="http://minio:9000 或 https://silo.example.com" /></label>
      <label><span>Bucket</span><input bind:value={s3Bucket} placeholder="opskeeper" /></label>
      <label><span>对象前缀</span><input bind:value={s3Prefix} placeholder="repositories" /></label>
    {/if}
    <p class="muted">上传 .bundle 后会自动覆盖同名分支并追加新分支。</p>
  {/if}
  {#if configurationAttempted && !valid}<p class="form-error">请填写：{issues.join('、')}。</p>{/if}
</div>
