<script lang="ts">
  import { Monitor, Moon, Sun, Upload, UsersRound } from 'lucide-svelte';
  import { api, ApiError, type User, type UserPreferences } from '../../lib/api';

  export let currentUser: User | null = null;
  export let avatarURL = '';
  export let preferences: UserPreferences;
  export let onApplyTheme: () => void;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;

  let profileDisplayName = '';
  let profileEmail = '';
  let profilePhone = '';
  let profileCurrentPassword = '';
  let profileNewPassword = '';
  let profileConfirmPassword = '';
  let avatarBusy = false;
  let busy = false;
  let profileUserID = '';

  $: if (currentUser?.id !== profileUserID) {
    profileUserID = currentUser?.id ?? '';
    profileDisplayName = currentUser?.display_name ?? '';
    profileEmail = currentUser?.email ?? '';
    profilePhone = currentUser?.phone ?? '';
    profileCurrentPassword = '';
    profileNewPassword = '';
    profileConfirmPassword = '';
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      return error.message || fallback;
    }
    if (error instanceof Error) return error.message || fallback;
    return fallback;
  }

  async function saveProfile() {
    busy = true;
    onError('');
    try {
      currentUser = await api.updateProfile({
        display_name: profileDisplayName,
        email: profileEmail,
        phone: profilePhone
      });
      preferences = await api.updatePreferences({
        theme: preferences.theme,
        sidebar_mode: preferences.sidebar_mode,
        sidebar_collapsed: preferences.sidebar_collapsed
      });
      onApplyTheme();
      onNotice('个人中心配置已保存');
    } catch (error) {
      onError(describeError(error, '个人中心配置保存失败'));
    } finally {
      busy = false;
    }
  }

  async function changePassword() {
    if (profileNewPassword !== profileConfirmPassword) {
      onError('两次输入的新密码不一致。');
      return;
    }
    busy = true;
    onError('');
    try {
      currentUser = await api.changePassword({
        current_password: profileCurrentPassword,
        new_password: profileNewPassword
      });
      profileCurrentPassword = '';
      profileNewPassword = '';
      profileConfirmPassword = '';
      onNotice('密码已更新');
    } catch (error) {
      onError(describeError(error, '密码更新失败'));
    } finally {
      busy = false;
    }
  }

  async function uploadAvatar(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    if (
      !['image/jpeg', 'image/png'].includes(file.type) ||
      file.size > 1 << 20
    ) {
      onError('头像仅支持不超过 1 MiB 的 PNG 或 JPEG 图片。');
      input.value = '';
      return;
    }
    avatarBusy = true;
    onError('');
    try {
      preferences = await api.updateAvatar(file);
      onNotice('头像已更新');
    } catch (error) {
      onError(describeError(error, '头像上传失败'));
    } finally {
      avatarBusy = false;
      input.value = '';
    }
  }
</script>

