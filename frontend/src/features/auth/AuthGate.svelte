<script lang="ts">
  import { Eye, EyeOff } from 'lucide-svelte';
  import type { User } from '../../lib/api';
  import MessageBanner from '../../components/MessageBanner.svelte';

  export let authState: 'loading' | 'login' | 'ready' = 'loading';
  export let currentUser: User | null = null;
  export let loginIdentifier = '';
  export let password = '';
  export let passwordVisible = false;
  export let loginError = '';
  export let errorMessage = '';
  export let requiredNewPassword = '';
  export let requiredConfirmPassword = '';
  export let requiredNewPasswordVisible = false;
  export let requiredConfirmPasswordVisible = false;
  export let busy = false;
  export let onLogin: () => void | Promise<void>;
  export let onChangePassword: (required?: boolean) => void | Promise<void>;

  function focusOnMount(node: HTMLInputElement) {
    node.focus();
  }
</script>

{#if authState === 'loading'}
  <div class="loading-screen"><div class="loading-state"><span class="spinner"></span><p>正在恢复工作区会话…</p></div></div>
{:else if authState === 'login'}
  <main class="login-shell">
    <div class="login-brand" aria-label="OpsKeeper 智能值守平台"><span class="login-logo" aria-hidden="true">O</span><span class="login-brand-copy"><strong>OpsKeeper</strong><small>智能值守平台</small></span></div>
    <section class="login-panel" aria-labelledby="login-heading">
      <header class="login-panel-header"><p class="login-kicker">账号登录</p><h1 id="login-heading">欢迎回来</h1><p class="login-intro">使用平台账号继续访问 OpsKeeper。</p>{#if loginError}<MessageBanner message={loginError} tone="error" />{/if}</header>
      <form class="stack-form login-form" on:submit|preventDefault={onLogin}>
        <div class="login-field"><label for="login-identifier">账号</label><input id="login-identifier" type="text" bind:value={loginIdentifier} autocomplete="username" required use:focusOnMount placeholder="用户名、邮箱或手机号" /></div>
        <div class="login-field"><label for="login-password">密码</label><span class="password-control"><input id="login-password" type={passwordVisible ? 'text' : 'password'} bind:value={password} autocomplete="current-password" required placeholder="请输入登录密码" /><button class="password-toggle" type="button" aria-label={passwordVisible ? '隐藏密码' : '显示密码'} aria-pressed={passwordVisible} data-tooltip={passwordVisible ? '隐藏密码' : '显示密码'} on:click={() => (passwordVisible = !passwordVisible)}>{#if passwordVisible}<EyeOff size={18} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={18} strokeWidth={1.8} aria-hidden="true" />{/if}</button></span></div>
        <span class="login-submit-wrap" data-tooltip={!loginIdentifier.trim() || !password ? '请先填写账号和密码' : undefined}><button class="login-submit" type="submit" disabled={busy || !loginIdentifier.trim() || !password} aria-busy={busy}>{#if busy}<span class="button-spinner" aria-hidden="true"></span>{/if}<span>{busy ? '正在登录' : '登录'}</span></button></span>
      </form>
      <p class="login-footnote">账号权限由平台管理员统一配置</p>
    </section>
  </main>
{:else if currentUser?.must_change_password}
  <main class="login-shell">
    <section class="login-panel" aria-labelledby="required-password-heading">
      <header class="login-panel-header"><p class="login-kicker">安全验证</p><h1 id="required-password-heading">请修改一次性密码</h1><p class="login-intro">为保护账号安全，完成修改前无法访问平台内容。</p>{#if errorMessage}<MessageBanner message={errorMessage} tone="error" />{/if}</header>
      <form class="stack-form login-form" on:submit|preventDefault={() => onChangePassword(true)}>
        <label class="login-field">新密码<span class="password-control"><input type={requiredNewPasswordVisible ? 'text' : 'password'} bind:value={requiredNewPassword} required minlength="8" autocomplete="new-password" placeholder="至少 8 位" /><button class="password-toggle" type="button" aria-label={requiredNewPasswordVisible ? '隐藏新密码' : '显示新密码'} aria-pressed={requiredNewPasswordVisible} data-tooltip={requiredNewPasswordVisible ? '隐藏新密码' : '显示新密码'} on:click={() => (requiredNewPasswordVisible = !requiredNewPasswordVisible)}>{#if requiredNewPasswordVisible}<EyeOff size={18} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={18} strokeWidth={1.8} aria-hidden="true" />{/if}</button></span></label>
        <label class="login-field">确认新密码<span class="password-control"><input type={requiredConfirmPasswordVisible ? 'text' : 'password'} bind:value={requiredConfirmPassword} required minlength="8" autocomplete="new-password" placeholder="再次输入新密码" /><button class="password-toggle" type="button" aria-label={requiredConfirmPasswordVisible ? '隐藏确认密码' : '显示确认密码'} aria-pressed={requiredConfirmPasswordVisible} data-tooltip={requiredConfirmPasswordVisible ? '隐藏确认密码' : '显示确认密码'} on:click={() => (requiredConfirmPasswordVisible = !requiredConfirmPasswordVisible)}>{#if requiredConfirmPasswordVisible}<EyeOff size={18} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={18} strokeWidth={1.8} aria-hidden="true" />{/if}</button></span></label>
        <button class="primary login-submit" disabled={busy}>{busy ? '正在更新' : '更新密码并继续'}</button>
      </form>
    </section>
  </main>
{/if}
