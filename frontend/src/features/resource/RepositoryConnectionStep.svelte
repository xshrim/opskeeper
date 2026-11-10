<script lang="ts">
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import RepositoryBundleGuide from './RepositoryBundleGuide.svelte';
  export let subtype = 'Git';
  export let url = '';
  export let username = '';
  export let password = '';
  export let credentialConfigured = false;
  export let editing = false;
  export let defaultBranch = 'main';
  export let bundleFile: File | null = null;
  export let configurationAttempted = false;
  $: issues = subtype === 'Git'
    ? (!url.trim() ? ['Git 仓库 URL'] : [])
    : (!bundleFile && !editing ? ['Bundle 文件'] : []);
  $: valid = issues.length === 0;
</script>
<div class="stack-form">
  {#if subtype === 'Git'}
    <FormField label="Git 仓库 URL"><TextInput bind:value={url} placeholder="https://git.example.com/team/repo.git" /></FormField>
    <FormField label="默认分支"><TextInput bind:value={defaultBranch} placeholder="main" /></FormField>
    <FormField label="HTTPS 用户名（私有仓库必填）"><TextInput bind:value={username} autocomplete="username" placeholder={editing && credentialConfigured ? '已配置凭据，留空以保留' : ''} /></FormField>
    <FormField label="HTTPS 密码或访问令牌（私有仓库必填）"><PasswordInput bind:value={password} autocomplete="new-password" placeholder={editing && credentialConfigured ? '已配置凭据，留空以保留' : ''} ariaLabel="Git HTTPS 密码或访问令牌" /></FormField>
    <p class="muted">公开仓库可不填写凭据；私有仓库需填写可拉取远程仓库的用户名和密码或访问令牌。凭据加密保存，不要写入 URL。</p>
  {:else}
    <FormField label="Bundle 文件" required={!editing}>
      <input
        class="bundle-file-input"
        type="file"
        accept=".bundle,application/octet-stream"
        aria-label="Bundle 文件"
        on:change={(event) => (bundleFile = (event.currentTarget as HTMLInputElement).files?.[0] ?? null)}
      />
      {#if bundleFile}<small class="file-meta">{bundleFile.name} · {Math.ceil(bundleFile.size / 1024)} KB</small>{/if}
    </FormField>
    <div class="muted bundle-description">
      <span>请先将代码仓库打包为 Git Bundle 文件，再上传至平台；平台会校验 Bundle 内容并用于后续的源码级诊断。</span>
      <RepositoryBundleGuide resource={null} workflow summaryLabel="了解详情" />
    </div>
  {/if}
  {#if configurationAttempted && !valid}<p class="form-error">请填写：{issues.join('、')}。</p>{/if}
</div>

<style>
  .file-meta { color: var(--theme-fg-muted); font-size: 12px; }
  .bundle-file-input { color: var(--theme-fg); font-size: 12px; }
  .bundle-description {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    color: var(--theme-fg);
  }
  .bundle-description :global(.repository-bundle-guide.compact[open]) {
    flex-basis: 100%;
  }
</style>
