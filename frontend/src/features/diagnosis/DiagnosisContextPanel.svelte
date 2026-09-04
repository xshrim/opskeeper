<script lang="ts">
  import type {
    DiagnosisCausalChain,
    DiagnosisEvidence,
    DiagnosisSnapshot,
    Resource
  } from '../../lib/api';

  type DiagnosisEvidenceTimelineItem = {
    id: string;
    kind: 'turn' | 'analysis' | 'observation' | 'phase' | 'tool-group' | 'tool' | 'status';
    title: string;
    detail?: string;
    status?: string;
    tool?: string;
    input?: string;
    output?: string;
    duration?: string;
    children?: DiagnosisEvidenceTimelineItem[];
    evidenceIds?: string[];
  };

  export let diagnosisSnapshot: DiagnosisSnapshot | null = null;
  export let diagnosisTargets: Resource[] = [];
  export let diagnosisTargetIds: string[] = [];
  export let diagnosisContextTab: 'context' | 'evidence' = 'context';
  export let toggleDiagnosisContext: (resourceID: string) => void;
  export let resourceIcon: (kind: string) => string;
  export let resourceSchemaName: (kind: string) => string;
  export let scopeName: (id: string) => string;
  export let diagnosisActiveCausalChain: (snapshot: DiagnosisSnapshot | null) => DiagnosisCausalChain | null;
  export let diagnosisCausalNodes: (chain: DiagnosisCausalChain) => DiagnosisCausalChain['nodes'];
  export let diagnosisCausalEvidenceIDs: (chain: DiagnosisCausalChain, nodeID: string) => string[];
  export let scrollToDiagnosisEvidence: (id: string) => void;
  export let diagnosisEvidenceTimeline: (snapshot: DiagnosisSnapshot | null) => DiagnosisEvidenceTimelineItem[];
  export let diagnosisEvidenceSourceTools: (snapshot: DiagnosisSnapshot, evidence: DiagnosisEvidence) => string[];
  export let diagnosisEvidenceSummary: (evidence: DiagnosisEvidence) => string;
  export let diagnosisResourceName: (resourceID?: string) => string;
  export let formatDate: (value: string) => string;
</script>

