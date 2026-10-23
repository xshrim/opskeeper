<script lang="ts">
  import { Plus, Trash2 } from 'lucide-svelte';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import ModalDialog from '../../components/ModalDialog.svelte';
  import { formatLabels, parseLabels } from '../../lib/labels';
  import type { Project, Resource, Team, User } from '../../lib/api';
  import ApplicationWizard from './ApplicationWizard.svelte';
  import { runtimeLabel, type ApplicationDraft } from './projectTypes';

  export let teams: Team[] = [];
  export let resources: Resource[] = [];
  export let users: User[] = [];
  export let initialProject: Project | null = null;
  export let defaultTeamId = '';
  export let onSave: (input: {
    teamId: string;
    name: string;
    code: string;
    icon: string;
    description: string;
    labels: Record<string, string>;
    importanceLevel: 'ITP1' | 'ITP2' | 'ITP3' | 'ITP4';
    technicalOwnerIds: string[];
    operationsOwnerIds: string[];
    applications: ApplicationDraft[];
  }) => Promise<void> | void;
  export let onCancel: () => void;

  let teamId = initialProject?.team_id || defaultTeamId || teams[0]?.id || '';
  let name = initialProject?.name || '';
  let code = initialProject?.code || generateProjectCode();
  let icon = initialProject?.icon || 'lucide:FolderKanban';
  let description = initialProject?.description || '';
  let labels = formatLabels(initialProject?.labels);
  let importanceLevel: '' | 'ITP1' | 'ITP2' | 'ITP3' | 'ITP4' = initialProject?.importance_level || '';
  let importanceLevelError = '';
  let technicalOwnerIds: string[] = initialProject?.technical_owner_ids || [];
  let operationsOwnerIds: string[] = initialProject?.operations_owner_ids || [];
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
  $: ownerOptions = users
    .filter((user) => user.status === 'active')
    .map((user) => ({
      value: user.id,
      label: user.display_name || user.username,
      description: user.display_name ? user.username : user.email || user.phone
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

  async function save() {
    error = '';
    importanceLevelError = '';
    if (!importanceLevel) {
      importanceLevelError = '请选择项目等级。';
      return;
    }
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
        importanceLevel,
        technicalOwnerIds,
        operationsOwnerIds,
        applications
      });
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '保存项目失败。';
    } finally {
      saving = false;
    }
  }
</script>

<ModalDialog dialogClass="project-dialog project-create-dialog" size="large" title={initialProject ? '编辑项目' : '新增项目'} description={initialProject ? '更新项目基本信息。' : '填写项目基本信息并新增应用。'} confirmLabel={initialProject ? '保存' : '创建'} busyLabel={initialProject ? '保存中…' : '创建中…'} busy={saving} message={error} onCancel={onCancel} onConfirm={save}>
  <div class="project-dialog-content project-create-content">
      <section class="create-section">
        <div class="create-section-heading">
          <div>
            <h3>项目基本信息</h3>
            <p>名称、编号和说明用于识别项目。</p>
          </div>
          {#if !initialProject}<FormField label="所属团队" required className="project-team-picker"
            ><DropdownSelect
              options={teamOptions}
              value={teamId}
              showIcons
              searchable
              placeholder="选择团队"
              ariaLabel="所属团队"
              on:change={(event) => (teamId = String(event.detail))}
            /></FormField>{/if}
        </div>
        <div class="project-identity-row">
          <div class="project-icon-code-fields">
            <FormField label="项目图标" className="project-icon-field">
              <IconPicker
                value={icon}
                onSelect={(value) => (icon = value)}
                ariaLabel="选择项目图标"
              />
            </FormField>
            <FormField label="项目等级" required invalid={Boolean(importanceLevelError)} error={importanceLevelError} className="project-importance-field">
              <DropdownSelect bind:value={importanceLevel} options={['ITP1', 'ITP2', 'ITP3', 'ITP4'].map((value) => ({ value, label: value }))} placeholder="选择项目等级" ariaLabel="项目等级" invalid={Boolean(importanceLevelError)} on:change={() => (importanceLevelError = '')} />
            </FormField>
            <FormField label="项目编号" required className="project-code-field">
              <TextInput
                bind:value={code}
                required
                placeholder="ABC1234"
              />
            </FormField>
          </div>
          <FormField label="项目名称" required className="project-name-field">
            <TextInput
              bind:value={name}
              required
              placeholder="支付结算平台"
            />
          </FormField>
        </div>
        <div class="owner-fields-row">
          <FormField label="技术负责人" asLabel={false}><DropdownSelect options={ownerOptions} bind:value={technicalOwnerIds} multiple taggedSelection reorderableSelection searchable placeholder="选择负责人(主备排序)" ariaLabel="选择技术负责人" /></FormField>
          <FormField label="运维负责人" asLabel={false}><DropdownSelect options={ownerOptions} bind:value={operationsOwnerIds} multiple taggedSelection reorderableSelection searchable placeholder="选择负责人(主备排序)" ariaLabel="选择运维负责人" /></FormField>
        </div>
        <FormField label="项目标签" className="project-labels-field full-field" asLabel={false}
          ><TextInput
            bind:value={labels}
            tagged
            placeholder="填写 key=value、key:value 或 key，按回车确认"
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
      {#if !initialProject}<section class="create-section">
        <div class="create-section-heading">
          <div>
            <h3>项目应用</h3>
            <p>应用可在这里预先新增，也可以稍后从项目驾驶舱新增。</p>
          </div>
          <button
            class="secondary"
            type="button"
            on:click={() => (applicationWizardOpen = true)}
            ><Plus size={14} />新增应用</button
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
                  class="icon-button delete-action"
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
              >从应用向导新增</button
            >
          </div>{/if}
      </section>{/if}
  </div>
  </ModalDialog>
  {#if applicationWizardOpen}<ApplicationWizard
      {resources}
      {users}
      onSave={addApplications}
      onCancel={() => (applicationWizardOpen = false)}
    />{/if}
