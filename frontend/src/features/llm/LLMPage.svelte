<script lang="ts">
  import { onMount, tick } from 'svelte';
  import {
    Cpu,
    Plus,
    Pencil,
    X,
    ChevronDown,
    RefreshCw,
    Save,
    Trash2
  } from 'lucide-svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import IconValue from '../../components/IconValue.svelte';
  import MessageBanner from '../../components/MessageBanner.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import { providerTypeOptions } from './providerCatalog';
  import ProviderBrandIcon from './ProviderBrandIcon.svelte';
  import {
    api,
    ApiError,
    type EngineCatalogItem,
    type Provider,
    type ProviderConfig,
    type ProviderModel,
    type AIConnectionResult
  } from '../../lib/api';
  import type { ScopeChoice } from '../../lib/scope';

  export let providers: Provider[] = [];
  export let scopeId = '';
  export let scopeName: (id: string) => string;
  export let scopeType: (id: string) => string = () => 'platform';
  export let scopeChoices: ScopeChoice[] = [];
  export let canManageProvider = false;
  export let canManageEngine = false;
  export let providerPermissionLabel: (provider: Provider) => string = () => '可查看';
  export let onProvidersChanged: (items: Provider[]) => void = () => {};
  export let onNotice: (message: string) => void = () => {};
  export let onError: (message: string) => void = () => {};

  type ScenarioTag = { value: string; label: string; hint: string };
  const scenarioTags: ScenarioTag[] = [
    { value: 'general', label: '通用', hint: '默认执行' },
    { value: 'diagnosis', label: '诊断', hint: '故障分析' },
    { value: 'inspection', label: '巡检', hint: '定时检查' },
    { value: 'workflow', label: '工作流', hint: '流程执行' }
  ];
  const modelTags = [
    { value: 'text', label: '文本' },
    { value: 'stream', label: '流式' },
    { value: 'tool_calling', label: '工具' },
    { value: 'reasoning', label: '推理' },
    { value: 'embedding', label: '向量' },
    { value: 'audio', label: '音频' },
    { value: 'vision', label: '视觉' },
    { value: 'image_generation', label: '生图' }
  ];
  const defaultModelTags = ['text', 'stream', 'tool_calling'];
  const fallbackCapabilityTerms = [
    '智能体循环',
    '上下文编排',
    '工具调用',
    '工具网关',
    '技能编排',
    '专家路由',
    '结构化输出',
    '检索增强',
    '工作流编排',
    '流式事件'
  ];

  let engines: EngineCatalogItem[] = [];
  let bindings: Array<{ scope_id: string; engine_id: string; tag: string }> =
    [];
  let selectedEngineId = '';
  let selectedProviderId = '';
  let drawerOpen = false;
  let editingProvider = false;
  let editingEngine = false;
  let busy = false;
  let testingProviders: Record<string, boolean> = {};
  let engineDraft: {
    name: string;
    description: string;
    icon: string;
    capabilities: string[];
    tags: string[];
    status: string;
  } = {
    name: '',
    description: '',
    icon: 'lucide:Cpu',
    capabilities: [],
    tags: [],
    status: 'active'
  };
  let capabilityMenuOpen = false;
  let capabilitySearch = '';
  let capabilityInput: HTMLInputElement;
  let capabilityTerms: string[] = [...fallbackCapabilityTerms];
  let providerDraft: ProviderDraft = emptyProvider();
  let engineErrors: Record<string, string> = {};
  let providerErrors: Record<string, string> = {};
  let providerTypeMenuOpen = false;
  let protocolMenuOpen = false;
  let sourceProviders: Provider[] = providers;
  let providerSearch = '';
  let loadedScopeId = '';
  let providerEditorMessage = '';
  let providerEditorMessageTone: 'success' | 'error' = 'success';
  let providerEditorMessageTimer: number | null = null;
  let engineEditorMessage = '';
  let engineEditorMessageTone: 'success' | 'error' = 'success';
  let engineEditorMessageTimer: number | null = null;
  let connectionAgeNow = Date.now();

  onMount(() => {
    const timer = window.setInterval(() => {
      connectionAgeNow = Date.now();
    }, 1000);
    return () => window.clearInterval(timer);
  });

  // Keep the shared scopeName prop for the page contract; the drawer subtitle no longer renders scope text.
  $: void scopeName;

  type ProviderDraft = {
    id?: string;
    name: string;
    status: string;
    icon: string;
    tags: string[];
    scopeId: string;
    providerType: string;
    protocol: string;
    baseUrl: string;
    apiKey: string;
    timeoutSeconds: number;
    maxConcurrency: number;
    rateLimitPerMinute: number;
    enabled: boolean;
    defaultModel: string;
    models: DraftModel[];
  };
  type DraftModel = ProviderModel & { tags: string[] };

  function emptyModel(): DraftModel {
    return {
      name: '',
      context_window_tokens: 128000,
      max_output_tokens: 128000,
      temperature: 0.2,
      temperature_mutable: true,
      tags: [...defaultModelTags],
      capabilities: [...defaultModelTags],
      enabled: true,
      priority: 1
    };
  }
  function emptyProvider(): ProviderDraft {
    return {
      name: '',
      status: 'active',
      icon: 'lucide:Bot',
      tags: ['general'],
      scopeId: '',
      providerType: 'openai_compatible',
      protocol: 'chat_completions',
      baseUrl: '',
      apiKey: '',
      timeoutSeconds: 60,
      maxConcurrency: 5,
      rateLimitPerMinute: 0,
      enabled: true,
      defaultModel: '',
      models: [emptyModel()]
    };
  }
  function modelTagsOf(model: ProviderModel) {
    const tags = model.tags?.length ? model.tags : (model.capabilities ?? []);
    return tags.filter((tag) => tag !== 'structured_output');
  }
  function engineCapabilities(engine: EngineCatalogItem) {
    return Array.isArray(engine.config?.capabilities)
      ? engine.config.capabilities.map(String).filter(Boolean)
      : [];
  }
  function capabilityValues() {
    return engineDraft.capabilities.map((item) => item.trim()).filter(Boolean);
  }
  function capabilityTagClass(tag: string) {
    return `tag-${tag}`;
  }
  function setEngineDraft(engine: EngineCatalogItem | null) {
    const capabilities = engine
      ? engineCapabilities(engine)
      : [...fallbackCapabilityTerms];
    const missingTerms = capabilities.filter(
      (term) =>
        !capabilityTerms.some(
          (item) => item.toLowerCase() === term.toLowerCase()
        )
    );
    if (missingTerms.length) capabilityTerms = [...capabilityTerms, ...missingTerms];
    engineDraft = {
      name: engine?.name ?? '',
      description: engine?.description ?? '',
      icon: engine?.icon || 'lucide:Cpu',
      capabilities,
      tags: engine?.tags?.length ? [...engine.tags] : ['general'],
      status: engine?.status === 'disabled' ? 'disabled' : 'active'
    };
    capabilitySearch = '';
    capabilityMenuOpen = false;
  }
  $: filteredCapabilityTerms = capabilityTerms.filter(
    (term) =>
      !engineDraft.capabilities.some(
        (value) => value.toLowerCase() === term.toLowerCase()
      ) &&
      (!capabilitySearch.trim() ||
        term.toLowerCase().includes(capabilitySearch.trim().toLowerCase()))
  );
  function defaultModelOf(provider: Provider): ProviderModel | undefined {
    return (
      provider.config.models?.find(
        (model) => model.name === provider.config.default_model
      ) ?? provider.config.models?.[0]
    );
  }
  function providerModelSummary(provider: Provider) {
    const models = [...(provider.config.models ?? [])];
    const defaultName = provider.config.default_model || models[0]?.name;
    const defaultIndex = models.findIndex((model) => model.name === defaultName);
    if (defaultIndex > 0) {
      const [defaultModel] = models.splice(defaultIndex, 1);
      models.unshift(defaultModel);
    }
    return { visible: models.slice(0, 2), hidden: Math.max(0, models.length - 2) };
  }
  function modelCapabilityLabels(model: ProviderModel | undefined) {
    return (model ? modelTagsOf(model) : []).map(
      (tag) => modelTags.find((item) => item.value === tag)?.label ?? tag
    );
  }
  function providerHealthLabel(provider: Provider) {
    if (providerIsTesting(provider.id)) {
      return { label: '测试中', title: '正在测试渠道连接', tone: 'testing' };
    }
    const status = providerConnectionSummary(provider);
    return { label: status.label, title: status.title, tone: status.tone };
  }
  function providerConnectionSummary(provider: Provider | undefined) {
    const test = provider?.last_connection_test;
    if (!test) return { health: '未测试', latency: '未知', label: '连接测试', title: '点击进行渠道连接测试', tone: 'muted' };
    if (test.status === 'succeeded') {
      const label = `正常·${test.latency_ms}ms`;
      return { health: '正常', latency: `${test.latency_ms}ms`, label, title: label, tone: 'success' };
    }
    const message = (test.message || '连接测试失败').replace(/\s+/g, ' ').trim();
    return { health: '异常', latency: '未知', label: `异常·${message}`, title: message, tone: 'danger' };
  }
  function formatConnectionAge(checkedAt: string | undefined) {
    if (!checkedAt) return '';
    const checkedTime = Date.parse(checkedAt);
    if (!Number.isFinite(checkedTime)) return '';
    const elapsed = Math.max(connectionAgeNow - checkedTime, 0);
    const minute = 60_000;
    const hour = 60 * minute;
    const day = 24 * hour;
    const week = 7 * day;
    const month = 30 * day;
    const year = 365 * day;
    const value = (amount: number, unit: string) =>
      `${amount.toFixed(1).padStart(4, '0')} ${unit}`;
    if (elapsed < minute) return value(elapsed / 1000, '秒');
    if (elapsed < hour) return value(elapsed / minute, '分');
    if (elapsed < day) return value(elapsed / hour, '时');
    if (elapsed < week) return value(elapsed / day, '天');
    if (elapsed < month) return value(elapsed / week, '周');
    if (elapsed < year) return value(elapsed / month, '月');
    return value(elapsed / year, '年');
  }
  function providerMatches(provider: Provider) {
    const query = providerSearch.trim().toLowerCase();
    if (!query) return true;
    const modelValues = (provider.config.models ?? []).flatMap((model) => [
      model.name,
      ...modelTagsOf(model),
      ...modelCapabilityLabels(model)
    ]);
    return [
      provider.name,
      provider.config.base_url,
      ...modelValues
    ].some((value) => String(value ?? '').toLowerCase().includes(query));
  }
  function scopeLevelLabel(type: string) {
    return ({ platform: '平台', team: '团队', project: '项目' } as Record<string, string>)[type] ?? '当前级别';
  }
  function scopeBadgeClass(type: string) {
    return ({ platform: 'scope-platform', team: 'scope-team', project: 'scope-project' } as Record<string, string>)[type] ?? 'scope-platform';
  }
  function providerScopeOptions(currentScopeId: string, includeScopeId = '') {
    const options: ScopeChoice[] = [];
    const seen = new Set<string>();
    let current = scopeChoices.find((item) => item.id === currentScopeId);
    while (current && !seen.has(current.id)) {
      options.unshift(current);
      seen.add(current.id);
      current = current.parentId
        ? scopeChoices.find((item) => item.id === current?.parentId)
        : undefined;
    }
    if (includeScopeId && !seen.has(includeScopeId)) {
      const existing = scopeChoices.find((item) => item.id === includeScopeId);
      if (existing) options.unshift(existing);
    }
    return options;
  }
  $: if (providers !== sourceProviders) {
    sourceProviders = providers;
    if (!selectedProviderId && providers[0]) {
      selectedProviderId = providers[0].id;
      if (!editingProvider) drawerOpen = true;
    }
  }
  $: selectedProvider = sourceProviders.find(
    (item) => item.id === selectedProviderId
  );
  $: filteredProviders = sourceProviders.filter(providerMatches);
  $: selectedEngine =
    engines.find((item) => item.id === selectedEngineId) ?? engines[0];
  $: if (scopeId && scopeId !== loadedScopeId) {
    loadedScopeId = scopeId;
    void loadCatalog();
  }

  async function loadCatalog() {
    try {
      const [engineResult, bindingResult, providerResult, termResult] =
        await Promise.all([
          api.engines(scopeId),
          api.engineBindings(scopeId),
          api.providers(scopeId),
          api.engineCapabilityTerms()
        ]);
      if (scopeId !== loadedScopeId) return;
      engines = engineResult.items;
      bindings = bindingResult;
      capabilityTerms = termResult.items.length
        ? termResult.items
        : [...fallbackCapabilityTerms];
      sourceProviders = providerResult.items;
      onProvidersChanged(providerResult.items);
      providerSearch = '';
      selectedEngineId =
        engines.find((item) => item.tags?.includes('general'))?.id ??
        engines[0]?.id ??
        '';
      selectedProviderId =
        sourceProviders[0]?.id ?? '';
      drawerOpen = Boolean(sourceProviders[0]);
    } catch (error) {
      onError(describeError(error, '大模型目录加载失败'));
    }
  }
  async function reloadProviders(): Promise<Provider[]> {
    const result = await api.providers(scopeId);
    sourceProviders = result.items;
    onProvidersChanged(result.items);
    if (!selectedProviderId && result.items[0]) {
      selectedProviderId = result.items[0].id;
      if (!editingProvider) drawerOpen = true;
    }
    return result.items;
  }
  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError)
      return error.status === 403
        ? '当前账号没有管理权限。'
        : error.message || fallback;
    return error instanceof Error ? error.message || fallback : fallback;
  }
  function clearProviderEditorMessage() {
    if (providerEditorMessageTimer !== null) {
      window.clearTimeout(providerEditorMessageTimer);
      providerEditorMessageTimer = null;
    }
    providerEditorMessage = '';
    onNotice('');
    onError('');
  }
  function setProviderEditorMessage(message: string, tone: 'success' | 'error') {
    if (providerEditorMessageTimer !== null) {
      window.clearTimeout(providerEditorMessageTimer);
    }
    providerEditorMessage = message;
    providerEditorMessageTone = tone;
    providerEditorMessageTimer = window.setTimeout(() => {
      providerEditorMessage = '';
      providerEditorMessageTimer = null;
    }, 5_000);
  }
  function providerEditorNotice(message: string) {
    if (editingProvider) {
      setProviderEditorMessage(message, 'success');
      onNotice('');
      onError('');
      return;
    }
    onNotice(message);
  }
  function providerEditorError(message: string) {
    if (editingProvider) {
      setProviderEditorMessage(message, 'error');
      onNotice('');
      onError('');
      return;
    }
    onError(message);
  }
  function selectProvider(provider: Provider) {
    selectedProviderId = provider.id;
    drawerOpen = true;
    editingProvider = false;
    clearProviderEditorMessage();
    providerDraft = toDraft(provider);
  }
  function editProviderFromList(provider: Provider) {
    if (!canManageProvider) return;
    selectProvider(provider);
    openEditProvider();
  }
  function toDraft(provider: Provider): ProviderDraft {
    const config = provider.config;
    return {
      id: provider.id,
      name: provider.name,
      status: provider.status,
      icon: config.icon || 'lucide:Bot',
      tags: provider.tags?.length ? [...provider.tags] : ['general'],
      scopeId: provider.scope_id,
      providerType: config.provider_type || 'openai_compatible',
      protocol: config.protocol || 'chat_completions',
      baseUrl: config.base_url || '',
      apiKey: '',
      timeoutSeconds: config.timeout_seconds || 60,
      maxConcurrency: config.max_concurrency || 5,
      rateLimitPerMinute: config.rate_limit_per_minute || 0,
      enabled: config.enabled !== false,
      defaultModel: config.default_model || config.models?.[0]?.name || '',
      models: (config.models || []).map((model) => {
        const tags = modelTagsOf(model);
        const effectiveTags = tags.length ? tags : [...defaultModelTags];
        return {
          ...model,
          tags: effectiveTags,
          capabilities: model.capabilities?.length ? model.capabilities : effectiveTags
        };
      })
    };
  }
  function openNewProvider() {
    providerDraft = { ...emptyProvider(), scopeId };
    providerErrors = {};
    clearProviderEditorMessage();
    providerTypeMenuOpen = false;
    editingProvider = true;
    drawerOpen = true;
    selectedProviderId = '';
  }
  function openEditProvider() {
    if (selectedProvider) {
      providerDraft = toDraft(selectedProvider);
      providerErrors = {};
      clearProviderEditorMessage();
      providerTypeMenuOpen = false;
      editingProvider = true;
      drawerOpen = true;
    }
  }
  function cancelProviderEdit() {
    providerErrors = {};
    providerTypeMenuOpen = false;
    clearProviderEditorMessage();
    editingProvider = false;
    if (selectedProvider) providerDraft = toDraft(selectedProvider);
    else drawerOpen = false;
  }
  function toggleProviderTag(tag: string) {
    if (!canManageProvider || busy) return;
    providerDraft = {
      ...providerDraft,
      tags: isBound(providerDraft.tags, tag)
        ? providerDraft.tags.filter((item) => item !== tag)
        : [...providerDraft.tags, tag]
    };
  }
  function openEditEngine(engine: EngineCatalogItem) {
    selectedEngineId = engine.id;
    setEngineDraft(engine);
    engineErrors = {};
    clearEngineEditorMessage();
    editingEngine = true;
  }
  function clearEngineEditorMessage() {
    if (engineEditorMessageTimer !== null) {
      window.clearTimeout(engineEditorMessageTimer);
      engineEditorMessageTimer = null;
    }
    engineEditorMessage = '';
    onNotice('');
    onError('');
  }
  function setEngineEditorMessage(message: string, tone: 'success' | 'error') {
    if (engineEditorMessageTimer !== null) {
      window.clearTimeout(engineEditorMessageTimer);
    }
    engineEditorMessage = message;
    engineEditorMessageTone = tone;
    engineEditorMessageTimer = window.setTimeout(() => {
      engineEditorMessage = '';
      engineEditorMessageTimer = null;
    }, 5_000);
  }
  function engineEditorNotice(message: string) {
    if (editingEngine) {
      setEngineEditorMessage(message, 'success');
      onNotice('');
      onError('');
      return;
    }
    onNotice(message);
  }
  function engineEditorError(message: string) {
    if (editingEngine) {
      setEngineEditorMessage(message, 'error');
      onNotice('');
      onError('');
      return;
    }
    onError(message);
  }
  function cancelEngineEdit() {
    clearEngineEditorMessage();
    editingEngine = false;
  }
  function selectCapability(term: string) {
    const value = term.trim();
    if (!value) return;
    if (
      !capabilityTerms.some(
        (item) => item.toLowerCase() === value.toLowerCase()
      )
    )
      capabilityTerms = [...capabilityTerms, value];
    if (
      !engineDraft.capabilities.some(
        (item) => item.toLowerCase() === value.toLowerCase()
      )
    ) {
      engineDraft = {
        ...engineDraft,
        capabilities: [...engineDraft.capabilities, value]
      };
    }
    capabilitySearch = '';
    capabilityMenuOpen = false;
  }
  function toggleCapability(term: string) {
    const selected = engineDraft.capabilities.some(
      (item) => item.toLowerCase() === term.toLowerCase()
    );
    engineDraft = {
      ...engineDraft,
      capabilities: selected
        ? engineDraft.capabilities.filter(
            (item) => item.toLowerCase() !== term.toLowerCase()
          )
        : [...engineDraft.capabilities, term]
    };
  }
  function handleCapabilityKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault();
      const term = capabilitySearch.trim();
      if (term) selectCapability(term);
    }
    if (event.key === 'Escape') capabilityMenuOpen = false;
  }
  function openCapabilityMenu() {
    capabilitySearch = '';
    capabilityMenuOpen = !capabilityMenuOpen;
    if (capabilityMenuOpen)
      void tick().then(() => capabilityInput?.focus());
  }
  async function persistCapabilityTerms(values: string[]) {
    for (const term of values) {
      const result = await api.createEngineCapabilityTerm(term);
      if (
        !capabilityTerms.some(
          (item) => item.toLowerCase() === result.term.toLowerCase()
        )
      )
        capabilityTerms = [...capabilityTerms, result.term].sort((a, b) =>
          a.localeCompare(b)
        );
    }
  }
  function toggleModelTag(model: DraftModel, tag: string) {
    model.tags = model.tags.includes(tag)
      ? model.tags.filter((item) => item !== tag)
      : [...model.tags, tag];
    model.capabilities = model.tags;
    providerDraft.models = providerDraft.models;
  }
  function addModel() {
    providerDraft.models = [...providerDraft.models, emptyModel()];
  }
  function removeModel(index: number) {
    if (providerDraft.models.length > 1)
      providerDraft.models = providerDraft.models.filter(
        (_, item) => item !== index
      );
  }
  function setDraftDefault(model: DraftModel) {
    providerDraft.defaultModel = model.name;
  }
  function providerConfig(): ProviderConfig {
    return {
      provider_type: providerDraft.providerType,
      protocol: providerDraft.protocol,
      base_url: providerDraft.baseUrl,
      timeout_seconds: providerDraft.timeoutSeconds,
      max_concurrency: providerDraft.maxConcurrency,
      rate_limit_per_minute: providerDraft.rateLimitPerMinute,
      enabled: providerDraft.status === 'active' && providerDraft.enabled,
      default_model:
        providerDraft.defaultModel || providerDraft.models[0]?.name,
      icon: providerDraft.icon,
      models: providerDraft.models.map(({ tags, ...model }) => ({
        ...model,
        tags,
        capabilities: tags
      }))
    };
  }
  function providerTypeLabel(value: string) {
    return providerTypeOptions.find((option) => option.value === value)?.label ?? 'OpenAI 兼容';
  }
  function selectProviderType(value: string) {
    const option = providerTypeOptions.find((item) => item.value === value);
    providerDraft = {
      ...providerDraft,
      providerType: value,
      baseUrl: option?.baseURL ?? ''
    };
    providerTypeMenuOpen = false;
  }
  function protocolLabel(value: string) {
    return value === 'messages' ? 'Messages API' : 'Chat Completions';
  }
  function selectProtocol(value: string) {
    providerDraft = { ...providerDraft, protocol: value };
    protocolMenuOpen = false;
  }
  function parseNumber(value: unknown) {
    if (value === '' || value === null || value === undefined) return null;
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  function validBaseURL(value: string) {
    try {
      const url = new URL(value.trim());
      return (url.protocol === 'http:' || url.protocol === 'https:') && Boolean(url.hostname);
    } catch {
      return false;
    }
  }
  function validateEngineDraft() {
    const errors: Record<string, string> = {};
    if (!engineDraft.name.trim()) errors.name = 'required';
    return errors;
  }
  function validateProviderDraft() {
    const errors: Record<string, string> = {};
    if (!providerDraft.name.trim()) errors.name = 'required';
    if (!providerTypeOptions.some((option) => option.value === providerDraft.providerType)) errors.providerType = 'invalid';
    if (!providerDraft.baseUrl.trim()) errors.baseUrl = 'required';
    else if (!validBaseURL(providerDraft.baseUrl)) errors.baseUrl = 'invalid';

    const timeout = parseNumber(providerDraft.timeoutSeconds);
    if (timeout === null || timeout < 0 || timeout > 300) errors.timeoutSeconds = 'invalid';
    const concurrency = parseNumber(providerDraft.maxConcurrency);
    if (concurrency === null || concurrency < 1) errors.maxConcurrency = 'invalid';
    const rateLimit = parseNumber(providerDraft.rateLimitPerMinute);
    if (rateLimit === null || rateLimit < 0) errors.rateLimitPerMinute = 'invalid';

    if (providerDraft.models.length === 0) {
      errors.models = 'required';
    }
    const seenNames = new Map<string, number>();
    providerDraft.models.forEach((model, index) => {
      const name = model.name.trim();
      const nameKey = name.toLowerCase();
      if (!name) errors[`models.${index}.name`] = 'required';
      else if (seenNames.has(nameKey)) errors[`models.${index}.name`] = 'duplicate';
      else seenNames.set(nameKey, index);

      const contextWindow = parseNumber(model.context_window_tokens);
      if (contextWindow === null || contextWindow <= 0)
        errors[`models.${index}.context`] = 'invalid';
      const maxOutput = parseNumber(model.max_output_tokens);
      if (maxOutput !== null && maxOutput <= 0)
        errors[`models.${index}.maxOutput`] = 'invalid';
      const temperature = parseNumber(model.temperature);
      if (temperature === null || temperature < 0 || temperature > 2)
        errors[`models.${index}.temperature`] = 'invalid';
    });
    if (
      providerDraft.defaultModel.trim() &&
      !providerDraft.models.some(
        (model) => model.name === providerDraft.defaultModel && model.enabled !== false
      )
    ) {
      errors.defaultModel = 'invalid';
    }
    return errors;
  }
  async function saveProvider() {
    providerErrors = validateProviderDraft();
    clearProviderEditorMessage();
    if (Object.keys(providerErrors).length > 0) {
      return;
    }
    busy = true;
    try {
      const existingTags = providerDraft.id
        ? [...(selectedProvider?.tags ?? [])]
        : [];
      const selectedTags = [...providerDraft.tags];
      const body = {
        name: providerDraft.name.trim(),
        status: providerDraft.status,
        config: providerConfig(),
        ...(providerDraft.apiKey.trim()
          ? { api_key: providerDraft.apiKey.trim() }
          : {})
      };
      const saved = providerDraft.id
        ? await api.updateProvider(providerDraft.id, body)
        : await api.createProvider({ scope_id: providerDraft.scopeId || scopeId, ...body });
      await syncProviderBindings(
        saved.id,
        existingTags,
        providerDraft.status === 'active' ? selectedTags : [],
        providerDraft.scopeId || scopeId
      );
      const refreshedProviders = await reloadProviders();
      selectedProviderId = saved.id;
      providerDraft = toDraft(
        refreshedProviders.find((item) => item.id === saved.id) ?? saved
      );
      providerErrors = {};
      providerEditorNotice('AI 渠道已保存');
      editingProvider = false;
      void testProvider(
        refreshedProviders.find((item) => item.id === saved.id) ?? saved
      );
    } catch (error) {
      providerEditorError(describeError(error, '保存 AI 渠道失败'));
    } finally {
      busy = false;
    }
  }
  async function deleteProvider(provider: Provider) {
    if (!canManageProvider || busy) return;
    if (typeof window !== 'undefined' && !window.confirm(`确定删除渠道“${provider.name}”吗？`)) return;
    busy = true;
    try {
      await api.deleteProvider(provider.id);
      const remaining = providers.filter((item) => item.id !== provider.id);
      onProvidersChanged(remaining);
      if (selectedProviderId === provider.id) {
        selectedProviderId = remaining[0]?.id ?? '';
        drawerOpen = Boolean(selectedProviderId);
        editingProvider = false;
      }
      onNotice(`渠道“${provider.name}”已删除`);
    } catch (error) {
      onError(describeError(error, '删除 AI 渠道失败'));
    } finally {
      busy = false;
    }
  }
  async function syncEngineBindings(engineId: string, selectedTags: string[]) {
    const selected = new Set(selectedTags);
    const localBindings = bindings.filter(
      (item) => item.engine_id === engineId && item.scope_id === scopeId
    );
    for (const tag of scenarioTags) {
      if (selected.has(tag.value)) {
        await api.setEngineBinding(scopeId, tag.value, engineId);
      } else if (localBindings.some((item) => item.tag === tag.value)) {
        await api.removeEngineBinding(scopeId, tag.value, engineId);
      }
    }
  }
  async function syncProviderBindings(
    providerId: string,
    existingTags: string[],
    selectedTags: string[],
    bindingScopeId: string
  ) {
    const selected = new Set(selectedTags);
    for (const tag of scenarioTags) {
      if (selected.has(tag.value)) {
        await api.setProviderBinding(bindingScopeId, tag.value, providerId);
      } else if (existingTags.includes(tag.value)) {
        await api.removeProviderBinding(bindingScopeId, tag.value);
      }
    }
  }
  async function reloadEngineCatalog() {
    const [engineResult, bindingResult] = await Promise.all([
      api.engines(scopeId),
      api.engineBindings(scopeId)
    ]);
    engines = engineResult.items;
    bindings = bindingResult;
  }
  async function saveEngine() {
    engineErrors = validateEngineDraft();
    clearEngineEditorMessage();
    if (Object.keys(engineErrors).length > 0) {
      return;
    }
    busy = true;
    try {
      const capabilities = capabilityValues();
      await persistCapabilityTerms(capabilities);
      const body = {
        name: engineDraft.name.trim(),
        description: engineDraft.description.trim(),
        icon: engineDraft.icon,
        status: engineDraft.status,
        config: { capabilities }
      };
      const saved = selectedEngineId
        ? await api.updateEngine(selectedEngineId, body)
        : await api.createEngine({ scope_id: scopeId, ...body });
      await syncEngineBindings(saved.id, engineDraft.tags);
      await reloadEngineCatalog();
      selectedEngineId = saved.id;
      engineErrors = {};
      engineEditorNotice('AI 引擎已保存，场景已同步');
    } catch (error) {
      engineEditorError(describeError(error, '保存 AI 引擎失败'));
    } finally {
      busy = false;
    }
  }
  async function bindEngine(tag: string) {
    if (!canManageEngine || !selectedEngine) return;
    busy = true;
    try {
      await api.setEngineBinding(scopeId, tag, selectedEngine.id);
      engines = (await api.engines(scopeId)).items;
      bindings = await api.engineBindings(scopeId);
      engineEditorNotice(
        `已将${scenarioTags.find((item) => item.value === tag)?.label}默认引擎切换为 ${selectedEngine.name}`
      );
    } catch (error) {
      engineEditorError(describeError(error, '更新引擎默认场景失败'));
    } finally {
      busy = false;
    }
  }
  async function bindProvider(tag: string) {
    if (!canManageProvider || !selectedProvider) return;
    busy = true;
    try {
      await api.setProviderBinding(scopeId, tag, selectedProvider.id);
      await reloadProviders();
      onNotice(
        `已将${scenarioTags.find((item) => item.value === tag)?.label}默认渠道切换为 ${selectedProvider.name}`
      );
    } catch (error) {
      onError(describeError(error, '更新渠道默认场景失败'));
    } finally {
      busy = false;
    }
  }
  function providerIsTesting(providerId: string | undefined) {
    return providerId ? Boolean(testingProviders[providerId]) : false;
  }
  async function testProvider(provider = selectedProvider) {
    if (!provider || providerIsTesting(provider.id)) return;
    testingProviders = { ...testingProviders, [provider.id]: true };
    try {
      const model =
        (provider.id === selectedProviderId && providerDraft.id
          ? providerDraft.defaultModel
          : provider.config.default_model) ||
        provider.config.models[0]?.name ||
        '';
      const result: AIConnectionResult = await api.testProvider(provider.id, {
        scope_id: scopeId,
        model_name: model,
        stream: true
      });
      const refreshed = await reloadProviders();
      const checkedAt = new Date().toISOString();
      const updated = refreshed.map((item) =>
        item.id === provider.id
          ? {
              ...item,
              last_connection_test: {
                status: result.status,
                message: result.message,
                latency_ms: result.latency_ms,
                checked_at: checkedAt
              }
            }
          : item
      );
      sourceProviders = updated;
      onProvidersChanged(updated);
    } catch {
      await reloadProviders().catch(() => undefined);
    } finally {
      const next = { ...testingProviders };
      delete next[provider.id];
      testingProviders = next;
    }
  }
  function isBound(tags: string[] | undefined, tag: string) {
    return tags?.includes(tag) ?? false;
  }
  function closeProviderTypeMenu() {
    providerTypeMenuOpen = false;
    protocolMenuOpen = false;
  }
</script>

<svelte:window on:click={closeProviderTypeMenu} />

<section class="llm-page">
  <section class="engine-section">
    <div class="section-heading">
      <div>
        <h2>AI 引擎 <small class="heading-code">ENGINE</small></h2>
        <p>
          引擎决定诊断、巡检和工作流的执行能力；场景由当前级别管理员维护，并自动继承上层级别。
        </p>
      </div>
    </div>
    <div class="engine-directory">
      {#each engines as engine}
        <article
          class="engine-card"
          class:selected={selectedEngineId === engine.id}
        >
          <button
            class="engine-select"
            type="button"
            on:click={() => (selectedEngineId = engine.id)}
          >
            <span class="entity-icon engine-icon"
              ><IconValue value={engine.icon || 'lucide:Cpu'} size={20} /></span
            ><span class="engine-copy"
              ><strong>{engine.name}</strong><small
                >{engine.description || '未填写引擎说明'}</small
              ></span
            ><span class="engine-state"
              >{engine.status === 'active' ? '运行中' : '已停用'}</span
            >
          </button>
          <div class="engine-capabilities">
            {#each engineCapabilities(engine) as capability}<span
                >{capability}</span
              >{/each}
          </div>
          <div class="engine-tag-line">
            <div class="tag-row">
              {#each scenarioTags.filter((tag) => isBound(engine.tags, tag.value)) as tag}<button
                  type="button"
                  class={`tag-chip active ${capabilityTagClass(tag.value)}`}
                  disabled={!canManageEngine || busy}
                  on:click={() => bindEngine(tag.value)}>{tag.label}</button
                >{/each}
            </div>
            {#if canManageEngine}<button
                class="icon-action provider-models-action"
                type="button"
                title="编辑引擎"
                aria-label="编辑引擎"
                on:click={() => openEditEngine(engine)}
                ><Pencil size={14} /></button
              >{/if}
          </div>
        </article>
      {:else}<div class="empty-state">当前级别暂无 AI 引擎。</div>{/each}
    </div>
    {#if editingEngine}
      <form class="engine-editor panel" novalidate on:submit|preventDefault={saveEngine}>
        <div class="editor-heading">
          <div>
            <p class="eyebrow">ENGINE EDITOR</p>
            <h3>编辑 AI 引擎</h3>
          </div>
          {#if engineEditorMessage}<div class="editor-heading-message">
              <MessageBanner
                message={engineEditorMessage}
                tone={engineEditorMessageTone}
              />
            </div>{/if}
          <div class="editor-heading-actions">
            <button
              class="secondary"
              type="button"
              on:click={cancelEngineEdit}
              ><X size={14} />取消</button
            ><button class="primary" type="submit" disabled={busy}
              >保存引擎<Save
                size={14}
              /></button>
          </div>
        </div>
        <div class="engine-editor-identity-row">
          <div class="engine-icon-field">
            <span class="field-label">引擎图标</span><IconPicker
              value={engineDraft.icon}
              onSelect={(icon) => (engineDraft.icon = icon)}
              ariaLabel="选择引擎图标"
            />
          </div>
          <label class="engine-name-field" class:invalid={Boolean(engineErrors.name)}
            ><span class="field-label"><i>*</i>引擎名称</span><input
              bind:value={engineDraft.name}
              aria-label="引擎名称"
              aria-invalid={Boolean(engineErrors.name)}
              placeholder="引擎名称"
              required
            /></label
          >
          <div class="engine-tag-editor">
            <span class="field-label">场景</span>
            <div class="tag-row">
              {#each scenarioTags as tag}<button
                  class={`tag-chip ${capabilityTagClass(tag.value)}`}
                  class:active={isBound(engineDraft.tags, tag.value)}
                  type="button"
                  on:click={() =>
                    (engineDraft.tags = isBound(engineDraft.tags, tag.value)
                      ? engineDraft.tags.filter((item) => item !== tag.value)
                      : [...engineDraft.tags, tag.value])}>{tag.label}</button
                >{/each}
            </div>
          </div>
          <label class="engine-status-switch"
            ><span class="field-label">状态</span><span class="switch-control"
              ><input
                type="checkbox"
                checked={engineDraft.status === 'active'}
                on:change={(event) =>
                  (engineDraft.status = (
                    event.currentTarget as HTMLInputElement
                  ).checked
                    ? 'active'
                    : 'disabled')}
                aria-label="启用引擎"
              /><i aria-hidden="true"></i></span
            ></label
          >
        </div>
        <div class="engine-editor-content">
          <div class="capability-field">
            <span class="field-label">引擎能力</span>
            <div class="capability-picker">
              <div class="capability-chip-list" aria-label="引擎能力词条">
                {#each capabilityTerms as term}<button
                    class="capability-chip"
                    class:active={engineDraft.capabilities.includes(term)}
                    type="button"
                    aria-pressed={engineDraft.capabilities.includes(term)}
                    on:click={() => toggleCapability(term)}>{term}</button
                  >{/each}{#if capabilityMenuOpen}<div
                    class="capability-inline-add"
                    role="listbox"
                    aria-label="新增能力词条"
                  >
                    <input
                      bind:this={capabilityInput}
                      value={capabilitySearch}
                      on:input={(event) => {
                        capabilitySearch = (
                          event.currentTarget as HTMLInputElement
                        ).value;
                      }}
                      on:keydown={handleCapabilityKeydown}
                      placeholder="筛选或输入"
                      aria-label="筛选或输入新能力词条"
                    />
                    <div class="capability-inline-options">
                      {#each filteredCapabilityTerms as term}<button
                          type="button"
                          role="option"
                          aria-selected="false"
                          on:click={() => selectCapability(term)}>{term}</button
                        >{/each}{#if capabilitySearch.trim() && !capabilityTerms.some((term) => term.toLowerCase() === capabilitySearch.trim().toLowerCase())}<button
                          class="capability-create"
                          type="button"
                          role="option"
                          aria-selected="false"
                          on:click={() => selectCapability(capabilitySearch)}
                          >新增“{capabilitySearch.trim()}”</button
                        >{:else if filteredCapabilityTerms.length === 0}<span
                          class="capability-empty"
                          >没有匹配词条，可直接输入新词条</span
                        >{/if}
                    </div>
                  </div>{:else}<button
                    class="capability-add"
                    type="button"
                    aria-label="新增能力词条"
                    title="新增能力词条"
                    on:click={openCapabilityMenu}><Plus size={14} /></button
                  >{/if}
              </div>
            </div>
          </div>
          <label class="engine-description-field"
            ><span class="field-label">描述</span><textarea
              bind:value={engineDraft.description}
              rows="1"
              placeholder="可选，说明引擎的运行定位"
            ></textarea></label
          >
        </div>
      </form>
    {/if}
  </section>

  <section class="provider-section">
    <div class="section-heading">
      <div>
        <h2>AI 渠道 <small class="heading-code">PROVIDER</small></h2>
        <p>
          渠道负责协议、凭据、模型和限流；点击列表项后，详情抽屉直接进入编辑状态。
        </p>
      </div>
      <div class="section-actions">
        {#if canManageProvider}<button
            class="primary"
            type="button"
            on:click={openNewProvider}><Plus size={14} />添加渠道</button
          >{/if}
      </div>
    </div>
    <div class="provider-workspace" class:drawer-open={drawerOpen}>
      <section class="provider-list panel">
        <div class="list-toolbar">
          <strong>{filteredProviders.length} 个渠道</strong><span class="list-toolbar-actions"
            ><input
              class="provider-search"
              type="search"
              bind:value={providerSearch}
              placeholder="搜索渠道、URL、模型或能力"
              aria-label="搜索渠道"
            /><button
              class="icon-button"
              type="button"
              on:click={reloadProviders}
              title="刷新渠道"><RefreshCw size={14} /></button
            >{#if canManageProvider}<button
                class="icon-button"
                type="button"
                on:click={openNewProvider}
                title="添加渠道"
                aria-label="添加渠道"><Plus size={14} /></button
              >{/if}
            </span
          >
        </div>
        {#each filteredProviders as provider}
          {@const isTesting = Boolean(testingProviders[provider.id])}
          {@const health = providerHealthLabel(provider)}
          {@const modelSummary = providerModelSummary(provider)}
          <button
            class="provider-row"
            class:selected={provider.id === selectedProviderId}
            class:provider-disabled={provider.status !== 'active'}
            type="button"
            on:click={() => selectProvider(provider)}
          >
            <span class="provider-row-head">
              <span class="provider-title">
                <span class="entity-icon provider-icon">
                  <IconValue value={provider.config.icon || 'lucide:Bot'} size={24} />
                </span>
                <span class="provider-title-copy">
                  <span class="provider-name-line">
                    <strong>{provider.name}</strong>
                    <span class="provider-scenario-tags">
                      {#each scenarioTags.filter((tag) => isBound(provider.tags, tag.value)) as tag}
                        <span class={`tag-chip ${capabilityTagClass(tag.value)} active`}>{tag.label}</span>
                      {/each}
                    </span>
                  </span>
                  <small title={provider.config.base_url || '未填写 Base URL'}>{provider.config.base_url || '未填写 Base URL'}</small>
                </span>
              </span>
            </span>
            <span class="provider-row-meta">
              <span>
                <strong class={`scope-badge ${scopeBadgeClass(scopeType(provider.scope_id))}`}>{scopeLevelLabel(scopeType(provider.scope_id))}</strong>
                <small>{providerPermissionLabel(provider)}</small>
              </span>
            </span>
            <span class="provider-row-foot">
              <span class="provider-default-capabilities">
                {#each modelCapabilityLabels(defaultModelOf(provider)) as capability}
                  <span class="tag-chip active">{capability}</span>
                {/each}
              </span>
              <span
                class="provider-health"
                class:connection-success={health.label.startsWith('正常')}
                class:connection-danger={health.label.startsWith('异常')}
                class:connection-muted={health.tone === 'muted'}
                class:testing={isTesting}
                role="button"
                tabindex="0"
                aria-label={`测试${provider.name}连接`}
                on:click|stopPropagation={() => void testProvider(provider)}
                on:keydown|stopPropagation={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    void testProvider(provider);
                  }
                }}
              >
                <strong title={isTesting ? '连接测试中' : '点击进行连接测试'}>{isTesting ? '测试中' : health.label}</strong>
              </span>
            </span>
            <span class="provider-row-models">
              {#each modelSummary.visible as model}
                <span class="model-chip">
                  {model.name}<em>{(provider.config.default_model || provider.config.models?.[0]?.name) === model.name ? '默认' : ''}</em>
                </span>
              {/each}
              {#if modelSummary.hidden > 0}<span class="model-chip model-overflow">+{modelSummary.hidden}</span>{/if}
              {#if canManageProvider}<span
                  class="icon-action provider-models-action"
                  role="button"
                  tabindex="0"
                  aria-label={`编辑${provider.name}`}
                  title={`编辑${provider.name}`}
                  on:click|stopPropagation={() => editProviderFromList(provider)}
                  on:keydown|stopPropagation={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      editProviderFromList(provider);
                    }
                  }}
                  ><Pencil size={14} /></span
                ><span
                  class="icon-action provider-models-action provider-delete-action"
                  role="button"
                  tabindex="0"
                  aria-label={`删除${provider.name}`}
                  title={`删除${provider.name}`}
                  on:click|stopPropagation={() => void deleteProvider(provider)}
                  on:keydown|stopPropagation={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      void deleteProvider(provider);
                    }
                  }}
                  ><Trash2 size={14} /></span
                >{/if}
            </span>
          </button
          >{:else}<div class="empty-state">
            {#if providerSearch.trim()}没有匹配的渠道。{:else}当前级别暂无渠道，添加一个渠道开始配置。{/if}
          </div>{/each}
      </section>
      {#if drawerOpen}
        <aside class="provider-drawer panel">
          <header class="drawer-heading">
            <div class="drawer-title">
              <div>
                <h2>
                  {editingProvider
                    ? providerDraft.id
                      ? providerDraft.name || '编辑 AI 渠道'
                      : '新增 AI 渠道'
                    : selectedProvider?.name}
                </h2>
                {#if editingProvider || selectedProvider}<small
                    >{`${(editingProvider ? providerTypeLabel(providerDraft.providerType) : providerTypeLabel(selectedProvider?.config.provider_type ?? '')).toLowerCase()} · ${editingProvider ? providerDraft.defaultModel || providerDraft.models[0]?.name || '未设置默认模型' : selectedProvider?.config.default_model || selectedProvider?.config.models?.[0]?.name || '未设置默认模型'}`}</small
                  >{/if}
              </div>
            </div>
            {#if providerEditorMessage}<div class="drawer-heading-message">
                <MessageBanner
                  message={providerEditorMessage}
                  tone={providerEditorMessageTone}
                />
              </div>{/if}
            {#if editingProvider}<div class="drawer-heading-actions">
                <button
                  class="secondary"
                  type="button"
                  on:click={cancelProviderEdit}><X size={14} />取消</button
                ><button class="primary" type="submit" form="provider-editor" disabled={busy}
                  ><Save size={14} />保存</button
                >
              </div>{:else if selectedProvider}<div class="drawer-heading-actions">
                {#if canManageProvider}<button
                    class="secondary"
                    type="button"
                    on:click={openEditProvider}><Pencil size={14} />编辑</button
                  ><button
                    class="secondary danger-action"
                    type="button"
                    on:click={() => selectedProvider && void deleteProvider(selectedProvider)}
                    ><Trash2 size={14} />删除</button
                  >{/if}
              </div>{/if}
          </header>
          {#if editingProvider}
            <form id="provider-editor" class="drawer-form" novalidate on:submit|preventDefault={saveProvider}>
              <div class="drawer-section">
                <div class="section-title">
                  <h3>基本信息</h3>
                </div>
                <div class="provider-identity-row">
                  <div class="provider-identity-primary">
                  <div class="provider-icon-field">
                    <span class="field-label">图标</span><IconPicker
                      value={providerDraft.icon}
                      onSelect={(icon) => (providerDraft.icon = icon)}
                      ariaLabel="选择图标"
                    />
                  </div>
                  <label class="provider-name-field" class:invalid={Boolean(providerErrors.name)}
                    ><span class="field-label"><i>*</i>名称</span><input
                      bind:value={providerDraft.name}
                      aria-invalid={Boolean(providerErrors.name)}
                      required
                      placeholder="名称"
                    /></label
                  >
                  <label class="provider-level-field"
                    ><span class="field-label">级别</span><span class="scope-select-wrap"
                      ><select
                        bind:value={providerDraft.scopeId}
                        disabled={Boolean(providerDraft.id)}
                        aria-label="渠道级别"
                      >{#each providerScopeOptions(scopeId, providerDraft.scopeId) as option}<option value={option.id}>{scopeLevelLabel(option.type)}</option>{/each}</select
                      ><ChevronDown size={13} aria-hidden="true" /></span
                    ></label
                  >
                  </div>
                  <div class="provider-identity-secondary">
                  <div class="provider-tag-editor">
                    <span class="field-label">场景</span>
                    <div class="tag-row">
                      {#each scenarioTags as tag}<button
                          class={`tag-chip ${capabilityTagClass(tag.value)}`}
                          class:active={isBound(providerDraft.tags, tag.value)}
                          type="button"
                          disabled={!canManageProvider || busy}
                          on:click={() => toggleProviderTag(tag.value)}
                          >{tag.label}</button
                        >{/each}
                    </div>
                  </div>
                  <div class="provider-connection-test"
                    ><span class="field-label">连接</span><button
                      class="connection-status-button"
                      class:testing={Boolean(selectedProvider?.id && testingProviders[selectedProvider.id])}
                      class:connection-success={providerConnectionSummary(selectedProvider).tone === 'success'}
                      class:connection-danger={providerConnectionSummary(selectedProvider).tone === 'danger'}
                      class:connection-muted={providerConnectionSummary(selectedProvider).tone === 'muted'}
                      type="button"
                      title={Boolean(selectedProvider?.id && testingProviders[selectedProvider.id]) ? '连接测试中' : '点击进行连接测试'}
                      disabled={!providerDraft.id || Boolean(selectedProvider?.id && testingProviders[selectedProvider.id])}
                      on:click|stopPropagation={() => testProvider()}
                      >{Boolean(selectedProvider?.id && testingProviders[selectedProvider.id]) ? '测试中' : providerConnectionSummary(selectedProvider).label}</button
                    ><small class="connection-age">{formatConnectionAge(selectedProvider?.last_connection_test?.checked_at)}</small></div
                  ><div class="provider-status-switch"
                    ><span class="field-label">状态</span><span class="switch-control"
                      ><input
                        type="checkbox"
                        checked={providerDraft.status === 'active'}
                        on:change={(event) => {
                          const enabled = (
                            event.currentTarget as HTMLInputElement
                          ).checked;
                          providerDraft.status = enabled ? 'active' : 'disabled';
                          providerDraft.enabled = enabled;
                        }}
                        aria-label="启用渠道"
                      /><i aria-hidden="true"></i></span
                    ></div
                  >
                  </div>
                </div>
                <div class="provider-runtime-grid">
                  <label class:invalid={Boolean(providerErrors.timeoutSeconds)}
                    >超时（秒）<input
                      bind:value={providerDraft.timeoutSeconds}
                      aria-invalid={Boolean(providerErrors.timeoutSeconds)}
                      type="number"
                      min="1"
                      max="300"
                    /></label
                  ><label class:invalid={Boolean(providerErrors.maxConcurrency)}
                    >最大并发<input
                      bind:value={providerDraft.maxConcurrency}
                      aria-invalid={Boolean(providerErrors.maxConcurrency)}
                      type="number"
                      min="1"
                    /></label
                  ><label class:invalid={Boolean(providerErrors.rateLimitPerMinute)}
                    >限流（请求 / 分钟）<input
                      bind:value={providerDraft.rateLimitPerMinute}
                      aria-invalid={Boolean(providerErrors.rateLimitPerMinute)}
                      type="number"
                      min="0"
                    /></label
                  >
                </div>
              </div>
              <div class="drawer-section">
                <div class="section-title">
                  <h3>连接凭据</h3>
                </div>
                <div class="provider-connection-row">
                  <label class:invalid={Boolean(providerErrors.providerType)}
                    ><span><i>*</i>类型</span>
                    <div class="provider-type-picker">
                      <button
                        class="provider-type-trigger"
                        class:open={providerTypeMenuOpen}
                        type="button"
                        aria-haspopup="listbox"
                        aria-expanded={providerTypeMenuOpen}
                        on:click|stopPropagation={() => (providerTypeMenuOpen = !providerTypeMenuOpen)}
                      ><ProviderBrandIcon
                          providerType={providerDraft.providerType}
                          label={providerTypeLabel(providerDraft.providerType)}
                          size={16}
                        /><span>{providerTypeLabel(providerDraft.providerType)}</span><span class="provider-type-chevron"><ChevronDown size={13} /></span></button
                      >{#if providerTypeMenuOpen}<div class="provider-type-options" role="listbox">
                          {#each providerTypeOptions as option}<button
                              class="provider-type-option"
                              class:selected={option.value === providerDraft.providerType}
                              type="button"
                              role="option"
                              aria-selected={option.value === providerDraft.providerType}
                              on:click|stopPropagation={() => selectProviderType(option.value)}
                            ><ProviderBrandIcon providerType={option.value} label={option.label} size={16} /><span>{option.label}</span></button
                          >{/each}
                        </div>{/if}
                    </div>
                  </label
                  ><label class:invalid={Boolean(providerErrors.baseUrl)}
                    ><span><i>*</i>Base URL</span><input
                      bind:value={providerDraft.baseUrl}
                      aria-invalid={Boolean(providerErrors.baseUrl)}
                      required
                      placeholder="https://api.example.com/v1"
                    /></label
                  >
                </div>
                <div class="provider-connection-row">
                  <label
                    >协议<div class="provider-type-picker protocol-picker">
                      <button
                        class="provider-type-trigger"
                        class:open={protocolMenuOpen}
                        type="button"
                        aria-haspopup="listbox"
                        aria-expanded={protocolMenuOpen}
                        on:click|stopPropagation={() => (protocolMenuOpen = !protocolMenuOpen)}
                      ><span>{protocolLabel(providerDraft.protocol)}</span><span class="provider-type-chevron"><ChevronDown size={13} /></span></button
                      >{#if protocolMenuOpen}<div class="provider-type-options" role="listbox">
                          {#each [{ value: 'chat_completions', label: 'Chat Completions' }, { value: 'messages', label: 'Messages API' }] as option}<button
                              class="provider-type-option"
                              class:selected={option.value === providerDraft.protocol}
                              type="button"
                              role="option"
                              aria-selected={option.value === providerDraft.protocol}
                              on:click|stopPropagation={() => selectProtocol(option.value)}
                            ><span>{option.label}</span></button
                          >{/each}
                        </div>{/if}
                    </div></label
                  ><label
                    >API Key<PasswordInput
                      bind:value={providerDraft.apiKey}
                      placeholder={providerDraft.id
                        ? '留空表示保持现有凭据'
                        : '输入 API Key'}
                      autocomplete="new-password"
                      ariaLabel="API Key"
                      secretLabel="API Key"
                    /></label
                  >
                </div>
              </div>
              <div class="drawer-section">
                <div class="section-title">
                  <h3>模型能力</h3>
                </div>
                <div class="model-editor-list">
                  <div class="model-columns-header" aria-hidden="true">
                    <span><i>*</i>模型</span><span>能力</span><span><i>*</i>上下文</span><span>最大输出</span><span>温度</span><span>默认</span><span>状态</span>
                  </div>
                  {#each providerDraft.models as model, index}<div
                      class="model-editor model-card"
                      class:is-default={providerDraft.defaultModel === model.name}
                    >
                      <div class="model-primary-row">
                        <label class="model-name-field" class:invalid={Boolean(providerErrors[`models.${index}.name`])}
                          ><span class="field-label"><i>*</i>模型</span><input
                            bind:value={model.name}
                            aria-invalid={Boolean(providerErrors[`models.${index}.name`])}
                            required
                          /></label
                        ><div class="model-capability-editor">
                          <span class="field-label">能力</span>
                          <div class="tag-row">
                            {#each modelTags as tag}<button
                                class={`tag-chip ${capabilityTagClass(tag.value)}`}
                                class:active={model.tags.includes(tag.value)}
                                type="button"
                                on:click={() => toggleModelTag(model, tag.value)}
                                >{tag.label}</button
                              >{/each}
                          </div>
                        </div>
                        <label class="model-runtime-field" class:invalid={Boolean(providerErrors[`models.${index}.context`])}
                          ><span class="field-label"><i>*</i>上下文</span><input
                            bind:value={model.context_window_tokens}
                            aria-invalid={Boolean(providerErrors[`models.${index}.context`])}
                            required
                            type="number"
                            min="1"
                          /></label
                        ><label class="model-runtime-field" class:invalid={Boolean(providerErrors[`models.${index}.maxOutput`])}
                          ><span class="field-label">最大输出</span><input
                            bind:value={model.max_output_tokens}
                            aria-invalid={Boolean(providerErrors[`models.${index}.maxOutput`])}
                            type="number"
                            min="1"
                          /></label
                        ><label class="model-runtime-field" class:invalid={Boolean(providerErrors[`models.${index}.temperature`])}
                          ><span class="field-label">温度</span><input
                            bind:value={model.temperature}
                            aria-invalid={Boolean(providerErrors[`models.${index}.temperature`])}
                            type="number"
                            min="0"
                            max="2"
                            step="0.1"
                          /></label
                        ><label class="model-default-control"
                          ><span class="field-label">默认</span><input
                            type="radio"
                            name="default-model"
                            checked={providerDraft.defaultModel === model.name}
                            on:change={() => setDraftDefault(model)}
                          /></label
                        ><label class="model-enabled-control"
                          ><span class="field-label">状态</span><span
                            class="switch-control"
                            ><input
                              type="checkbox"
                              checked={model.enabled !== false}
                              on:change={(event) => {
                                model.enabled = (
                                  event.currentTarget as HTMLInputElement
                                ).checked;
                                providerDraft.models = providerDraft.models;
                              }}
                              aria-label="启用模型"
                            /><i aria-hidden="true"></i></span
                          ></label
                        >
                      </div>
                      <button
                        class="icon-action model-remove"
                        type="button"
                        title="删除模型"
                        aria-label="删除模型"
                        on:click={() => removeModel(index)}
                        disabled={providerDraft.models.length === 1}
                        ><X size={14} /></button
                      >
                    </div>{/each}<button
                    class="secondary add-model"
                    type="button"
                    on:click={addModel}><Plus size={14} />添加模型</button
                  >
                </div>
              </div>
            </form>
          {:else if selectedProvider}
            <div class="drawer-section provider-preview-section">
              <div class="section-title">
                <h3>基本信息</h3>
              </div>
              <div class="provider-preview-identity-row">
                <div class="provider-preview-field">
                  <span class="field-label">图标</span><span
                    class="provider-preview-value provider-preview-icon"><IconValue
                      value={selectedProvider.config.icon || 'lucide:Bot'}
                      size={18}
                    /></span
                  >
                </div>
                <div class="provider-preview-field">
                  <span class="field-label"><i>*</i>名称</span><strong
                    class="provider-preview-value"
                    >{selectedProvider.name}</strong
                  >
                </div>
                <div class="provider-preview-field provider-preview-level-field">
                  <span class="field-label">级别</span><strong
                    class="provider-preview-value provider-preview-plain"
                    >{scopeLevelLabel(scopeType(selectedProvider.scope_id))}</strong
                  >
                </div>
                <div class="provider-preview-field provider-preview-tags">
                  <span class="field-label">场景</span><span class="tag-row"
                    >{#each scenarioTags as tag}<span
                        class={`tag-chip ${capabilityTagClass(tag.value)}`}
                        class:active={isBound(selectedProvider.tags, tag.value)}
                        class:preview-tag-hidden={!isBound(selectedProvider.tags, tag.value)}
                        aria-hidden={!isBound(selectedProvider.tags, tag.value)}
                        >{tag.label}</span
                      >{/each}</span
                  >
                </div>
                <div class="provider-preview-field provider-preview-connection-test">
                  <span class="field-label">连接</span><span
                    class="provider-preview-value provider-preview-connection-value"><button
                      class="connection-status-button"
                      class:testing={Boolean(testingProviders[selectedProvider.id])}
                      class:connection-success={providerConnectionSummary(selectedProvider).tone === 'success'}
                      class:connection-danger={providerConnectionSummary(selectedProvider).tone === 'danger'}
                      class:connection-muted={providerConnectionSummary(selectedProvider).tone === 'muted'}
                      type="button"
                      title={Boolean(testingProviders[selectedProvider.id]) ? '连接测试中' : '点击进行连接测试'}
                      disabled={Boolean(testingProviders[selectedProvider.id])}
                      on:click|stopPropagation={() => testProvider()}
                      >{Boolean(testingProviders[selectedProvider.id]) ? '测试中' : providerConnectionSummary(selectedProvider).label}</button
                    ><small class="connection-age">{formatConnectionAge(selectedProvider.last_connection_test?.checked_at)}</small></span
                  >
                </div>
                <div class="provider-preview-field provider-preview-status">
                  <span class="field-label">状态</span><span
                    class="provider-preview-value provider-preview-switch-value"><span class="switch-control"
                      ><input
                        type="checkbox"
                        checked={selectedProvider.status === 'active'}
                        disabled
                        aria-label="渠道状态"
                      /><i aria-hidden="true"></i></span
                    ></span
                  >
                </div>
              </div>
              <div class="provider-runtime-grid">
                <div class="provider-preview-field">
                  <span class="field-label">超时（秒）</span><strong
                    class="provider-preview-value"
                    >{selectedProvider.config.timeout_seconds ?? 60}</strong
                  >
                </div>
                <div class="provider-preview-field">
                  <span class="field-label">最大并发</span><strong
                    class="provider-preview-value"
                    >{selectedProvider.config.max_concurrency ?? '未限制'}</strong
                  >
                </div>
                <div class="provider-preview-field">
                  <span class="field-label">限流（请求 / 分钟）</span><strong
                    class="provider-preview-value"
                    >{selectedProvider.config.rate_limit_per_minute ?? 0}</strong
                  >
                </div>
              </div>
            </div>
            <div class="drawer-section provider-preview-section">
              <div class="section-title">
                <h3>连接凭据</h3>
              </div>
              <div class="provider-connection-row">
                <div class="provider-preview-field">
                  <span class="field-label"><i>*</i>类型</span><strong
                    class="provider-preview-value provider-preview-type"
                    ><ProviderBrandIcon
                      providerType={selectedProvider.config.provider_type}
                      label={providerTypeLabel(selectedProvider.config.provider_type)}
                      size={16}
                    /><span>{providerTypeLabel(selectedProvider.config.provider_type)}</span></strong
                  >
                </div>
                <div class="provider-preview-field">
                  <span class="field-label"><i>*</i>Base URL</span><strong
                    class="provider-preview-value mono"
                    >{selectedProvider.config.base_url}</strong
                  >
                </div>
              </div>
              <div class="provider-connection-row">
                <div class="provider-preview-field">
                  <span class="field-label">协议</span><strong
                    class="provider-preview-value"
                    >{selectedProvider.config.protocol || 'Chat Completions'}</strong
                  >
                </div>
                <div class="provider-preview-field">
                  <span class="field-label">API Key</span><strong
                    class="provider-preview-value"
                    >服务端加密保存</strong
                  >
                </div>
              </div>
            </div>
            <div class="drawer-section provider-preview-section">
              <div class="section-title">
                <h3>模型能力</h3>
              </div>
              <div class="model-editor-list">
                <div class="model-columns-header" aria-hidden="true">
                  <span><i>*</i>模型</span><span>能力</span><span><i>*</i>上下文</span><span>最大输出</span><span>温度</span><span>默认</span><span>状态</span>
                </div>
                {#each selectedProvider.config.models ?? [] as model}
                  {@const selectedModelTags = new Set(modelTagsOf(model))}
                  <div
                    class="model-editor model-card model-preview"
                    class:is-default={(selectedProvider.config.default_model || selectedProvider.config.models[0]?.name) === model.name}
                  >
                    <div class="model-primary-row">
                      <div class="model-name-field provider-preview-field">
                        <span class="field-label"><i>*</i>模型</span><strong
                          class="provider-preview-value"
                          >{model.name}</strong
                        >
                      </div>
                      <div class="model-capability-editor">
                        <span class="field-label">能力</span>
                        <div class="tag-row">
                          {#each modelTags as tag}<span
                              class="tag-chip"
                              class:active={selectedModelTags.has(tag.value)}
                              class:preview-tag-hidden={!selectedModelTags.has(tag.value)}
                              aria-hidden={!selectedModelTags.has(tag.value)}
                              >{tag.label}</span
                            >{/each}
                        </div>
                        </div>
                      <div class="model-runtime-field provider-preview-field">
                        <span class="field-label"><i>*</i>上下文</span><strong
                          class="provider-preview-value"
                          >{model.context_window_tokens?.toLocaleString()}</strong
                        >
                      </div>
                      <div class="model-runtime-field provider-preview-field">
                        <span class="field-label">最大输出</span><strong
                          class="provider-preview-value"
                          >{model.max_output_tokens?.toLocaleString() ||
                            '未限制'}</strong
                        >
                      </div>
                      <div class="model-runtime-field provider-preview-field">
                        <span class="field-label">温度</span><strong
                          class="provider-preview-value"
                          >{model.temperature ?? 0.2}</strong
                        >
                      </div>
                      <div class="model-default-control">
                        <span class="field-label">默认</span><input
                          type="radio"
                          name="preview-default-model"
                          checked={(selectedProvider.config.default_model ||
                            selectedProvider.config.models[0]?.name) === model.name}
                          disabled
                          tabindex="-1"
                          aria-label="默认模型"
                        />
                      </div>
                      <div class="model-enabled-control">
                        <span class="field-label">状态</span><span
                          class="switch-control"
                          ><input
                            type="checkbox"
                            checked={model.enabled !== false}
                            disabled
                            aria-label="模型状态"
                          /><i aria-hidden="true"></i></span
                        >
                      </div>
                    </div>
                  </div>{/each}
              </div>
            </div>
          {:else}<div class="empty-state">选择一个渠道查看详情。</div>{/if}
        </aside>
      {:else}<div class="drawer-placeholder">
          <Cpu size={22} /><span>选择渠道打开详情抽屉</span>
        </div>{/if}
    </div>
  </section>
</section>

<style>
  .llm-page {
    display: grid;
    gap: 28px;
    padding-top: 27px;
  }
  .section-heading,
  .drawer-heading,
  .list-toolbar,
  .editor-heading,
  .section-title,
  .model-editor-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }
  .eyebrow {
    margin: 0 0 7px;
    color: var(--theme-accent);
    font-size: 10px;
    font-weight: 800;
  }
  .engine-section,
  .provider-section {
    display: grid;
    gap: 14px;
  }
  .provider-section {
    order: 1;
  }
  .engine-section {
    order: 2;
  }
  .section-heading {
    align-items: end;
  }
  .section-heading h2 {
    color: var(--theme-fg-strong);
  }
  .heading-code {
    margin-left: 5px;
    color: var(--theme-fg-muted);
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.04em;
    vertical-align: middle;
  }
  .section-heading p:not(.eyebrow) {
    max-width: 760px;
    margin: 5px 0 0;
    color: var(--theme-fg-muted);
    font-size: 12px;
    line-height: 1.5;
  }
  .section-actions {
    display: flex;
    gap: 8px;
  }
  button.primary,
  button.secondary,
  .quiet-button,
  .icon-action,
  .text-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    border: 1px solid var(--theme-border);
    border-radius: 5px;
  }
  button.primary,
  button.secondary {
    min-height: 37px;
    padding: 0 14px;
    font-size: 12px;
    font-weight: 700;
  }
  button.primary {
    color: var(--theme-accent-contrast);
    background: var(--color-primary);
    border-color: var(--color-primary);
  }
  button.primary:hover {
    background: var(--color-primary-hover);
  }
  button.secondary {
    color: var(--theme-fg);
    background: var(--theme-bg-surface);
  }
  button.secondary:hover,
  .quiet-button:hover,
  .icon-action:hover {
    color: var(--theme-accent);
    border-color: var(--theme-border-focus);
  }
  .engine-directory {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px;
  }
  .engine-card,
  .provider-list,
  .provider-drawer,
  .engine-editor,
  .drawer-placeholder {
    color: var(--theme-fg);
    background: var(--theme-bg-surface);
    border: 1px solid var(--theme-border);
    border-radius: 7px;
    box-shadow: var(--theme-shadow-soft);
  }
  .engine-card {
    display: grid;
    gap: 12px;
    min-width: 0;
    padding: 14px;
  }
  .engine-card.selected {
    border-color: var(--theme-accent);
    box-shadow:
      inset 3px 0 0 var(--theme-accent),
      var(--theme-shadow-soft);
  }
  .engine-select {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: start;
    gap: 9px;
    padding: 0;
    color: inherit;
    background: transparent;
    border: 0;
    text-align: left;
  }
  .entity-icon {
    display: inline-grid;
    place-items: center;
    flex: 0 0 auto;
    width: 36px;
    height: 36px;
    color: var(--theme-accent);
    background: var(--theme-bg-accent-soft);
    border-radius: 7px;
  }
  .provider-icon {
    color: var(--theme-teal);
    background: var(--theme-teal-soft);
  }
  .engine-copy {
    min-width: 0;
  }
  .engine-copy strong {
    display: block;
    overflow: hidden;
    color: var(--theme-fg-strong);
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .engine-copy small {
    display: block;
    margin-top: 4px;
    overflow: hidden;
    color: var(--theme-fg-muted);
    font-size: 10px;
    line-height: 1.4;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .engine-state {
    color: var(--theme-success);
    font-size: 10px;
    white-space: nowrap;
  }
  .engine-capabilities,
  .tag-row,
  .model-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
  }
  .engine-capabilities {
    max-height: 53px;
    overflow: hidden;
    align-content: flex-start;
  }
  .engine-capabilities span {
    min-width: 0;
    overflow: hidden;
    padding: 4px 6px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border-radius: 4px;
    font-size: 10px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tag-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    min-height: 24px;
    padding: 0 7px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border: 1px solid transparent;
    border-radius: 4px;
    font-size: 10px;
  }
  .tag-chip:hover:not(:disabled),
  .tag-chip.active {
    color: var(--theme-action);
    background: var(--theme-bg-accent-soft);
    border-color: var(--theme-border-focus);
  }
  .tag-chip.tag-general.active {
    color: var(--theme-tag-general-fg);
    background: var(--theme-tag-general-bg);
    border-color: var(--theme-tag-general-border);
  }
  .tag-chip.tag-diagnosis.active {
    color: var(--theme-tag-diagnosis-fg);
    background: var(--theme-tag-diagnosis-bg);
    border-color: var(--theme-tag-diagnosis-border);
  }
  .tag-chip.tag-inspection.active {
    color: var(--theme-tag-inspection-fg);
    background: var(--theme-tag-inspection-bg);
    border-color: var(--theme-tag-inspection-border);
  }
  .tag-chip.tag-workflow.active {
    color: var(--theme-tag-workflow-fg);
    background: var(--theme-tag-workflow-bg);
    border-color: var(--theme-tag-workflow-border);
  }
  .tag-chip:disabled {
    cursor: not-allowed;
    opacity: 0.65;
  }
  .engine-tag-line {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-width: 0;
  }
  .icon-action,
  .quiet-button {
    width: 30px;
    height: 30px;
    padding: 0;
    color: var(--theme-fg-muted);
    background: transparent;
    border-color: transparent;
  }
  .panel {
    padding: 0;
  }
  .provider-workspace {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 3fr);
    gap: 11px;
    align-items: stretch;
  }
  .provider-list {
    min-width: 0;
    height: 800px;
    min-height: 800px;
    overflow: auto;
    padding: 0 17px 14px;
  }
  .list-toolbar {
    align-items: center;
    height: 60px;
    min-height: 60px;
    overflow: hidden;
    padding: 10px 17px;
    margin: 0 -17px;
    color: var(--theme-fg-strong);
    border-bottom: 1px solid var(--theme-divider);
    font-size: 12px;
  }
  .list-toolbar-actions {
    display: flex;
    flex: 1 1 auto;
    align-items: center;
    gap: 2px;
    min-width: 0;
    margin-left: auto;
  }
  .list-toolbar-actions .icon-button {
    width: 29px;
    min-width: 29px;
    height: 29px;
    min-height: 29px;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--theme-fg-muted);
  }
  .list-toolbar-actions .icon-button:hover {
    color: var(--theme-accent);
    background: var(--theme-bg-hover);
  }
  :global(:root[data-theme='dark']) .list-toolbar-actions .icon-button:hover {
    color: var(--theme-teal);
    background: var(--theme-bg-accent-soft);
  }
  .provider-search {
    flex: 1 1 auto;
    width: auto;
    min-width: 100px;
    max-width: none;
    min-height: 30px;
    padding: 6px 9px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    font-size: 11px;
  }
  .provider-search:focus {
    border-color: var(--theme-border-focus);
    outline: none;
  }
  .list-toolbar strong {
    display: block;
    flex: 0 0 auto;
    font-size: 14px;
    line-height: 14px;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
  }
  .list-toolbar strong::after {
    display: block;
    margin-top: 4px;
    color: var(--theme-fg-muted);
    content: '选择渠道查看详情';
    font-size: 10px;
    font-weight: 400;
    line-height: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .provider-list > .provider-row {
    width: 100%;
    margin-top: 7px;
  }
  .provider-row {
    position: relative;
    display: grid;
    grid-template-columns: minmax(0, 1fr) max-content;
    align-items: stretch;
    gap: 12px;
    width: 100%;
    min-height: 0;
    padding: 13px 12px;
    color: var(--theme-fg);
    background: var(--theme-bg-surface);
    border: 1px solid var(--theme-border);
    border-radius: 7px;
    text-align: left;
  }
  .provider-row:hover,
  .provider-row.selected {
    background: var(--theme-bg-hover);
    border-color: var(--theme-accent);
  }
  .provider-row.selected::before {
    position: absolute;
    top: 6px;
    bottom: 6px;
    left: -1px;
    width: 4px;
    border-radius: 0 2px 2px 0;
    background: var(--theme-action);
    content: '';
  }
  .provider-row.provider-disabled .entity-icon {
    filter: grayscale(1);
    opacity: 0.58;
  }
  .provider-row .entity-icon {
    width: 40px;
    height: 40px;
    background: transparent;
    border: 0;
    border-radius: 0;
  }
  .provider-row .provider-icon {
    color: var(--theme-teal);
  }
  .provider-title {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    width: 100%;
    min-width: 0;
  }
  .provider-title > span:last-child {
    min-width: 0;
  }
  .provider-title-copy {
    display: grid;
    flex: 1 1 auto;
    gap: 3px;
    min-width: 0;
    width: 100%;
  }
  .provider-name-line {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    max-width: 100%;
    width: 100%;
  }
  .provider-title strong {
    display: block;
    min-width: 0;
    overflow: hidden;
    color: var(--theme-fg-strong);
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .provider-title small {
    display: block;
    overflow: hidden;
    color: var(--theme-fg-muted);
    font-size: 10px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .provider-scenario-tags {
    display: flex;
    flex: 0 1 auto;
    flex-wrap: nowrap;
    gap: 4px;
    min-width: 0;
    overflow: hidden;
  }
  .provider-scenario-tags .tag-chip {
    flex: 0 0 auto;
    min-height: 20px;
    padding: 0 6px;
  }
  .provider-row-head {
    display: grid;
    width: 100%;
    min-width: 0;
    gap: 8px;
  }
  .provider-row-meta {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    width: max-content;
    max-width: 100%;
    min-width: 0;
  }
  .provider-row-meta > span {
    width: max-content;
    max-width: 100%;
  }
  .provider-row-meta > span,
  .provider-health {
    display: grid;
    align-content: center;
    justify-items: end;
    gap: 3px;
    color: var(--theme-fg-muted);
    font-size: 10px;
    text-align: right;
  }
  .provider-health {
    cursor: pointer;
  }
  .provider-health:focus-visible {
    outline: 2px solid var(--theme-border-focus);
    outline-offset: 2px;
    border-radius: 999px;
  }
  .provider-row-meta strong {
    color: var(--theme-fg-strong);
    font-size: 11px;
  }
  .scope-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: auto;
    min-width: 0;
    min-height: 20px;
    padding: 0 7px;
    border-radius: 4px;
    font-size: 10px;
    font-weight: 700;
    line-height: 1;
    white-space: nowrap;
  }
  .scope-badge.scope-platform {
    color: var(--theme-scope-platform-fg);
    background: var(--theme-scope-platform-bg);
    border: 1px solid var(--theme-scope-platform-border);
  }
  .scope-badge.scope-team {
    color: var(--theme-scope-team-fg);
    background: var(--theme-scope-team-bg);
    border: 1px solid var(--theme-scope-team-border);
  }
  .scope-badge.scope-project {
    color: var(--theme-scope-project-fg);
    background: var(--theme-scope-project-bg);
    border: 1px solid var(--theme-scope-project-border);
  }
  .provider-row-meta strong.scope-badge {
    min-height: 20px;
  }
  .provider-row-meta small {
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .provider-health strong {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    max-width: 180px;
    min-height: 21px;
    overflow: hidden;
    padding: 0 7px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border-radius: 999px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .provider-health.connection-success strong {
    color: var(--theme-success);
    background: var(--theme-bg-success-soft);
  }
  .provider-health.connection-danger strong {
    color: var(--theme-danger);
    background: var(--theme-bg-danger-soft);
  }
  .provider-health.testing strong {
    color: var(--theme-accent-contrast);
    background: var(--theme-accent-hover);
  }
  .provider-health:hover strong {
    box-shadow: 0 0 0 1px var(--theme-border-focus);
  }
  .provider-row-foot {
    display: flex;
    grid-column: 1 / -1;
    align-items: center;
    justify-content: space-between;
    gap: 9px;
    min-width: 0;
    padding-top: 2px;
  }
  .provider-default-capabilities {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
    min-width: 0;
  }
  .provider-default-capabilities .tag-chip {
    min-height: 22px;
    padding: 0 6px;
  }
  .provider-row-models {
    display: flex;
    grid-column: 1 / -1;
    align-items: center;
    flex-wrap: nowrap;
    gap: 5px;
    height: 31px;
    min-width: 0;
    overflow: hidden;
    padding-top: 8px;
    border-top: 1px solid var(--theme-divider);
  }
  .provider-row-models .model-chip {
    flex: 0 1 auto;
    max-width: min(230px, 40%);
  }
  .provider-row-models .model-overflow {
    flex: 0 0 auto;
    max-width: none;
  }
  .provider-row-models > .provider-models-action {
    margin-left: 0;
  }
  .provider-row-models > .provider-models-action:not(.provider-delete-action) {
    margin-left: auto;
  }
  .provider-models-action {
    flex: 0 0 30px;
    margin-left: auto;
  }
  .provider-models-action:hover,
  .provider-models-action:focus,
  .provider-models-action:focus-visible,
  .provider-models-action:active {
    color: var(--theme-accent);
    background: transparent;
    border-color: transparent;
    outline: none;
    box-shadow: none;
  }
  :global(:root[data-theme='dark']) .provider-models-action:hover,
  :global(:root[data-theme='dark']) .provider-models-action:focus,
  :global(:root[data-theme='dark']) .provider-models-action:focus-visible,
  :global(:root[data-theme='dark']) .provider-models-action:active {
    color: var(--theme-teal);
    background: var(--theme-bg-accent-soft);
  }
  .model-chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    min-height: 22px;
    max-width: 100%;
    padding: 0 7px;
    overflow: hidden;
    color: var(--theme-teal);
    background: var(--theme-teal-soft);
    border-radius: 5px;
    font-size: 10px;
    font-weight: 700;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .model-chip em {
    flex: 0 0 auto;
    white-space: nowrap;
    color: var(--theme-fg-muted);
    font-style: normal;
    font-weight: 400;
  }
  .provider-drawer {
    position: sticky;
    top: 16px;
    min-width: 0;
    height: 800px;
    min-height: 800px;
    max-height: calc(100vh - 32px);
    overflow: auto;
  }
  .drawer-heading {
    position: relative;
    align-items: center;
    height: 60px;
    min-height: 60px;
    padding: 12px 15px;
    border-bottom: 1px solid var(--theme-divider);
  }
  .drawer-heading-actions {
    display: flex;
    align-items: center;
    gap: 7px;
    margin-left: auto;
  }
  .drawer-heading-message,
  .editor-heading-message {
    position: absolute;
    z-index: 2;
    top: 50%;
    left: 50%;
    width: min(420px, calc(100% - 160px));
    transform: translate(-50%, -50%);
  }
  .drawer-heading-message :global(.message-banner),
  .editor-heading-message :global(.message-banner) {
    width: 100%;
  }
  .drawer-title {
    display: flex;
    align-items: center;
    gap: 0;
    min-width: 0;
  }
  .drawer-title h2 {
    color: var(--theme-fg-strong);
    font-size: 14px;
    line-height: 1.25;
  }
  .drawer-title small {
    display: block;
    margin-top: 4px;
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .drawer-section {
    padding: 15px 17px;
    border-bottom: 1px solid var(--theme-divider);
  }
  .section-title {
    align-items: baseline;
    margin-bottom: 10px;
  }
  .section-title h3 {
    color: var(--theme-fg-strong);
    font-size: 13px;
  }
  .form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  label {
    display: grid;
    gap: 5px;
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .full {
    grid-column: 1 / -1;
  }
  label input,
  label select,
  label textarea {
    width: 100%;
    min-height: 35px;
    padding: 7px 9px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    font-size: 11px;
  }
  .drawer-form label > input,
  .drawer-form .provider-type-trigger {
    justify-self: start;
    margin-left: 0;
    text-align: left;
  }
  .field-label i,
  label > span > i {
    margin-right: 2px;
    color: var(--theme-required);
    font-style: normal;
  }
  .engine-editor label.invalid input,
  .drawer-form label.invalid input,
  .drawer-form label.invalid .provider-type-trigger {
    border-color: var(--theme-danger) !important;
    box-shadow: 0 0 0 2px var(--theme-bg-danger-soft) !important;
  }
  label textarea {
    resize: vertical;
  }
  .provider-identity-row {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    align-items: start;
    column-gap: 10px;
    row-gap: 12px;
    min-height: 58px;
    min-width: 0;
  }
  .provider-identity-primary {
    display: grid;
    grid-column: 1 / span 2;
    grid-template-columns: auto minmax(0, 1fr) 65px;
    align-items: start;
    gap: 10px;
    min-width: 0;
  }
  .provider-identity-secondary {
    display: grid;
    grid-column: 3;
    grid-template-columns: minmax(0, 1fr) 80px 46px;
    align-items: start;
    gap: 10px;
    min-width: 0;
  }
  .provider-preview-identity-row {
    display: grid;
    grid-template-columns: auto minmax(140px, 3fr) 65px minmax(0, 1fr) 80px 46px;
    align-items: start;
    column-gap: 10px;
    row-gap: 12px;
    min-height: 58px;
    min-width: 0;
  }
  .provider-preview-field {
    display: grid;
    min-width: 0;
    min-height: 55px;
    gap: 5px;
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .provider-preview-field .field-label {
    margin-bottom: 0;
  }
  .provider-preview-value {
    display: flex;
    align-items: center;
    width: 100%;
    min-height: 35px;
    min-width: 0;
    padding: 7px 9px;
    overflow: hidden;
    color: var(--theme-fg);
    background: transparent;
    border: 1px solid transparent;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 400;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .provider-preview-field > .provider-preview-value:not(.provider-preview-icon):not(.provider-preview-switch-value):not(.provider-preview-connection-value) {
    height: 35px;
    min-height: 35px;
    justify-content: flex-start;
    padding-right: 0;
    padding-left: 0;
    padding-top: 0;
    padding-bottom: 0;
    text-align: left;
  }
  .provider-preview-icon {
    justify-content: center;
    width: 38px;
    min-height: 38px;
    padding: 0;
    color: var(--theme-teal);
    background: transparent;
    border-color: transparent;
    transform: translate(-6px, -2px);
  }
  .provider-preview-plain {
    padding-right: 3px;
    padding-left: 3px;
  }
  .provider-preview-level-field {
    justify-items: start;
  }
  .provider-preview-level-field .field-label,
  .provider-preview-status .field-label {
    text-align: left;
  }
  .provider-preview-status {
    width: 46px;
    min-width: 46px;
    height: 35px;
    min-height: 35px;
    padding-right: 0;
    padding-left: 0;
  }
  .provider-preview-switch-value {
    justify-content: center;
    width: 46px;
    min-width: 46px;
    height: 35px;
    min-height: 35px;
    padding: 0;
  }
  .provider-preview-connection-test {
    width: 80px;
    min-width: 80px;
  }
  .provider-preview-connection-test .field-label {
    text-align: center;
  }
  .provider-preview-connection-value {
    flex-direction: column;
    gap: 2px;
    justify-content: center;
    height: 42px;
    min-height: 42px;
    padding: 0;
  }
  .provider-preview-connection-value .connection-status-button {
    width: fit-content;
    max-width: 100%;
    transform: translateY(0);
  }
  .provider-preview-type {
    gap: 7px;
  }
  .provider-preview-tags {
    justify-self: start;
  }
  .provider-preview-tags .tag-row {
    min-height: 35px;
    align-items: center;
    justify-content: flex-start;
  }
  .preview-tag-hidden {
    visibility: hidden;
    pointer-events: none;
  }
  .provider-preview-section .provider-connection-row + .provider-connection-row {
    margin-top: 10px;
  }
  .model-preview .model-capability-editor .tag-row {
    justify-content: flex-start;
  }
  .drawer-form .model-runtime-field {
    transform: translateY(-2px);
  }
  .provider-icon-field,
  .provider-name-field,
  .provider-tag-editor,
  .provider-level-field,
  .provider-status-switch {
    min-width: 0;
  }
  .provider-icon-field,
  .provider-tag-editor,
  .provider-level-field,
  .provider-status-switch,
  .provider-connection-test {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .provider-tag-editor .tag-row {
    min-height: 35px;
    align-items: center;
    justify-content: flex-start;
  }
  .provider-tag-editor,
  .provider-level-field {
    justify-self: start;
  }
  .provider-level-field {
    width: 65px;
    min-width: 65px;
  }
  .provider-level-field .scope-select-wrap {
    justify-self: start;
  }
  .provider-level-field .field-label {
    text-align: left;
  }
  .scope-select-wrap {
    position: relative;
    display: inline-flex;
    align-items: center;
    width: 65px;
    min-width: 65px;
  }
  .scope-select-wrap select {
    width: 65px;
    min-width: 65px;
    height: 35px;
    min-height: 35px;
    padding: 7px 25px 7px 9px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    font: inherit;
    font-size: 11px;
    appearance: none;
  }
  .scope-select-wrap select:hover {
    background: var(--theme-bg-input);
    border-color: var(--theme-border);
  }
  .scope-select-wrap select:focus {
    border-color: var(--theme-border-focus);
    outline: none;
    box-shadow: 0 0 0 2px var(--theme-focus);
  }
  .scope-select-wrap :global(svg) {
    position: absolute;
    right: 8px;
    pointer-events: none;
    color: var(--theme-fg-muted);
  }
  .provider-status-switch {
    display: grid;
    grid-template-rows: auto 35px;
    align-items: start;
    justify-items: center;
    gap: 5px;
    width: 46px;
    min-width: 46px;
  }
  .provider-status-switch .field-label {
    align-self: stretch;
    text-align: center;
  }
  .provider-status-switch .switch-control {
    align-self: center;
    justify-self: center;
  }
  .provider-connection-test {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 5px;
    width: 80px;
    min-width: 80px;
  }
  .provider-connection-test .field-label {
    text-align: center;
  }
  .provider-connection-test .connection-status-button {
    width: fit-content;
    max-width: 100%;
    transform: translateY(5px);
  }
  .connection-age {
    display: block;
    max-width: 100%;
    overflow: hidden;
    color: var(--theme-fg-subtle);
    font-size: 9px;
    line-height: 1.2;
    text-align: center;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .drawer-form .model-runtime-field input {
    transform: translateY(3px);
  }
  .connection-status-button {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    min-width: 0;
    max-width: 76px;
    min-height: 21px;
    overflow: hidden;
    padding: 0 7px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border: 1px solid transparent;
    border-radius: 999px;
    font-size: 10px;
    text-align: right;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .connection-status-button:hover:not(:disabled),
  .connection-status-button:focus-visible {
    border-color: currentColor;
    outline: none;
  }
  .connection-status-button.connection-success {
    color: var(--theme-success);
    background: var(--theme-bg-success-soft);
  }
  .connection-status-button.connection-danger {
    color: var(--theme-danger);
    background: var(--theme-bg-danger-soft);
  }
  .connection-status-button.connection-success:hover:not(:disabled),
  .connection-status-button.connection-success:focus-visible {
    color: var(--theme-success);
    background: var(--theme-bg-success-soft);
  }
  .connection-status-button.connection-danger:hover:not(:disabled),
  .connection-status-button.connection-danger:focus-visible {
    color: var(--theme-danger);
    background: var(--theme-bg-danger-soft);
  }
  .connection-status-button.connection-muted {
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
  }
  .connection-status-button.connection-muted:hover:not(:disabled),
  .connection-status-button.connection-muted:focus-visible {
    color: var(--theme-fg-strong);
    background: var(--theme-bg-subtle);
  }
  .connection-status-button.testing {
    color: var(--theme-accent-contrast);
    background: var(--theme-accent-hover);
  }
  .connection-status-button.testing:hover:not(:disabled),
  .connection-status-button.testing:focus-visible {
    color: var(--theme-accent-contrast);
    background: var(--theme-accent-hover);
  }
  .connection-status-button:disabled {
    cursor: not-allowed;
    opacity: 0.72;
  }
  .provider-runtime-grid,
  .provider-connection-row {
    display: grid;
    gap: 10px;
  }
  .provider-runtime-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    margin-top: 10px;
    min-height: 55px;
  }
  .provider-connection-row {
    grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
  }
  .provider-connection-row > label,
  .provider-connection-row > .provider-preview-field {
    min-height: 55px;
  }
  .provider-connection-row + .provider-connection-row {
    margin-top: 10px;
  }
  .provider-type-picker {
    position: relative;
  }
  .provider-type-trigger {
    display: flex;
    align-items: center;
    justify-content: flex-start;
    gap: 7px;
    width: 100%;
    min-height: 35px;
    padding: 7px 9px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    font-size: 11px;
    text-align: left;
  }
  .provider-type-trigger > span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .provider-type-chevron {
    display: inline-flex;
    position: absolute;
    right: 9px;
    color: var(--theme-fg-muted);
  }
  .provider-type-picker.protocol-picker {
    position: relative;
  }
  .provider-type-trigger:hover,
  .provider-type-trigger.open {
    border-color: var(--theme-border-focus);
    background: var(--theme-bg-hover);
  }
  .provider-type-options {
    position: absolute;
    z-index: 12;
    top: calc(100% + 4px);
    right: 0;
    left: 0;
    display: grid;
    max-height: 260px;
    overflow: auto;
    padding: 4px;
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border-strong);
    border-radius: 5px;
    box-shadow: var(--theme-shadow);
  }
  .provider-type-option {
    display: flex;
    align-items: center;
    gap: 7px;
    min-height: 32px;
    padding: 5px 7px;
    color: var(--theme-fg);
    background: transparent;
    border: 0;
    border-radius: 3px;
    font-size: 11px;
    text-align: left;
    justify-content: flex-start;
  }
  .provider-type-option:hover,
  .provider-type-option:focus-visible,
  .provider-type-option.selected {
    color: var(--theme-fg-strong);
    background: var(--theme-bg-selected);
    outline: none;
  }
  .model-primary-row {
    display: grid;
    grid-template-columns:
      minmax(130px, 1fr)
      minmax(0, 325px)
      repeat(3, 90px)
      36px 36px;
    align-items: center;
    column-gap: 8px;
    row-gap: 6px;
    min-height: 52px;
    min-width: 0;
  }
  .model-columns-header {
    display: grid;
    grid-template-columns:
      minmax(130px, 1fr)
      minmax(0, 325px)
      repeat(3, 90px)
      36px 36px;
    align-items: center;
    column-gap: 8px;
    min-height: 28px;
    padding: 0 12px;
    color: var(--theme-fg-subtle);
    background: var(--theme-bg-subtle);
    border-bottom: 1px solid var(--theme-divider);
    font-size: 10px;
    font-weight: 600;
  }
  .model-columns-header span:nth-last-child(-n + 2) {
    text-align: center;
  }
  .model-columns-header i {
    color: var(--theme-required);
    font-style: normal;
  }
  .model-primary-row .field-label {
    display: none;
  }
  .model-name-field,
  .model-capability-editor,
  .model-runtime-field,
  .model-default-control,
  .model-enabled-control {
    min-width: 0;
  }
  .model-runtime-field {
    display: grid;
    gap: 5px;
    width: 90px;
    min-width: 90px;
    max-width: 90px;
    transform: translateY(-2px);
  }
  .model-runtime-field + .model-runtime-field {
    margin-left: 0;
  }
  .model-capability-editor + .model-runtime-field {
    margin-left: 0;
  }
  .model-runtime-field input,
  .model-runtime-field .provider-preview-value {
    width: 90px;
    max-width: 90px;
    height: 35px;
    min-height: 35px;
    justify-self: start;
    align-self: center;
  }
  .model-runtime-field .field-label {
    margin-bottom: 0;
    text-align: left;
  }
  .model-capability-editor .tag-row {
    min-height: 35px;
    align-items: center;
    justify-content: flex-start;
    flex-wrap: nowrap;
    overflow: hidden;
  }
  .model-capability-editor {
    display: grid;
    gap: 5px;
    align-self: stretch;
    justify-self: stretch;
    max-width: 100%;
    margin-right: 0;
    overflow: hidden;
  }
  .model-capability-editor .tag-chip {
    flex: 0 0 auto;
    min-height: 24px;
  }
  .model-capability-editor .tag-chip:not(.active) {
    color: var(--theme-fg-muted);
    background: var(--theme-bg-input);
    border-color: var(--theme-border);
  }
  .model-default-control,
  .model-enabled-control {
    display: grid;
    grid-template-rows: 35px;
    align-items: start;
    justify-items: center;
    gap: 5px;
  }
  .model-default-control,
  .model-enabled-control {
    justify-self: center;
    justify-items: center;
  }
  .model-default-control {
    margin-left: 0;
  }
  .model-enabled-control {
    margin-left: 0;
  }
  .model-enabled-control .field-label {
    width: 100%;
    text-align: center;
  }
  .model-default-control input,
  .model-enabled-control .switch-control {
    align-self: center;
  }
  .model-default-control input {
    width: 16px;
    min-height: 16px;
    height: 16px;
    margin: 0;
    padding: 0;
    appearance: none;
    position: relative;
    background: transparent;
    border: 2px solid var(--color-primary);
    border-radius: 50%;
  }
  .model-default-control input::after {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 8px;
    height: 8px;
    background: var(--color-primary);
    border-radius: 50%;
    content: '';
    transform: scale(0);
    transition: transform 0.12s ease;
  }
  .model-default-control input:checked::after {
    transform: scale(1);
  }
  .model-default-control input:disabled {
    opacity: 1;
  }
  .model-default-control input[type='radio']:focus,
  .model-default-control input[type='radio']:focus-visible {
    outline: none;
    box-shadow: none;
  }
  .model-enabled-control .switch-control {
    align-self: center;
  }
  .engine-editor {
    display: grid;
    gap: 16px;
    padding: 16px;
  }
  .editor-heading {
    position: relative;
    align-items: center;
  }
  .editor-heading h3 {
    color: var(--theme-fg-strong);
  }
  .editor-heading-actions {
    display: flex;
    align-items: center;
    gap: 7px;
  }
  .engine-editor-identity-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto auto;
    align-items: start;
    gap: 12px;
    min-width: 0;
  }
  .engine-icon-field,
  .engine-name-field,
  .engine-tag-editor,
  .engine-status-switch {
    min-width: 0;
  }
  .engine-icon-field,
  .engine-tag-editor,
  .engine-status-switch {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .engine-editor-identity-row .field-label {
    margin-bottom: 0;
  }
  .engine-name-field input {
    min-width: 0;
  }
  .field-label {
    display: block;
    margin-bottom: 5px;
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .capability-field {
    display: flex;
    min-width: 0;
    flex-direction: column;
  }
  .capability-picker {
    position: relative;
    display: flex;
    min-width: 0;
    flex: 1;
  }
  .capability-chip-list {
    display: flex;
    flex: 1;
    flex-wrap: wrap;
    align-content: flex-start;
    gap: 6px;
    min-height: 35px;
    padding: 8px;
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
  }
  .capability-chip {
    min-height: 24px;
    padding: 0 8px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    font-size: 10px;
  }
  .capability-chip:hover,
  .capability-chip.active {
    color: var(--theme-action);
    background: var(--theme-bg-accent-soft);
    border-color: var(--theme-border-focus);
  }
  .capability-add {
    display: inline-grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    color: var(--theme-accent);
    background: transparent;
    border: 1px dashed var(--theme-border-focus);
    border-radius: 4px;
  }
  .capability-add:hover {
    background: var(--theme-bg-accent-soft);
  }
  .engine-description-field {
    display: flex;
    min-width: 0;
    min-height: 0;
    flex-direction: column;
  }
  .engine-description-field textarea {
    flex: 1;
    height: 100%;
    min-height: 35px;
    resize: vertical;
  }
  .engine-editor-content {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    align-items: stretch;
    gap: 12px;
    min-width: 0;
  }
  .capability-field,
  .engine-description-field {
    min-width: 0;
  }
  .capability-inline-add {
    position: relative;
    z-index: 8;
    display: grid;
    width: 180px;
    flex: 0 0 180px;
  }
  .capability-inline-add > input {
    width: 100%;
    min-height: 24px;
    padding: 6px 8px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    font-size: 11px;
  }
  .capability-inline-options {
    position: absolute;
    z-index: 1;
    top: calc(100% + 4px);
    right: 0;
    left: 0;
    display: grid;
    max-height: 190px;
    overflow: auto;
    padding: 5px;
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border-strong);
    border-radius: 5px;
    box-shadow: var(--theme-shadow);
  }
  .capability-inline-options button {
    padding: 7px 8px;
    color: var(--theme-fg);
    background: transparent;
    border: 0;
    border-radius: 3px;
    text-align: left;
    font-size: 11px;
  }
  .capability-inline-options button:hover {
    color: var(--theme-accent-contrast);
    background: var(--theme-action);
  }
  .capability-inline-options .capability-create {
    color: var(--theme-action);
    border-top: 1px solid var(--theme-divider);
  }
  .capability-inline-options .capability-empty {
    padding: 9px 8px;
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .engine-status-switch {
    display: grid;
    grid-template-rows: auto 35px;
    align-items: start;
    justify-items: end;
    gap: 5px;
    min-width: 34px;
  }
  .engine-status-switch .field-label {
    align-self: stretch;
    text-align: right;
  }
  .engine-status-switch .switch-control {
    align-self: center;
  }
  .switch-control {
    position: relative;
    display: inline-flex;
    width: 36px;
    height: 20px;
  }
  .switch-control input {
    position: absolute;
    z-index: 1;
    width: 100%;
    height: 100%;
    min-height: 0;
    opacity: 0;
    cursor: pointer;
  }
  .switch-control i {
    display: block;
    width: 36px;
    height: 20px;
    background: var(--theme-border-strong);
    border-radius: 999px;
    transition: background 0.15s ease;
  }
  .switch-control i::after {
    display: block;
    width: 16px;
    height: 16px;
    margin: 2px;
    background: var(--theme-bg-raised);
    border-radius: 50%;
    content: '';
    transition: transform 0.15s ease;
  }
  .switch-control input:checked + i {
    background: var(--color-primary);
  }
  .switch-control input:checked + i::after {
    transform: translateX(16px);
  }
  .switch-control input:focus-visible + i {
    outline: 3px solid var(--color-primary-focus);
    outline-offset: 2px;
  }
  .engine-tag-editor {
    min-width: 0;
  }
  .engine-tag-editor .tag-row {
    min-height: 35px;
    align-items: center;
  }
  .drawer-form {
    display: grid;
  }
  .muted-chip {
    padding: 5px 7px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border-radius: 4px;
    font-size: 10px;
  }
  .model-editor-list {
    display: grid;
    gap: 9px;
  }
  .model-editor {
    position: relative;
    display: grid;
    gap: 9px;
    padding: 0 12px;
    background: var(--theme-bg-surface);
    border: 1px solid var(--theme-border);
    border-radius: 5px;
  }
  .model-card {
    min-width: 0;
    overflow: visible;
  }
  .model-card.is-default {
    border-color: var(--theme-border);
    box-shadow: none;
  }

  .model-preview .model-name-field,
  .model-preview .model-runtime-field {
    min-height: 35px;
    align-self: center;
    align-content: center;
  }
  .model-preview .model-name-field .provider-preview-value,
  .model-preview .model-runtime-field .provider-preview-value {
    align-self: center;
  }
  .model-preview .model-capability-editor .tag-row {
    transform: none;
  }
  .model-editor-head {
    align-items: center;
    color: var(--theme-fg-strong);
    font-size: 11px;
  }
  .default-mark {
    display: inline-flex;
    margin-left: 6px;
    padding: 3px 5px;
    color: var(--theme-success);
    background: var(--theme-bg-success-soft);
    border-radius: 3px;
    font-size: 9px;
  }
  .model-tags {
    align-items: center;
  }
  .text-button {
    padding: 3px 5px;
    color: var(--theme-accent);
    background: transparent;
    border-color: transparent;
    font-size: 10px;
  }
  button.secondary.add-model {
    width: 100%;
    height: 15px;
    min-height: 15px;
    max-height: 15px;
    justify-content: center;
    font-size: 11px;
    line-height: 1;
  }
  .add-model :global(svg) {
    width: 12px;
    height: 12px;
  }
  .model-remove {
    position: absolute;
    z-index: 2;
    top: -4px;
    right: -4px;
    background: transparent !important;
    border: 0 !important;
    outline: 0 !important;
    box-shadow: none !important;
  }
  .model-remove:hover:not(:disabled),
  .model-remove:focus,
  .model-remove:focus-visible,
  .model-remove:active,
  .model-remove:hover:focus,
  .model-remove:hover:focus-visible {
    color: var(--theme-accent);
    background: transparent !important;
    border: 0 !important;
    outline: 0 !important;
    box-shadow: none !important;
  }
  .mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  }
  .drawer-placeholder {
    display: grid;
    place-items: center;
    gap: 8px;
    min-height: 360px;
    color: var(--theme-fg-muted);
    font-size: 12px;
  }
  .empty-state {
    padding: 28px 16px;
    color: var(--theme-fg-muted);
    text-align: center;
    font-size: 12px;
  }
  @media (max-width: 1100px) {
    .engine-directory {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    .provider-workspace {
      grid-template-columns: minmax(0, 1fr) minmax(0, 3fr);
    }
  }
  @media (max-width: 420px) {
    .section-heading {
      align-items: stretch;
      flex-direction: column;
    }
    .engine-directory,
    .provider-workspace {
      grid-template-columns: 1fr;
    }
    .provider-drawer {
      position: static;
      height: auto;
      min-height: 0;
      max-height: none;
    }
    .provider-list {
      height: auto;
      min-height: 0;
    }
    .provider-row {
      grid-template-columns: 1fr;
    }
    .provider-row-meta {
      min-width: 0;
    }
    .provider-search {
      width: auto;
    }
    .provider-workspace.drawer-open .provider-list {
      min-height: 0;
    }
    .form-grid,
    .provider-preview-identity-row,
    .engine-editor-identity-row,
    .engine-editor-content,
    .provider-identity-row,
    .provider-identity-primary,
    .provider-identity-secondary,
    .provider-runtime-grid,
    .provider-connection-row,
    .provider-level-field,
    .model-primary-row {
      grid-template-columns: 1fr;
    }
    .provider-identity-primary,
    .provider-identity-secondary {
      grid-column: auto;
    }
    .model-columns-header {
      display: none;
    }
    .model-editor {
      padding: 11px;
    }
    .model-primary-row .field-label {
      display: block;
    }
    .model-default-control,
    .model-enabled-control {
      grid-template-rows: auto 35px;
    }
    .full {
      grid-column: auto;
    }
    .editor-heading {
      flex-wrap: wrap;
    }
    .editor-heading-actions {
      margin-left: auto;
    }
    .provider-status-switch {
      justify-items: center;
      width: auto;
      min-width: 0;
    }
    .provider-connection-test,
    .provider-preview-connection-test {
      width: auto;
      min-width: 0;
    }
    .provider-level-field,
    .provider-preview-status {
      width: auto;
      min-width: 0;
    }
    .provider-status-switch .field-label {
      text-align: center;
    }
    .model-default-control {
      justify-self: center;
      justify-items: center;
    }
    .model-enabled-control {
      justify-self: center;
      justify-items: center;
    }
    .model-runtime-field {
      width: 90px;
      min-width: 90px;
      max-width: 90px;
    }
    .model-runtime-field input,
    .model-runtime-field .provider-preview-value {
      width: 90px;
      max-width: 90px;
    }
    .model-capability-editor + .model-runtime-field,
    .model-runtime-field + .model-runtime-field {
      margin-left: 0;
    }
    .model-default-control,
    .model-enabled-control {
      margin-left: 0;
    }
  }
</style>
