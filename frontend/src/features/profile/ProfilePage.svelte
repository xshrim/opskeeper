<script lang="ts">
  import { Monitor, Moon, Sun, Upload, UsersRound } from 'lucide-svelte';
  import type { User, UserPreferences } from '../../lib/api';

  export let currentUser: User | null = null;
  export let avatarURL = '';
  export let avatarBusy = false;
  export let profileDisplayName = '';
  export let profileEmail = '';
  export let profilePhone = '';
  export let profileCurrentPassword = '';
  export let profileNewPassword = '';
  export let profileConfirmPassword = '';
  export let preferences: UserPreferences;
  export let busy = false;
  export let onSaveProfile: () => void | Promise<void>;
  export let onChangePassword: () => void | Promise<void>;
  export let onUploadAvatar: (event: Event) => void | Promise<void>;
  export let onApplyTheme: () => void;
</script>

<section class="profile-layout">
  <section class="panel profile-panel">
    <form class="profile-form" on:submit|preventDefault={onSaveProfile}>
      <div class="panel-heading"><div><p class="eyebrow">PROFILE</p><h2>个人资料</h2></div></div>
      <div class="profile-avatar-row">
        {#if avatarURL}<img src={avatarURL} alt="当前头像" class="profile-avatar avatar-image" />{:else}<span class="profile-avatar">{(currentUser?.display_name || currentUser?.username || 'U').slice(0, 1).toUpperCase()}</span>{/if}
        <div>
          <strong>{currentUser?.username}</strong>
          <p>PNG 或 JPEG，最大 1 MiB。</p>
          <label class="secondary-button upload-button" for="profile-avatar-upload"><Upload size={15} strokeWidth={1.8} aria-hidden="true" />{avatarBusy ? '正在上传' : '更换头像'}</label>
          <input id="profile-avatar-upload" class="visually-hidden" type="file" accept="image/png,image/jpeg" disabled={avatarBusy} on:change={onUploadAvatar} />
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
    <form class="profile-password-form" on:submit|preventDefault={onChangePassword}>
      <div class="profile-password-row">
        <label><span class="visually-hidden">当前密码</span><input type="password" bind:value={profileCurrentPassword} required autocomplete="current-password" placeholder="当前密码" aria-label="当前密码" /></label>
        <label><span class="visually-hidden">新密码</span><input type="password" bind:value={profileNewPassword} required minlength="8" autocomplete="new-password" placeholder="新密码" aria-label="新密码" /></label>
        <label><span class="visually-hidden">确认新密码</span><input type="password" bind:value={profileConfirmPassword} required minlength="8" autocomplete="new-password" placeholder="确认新密码" aria-label="确认新密码" /></label>
        <button class="primary" disabled={busy} aria-busy={busy}>{busy ? '正在更新' : '更新密码'}</button>
      </div>
    </form>
  </section>
  <form class="profile-form" on:submit|preventDefault={onSaveProfile}>
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