<section class="profile-layout">
  <section class="panel profile-panel">
    <form class="profile-form" on:submit|preventDefault={saveProfile}>
      <div class="panel-heading"><div><p class="eyebrow">PROFILE</p><h2>个人资料</h2></div></div>
      <div class="profile-avatar-row">
        {#if avatarURL}<img src={avatarURL} alt="当前头像" class="profile-avatar avatar-image" />{:else}<span class="profile-avatar">{(currentUser?.display_name || currentUser?.username || 'U').slice(0, 1).toUpperCase()}</span>{/if}
        <div>
          <strong>{currentUser?.username}</strong>
          <p>PNG 或 JPEG，最大 1 MiB。</p>
          <label class="secondary-button upload-button" for="profile-avatar-upload"><Upload size={15} strokeWidth={1.8} aria-hidden="true" />{avatarBusy ? '正在上传' : '更换头像'}</label>
          <input id="profile-avatar-upload" class="visually-hidden" type="file" accept="image/png,image/jpeg" disabled={avatarBusy} on:change={uploadAvatar} />
        </div>
      </div>
      <div class="profile-fields">
        <label>用户名<input value={currentUser?.username ?? ''} disabled aria-label="用户名" /></label>
        <label>显示名<input bind:value={profileDisplayName} required maxlength="120" placeholder="请输入显示名" aria-label="显示名" /></label>
        <label>邮箱<input type="email" bind:value={profileEmail} placeholder="例如：name@example.com" aria-label="邮箱" /></label>
        <label>电话<input type="tel" bind:value={profilePhone} placeholder="例如：13800138000" aria-label="电话" /></label>
      </div>
      <div class="profile-team"><UsersRound size={17} strokeWidth={1.8} aria-hidden="true" /><span><strong>所属团队</strong><small>当前未配置团队成员关系</small></span></div>
    </form>
    <form class="profile-password-form" on:submit|preventDefault={changePassword}>
      <div class="profile-password-row">
        <label><span class="visually-hidden">当前密码</span><input type="password" bind:value={profileCurrentPassword} required autocomplete="current-password" placeholder="当前密码" aria-label="当前密码" /></label>
        <label><span class="visually-hidden">新密码</span><input type="password" bind:value={profileNewPassword} required minlength="8" autocomplete="new-password" placeholder="新密码" aria-label="新密码" /></label>
        <label><span class="visually-hidden">确认新密码</span><input type="password" bind:value={profileConfirmPassword} required minlength="8" autocomplete="new-password" placeholder="确认新密码" aria-label="确认新密码" /></label>
        <button class="primary" disabled={busy} aria-busy={busy}>{busy ? '正在更新' : '更新密码'}</button>
      </div>
    </form>
  </section>
  <form class="profile-form" on:submit|preventDefault={saveProfile}>
    <section class="panel profile-panel">
      <div class="panel-heading"><div><p class="eyebrow">PREFERENCES</p><h2>界面偏好</h2></div></div>
      <fieldset class="preference-group"><legend>系统主题</legend><div class="segmented-control" role="radiogroup" aria-label="系统主题">
        <button type="button" class:active={preferences.theme === 'auto'} role="radio" aria-checked={preferences.theme === 'auto'} on:click={() => { preferences = { ...preferences, theme: 'auto' }; onApplyTheme(); }}><Monitor size={16} strokeWidth={1.8} aria-hidden="true" />自动</button>
        <button type="button" class:active={preferences.theme === 'light'} role="radio" aria-checked={preferences.theme === 'light'} on:click={() => { preferences = { ...preferences, theme: 'light' }; onApplyTheme(); }}><Sun size={16} strokeWidth={1.8} aria-hidden="true" />浅色</button>
        <button type="button" class:active={preferences.theme === 'dark'} role="radio" aria-checked={preferences.theme === 'dark'} on:click={() => { preferences = { ...preferences, theme: 'dark' }; onApplyTheme(); }}><Moon size={16} strokeWidth={1.8} aria-hidden="true" />深色</button>
      </div></fieldset>
      <fieldset class="preference-group"><legend>侧边导航栏</legend><div class="segmented-control" role="radiogroup" aria-label="侧边导航栏模式">
        <button type="button" class:active={preferences.sidebar_mode === 'fixed'} role="radio" aria-checked={preferences.sidebar_mode === 'fixed'} on:click={() => (preferences = { ...preferences, sidebar_mode: 'fixed' })}>固定模式</button>
        <button type="button" class:active={preferences.sidebar_mode === 'hover'} role="radio" aria-checked={preferences.sidebar_mode === 'hover'} on:click={() => (preferences = { ...preferences, sidebar_mode: 'hover', sidebar_collapsed: true })}>窄栏悬浮展开</button>
      </div><p class="preference-help">固定模式可通过侧栏右下角图标展开或收起；悬浮模式默认显示图标，鼠标移入后展开。</p></fieldset>
      <div class="profile-actions"><button class="primary" disabled={busy} aria-busy={busy}>{busy ? '正在保存' : '保存配置'}</button></div>
    </section>
  </form>
</section>
