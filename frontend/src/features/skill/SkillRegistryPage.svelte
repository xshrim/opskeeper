<script lang="ts">
  import { api, ApiError, type Resource, type SkillVersion } from '../../lib/api';

  type SkillToolOption = {
    name: string;
    title: string;
    description: string;
    inputSchema: Record<string, unknown>;
  };

  const toolOptions: SkillToolOption[] = [
    {
      name: 'connector_kubernetes_read',
      title: 'Kubernetes 只读查询',
      description: '查询目标 Kubernetes 资源；不会修改集群。',
      inputSchema: {
        type: 'object',
        required: ['resource'],
        properties: {
          target_resource_id: { type: 'string' },
          resource: { type: 'string' },
          namespace: { type: 'string' },
          name: { type: 'string' },
          label_selector: { type: 'string' },
          limit: { type: 'integer' }
        },
        additionalProperties: false
      }
    },
    {
      name: 'connector_metrics_query',
      title: '指标查询',
      description: '通过已关联的 Prometheus 查询时间序列指标。',
      inputSchema: {
        type: 'object',
        required: ['query', 'start', 'end', 'step_seconds'],
        properties: {
          target_resource_id: { type: 'string' },
          query: { type: 'string' },
          start: { type: 'string', format: 'date-time' },
          end: { type: 'string', format: 'date-time' },
          step_seconds: { type: 'integer' }
        },
        additionalProperties: false
      }
    },
    {
      name: 'connector_logs_query',
      title: '日志查询',
      description: '通过已关联的 Loki 查询限定范围内的日志。',
      inputSchema: {
        type: 'object',
        required: ['query', 'start', 'end', 'limit'],
        properties: {
          target_resource_id: { type: 'string' },
          query: { type: 'string' },
          start: { type: 'string', format: 'date-time' },
          end: { type: 'string', format: 'date-time' },
          limit: { type: 'integer' },
          keyword: { type: 'string', description: '多个关键字可用 &（同时匹配）或 |（任一匹配）分隔。' }
        },
        additionalProperties: false
      }
    },
    {
      name: 'connector_traces_query',
      title: '链路查询',
      description: '通过已关联的追踪平台查询服务调用链。',
      inputSchema: {
        type: 'object',
        required: ['service', 'start', 'end', 'limit'],
        properties: {
          target_resource_id: { type: 'string' },
          service: { type: 'string' },
          operation: { type: 'string' },
          start: { type: 'string', format: 'date-time' },
          end: { type: 'string', format: 'date-time' },
          limit: { type: 'integer' }
        },
        additionalProperties: false
      }
    },
    {
      name: 'connector_alerts_get',
      title: '告警查询',
      description: '查询目标关联监控平台中的当前告警。',
      inputSchema: {
        type: 'object',
        properties: {
          target_resource_id: { type: 'string' },
          active_only: { type: 'boolean' }
        },
        additionalProperties: false
      }
    }
  ];

  export let resources: Resource[] = [];
  export let scopeId = '';
  export let scopeName: (id: string) => string;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;
  let selectedSkillId = ''; let selectedVersionId = ''; let versions: SkillVersion[] = [];
  let instruction = ''; let targetKinds = 'Application'; let selectedToolNames: string[] = [];
  let inputSchema = '{"type":"object","additionalProperties":true}'; let outputSchema = '{"type":"object","additionalProperties":true}'; let busy = false;
  $: if (!selectedSkillId && resources[0]) { selectedSkillId = resources[0].id; void loadVersions(); }
  function describeError(error: unknown, fallback: string) { if (error instanceof ApiError) { if (error.status === 403) return '当前账号没有执行此操作的权限。'; if (error.status === 401) return '会话已过期，请重新登录。'; return error.message || fallback; } return error instanceof Error ? error.message || fallback : fallback; }
  async function loadVersions() { if (!selectedSkillId) return; try { versions = await api.skillVersions(selectedSkillId); selectedVersionId = versions.find((item) => item.status === 'published')?.id || versions[0]?.id || ''; } catch (error) { onError(describeError(error, 'Skill 版本加载失败')); } }
  function toggleTool(name: string) { selectedToolNames = selectedToolNames.includes(name) ? selectedToolNames.filter((item) => item !== name) : [...selectedToolNames, name]; }
  async function createVersion() { if (!selectedSkillId) return; busy = true; onError(''); try { const version = await api.createSkillVersion(selectedSkillId, { manifest: { name: resources.find((item) => item.id === selectedSkillId)?.name || 'Skill', description: '由 OpsKeeper 管理的声明式 Skill', instruction, target_kinds: targetKinds.split(',').map((item) => item.trim()).filter(Boolean) }, input_schema: JSON.parse(inputSchema), output_schema: JSON.parse(outputSchema), tools: toolOptions.filter((tool) => selectedToolNames.includes(tool.name)).map((tool) => ({ name: tool.name, description: tool.description, input_schema: tool.inputSchema })), risk_level: 'read_only' }); versions = [version, ...versions]; selectedVersionId = version.id; onNotice(`Skill v${version.version} 草稿已创建`); } catch (error) { onError(describeError(error, '创建 Skill 版本失败')); } finally { busy = false; } }
  async function publishVersion() { if (!selectedSkillId || !selectedVersionId) return; busy = true; onError(''); try { await api.publishSkillVersion(selectedSkillId, selectedVersionId); await loadVersions(); onNotice('Skill 版本已发布'); } catch (error) { onError(describeError(error, '发布 Skill 版本失败')); } finally { busy = false; } }
  async function setDefault() { if (!scopeId || !selectedSkillId || !selectedVersionId) return; busy = true; onError(''); try { await api.setSkillDefault({ scope_id: scopeId, skill_resource_id: selectedSkillId, skill_version_id: selectedVersionId }); onNotice('默认 Skill 已更新'); } catch (error) { onError(describeError(error, '设置默认 Skill 失败')); } finally { busy = false; } }
