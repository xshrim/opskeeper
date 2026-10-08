<script lang="ts">
  import { Plus, Trash2 } from 'lucide-svelte';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import type { Resource, Team } from '../../lib/api';
  import ApplicationWizard from './ApplicationWizard.svelte';
  import { runtimeLabel, type ApplicationDraft } from './projectTypes';

  export let teams: Team[] = [];
  export let resources: Resource[] = [];
  export let defaultTeamId = '';
  export let onSave: (input: {
    teamId: string;
    name: string;
    code: string;
    icon: string;
    description: string;
    labels: Record<string, string>;
    applications: ApplicationDraft[];
  }) => Promise<void> | void;
  export let onCancel: () => void;

  let teamId = defaultTeamId || teams[0]?.id || '';
  let name = '';
  let code = generateProjectCode();
  let icon = 'lucide:FolderKanban';
  let description = '';
  let labels = '';
  let applications: ApplicationDraft[] = [];
  let applicationWizardOpen = false;
  let saving = false;
  let error = '';

  $: if (!teamId && teams[0]) teamId = teams[0].id;
  $: teamOptions = teams.map((team) => ({
    value: team.id,
    label: team.name,
    icon: team.icon || 'lucide:UsersRound'
  }));

  function generateProjectCode() {
    const letters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    const digits = '0123456789';
    const random = (source: string) =>
      source[Math.floor(Math.random() * source.length)];
    return `${random(letters)}${random(letters)}${random(letters)}${Array.from({ length: 4 }, () => random(digits)).join('')}`;
  }

  function addApplications(next: ApplicationDraft[]) {
    applications = [...applications, ...next];
    applicationWizardOpen = false;
  }

  function parseLabels(value: string): Record<string, string> {
    return Object.fromEntries(
      value
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
        .map((item) => {
          const [key, ...rest] = item.split('=');
          return [key.trim(), rest.join('=').trim()];
        })
        .filter(([key]) => key)
    );
  }

  async function save() {
    error = '';
    if (!teamId || !name.trim() || !code.trim()) {
      error = '请选择所属团队并填写项目名称和项目编号。';
      return;
    }
    saving = true;
    try {
      await onSave({
        teamId,
        name: name.trim(),
        code: code.trim(),
        icon,
        description: description.trim(),
        labels: parseLabels(labels),
        applications
      });
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '保存项目失败。';
    } finally {
      saving = false;
    }
  }
</script>

<div
  class="project-overlay"
  role="presentation"
  on:click={(event) => event.target === event.currentTarget && onCancel()}
>
  <div
    class="project-dialog project-create-dialog"
    role="dialog"
    aria-modal="true"
    aria-labelledby="project-create-title"
    tabindex="-1"
  >
    <header class="project-dialog-header">
      <div>
        <h2 id="project-create-title">新增项目</h2>
        <p>先填写项目基本信息，再将应用加入项目。项目本身不区分来源类型。</p>
      </div>
      <div class="project-dialog-actions">
        <button class="secondary" type="button" on:click={onCancel}>取消</button
        ><button class="primary" type="button" disabled={saving} on:click={save}
          >{saving ? '创建中…' : '创建项目'}</button
        >
      </div>
    </header>
    {#if error}<div class="form-error" role="alert">{error}</div>{/if}
    <div class="project-dialog-content project-create-content">
      <section class="create-section">
        <div class="create-section-heading">
          <div>
            <h3>项目基本信息</h3>
            <p>名称、编号和说明用于识别项目。</p>
          </div>
          <FormField label="所属团队" required className="project-team-picker"
            ><DropdownSelect
              options={teamOptions}
              value={teamId}
              showIcons
              searchable
              placeholder="选择团队"
              ariaLabel="所属团队"
              on:change={(event) => (teamId = String(event.detail))}
            /></FormField
          >
        </div>
        <div class="project-identity-row">
          <div class="project-icon-code-fields">
            <FormField label="项目图标" className="project-icon-field"
              ><IconPicker
                value={icon}
                onSelect={(value) => (icon = value)}
                ariaLabel="选择项目图标"
              /></FormField
            ><FormField label="项目编号" required className="project-code-field"
              ><TextInput
                bind:value={code}
                required
                placeholder="ABC1234"
              /></FormField
            >
          </div>
          <FormField label="项目名称" required className="project-name-field"
            ><TextInput
              bind:value={name}
              required
              placeholder="支付结算平台"
            /></FormField
          >
        </div>
        <FormField label="项目标签" className="project-labels-field full-field"
          ><TextInput
            bind:value={labels}
            placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform"
            autocomplete="off"
          /></FormField
        ><FormField label="项目描述" className="full-field"
          ><TextArea
            bind:value={description}
            rows={3}
            placeholder="项目职责、边界和负责人说明"
          /></FormField
        >
      </section>
      <section class="create-section">
        <div class="create-section-heading">
          <div>
            <h3>项目应用</h3>
            <p>应用可在这里预先添加，也可以稍后从项目驾驶舱添加。</p>
          </div>
          <button
            class="secondary"
            type="button"
            on:click={() => (applicationWizardOpen = true)}
            ><Plus size={14} />添加应用</button
          >
        </div>
        {#if applications.length}<div class="project-draft-list">
            {#each applications as application, index}<article
                class="project-draft-row"
              >
                <EntityBrandIcon
                  kind="Application"
                  fallback={application.icon}
                  size={20}
                />
                <div>
                  <strong>{application.name}</strong><small
                    >{runtimeLabel(application.runtimeKind)} · {application
                      .instances.length} 个实例 · {application.code}</small
                  >
                </div>
                <button
                  class="icon-button"
                  type="button"
                  aria-label="删除待添加应用"
                  data-tooltip="删除"
                  on:click={() =>
                    (applications = applications.filter(
                      (_, itemIndex) => itemIndex !== index
                    ))}><Trash2 size={15} /></button
                >
              </article>{/each}
          </div>{:else}<div class="create-empty">
            <Plus size={18} /><span>尚未添加应用</span><button
              class="text-button"
              type="button"
              on:click={() => (applicationWizardOpen = true)}
              >从应用向导添加</button
            >
          </div>{/if}
      </section>
    </div>
  </div>
  {#if applicationWizardOpen}<ApplicationWizard
      {resources}
      onSave={addApplications}
      onCancel={() => (applicationWizardOpen = false)}
    />{/if}
</div>
