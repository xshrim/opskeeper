<script lang="ts">
  import Tabs from '../../components/Tabs.svelte';
  import FormActions from '../../components/FormActions.svelte';
  import {
    AlertTriangle,
    ChevronRight,
    Clock3,
    FileText,
    Plus,
    RefreshCw,
    Route,
    Save,
    Send,
    Settings2,
    ShieldCheck
  } from 'lucide-svelte';
  import DropdownSelect from '../../components/DropdownSelect.svelte';
  import FormField from '../../components/FormField.svelte';
  import NotificationChannelFields from './NotificationChannelFields.svelte';
  import SearchInput from '../../components/SearchInput.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import {
    api,
    ApiError,
    type InspectionPolicy,
    type NotificationChannel,
    type NotificationDelivery,
    type NotificationProvider,
    type NotificationRule,
    type NotificationTemplate,
    type NotificationTemplateVersion
  } from '../../lib/api';

  type Tab = 'overview' | 'channels' | 'templates' | 'rules' | 'deliveries';

  export let scopeId = '';
  export let onNotice: (message: string) => void = () => {};
  export let onError: (message: string) => void = () => {};

  let activeTab: Tab = 'overview';
  let channels: NotificationChannel[] = [];
  let providers: NotificationProvider[] = [];
  let templates: NotificationTemplate[] = [];
  let rules: NotificationRule[] = [];
  let deliveries: NotificationDelivery[] = [];
  let policies: InspectionPolicy[] = [];
  let loadedScopeId = '';
  let loading = false;
  let refreshing = false;
  let loadError = '';
  let search = '';
  let deliveryStatus = '';
  let selectedTemplateId = '';
  let selectedPolicyId = '';
  let policyRuleIds: string[] = [];
  let policyRulesLoading = false;
  let busyAction = '';

  let channelName = '';
  let channelKind = '';
  let channelRateLimit = 30;
  let channelShare = false;
  let channelConfig: Record<string, string> = {};

  let templateName = '';
  let templateFormat: 'text' | 'markdown' | 'json' = 'markdown';
  let templateTitle = '【{{.severity}}】{{.rule_name}}';
  let templateBody = '事件：{{.event_type}}\n摘要：{{.finding_summary}}';
  let templateShare = false;

  let ruleName = '';
  let ruleEvent = 'finding.opened';
  let ruleSeverity = 'warning';
  let ruleChannelId = '';
  let ruleTemplateVersionId = '';
  let ruleShare = false;
  let ruleStatus = 'active';

  $: if (scopeId && scopeId !== loadedScopeId) {
    loadedScopeId = scopeId;
    selectedPolicyId = '';
    policyRuleIds = [];
    void loadData(scopeId);
  }

  $: selectedTemplate =
    templates.find((template) => template.id === selectedTemplateId) ??
    templates[0];
  $: overviewTemplateVersion = selectedTemplate
    ? latestPublishedVersion(selectedTemplate)
    : undefined;
  $: filteredChannels = filterItems(
    channels,
    search,
    (item) => item.name,
    (item) => item.kind
  );
  $: filteredTemplates = filterItems(templates, search, (item) => item.name);
  $: filteredRules = filterItems(
    rules,
    search,
    (item) => item.name,
    (item) => item.status
  );
  $: filteredDeliveries = deliveries.filter((delivery) => {
    if (deliveryStatus && delivery.status !== deliveryStatus) return false;
    if (!search.trim()) return true;
    const query = search.trim().toLocaleLowerCase();
    return `${delivery.event_type} ${delivery.status} ${delivery.id}`
      .toLocaleLowerCase()
      .includes(query);
  });
  $: activeChannels = channels.filter(
    (channel) => channel.status === 'active'
  ).length;
  $: activeRules = rules.filter((rule) => rule.status === 'active').length;
  $: pendingDeliveries = deliveries.filter((delivery) =>
    ['queued', 'delivering', 'retrying'].includes(delivery.status)
  ).length;
  $: deadLetters = deliveries.filter(
    (delivery) => delivery.status === 'dead_letter'
  ).length;
  $: firstRoute = rules.find(
    (rule) => rule.status === 'active' && rule.routes.length > 0
  )?.routes[0];
  $: routeRule = firstRoute
    ? rules.find((rule) =>
        rule.routes.some(
          (route) =>
            route.channel_id === firstRoute?.channel_id &&
            route.template_version_id === firstRoute?.template_version_id
        )
      )
    : undefined;
  $: routeChannel = firstRoute
    ? channels.find((channel) => channel.id === firstRoute.channel_id)
    : undefined;
  $: routeTemplate = firstRoute
    ? findTemplateVersion(firstRoute.template_version_id)
    : undefined;

  function filterItems<T>(
    items: T[],
    query: string,
    ...getters: Array<(item: T) => string | undefined>
  ) {
    const normalized = query.trim().toLocaleLowerCase();
    if (!normalized) return items;
    return items.filter((item) =>
      getters.some((getter) =>
        getter(item)?.toLocaleLowerCase().includes(normalized)
      )
    );
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前范围没有通知查看权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      return error.message || fallback;
    }
    if (error instanceof Error) return error.message || fallback;
    return fallback;
  }

  async function loadData(targetScopeId = scopeId, silent = false) {
    if (!targetScopeId) return;
    if (silent) refreshing = true;
    else loading = true;
    loadError = '';
    const results = await Promise.allSettled([
      api.notificationChannels(targetScopeId),
      api.notificationProviders(targetScopeId),
      api.notificationTemplates(targetScopeId),
      api.notificationRules(targetScopeId),
      api.notificationDeliveries(targetScopeId, { limit: 50 }),
      api.inspectionPolicies(targetScopeId)
    ]);
    if (targetScopeId !== scopeId) return;
    const [
      channelsResult,
      providersResult,
      templatesResult,
      rulesResult,
      deliveriesResult,
      policiesResult
    ] = results;
    if (channelsResult.status === 'fulfilled') channels = channelsResult.value;
    if (providersResult.status === 'fulfilled') {
      providers = providersResult.value;
      if (!channelKind)
        channelKind =
          providers.find((provider) => provider.supported)?.kind ?? '';
    }
    if (templatesResult.status === 'fulfilled')
      templates = templatesResult.value;
    if (rulesResult.status === 'fulfilled') rules = rulesResult.value;
    if (deliveriesResult.status === 'fulfilled')
      deliveries = deliveriesResult.value;
    if (policiesResult.status === 'fulfilled') policies = policiesResult.value;
    const failedReads = results
      .slice(0, 5)
      .filter((result) => result.status === 'rejected');
    if (failedReads.length === 5) {
      loadError = describeError(
        (failedReads[0] as PromiseRejectedResult).reason,
        '通知数据加载失败'
      );
    } else if (failedReads.length > 0) {
      loadError = '部分通知数据暂时不可用，已显示可见内容。';
    }
    loading = false;
    refreshing = false;
  }

  function statusLabel(status: string) {
    return (
      (
        {
          active: '启用',
          disabled: '停用',
          succeeded: '已送达',
          queued: '待投递',
          delivering: '发送中',
          retrying: '重试中',
          dead_letter: '死信',
          failed: '失败',
          draft: '草稿',
          published: '已发布'
        } as Record<string, string>
      )[status] ?? status
    );
  }

  function statusTone(status: string) {
    if (['active', 'succeeded', 'published'].includes(status)) return 'success';
    if (['queued', 'delivering', 'retrying', 'draft'].includes(status))
      return 'warning';
    if (['dead_letter', 'failed'].includes(status)) return 'danger';
    return 'neutral';
  }

  function formatTime(value?: string) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '—';
    return date.toLocaleString([], {
      month: 'numeric',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  function durationSeconds(value: number | undefined) {
    if (!value) return 0;
    return value > 1_000_000
      ? Math.round(value / 1_000_000_000)
      : Math.round(value);
  }

  function findTemplateVersion(versionId: string) {
    for (const template of templates) {
      const version = template.versions.find((item) => item.id === versionId);
      if (version) return { template, version };
    }
    return undefined;
  }

  function latestPublishedVersion(template: NotificationTemplate | undefined) {
    return (
      template?.versions.find((version) => version.status === 'published') ??
      template?.versions[0]
    );
  }

  async function createChannel() {
    busyAction = 'channel-create';
    try {
      const created = await api.createConfiguredNotificationChannel({
        scope_id: scopeId,
        name: channelName.trim(),
        kind: channelKind,
        config: channelConfig,
        status: 'active',
        rate_limit_per_minute: channelRateLimit,
        share_with_children: channelShare
      });
      channels = [created, ...channels];
      channelName = '';
      channelConfig = {};
      onNotice('通知渠道已创建。');
    } catch (error) {
      onError(describeError(error, '创建通知渠道失败'));
    } finally {
      busyAction = '';
    }
  }

  async function testChannel(channel: NotificationChannel) {
    busyAction = `channel-test-${channel.id}`;
    try {
      await api.testConfiguredNotificationChannel(channel.id, scopeId);
      onNotice(`已向「${channel.name}」发送测试通知。`);
    } catch (error) {
      onError(describeError(error, '测试通知渠道失败'));
    } finally {
      busyAction = '';
    }
  }

  async function createTemplate() {
    busyAction = 'template-create';
    try {
      const created = await api.createNotificationTemplate({
        scope_id: scopeId,
        name: templateName.trim(),
        format: templateFormat,
        title_template: templateTitle,
        body_template: templateBody,
        variables: templateVariables(templateTitle, templateBody),
        share_with_children: templateShare
      });
      templates = [created, ...templates];
      selectedTemplateId = created.id;
      templateName = '';
      onNotice('通知模板已创建。');
    } catch (error) {
      onError(describeError(error, '创建通知模板失败'));
    } finally {
      busyAction = '';
    }
  }

  function templateVariables(title: string, body: string) {
    const names = new Set<string>();
    for (const source of [title, body]) {
      for (const match of source.matchAll(/{{\.([a-z_]+)}}/g))
        names.add(match[1]);
    }
    return Array.from(names).map((name) => ({
      name,
      type: 'string',
      required: false
    }));
  }

  async function previewTemplate(
    template: NotificationTemplate,
    version: NotificationTemplateVersion
  ) {
    busyAction = `template-preview-${template.id}`;
    try {
      const preview = await api.previewNotificationTemplate(
        template.id,
        version.id,
        {
          scope_id: scopeId,
          values: {
            event_type: 'finding.opened',
            severity: 'high',
            rule_name: '支付接口异常',
            finding_summary: '连接池等待超过阈值',
            resource_name: 'pay-api-03',
            policy_name: '核心链路巡检'
          }
        }
      );
      onNotice(preview.title ? `预览：${preview.title}` : '模板预览已生成。');
    } catch (error) {
      onError(describeError(error, '生成模板预览失败'));
    } finally {
      busyAction = '';
    }
  }

  async function publishTemplate(
    template: NotificationTemplate,
    version: NotificationTemplateVersion
  ) {
    busyAction = `template-publish-${version.id}`;
    try {
      const published = await api.publishNotificationTemplate(
        template.id,
        version.id,
        scopeId
      );
      templates = templates.map((item) =>
        item.id === template.id
          ? {
              ...item,
              versions: item.versions.map((entry) =>
                entry.id === published.id ? published : entry
              )
            }
          : item
      );
      onNotice(`模板「${template.name}」v${version.version} 已发布。`);
    } catch (error) {
      onError(describeError(error, '发布模板失败'));
    } finally {
      busyAction = '';
    }
  }

  async function createRule() {
    busyAction = 'rule-create';
    const route =
      ruleChannelId && ruleTemplateVersionId
        ? [
            {
              channel_id: ruleChannelId,
              template_version_id: ruleTemplateVersionId
            }
          ]
        : [];
    try {
      const created = await api.createNotificationRule({
        scope_id: scopeId,
        name: ruleName.trim(),
        status: route.length > 0 ? ruleStatus : 'disabled',
        event_types: [ruleEvent],
        minimum_severity: ruleSeverity,
        filters: {},
        cooldown_seconds: 120,
        aggregation_seconds: 120,
        max_batch_size: 20,
        silence: {},
        routes: route,
        share_with_children: ruleShare
      });
      rules = [created, ...rules];
      ruleName = '';
      onNotice('通知规则已创建。');
    } catch (error) {
      onError(describeError(error, '创建通知规则失败'));
    } finally {
      busyAction = '';
    }
  }

  async function testRule(rule: NotificationRule) {
    busyAction = `rule-test-${rule.id}`;
    try {
      const result = await api.testNotificationRule(rule.id, scopeId);
      onNotice(`规则测试已提交，覆盖 ${result.routes_tested} 条路由。`);
    } catch (error) {
      onError(describeError(error, '测试通知规则失败'));
    } finally {
      busyAction = '';
    }
  }

  async function retryDelivery(delivery: NotificationDelivery) {
    busyAction = `delivery-retry-${delivery.id}`;
    try {
      await api.retryNotificationDelivery(delivery.id, scopeId);
      deliveries = deliveries.map((item) =>
        item.id === delivery.id ? { ...item, status: 'queued' } : item
      );
      onNotice('投递已重新加入队列。');
    } catch (error) {
      onError(describeError(error, '重试投递失败'));
    } finally {
      busyAction = '';
    }
  }

  async function loadPolicyRules(policyId: string) {
    selectedPolicyId = policyId;
    policyRuleIds = [];
    if (!policyId) return;
    policyRulesLoading = true;
    try {
      policyRuleIds = (
        await api.inspectionPolicyNotificationRules(policyId, scopeId)
      ).rule_ids;
    } catch (error) {
      onError(describeError(error, '读取策略通知规则失败'));
    } finally {
      policyRulesLoading = false;
    }
  }

  async function savePolicyRules() {
    if (!selectedPolicyId) return;
    busyAction = 'policy-rules';
    try {
      await api.setInspectionPolicyNotificationRules(selectedPolicyId, {
        scope_id: scopeId,
        rule_ids: policyRuleIds
      });
      onNotice('巡检策略的通知规则关联已更新。');
    } catch (error) {
      onError(describeError(error, '更新策略通知规则失败'));
    } finally {
      busyAction = '';
    }
  }

  function toggleRuleId(id: string) {
    policyRuleIds = policyRuleIds.includes(id)
      ? policyRuleIds.filter((item) => item !== id)
      : [...policyRuleIds, id];
  }

  function channelNameForId(channelId: string) {
    return (
      channels.find((channel) => channel.id === channelId)?.name ?? '未找到渠道'
    );
  }
</script>

<section class="notification-page">
  {#if loading}
    <div class="notification-state">
      <RefreshCw class="spin" size={22} /><strong>正在读取通知状态</strong><span
        >正在加载当前范围的渠道、规则和投递记录。</span
      >
    </div>
  {:else if loadError && channels.length === 0 && templates.length === 0 && rules.length === 0 && deliveries.length === 0}
    <div class="notification-state notification-state-error">
      <AlertTriangle size={22} /><strong>无法读取通知数据</strong><span
        >{loadError}</span
      ><button
        class="secondary"
        type="button"
        on:click={() => loadData(scopeId)}>重试加载</button
      >
    </div>
  {:else}
    <div class="notification-metrics" aria-label="通知状态指标">
      <div class="notification-metric">
        <div>
          <span>活跃渠道</span><strong
            >{activeChannels}<small> / {channels.length}</small></strong
          >
        </div>
        <ShieldCheck size={19} />
      </div>
      <div class="notification-metric">
        <div><span>启用规则</span><strong>{activeRules}</strong></div>
        <Route size={19} />
      </div>
      <div class="notification-metric warning">
        <div><span>待投递</span><strong>{pendingDeliveries}</strong></div>
        <Clock3 size={19} />
      </div>
      <div class="notification-metric danger">
        <div><span>死信</span><strong>{deadLetters}</strong></div>
        <AlertTriangle size={19} />
      </div>
    </div>

    {#if loadError}<div class="notification-inline-warning">
        <AlertTriangle size={14} />{loadError}
      </div>{/if}

    <div class="notification-tab-bar">
      <Tabs
        className="notification-tabs"
        ariaLabel="通知管理区域"
        value={activeTab}
        items={[
          { value: 'overview', label: '概览' },
          { value: 'channels', label: '渠道', count: channels.length },
          { value: 'templates', label: '模板', count: templates.length },
          { value: 'rules', label: '规则', count: rules.length },
          { value: 'deliveries', label: '投递', count: deliveries.length }
        ]}
        onChange={(next) => { activeTab = next as Tab; search = ''; }}
      />
      <div class="notification-tab-actions">
        <button
          class="secondary notification-refresh"
          type="button"
          disabled={refreshing}
          on:click={() => loadData(scopeId, true)}
          title="刷新通知数据"
          aria-label="刷新通知数据"
        >
          <RefreshCw size={15} class={refreshing ? 'spin' : ''} />刷新
        </button>
        <button
          class="primary"
          type="button"
          on:click={() => {
            activeTab = 'rules';
          }}><Plus size={15} />新建规则</button
        >
      </div>
    </div>

    {#if activeTab === 'overview'}
      <div class="notification-overview-grid">
        <div class="notification-overview-left">
          <section class="panel notification-panel">
            <div class="notification-panel-heading">
              <div>
                <h3>渠道状态</h3>
                <p>通道健康与限流</p>
              </div>
              <button
                class="secondary compact"
                type="button"
                on:click={() => (activeTab = 'channels')}
                >管理渠道<ChevronRight size={13} /></button
              >
            </div>
            <div class="notification-table-scroll">
              <table class="notification-table">
                <thead
                  ><tr
                    ><th>渠道</th><th>近 1 小时</th><th>状态</th><th>范围</th
                    ></tr
                  ></thead
                ><tbody>
                  {#each channels.slice(0, 5) as channel}
                    <tr
                      ><td
                        ><strong>{channel.name}</strong><small
                          >{channel.kind} · {channel.rate_limit_per_minute} / 分钟</small
                        ></td
                      ><td>—</td><td
                        ><span
                          class={`notification-status ${statusTone(channel.status)}`}
                          >{statusLabel(channel.status)}</span
                        ></td
                      ><td
                        >{channel.inherited
                          ? '继承'
                          : channel.share_with_children
                            ? '共享'
                            : '当前范围'}</td
                      ></tr
                    >
                  {:else}<tr
                      ><td colspan="4" class="notification-table-empty"
                        >当前范围还没有通知渠道。</td
                      ></tr
                    >{/each}
                </tbody>
              </table>
            </div>
          </section>

          <section class="panel notification-panel">
            <div class="notification-panel-heading">
              <div>
                <h3>规则路由</h3>
                <p>规则 → 渠道 → 已发布模板</p>
              </div>
              <button
                class="secondary compact"
                type="button"
                on:click={() => (activeTab = 'rules')}
                >查看规则<ChevronRight size={13} /></button
              >
            </div>
            {#if routeRule && routeChannel && routeTemplate}
              <div class="notification-route">
                <div class="notification-route-node">
                  <strong>{routeRule.name}</strong><small
                    >{routeRule.minimum_severity} · {routeRule
                      .event_types[0]}</small
                  >
                </div>
                <ChevronRight size={16} />
                <div class="notification-route-node">
                  <strong>{routeChannel.name}</strong><small
                    >{routeChannel.kind} · {routeChannel.inherited
                      ? '共享渠道'
                      : '当前范围'}</small
                  >
                </div>
                <ChevronRight size={16} />
                <div class="notification-route-node">
                  <strong
                    >{routeTemplate.template.name} v{routeTemplate.version
                      .version}</strong
                  ><small
                    >{routeTemplate.version.status === 'published'
                      ? '已发布'
                      : '草稿'}</small
                  >
                </div>
              </div>
            {:else}<div class="notification-empty-inline">
                <Route size={18} /><span>还没有完整的启用路由。</span><button
                  class="text-button"
                  type="button"
                  on:click={() => (activeTab = 'rules')}>去配置</button
                >
              </div>{/if}
          </section>

          <section class="panel notification-panel">
            <div class="notification-panel-heading">
              <div>
                <h3>通知模板预览</h3>
                <p>使用当前范围可见的已发布版本</p>
              </div>
              {#if selectedTemplate}<span class="notification-chip"
                  >{selectedTemplate.name}</span
                >{/if}
            </div>
            {#if selectedTemplate && overviewTemplateVersion}
              <div class="notification-preview">
                <strong
                  >{overviewTemplateVersion.draft.title_template ||
                    '无标题'}</strong
                >
                <pre>{overviewTemplateVersion.draft.body_template ||
                    '无正文'}</pre>
              </div>
              <div class="notification-panel-foot">
                <small
                  >v{overviewTemplateVersion.version} · {statusLabel(
                    overviewTemplateVersion.status
                  )}</small
                ><button
                  class="secondary compact"
                  type="button"
                  disabled={busyAction !== ''}
                  on:click={() =>
                    previewTemplate(selectedTemplate, overviewTemplateVersion)}
                  >预览模板</button
                >
              </div>
            {:else}<div class="notification-empty-inline">
                <FileText size={18} /><span>当前范围还没有模板。</span><button
                  class="text-button"
                  type="button"
                  on:click={() => (activeTab = 'templates')}>新建模板</button
                >
              </div>{/if}
          </section>
        </div>

        <div class="notification-overview-right">
          <form
            class="panel notification-panel notification-channel-form"
            on:submit|preventDefault={createChannel}
          >
            <div class="notification-panel-heading">
              <div>
                <h3>快速新建渠道</h3>
                <p>新增渠道默认仅当前范围可用。</p>
              </div>
              <Settings2 size={17} />
            </div>
            <NotificationChannelFields {providers} bind:channelName bind:channelKind bind:channelRateLimit bind:channelShare bind:channelConfig />
            <FormActions className="notification-form-actions">
              <small>凭据只返回脱敏状态。</small><button
                class="primary compact"
                type="submit"
                disabled={busyAction === 'channel-create' || !channelKind}
                ><Save size={13} />{busyAction === 'channel-create'
                  ? '创建中…'
                  : '创建渠道'}</button
              >
            </FormActions>
          </form>

          <section class="panel notification-panel">
            <div class="notification-panel-heading">
              <div>
                <h3>最新投递</h3>
                <p>最近 50 条记录</p>
              </div>
              <button
                class="notification-chip button-link"
                type="button"
                on:click={() => (activeTab = 'deliveries')}>全部记录</button
              >
            </div>
            <div class="notification-delivery-list">
              {#each deliveries.slice(0, 4) as delivery}<div
                  class="notification-delivery-row"
                >
                  <div>
                    <strong>{delivery.event_type}</strong><small
                      >{formatTime(delivery.created_at)} · 第 {delivery.attempt} 次</small
                    >
                  </div>
                  <span
                    class={`notification-status ${statusTone(delivery.status)}`}
                    >{statusLabel(delivery.status)}</span
                  >
                </div>{:else}<div class="notification-empty-inline">
                  <Send size={18} /><span
                    >关联规则并产生事件后，投递记录会显示在这里。</span
                  >
                </div>{/each}
            </div>
          </section>
        </div>
      </div>

      <div class="notification-state-strip">
        <div>
          <strong>暂无投递</strong><span
            >没有记录时保持轻量提示，不影响查看渠道与规则。</span
          >
        </div>
        <div class="danger">
          <strong>队列异常</strong><span
            >读取失败时可在上方刷新，失败不会隐藏已加载内容。</span
          >
        </div>
        <div class="neutral">
          <strong>权限边界</strong><span
            >写入、测试和重试仍由后端权限重新校验。</span
          >
        </div>
      </div>
    {:else if activeTab === 'channels'}
      <div class="notification-section-grid">
        <section class="panel notification-panel notification-list-panel">
          <div class="notification-panel-heading">
            <div>
              <h3>通知渠道</h3>
              <p>当前范围可见的渠道与共享状态</p>
            </div>
            <SearchInput
              bind:value={search}
              width="220px"
              height="34px"
              placeholder="搜索渠道"
              ariaLabel="搜索渠道"
            />
          </div>
          <div class="notification-table-scroll">
            <table class="notification-table">
              <thead
                ><tr
                  ><th>渠道</th><th>类型</th><th>状态</th><th>范围</th><th
                    >操作</th
                  ></tr
                ></thead
              ><tbody
                >{#each filteredChannels as channel}<tr
                    ><td
                      ><strong>{channel.name}</strong><small
                        >配置 v{channel.config_version ?? '—'}</small
                      ></td
                    ><td>{channel.kind}</td><td
                      ><span
                        class={`notification-status ${statusTone(channel.status)}`}
                        >{statusLabel(channel.status)}</span
                      ></td
                    ><td
                      >{channel.inherited
                        ? '继承'
                        : channel.share_with_children
                          ? '对子 Scope 共享'
                          : '当前范围'}</td
                    ><td
                      ><button
                        class="text-button"
                        type="button"
                        disabled={busyAction !== '' || channel.inherited}
                        on:click={() => testChannel(channel)}
                        >{busyAction === `channel-test-${channel.id}`
                          ? '测试中…'
                          : '测试'}</button
                      ></td
                    ></tr
                  >{:else}<tr
                    ><td colspan="5" class="notification-table-empty"
                      >没有匹配的渠道。</td
                    ></tr
                  >{/each}</tbody
              >
            </table>
          </div>
        </section>
        <form
          class="panel notification-panel notification-channel-form"
          on:submit|preventDefault={createChannel}
        >
          <div class="notification-panel-heading">
            <div>
              <h3>新建通知渠道</h3>
              <p>凭据保存后仅显示脱敏摘要。</p>
            </div>
          </div>
          <NotificationChannelFields {providers} bind:channelName bind:channelKind bind:channelRateLimit bind:channelShare bind:channelConfig />
          <FormActions className="notification-form-actions">
            <small>HTTPS 与 Provider 字段由服务端校验。</small><button
              class="primary compact"
              type="submit"
              disabled={busyAction === 'channel-create' || !channelKind}
              ><Save size={13} />创建渠道</button
          >
          </FormActions>
        </form>
      </div>
    {:else if activeTab === 'templates'}
      <div class="notification-section-grid notification-template-layout">
        <section class="panel notification-panel notification-list-panel">
          <div class="notification-panel-heading">
            <div>
              <h3>通知模板</h3>
              <p>版本不可变，发布后生成新版本。</p>
            </div>
            <SearchInput
              bind:value={search}
              width="220px"
              height="34px"
              placeholder="搜索模板"
              ariaLabel="搜索模板"
            />
          </div>
          <div class="notification-template-list">
            {#each filteredTemplates as template}<button
                type="button"
                class:active={selectedTemplate?.id === template.id}
                class="notification-template-row"
                on:click={() => (selectedTemplateId = template.id)}
                ><span
                  ><strong>{template.name}</strong><small
                    >{template.versions.length} 个版本 · {template.inherited
                      ? '继承'
                      : template.share_with_children
                        ? '共享'
                        : '当前范围'}</small
                  ></span
                ><span
                  >{latestPublishedVersion(template)?.version
                    ? `v${latestPublishedVersion(template)?.version}`
                    : '—'}</span
                ></button
              >{:else}<div class="notification-empty-inline">
                <FileText size={18} /><span>当前范围没有模板。</span>
              </div>{/each}
          </div>
        </section>
        <section class="panel notification-panel notification-detail-panel">
          {#if selectedTemplate}{@const latest =
              latestPublishedVersion(selectedTemplate)}
            <div class="notification-panel-heading">
              <div>
                <h3>{selectedTemplate.name}</h3>
                <p>
                  {selectedTemplate.share_with_children
                    ? '对子 Scope 可用'
                    : '仅当前范围可用'}
                </p>
              </div>
              <span class="notification-chip"
                >{selectedTemplate.versions.length} 版本</span
              >
            </div>
            {#if latest}<div class="notification-preview">
                <strong>{latest.draft.title_template || '无标题'}</strong>
                <pre>{latest.draft.body_template ||
                    JSON.stringify(
                      latest.draft.payload_template ?? {},
                      null,
                      2
                    )}</pre>
              </div>
              <div class="notification-version-list">
                {#each selectedTemplate.versions as version}<div
                    class="notification-version-row"
                  >
                    <div>
                      <strong>v{version.version}</strong><span
                        class={`notification-status ${statusTone(version.status)}`}
                        >{statusLabel(version.status)}</span
                      ><small
                        >{formatTime(version.created_at)} · {version.content_hash.slice(
                          0,
                          10
                        )}</small
                      >
                    </div>
                    <div>
                      <button
                        class="text-button"
                        type="button"
                        disabled={busyAction !== ''}
                        on:click={() =>
                          previewTemplate(selectedTemplate, version)}
                        >预览</button
                      >{#if version.status === 'draft'}<button
                          class="text-button"
                          type="button"
                          disabled={busyAction !== ''}
                          on:click={() =>
                            publishTemplate(selectedTemplate, version)}
                          >发布</button
                        >{/if}
                    </div>
                  </div>{/each}
              </div>{:else}<div class="notification-empty-inline">
                <span>还没有模板版本。</span>
              </div>{/if}{:else}<div class="notification-empty-inline">
              <FileText size={18} /><span>选择一个模板查看版本。</span>
            </div>{/if}
        </section>
        <form
          class="panel notification-panel notification-create-panel"
          on:submit|preventDefault={createTemplate}
        >
          <div class="notification-panel-heading">
            <div>
              <h3>新建模板</h3>
              <p>变量必须使用受限字段。</p>
            </div>
          </div>
          <FormField label="模板名称" required><TextInput bind:value={templateName} required maxlength={120} placeholder="例如：事故通知" ariaLabel="模板名称" /></FormField>
          <div class="notification-form-grid">
            <FormField label="格式"><DropdownSelect bind:value={templateFormat} options={[{ value: 'text', label: '文本' }, { value: 'markdown', label: 'Markdown' }, { value: 'json', label: 'JSON' }]} ariaLabel="模板格式" /></FormField><FormField label="共享范围"><DropdownSelect value={templateShare ? 'shared' : 'local'} options={[{ value: 'local', label: '仅当前范围' }, { value: 'shared', label: '对子 Scope 可用' }]} ariaLabel="模板共享范围" on:change={(event) => (templateShare = event.detail === 'shared')} /></FormField>
          </div>
          <FormField label="标题模板"><TextInput bind:value={templateTitle} placeholder="例如：严重级别与规则名称" ariaLabel="标题模板" /></FormField><FormField label="正文模板"><TextArea rows={5} bind:value={templateBody} /></FormField>
          <FormActions className="notification-form-actions">
            <small>草稿创建后可独立预览与发布。</small><button
              class="primary compact"
              type="submit"
              disabled={busyAction === 'template-create'}
              ><Plus size={13} />创建模板</button
            >
          </FormActions>
        </form>
      </div>
    {:else if activeTab === 'rules'}
      <div class="notification-section-grid notification-rule-layout">
        <section class="panel notification-panel notification-list-panel">
          <div class="notification-panel-heading">
            <div>
              <h3>通知规则</h3>
              <p>事件、级别、冷却和路由策略</p>
            </div>
            <SearchInput
              bind:value={search}
              width="220px"
              height="34px"
              placeholder="搜索规则"
              ariaLabel="搜索规则"
            />
          </div>
          <div class="notification-rule-list">
            {#each filteredRules as rule}<div class="notification-rule-row">
                <div>
                  <strong>{rule.name}</strong><small
                    >{rule.event_types.join('、')} · {rule.minimum_severity} · 聚合
                    {durationSeconds(rule.aggregation)} 秒</small
                  >
                </div>
                <span class={`notification-status ${statusTone(rule.status)}`}
                  >{statusLabel(rule.status)}</span
                ><button
                  class="text-button"
                  type="button"
                  disabled={busyAction !== '' || rule.inherited}
                  on:click={() => testRule(rule)}
                  >{busyAction === `rule-test-${rule.id}`
                    ? '测试中…'
                    : '测试'}</button
                >
              </div>{:else}<div class="notification-empty-inline">
                <Route size={18} /><span>当前范围没有规则。</span>
              </div>{/each}
          </div>
        </section>
        <form
          class="panel notification-panel notification-create-panel"
          on:submit|preventDefault={createRule}
        >
          <div class="notification-panel-heading">
            <div>
              <h3>新建规则</h3>
              <p>启用规则至少需要一条完整路由。</p>
            </div>
          </div>
          <FormField label="规则名称" required><TextInput bind:value={ruleName} required maxlength={120} placeholder="例如：核心链路异常" ariaLabel="规则名称" /></FormField>
          <div class="notification-form-grid">
            <FormField label="事件"><DropdownSelect bind:value={ruleEvent} options={[{ value: 'finding.opened', label: 'Finding 新建' }, { value: 'finding.reopened', label: 'Finding 恢复后再次打开' }, { value: 'finding.severity_changed', label: '严重级别变化' }, { value: 'finding.resolved', label: 'Finding 已恢复' }, { value: 'inspection.failed', label: '巡检失败' }, { value: 'inspection.degraded', label: '巡检降级' }]} searchable ariaLabel="通知事件" /></FormField><FormField label="最低级别"><DropdownSelect bind:value={ruleSeverity} options={[{ value: 'info', label: 'Info' }, { value: 'warning', label: 'Warning' }, { value: 'critical', label: 'Critical' }]} ariaLabel="最低级别" /></FormField>
            >
          </div>
          <FormField label="投递渠道"><DropdownSelect bind:value={ruleChannelId} options={[{ value: '', label: '先不配置' }, ...channels.filter((channel) => channel.status === 'active').map((channel) => ({ value: channel.id, label: channel.name }))]} ariaLabel="投递渠道" /></FormField>
          <FormField label="已发布模板"><DropdownSelect bind:value={ruleTemplateVersionId} options={[{ value: '', label: '先不配置' }, ...templates.flatMap((template) => template.versions.filter((version) => version.status === 'published').map((version) => ({ value: version.id, label: `${template.name} · v${version.version}` })))]} ariaLabel="已发布模板" /></FormField>
          <div class="notification-form-grid">
            <FormField label="状态"><DropdownSelect bind:value={ruleStatus} options={[{ value: 'active', label: '启用' }, { value: 'disabled', label: '停用' }]} ariaLabel="规则状态" /></FormField><FormField label="共享范围"><DropdownSelect value={ruleShare ? 'shared' : 'local'} options={[{ value: 'local', label: '仅当前范围' }, { value: 'shared', label: '对子 Scope 可用' }]} ariaLabel="规则共享范围" on:change={(event) => (ruleShare = event.detail === 'shared')} /></FormField>
          </div>
          <FormActions className="notification-form-actions">
            <small>无完整路由时会以停用状态保存。</small><button
              class="primary compact"
              type="submit"
              disabled={busyAction === 'rule-create'}
              ><Plus size={13} />创建规则</button
            >
          </FormActions>
        </form>
        <section class="panel notification-panel notification-policy-panel">
          <div class="notification-panel-heading">
            <div>
              <h3>巡检策略关联</h3>
              <p>策略只选择规则，不复制渠道和模板。</p>
            </div>
            <ShieldCheck size={17} />
          </div>
          <div class="notification-form-grid">
            <FormField label="巡检策略"><DropdownSelect options={[{ value: '', label: '选择策略' }, ...policies.map((policy) => ({ value: policy.id, label: policy.name }))]} value={selectedPolicyId} ariaLabel="巡检策略" on:change={(event) => loadPolicyRules(String(event.detail))} /></FormField>
            <div class="notification-policy-actions">
              <button
                class="secondary compact"
                type="button"
                disabled={!selectedPolicyId ||
                  policyRulesLoading ||
                  busyAction !== ''}
                on:click={savePolicyRules}><Save size={13} />保存关联</button
              >
            </div>
          </div>
          {#if selectedPolicyId}<div class="notification-policy-rules">
              {#each rules as rule}<label
                  ><input
                    type="checkbox"
                    checked={policyRuleIds.includes(rule.id)}
                    on:change={() => toggleRuleId(rule.id)}
                  />{rule.name}<small>{statusLabel(rule.status)}</small></label
                >{:else}<span>还没有可关联的规则。</span>{/each}
            </div>{/if}
        </section>
      </div>
    {:else}
      <section class="panel notification-panel notification-deliveries-panel">
        <div class="notification-panel-heading">
          <div>
            <h3>投递记录</h3>
            <p>按状态查看最近发送、重试和死信。</p>
          </div>
          <div class="notification-toolbar">
            <SearchInput
              bind:value={search}
              width="220px"
              height="34px"
              placeholder="搜索事件或 ID"
              ariaLabel="搜索事件或 ID"
            /><DropdownSelect bind:value={deliveryStatus} options={[{ value: '', label: '全部状态' }, { value: 'queued', label: '待投递' }, { value: 'delivering', label: '发送中' }, { value: 'retrying', label: '重试中' }, { value: 'succeeded', label: '已送达' }, { value: 'dead_letter', label: '死信' }, { value: 'failed', label: '失败' }]} ariaLabel="按投递状态筛选" />
          </div>
        </div>
        <div class="notification-table-scroll">
          <table class="notification-table notification-delivery-table">
            <thead
              ><tr
                ><th>事件</th><th>渠道</th><th>状态</th><th>尝试</th><th
                  >创建时间</th
                ><th>操作</th></tr
              ></thead
            ><tbody
              >{#each filteredDeliveries as delivery}<tr
                  ><td
                    ><strong>{delivery.event_type}</strong><small
                      >{delivery.id.slice(0, 12)}…</small
                    ></td
                  ><td>{channelNameForId(delivery.channel_id)}</td><td
                    ><span
                      class={`notification-status ${statusTone(delivery.status)}`}
                      >{statusLabel(delivery.status)}</span
                    ></td
                  ><td>{delivery.attempt} / {delivery.max_attempts}</td><td
                    >{formatTime(delivery.created_at)}</td
                  ><td
                    >{#if delivery.status === 'dead_letter'}<button
                        class="text-button"
                        type="button"
                        disabled={busyAction !== ''}
                        on:click={() => retryDelivery(delivery)}
                        >{busyAction === `delivery-retry-${delivery.id}`
                          ? '提交中…'
                          : '重试'}</button
                      >{:else}<span class="notification-muted">—</span>{/if}</td
                  ></tr
                >{:else}<tr
                  ><td colspan="6" class="notification-table-empty"
                    >当前筛选条件没有投递记录。</td
                  ></tr
                >{/each}</tbody
            >
          </table>
        </div>
      </section>
    {/if}
  {/if}
</section>