<aside class="diagnosis-context-panel-f">
  <div class="diagnosis-panel-top">
    <div>
      <h2>诊断上下文</h2>
      <small>{diagnosisTargetIds.length} / {diagnosisTargets.length} 已加载</small>
    </div>
  </div>
  <div class="diagnosis-context-tabs">
    <button
      class:active={diagnosisContextTab === 'context'}
      on:click={() => (diagnosisContextTab = 'context')}
      >上下文</button
    ><button
      class:active={diagnosisContextTab === 'evidence'}
      on:click={() => (diagnosisContextTab = 'evidence')}
      >证据链</button
    >
  </div>
  {#if diagnosisContextTab === 'context'}
    <p class="diagnosis-context-note">
      <strong>上下文开关</strong><br />只把打开的资源提供给当前
      Agent；关闭不会删除资源，也不会影响权限。
    </p>
    <div class="diagnosis-resource-list-f">
      {#each diagnosisTargets as resource}
        <label
          class:selected={diagnosisTargetIds.includes(resource.id)}
          ><span class="diagnosis-resource-icon">{resourceIcon(resource.kind)}</span
          ><span
            ><strong>{resource.name}</strong><small
              >{resourceSchemaName(resource.kind)} · {scopeName(resource.scope_id)}</small
            ></span
          ><input
            type="checkbox"
            checked={diagnosisTargetIds.includes(resource.id)}
            on:change={() => toggleDiagnosisContext(resource.id)}
          /></label
        >
      {:else}
        <p class="diagnosis-empty">当前作用域没有可用于诊断的活动资源。</p>
      {/each}
    </div>
  {:else}
    <div class="diagnosis-evidence-pane">
      <section class="diagnosis-causal-chain-panel" aria-label="精选因果证据链">
        <div class="diagnosis-evidence-section-title">
          <span>精选因果证据链</span>
          {#if diagnosisActiveCausalChain(diagnosisSnapshot)}
            <small>当前结论 · 第 {diagnosisActiveCausalChain(diagnosisSnapshot)?.version} 轮</small>
          {/if}
        </div>
        {#if diagnosisActiveCausalChain(diagnosisSnapshot)}
          <p class="diagnosis-causal-summary">{diagnosisActiveCausalChain(diagnosisSnapshot)?.summary}</p>
          <div class="diagnosis-causal-path">
            {#each diagnosisCausalNodes(diagnosisActiveCausalChain(diagnosisSnapshot)!) as node, index (node.id)}
              {#if index > 0}<span class="diagnosis-causal-arrow" aria-hidden="true">↓</span>{/if}
              {@const evidenceIDs = diagnosisCausalEvidenceIDs(diagnosisActiveCausalChain(diagnosisSnapshot)!, node.id)}
              <article class="diagnosis-causal-node {node.kind}" class:unverified={node.status === 'unverified'}>
                <div>
                  <small>{node.kind === 'cause' ? '原因' : node.kind === 'mechanism' ? '作用机制' : node.kind === 'effect' ? '结果' : node.kind === 'exclusion' ? '已排除' : '待核验'}</small>
                  <strong>{node.statement}</strong>
                </div>
                <span class="diagnosis-causal-status {node.status}">{node.status === 'confirmed' ? '已确认' : node.status === 'likely' ? '较可能' : node.status === 'refuted' ? '已推翻' : '待核验'}</span>
                {#if evidenceIDs.length}
                  <div class="diagnosis-causal-references">
                    {#each evidenceIDs as id (id)}<button type="button" on:click={() => scrollToDiagnosisEvidence(id)}>E{ id.slice(0, 8) }</button>{/each}
                  </div>
                {/if}
              </article>
            {/each}
          </div>
        {:else}
          <p class="diagnosis-empty">完成一轮带工具证据的诊断后，这里会显示与当前结论直接相关的因果证据链。</p>
        {/if}
      </section>
      {#if diagnosisSnapshot?.causal_chains && diagnosisSnapshot.causal_chains.length > 1}
        <details class="diagnosis-causal-history">
          <summary>诊断演进 <small>{diagnosisSnapshot.causal_chains.length} 个版本</small></summary>
          {#each diagnosisSnapshot.causal_chains as chain (chain.id)}
            <div class:active={chain.id === diagnosisActiveCausalChain(diagnosisSnapshot)?.id}>
              <strong>第 {chain.version} 轮</strong><small>{chain.status === 'active' ? '当前结论' : chain.status === 'partial' ? '结论不完整' : '已被后续证据更新'}</small><p>{chain.summary}</p>
            </div>
          {/each}
        </details>
      {/if}
      <details class="diagnosis-evidence-record">
        <summary>完整取证记录 <small>分析观察、工具调用与环境反馈</small></summary>
        <div class="diagnosis-evidence-timeline">
          {#each diagnosisEvidenceTimeline(diagnosisSnapshot) as item (item.id)}
            {#if item.kind === 'turn'}
              <details class="diagnosis-evidence-chain">
                <summary>
                  <span class="diagnosis-evidence-chain-marker" aria-hidden="true"></span>
                  <span class="diagnosis-evidence-chain-title" title={item.title}>{item.title}</span>
                  <small>{item.detail}</small>
                  <span class="diagnosis-evidence-chevron">›</span>
                </summary>
                <div class="diagnosis-evidence-chain-body">
                  {#each item.children ?? [] as child (child.id)}
                    {#if child.kind === 'tool-group'}
                      {@const group = child}
                      <details class="diagnosis-evidence-event tool-event tool-group">
                        <summary>
                          <span class="diagnosis-evidence-marker tool"></span>
                          <span class="diagnosis-evidence-event-main"><strong>{group.title}</strong><small>{group.status}{#if group.detail} · {group.detail}{/if}</small></span>
                          <span class="diagnosis-evidence-chevron">›</span>
                        </summary>
                        <div class="diagnosis-evidence-tool-group">
                          {#each group.children ?? [] as tool (tool.id)}
                            <details class="diagnosis-evidence-event tool-event">
                              <summary>
                                <span class="diagnosis-evidence-marker tool"></span>
                                <span class="diagnosis-evidence-event-main"><strong>{tool.tool ?? tool.title}</strong><small>{tool.status}{#if tool.duration} · {tool.duration}{/if}{#if tool.evidenceIds?.length} · 已生成证据 {tool.evidenceIds.length} 条{/if}</small></span>
                                <span class="diagnosis-evidence-chevron">›</span>
                              </summary>
                              <div class="diagnosis-evidence-tool-detail">
                                {#if tool.evidenceIds?.length}<small class="diagnosis-evidence-links">关联证据：{tool.evidenceIds.join('、')}</small>{/if}
                                <div><small>入参 JSON</small><pre>{tool.input ?? '暂无入参'}</pre></div>
                                <div><small>出参 JSON</small><pre>{tool.output ?? '暂无出参'}</pre></div>
                              </div>
                            </details>
                          {/each}
                        </div>
                      </details>
                    {:else if child.kind === 'tool'}
                      <details class="diagnosis-evidence-event tool-event">
                        <summary>
                          <span class="diagnosis-evidence-marker tool"></span>
                          <span class="diagnosis-evidence-event-main"><strong>{child.tool}</strong><small>{child.status}{#if child.duration} · {child.duration}{/if}{#if child.evidenceIds?.length} · 已生成证据 {child.evidenceIds.length} 条{/if}</small></span>
                          <span class="diagnosis-evidence-chevron">›</span>
                        </summary>
                        <div class="diagnosis-evidence-tool-detail">
                          {#if child.evidenceIds?.length}<small class="diagnosis-evidence-links">关联证据：{child.evidenceIds.join('、')}</small>{/if}
                          <div><small>入参 JSON</small><pre>{child.input ?? '暂无入参'}</pre></div>
                          <div><small>出参 JSON</small><pre>{child.output ?? '暂无出参'}</pre></div>
                        </div>
                      </details>
                    {:else if child.kind === 'observation' && child.detail}
                      <details class="diagnosis-evidence-event tool-event observation-event">
                        <summary>
                          <span class="diagnosis-evidence-marker observation"></span>
                          <span class="diagnosis-evidence-event-main"><strong>{child.title}</strong><small>环境反馈</small></span>
                          <span class="diagnosis-evidence-chevron">›</span>
                        </summary>
                        <div class="diagnosis-evidence-tool-detail observation-detail"><pre>{child.detail}</pre></div>
                      </details>
                    {:else}
                      <div class="diagnosis-evidence-event">
                        <span class="diagnosis-evidence-marker {child.kind}"></span>
                        <div class="diagnosis-evidence-event-main"><strong>{child.title}</strong>{#if child.detail}<p>{child.detail}</p>{/if}</div>
                      </div>
                    {/if}
                  {/each}
                </div>
              </details>
            {:else}
              <p class="diagnosis-empty">开始诊断后，模型思考、工具调用和环境观察会按时间顺序显示在这里。</p>
            {/if}
          {/each}
        </div>
      </details>
      {#if diagnosisSnapshot?.evidence?.length}
        <div class="diagnosis-evidence-snapshots">
          <div class="diagnosis-evidence-section-title"><span>证据快照（原始结果）</span><small>可由上方因果链直接引用</small><b>{diagnosisSnapshot.evidence.length} 条</b></div>
          {#each diagnosisSnapshot.evidence as evidence (evidence.id)}
            {@const sourceTools = diagnosisEvidenceSourceTools(diagnosisSnapshot, evidence)}
            <details class="diagnosis-evidence-snapshot" id={`evidence-${evidence.id}`}>
              <summary>
                <span class="diagnosis-evidence-snapshot-marker" aria-hidden="true"></span>
                <span class="diagnosis-evidence-snapshot-main">
                  <strong>Evidence {evidence.id.slice(0, 8)}</strong>
                  <small>{evidence.capability || 'Connector 只读结果'} · {diagnosisResourceName(evidence.source_resource_id ?? evidence.target_resource_id)} · {formatDate(evidence.collected_at)} · {evidence.partial ? '部分结果' : '完整结果'}{#if evidence.untrusted} · 外部结果，需核验{/if}</small>
                </span>
                <span class="diagnosis-evidence-chevron">›</span>
              </summary>
              <div class="diagnosis-evidence-snapshot-detail">
                <p>{diagnosisEvidenceSummary(evidence)}</p>
                {#if evidence.window_start || evidence.window_end}<small class="diagnosis-evidence-window">时间窗口：{evidence.window_start ? formatDate(evidence.window_start) : '—'} 至 {evidence.window_end ? formatDate(evidence.window_end) : '—'}</small>{/if}
                <small class="diagnosis-evidence-source">来源工具：{sourceTools.length ? sourceTools.join('、') : '未关联具体调用'}</small>
                <small class="diagnosis-evidence-id">Evidence ID：{evidence.id}</small>
                <pre>{JSON.stringify(evidence.content, null, 2)}</pre>
              </div>
            </details>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</aside>
