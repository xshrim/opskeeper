<script lang="ts">
  import { FolderPlus, Plus, Trash2 } from 'lucide-svelte';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import type { Resource, Team } from '../../lib/api';
  import ApplicationWizard from './ApplicationWizard.svelte';
  import { runtimeLabel, type ApplicationDraft } from './projectTypes';

  export let teams: Team[] = [];
  export let resources: Resource[] = [];
  export let defaultTeamId = '';
  export let onSave: (input: { teamId: string; name: string; code: string; icon: string; description: string; applications: ApplicationDraft[] }) => Promise<void> | void;
  export let onCancel: () => void;

  let teamId = defaultTeamId || teams[0]?.id || '';
  let name = '';
  let code = '';
  let icon = 'lucide:FolderKanban';
  let description = '';
  let applications: ApplicationDraft[] = [];
  let applicationWizardOpen = false;
  let saving = false;
  let error = '';

  $: if (!teamId && teams[0]) teamId = teams[0].id;

  function addApplications(next: ApplicationDraft[]) {
    applications = [...applications, ...next];
    applicationWizardOpen = false;
  }

  async function save() {
    error = '';
    if (!teamId || !name.trim() || !code.trim()) { error = '请选择所属团队并填写项目名称和项目编号。'; return; }
    saving = true;
    try {
      await onSave({ teamId, name: name.trim(), code: code.trim(), icon, description: description.trim(), applications });
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '保存项目失败。';
    } finally {
      saving = false;
    }
  }
</script>

<div class="project-overlay" role="presentation" on:click={(event) => event.target === event.currentTarget && onCancel()}>
  <div class="project-dialog project-create-dialog" role="dialog" aria-modal="true" aria-labelledby="project-create-title" tabindex="-1">
    <header class="project-dialog-header"><div><h2 id="project-create-title">新增项目</h2><p>先填写项目基本信息，再将应用加入项目。项目本身不区分来源类型。</p></div><div class="project-dialog-actions"><button class="secondary" type="button" on:click={onCancel}>取消</button><button class="primary" type="button" disabled={saving} on:click={save}>{saving ? '创建中…' : '创建项目'}</button></div></header>
    {#if error}<div class="form-error" role="alert">{error}</div>{/if}
    <div class="project-dialog-content project-create-content">
      <section class="create-section"><div class="create-section-heading"><div><h3>项目基本信息</h3><p>名称、编号和说明用于识别项目。</p></div><FolderPlus size={18} /></div><div class="wizard-form-grid"><label><span>所属团队<i class="required-mark">*</i></span><select bind:value={teamId}><option value="">选择团队</option>{#each teams as team}<option value={team.id}>{team.name}</option>{/each}</select></label><label><span>项目名称<i class="required-mark">*</i></span><input bind:value={name} placeholder="支付结算平台" /></label><label><span>项目编号<i class="required-mark">*</i></span><input bind:value={code} required placeholder="payments-platform" /></label><label><span>项目图标</span><IconPicker value={icon} onSelect={(value) => (icon = value)} ariaLabel="选择项目图标" /></label><label class="full-field"><span>项目描述</span><textarea bind:value={description} rows="3" placeholder="项目职责、边界和负责人说明"></textarea></label></div></section>
      <section class="create-section"><div class="create-section-heading"><div><h3>项目应用</h3><p>应用可在这里预先添加，也可以稍后从项目驾驶舱添加。</p></div><button class="secondary" type="button" on:click={() => (applicationWizardOpen = true)}><Plus size={14} />添加应用</button></div>{#if applications.length}<div class="project-draft-list">{#each applications as application, index}<article class="project-draft-row"><EntityBrandIcon kind="Application" fallback={application.icon} size={20} /><div><strong>{application.name}</strong><small>{runtimeLabel(application.runtimeKind)} · {application.instances.length} 个实例 · {application.code}</small></div><button class="icon-button" type="button" aria-label="删除待添加应用" data-tooltip="删除" on:click={() => (applications = applications.filter((_, itemIndex) => itemIndex !== index))}><Trash2 size={15} /></button></article>{/each}</div>{:else}<div class="create-empty"><Plus size={18} /><span>尚未添加应用</span><button class="text-button" type="button" on:click={() => (applicationWizardOpen = true)}>从应用向导添加</button></div>{/if}</section>
    </div>
  </div>
  {#if applicationWizardOpen}<ApplicationWizard {resources} onSave={addApplications} onCancel={() => (applicationWizardOpen = false)} />{/if}
</div>
