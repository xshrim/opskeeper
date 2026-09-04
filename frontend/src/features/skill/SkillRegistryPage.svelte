<script lang="ts">
  import type { Resource, SkillVersion } from '../../lib/api';

  type SkillToolOption = {
    name: string;
    title: string;
    description: string;
  };

  export let resources: Resource[] = [];
  export let selectedSkillId = '';
  export let selectedVersionId = '';
  export let versions: SkillVersion[] = [];
  export let instruction = '';
  export let targetKinds = 'Application';
  export let selectedToolNames: string[] = [];
  export let inputSchema = '{"type":"object","additionalProperties":true}';
  export let outputSchema = '{"type":"object","additionalProperties":true}';
  export let toolOptions: SkillToolOption[] = [];
  export let busy = false;
  export let scopeName: (id: string) => string;
  export let onLoadVersions: () => void | Promise<void>;
  export let onSetDefault: () => void | Promise<void>;
  export let onPublish: () => void | Promise<void>;
  export let onCreate: () => void | Promise<void>;
  export let onToggleTool: (name: string) => void;
</script>

<section class="content-grid two-column ai-runtime">
  <section class="panel">
    <div class="panel-heading"><div><p class="eyebrow">SKILL REGISTRY</p><h2>Skill 版本</h2></div><span class="count">{versions.length}</span></div>
    <div class="stack-form compact-form">
      <label>Skill<select bind:value={selectedSkillId} required on:change={onLoadVersions}><option value="" disabled>选择 Skill 资源</option>{#each resources as item}<option value={item.id}>{item.name} · {scopeName(item.scope_id)}</option>{/each}</select></label>
      <label>版本<select bind:value={selectedVersionId}><option value="" disabled>选择版本</option>{#each versions as version}<option value={version.id}>v{version.version} · {version.status} · {version.risk_level}</option>{/each}</select></label>
      <div class="form-actions">
        <button class="secondary" type="button" on:click={onSetDefault} disabled={busy || !selectedVersionId}>设为当前 Scope 默认</button>
        <button class="primary" type="button" on:click={onPublish} disabled={busy || !selectedVersionId}>发布版本</button>
      </div>
    </div>
  </section>
  <section class="panel wide-panel">
    <div class="panel-heading"><div><p class="eyebrow">NEW VERSION</p><h2>创建 Skill 草稿</h2></div><span class="scope-type">不可变版本</span></div>
    <form class="stack-form" on:submit|preventDefault={onCreate}>
      <label>Agent Instruction<textarea bind:value={instruction} rows="5" required placeholder="明确目标、边界与输出 JSON 结构"></textarea></label>
      <label>适用资源类型<input bind:value={targetKinds} required placeholder="Application, Kubernetes" /></label>
      <fieldset class="skill-tool-picker">
        <legend>允许调用的 Connector 工具</legend>
        <p>仅已勾选的只读工具会暴露给模型；每个版本创建后不可修改。</p>
        <div class="skill-tool-grid">
          {#each toolOptions as tool}
            <label class:selected={selectedToolNames.includes(tool.name)}>
              <input type="checkbox" checked={selectedToolNames.includes(tool.name)} on:change={() => onToggleTool(tool.name)} />
              <span><strong>{tool.title}</strong><small>{tool.description}</small></span>
            </label>
          {/each}
        </div>
      </fieldset>
      <div class="form-row">
        <label>输入 Schema<textarea bind:value={inputSchema} rows="5" spellcheck="false"></textarea></label>
        <label>输出 Schema<textarea bind:value={outputSchema} rows="5" spellcheck="false"></textarea></label>
      </div>
      <button class="primary" disabled={busy || !selectedSkillId}>创建版本</button>
    </form>
  </section>
</section>
