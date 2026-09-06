<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { Eye, EyeOff } from 'lucide-svelte';
  import { api, ApiError, type User } from '../../lib/api';
  import MessageBanner from '../../components/MessageBanner.svelte';

  export let authState: 'loading' | 'login' | 'ready' = 'loading';
  export let currentUser: User | null = null;
  export let onAuthenticated: (
    user: User,
    notice?: string
  ) => void | Promise<void>;

  let loginIdentifier = '';
  let password = '';
  let passwordVisible = false;
  let authError = '';
  let authErrorTimer: number | null = null;
  let requiredNewPassword = '';
  let requiredConfirmPassword = '';
  let requiredNewPasswordVisible = false;
  let requiredConfirmPasswordVisible = false;
  let busy = false;

  function focusOnMount(node: HTMLInputElement) {
    node.focus();
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      return error.message || fallback;
    }
    return error instanceof Error ? error.message || fallback : fallback;
  }

  function resetLoginForm() {
    password = '';
    passwordVisible = false;
  }

  function resetRequiredPasswordForm() {
    requiredNewPassword = '';
    requiredConfirmPassword = '';
    requiredNewPasswordVisible = false;
    requiredConfirmPasswordVisible = false;
  }

  async function restoreSession() {
    authState = 'loading';
    authError = '';
    try {
      const user = await api.me();
      await onAuthenticated(user);
      authState = 'ready';
    } catch (error) {
      authState = 'login';
      if (!(error instanceof ApiError && error.status === 401)) {
        authError = '无法恢复会话，请重新登录。';
      }
    }
  }

  async function login() {
    busy = true;
    authError = '';
    try {
      const user = await api.login(loginIdentifier.trim(), password);
      resetLoginForm();
      await onAuthenticated(user);
      authState = 'ready';
    } catch (error) {
      authError = describeError(
        error,
        '登录失败，请检查用户名、邮箱、手机号和密码'
      );
    } finally {
      busy = false;
    }
  }

  async function changeRequiredPassword() {
    if (requiredNewPassword !== requiredConfirmPassword) {
      authError = '两次输入的新密码不一致。';
      return;
    }

    busy = true;
    authError = '';
    try {
      const user = await api.changePassword({
        new_password: requiredNewPassword
      });
      resetRequiredPasswordForm();
      await onAuthenticated(user, '密码已更新');
      authState = 'ready';
    } catch (error) {
      authError = describeError(error, '密码更新失败');
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    void restoreSession();
  });

  onDestroy(() => {
    if (authErrorTimer !== null) window.clearTimeout(authErrorTimer);
  });

  $: if (authError) {
    if (authErrorTimer !== null) window.clearTimeout(authErrorTimer);
    authErrorTimer = window.setTimeout(() => {
      authError = '';
      authErrorTimer = null;
    }, 5_000);
  }
</script>

{#if authState === 'loading'}
  <div class="loading-screen"><div class="loading-state"><span class="spinner"></span><p>正在恢复工作区会话…</p></div></div>
{:else if authState === 'login'}
  <main class="login-shell">
    <div class="login-brand" aria-label="OpsKeeper 智能值守平台"><span class="login-logo" aria-hidden="true">O</span><span class="login-brand-copy"><strong>OpsKeeper</strong><small>智能值守平台</small></span></div>
    <section class="login-panel" aria-labelledby="login-heading">
      <header class="login-panel-header"><p class="login-kicker">账号登录</p><h1 id="login-heading">欢迎回来</h1><p class="login-intro">使用平台账号继续访问 OpsKeeper。</p>{#if authError}<MessageBanner message={authError} tone="error" />{/if}</header>
      <form class="stack-form login-form" on:submit|preventDefault={login}>
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
      <header class="login-panel-header"><p class="login-kicker">安全验证</p><h1 id="required-password-heading">请修改一次性密码</h1><p class="login-intro">为保护账号安全，完成修改前无法访问平台内容。</p>{#if authError}<MessageBanner message={authError} tone="error" />{/if}</header>
      <form class="stack-form login-form" on:submit|preventDefault={changeRequiredPassword}>
        <label class="login-field">新密码<span class="password-control"><input type={requiredNewPasswordVisible ? 'text' : 'password'} bind:value={requiredNewPassword} required minlength="8" autocomplete="new-password" placeholder="至少 8 位" /><button class="password-toggle" type="button" aria-label={requiredNewPasswordVisible ? '隐藏新密码' : '显示新密码'} aria-pressed={requiredNewPasswordVisible} data-tooltip={requiredNewPasswordVisible ? '隐藏新密码' : '显示新密码'} on:click={() => (requiredNewPasswordVisible = !requiredNewPasswordVisible)}>{#if requiredNewPasswordVisible}<EyeOff size={18} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={18} strokeWidth={1.8} aria-hidden="true" />{/if}</button></span></label>
        <label class="login-field">确认新密码<span class="password-control"><input type={requiredConfirmPasswordVisible ? 'text' : 'password'} bind:value={requiredConfirmPassword} required minlength="8" autocomplete="new-password" placeholder="再次输入新密码" /><button class="password-toggle" type="button" aria-label={requiredConfirmPasswordVisible ? '隐藏确认密码' : '显示确认密码'} aria-pressed={requiredConfirmPasswordVisible} data-tooltip={requiredConfirmPasswordVisible ? '隐藏确认密码' : '显示确认密码'} on:click={() => (requiredConfirmPasswordVisible = !requiredConfirmPasswordVisible)}>{#if requiredConfirmPasswordVisible}<EyeOff size={18} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={18} strokeWidth={1.8} aria-hidden="true" />{/if}</button></span></label>
        <button class="primary login-submit" disabled={busy}>{busy ? '正在更新' : '更新密码并继续'}</button>
      </form>
    </section>
  </main>
{/if}
