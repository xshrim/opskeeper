<script lang="ts">
  import FormField from '../../components/FormField.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import TextInput from '../../components/TextInput.svelte';
  export let subtype = 'Git';
  export let url = '';
  export let username = '';
  export let password = '';
  export let credentialConfigured = false;
  export let editing = false;
  export let defaultBranch = 'main';
  export let configurationAttempted = false;
  $: issues = subtype === 'Git'
    ? (!url.trim() ? ['Git 仓库 URL'] : [])
    : [];
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
    <p class="muted">Bundle 文件由平台启动配置指定的存储后端统一保存；此处无需选择存储方式。上传后会自动覆盖同名分支并追加新分支。</p>
  {/if}
  {#if configurationAttempted && !valid}<p class="form-error">请填写：{issues.join('、')}。</p>{/if}
</div>
