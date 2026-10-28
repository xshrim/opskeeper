<script lang="ts">
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FieldLabel from '../../components/FieldLabel.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import { resourceCategoryIcon } from './resourceCatalog';
  export let category = '';
  export let subtype = '';
  export let name = '';
  export let labels = '';
  export let scopeId = '';
  export let scopeOptions: Array<{ id: string; type: string; name: string }> = [];
  export let categoryOptions: Record<string, string[]> = {};
  export let subtypeOptions: string[] = [];
  export let typeSelectionAttempted = false;
  export let basicConfigurationAttempted = false;
  export let editing = false;
  export let scopeSummary = '';
  export let onSelectCategory: (category: string) => void = () => {};
  export let onSelectSubtype: (subtype: string) => void = () => {};
  export let onSelectScope: (scopeId: string) => void = () => {};

  const subtypeIcons: Record<string, string> = {
    Direct: 'lucide:Plug',
    Agent: 'lucide:Bot',
    Generic: 'lucide:Package',
    Git: 'lucide:GitBranch',
    Bundle: 'lucide:Archive',
    StreamHTTP: 'lucide:Radio',
    SSE: 'lucide:Radio',
    指标: 'lucide:ChartNoAxesCombined',
    日志: 'lucide:FileText',
    链路: 'lucide:Route',
    告警: 'lucide:Bell'
  };
  $: categoryDropdownOptions = Object.keys(categoryOptions).filter((option) => option !== '全部').map((option) => ({ value: option, label: option, icon: resourceCategoryIcon(option) }));
  $: subtypeDropdownOptions = subtypeOptions.map((option) => ({ value: option, label: option, icon: subtypeIcons[option] ?? 'lucide:Package' }));
  $: scopeDropdownOptions = scopeOptions.map((scope) => ({
    value: scope.id,
    label: `${scope.type === 'platform' ? '平台' : scope.type === 'team' ? '团队' : '项目'} · ${scope.name}`,
    icon: scope.type === 'platform' ? 'lucide:PanelsTopLeft' : scope.type === 'team' ? 'lucide:UsersRound' : 'lucide:Boxes'
  }));

  function selectCategory(value: string) {
    typeSelectionAttempted = false;
    onSelectCategory(value);
  }

  function selectSubtype(value: string) {
    typeSelectionAttempted = false;
    onSelectSubtype(value);
  }
</script>

<div class="resource-type-selection">
  <div class="resource-basic-type-row">
    <label>
      <FieldLabel text="资源类型" required invalid={typeSelectionAttempted && !category} />
      <DropdownSelect bind:value={category} options={categoryDropdownOptions} showIcons disabled={editing} placeholder="请选择资源类型" ariaLabel="资源类型" invalid={typeSelectionAttempted && !category} invalidMode="bubble" invalidMessage="请选择资源类型" on:change={(event) => selectCategory(String(event.detail))} />
    </label>
    <label>
      <FieldLabel text="资源子类型" required invalid={typeSelectionAttempted && !subtype} />
      <DropdownSelect bind:value={subtype} options={subtypeDropdownOptions} showIcons disabled={!category || editing} placeholder="请选择资源子类型" ariaLabel="资源子类型" invalid={typeSelectionAttempted && !subtype} invalidMode="bubble" invalidMessage="请选择资源子类型" on:change={(event) => selectSubtype(String(event.detail))} />
    </label>
  </div>
  <div class="resource-basic-identity-row">
    <label class="resource-basic-name">
      <FieldLabel text="资源名称" required invalid={basicConfigurationAttempted && !name.trim()} /><TextInput bind:value={name} required invalid={basicConfigurationAttempted && !name.trim()} invalidMode="bubble" invalidMessage="请填写资源名称" placeholder="例如 production-resource" autocomplete="off" />
    </label>
    <label class="resource-basic-level"><FieldLabel text="资源级别" required invalid={basicConfigurationAttempted && !(editing ? scopeSummary : scopeId)} />{#if editing}<TextInput value={scopeSummary} readonly ariaLabel="资源级别" />{:else}<DropdownSelect value={scopeId} options={scopeDropdownOptions} showIcons placeholder="请选择资源级别" ariaLabel="资源级别" invalid={basicConfigurationAttempted && !scopeId} invalidMode="bubble" invalidMessage="请选择资源级别" on:change={(event) => onSelectScope(String(event.detail))} />{#if !scopeOptions.length}<small>当前工作区下没有可创建资源的范围。</small>{/if}{/if}</label>
  </div>
  <div class="resource-basic-labels"><FieldLabel text="资源标签" /><TextInput bind:value={labels} tagged placeholder="填写 key=value、key:value 或 key，按回车确认" autocomplete="off" /></div>
</div>