</script>

<section class="content-grid two-column">
  <section class="panel">
    <div class="panel-heading"><div><p class="eyebrow">SKILL REGISTRY</p><h2>Skill 版本</h2></div><span class="count">{versions.length}</span></div>
    <div class="stack-form compact-form">
      <label>Skill<select bind:value={selectedSkillId} required on:change={loadVersions}><option value="" disabled>选择 Skill 资源</option>{#each resources as item}<option value={item.id}>{item.name} · {scopeName(item.scope_id)}</option>{/each}</select></label>
      <label>版本<select bind:value={selectedVersionId}><option value="" disabled>选择版本</option>{#each versions as version}<option value={version.id}>v{version.version} · {version.status} · {version.risk_level}</option>{/each}</select></label>
      <div class="form-actions">
        <button class="secondary" type="button" on:click={setDefault} disabled={busy || !selectedVersionId}>设为当前 Scope 默认</button>
        <button class="primary" type="button" on:click={publishVersion} disabled={busy || !selectedVersionId}>发布版本</button>
      </div>
    </div>
  </section>
  <section class="panel wide-panel">
    <div class="panel-heading"><div><p class="eyebrow">NEW VERSION</p><h2>创建 Skill 草稿</h2></div><span class="scope-type">不可变版本</span></div>
    <form class="stack-form" on:submit|preventDefault={createVersion}>
      <label>Agent Instruction<textarea bind:value={instruction} rows="5" required placeholder="明确目标、边界与输出 JSON 结构"></textarea></label>
      <label>适用资源类型<input bind:value={targetKinds} required placeholder="Application, Kubernetes" /></label>
      <fieldset class="skill-tool-picker">
        <legend>允许调用的 Connector 工具</legend>
        <p>仅已勾选的只读工具会暴露给模型；每个版本创建后不可修改。</p>
        <div class="skill-tool-grid">
          {#each toolOptions as tool}
            <label class:selected={selectedToolNames.includes(tool.name)}>
              <input type="checkbox" checked={selectedToolNames.includes(tool.name)} on:change={() => toggleTool(tool.name)} />
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
