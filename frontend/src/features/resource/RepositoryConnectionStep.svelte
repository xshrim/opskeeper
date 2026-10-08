<script lang="ts">
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import TextInput from '../../components/TextInput.svelte';
  export let subtype = 'Git';
  export let url = '';
  export let defaultBranch = 'main';
  export let storageBackend = 'postgres';
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
  const storageOptions = [
    { value: 'local', label: '本地磁盘' },
    { value: 'postgres', label: 'PostgreSQL' },
    { value: 's3', label: 'S3-compatible' }
  ];
</script>
<div class="stack-form">
  {#if subtype === 'Git'}
    <FormField label="Git 仓库 URL"><TextInput bind:value={url} placeholder="https://git.example.com/team/repo.git" /></FormField>
    <FormField label="默认分支"><TextInput bind:value={defaultBranch} placeholder="main" /></FormField>
    <p class="muted">Git 仓库按需实时读取；认证信息会随资源加密保存。</p>
  {:else}
    <FormField label="Bundle 存储后端"><DropdownSelect bind:value={storageBackend} options={storageOptions} ariaLabel="Bundle 存储后端" /></FormField>
    {#if storageBackend === 'local'}
      <FormField label="本地目录（可选）"><TextInput bind:value={localRoot} placeholder="由服务端默认目录管理" /></FormField>
    {:else if storageBackend === 's3'}
      <FormField label="S3 Endpoint"><TextInput bind:value={s3Endpoint} placeholder="http://minio:9000 或 https://silo.example.com" /></FormField>
      <FormField label="Bucket"><TextInput bind:value={s3Bucket} placeholder="opskeeper" /></FormField>
      <FormField label="对象前缀"><TextInput bind:value={s3Prefix} placeholder="repositories" /></FormField>
    {/if}
    <p class="muted">上传 .bundle 后会自动覆盖同名分支并追加新分支。</p>
  {/if}
  {#if configurationAttempted && !valid}<p class="form-error">请填写：{issues.join('、')}。</p>{/if}
</div>
