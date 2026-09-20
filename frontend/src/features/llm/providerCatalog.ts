export type ProviderTypeOption = {
  value: string;
  label: string;
  baseURL: string;
};

export type ProviderPurposeOption = {
  value: string;
  label: string;
  requiredCapabilities?: string[];
};

export const providerTypeOptions: ProviderTypeOption[] = [
  { value: 'openai_compatible', label: 'OpenAI 兼容', baseURL: '' },
  { value: 'openai', label: 'OpenAI', baseURL: 'https://api.openai.com/v1' },
  { value: 'anthropic', label: 'Anthropic', baseURL: 'https://api.anthropic.com/v1' },
  { value: 'gemini', label: 'Gemini', baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai' },
  { value: 'grok', label: 'Grok', baseURL: 'https://api.x.ai/v1' },
  { value: 'deepseek', label: 'DeepSeek', baseURL: 'https://api.deepseek.com/v1' },
  { value: 'qwen', label: 'Qwen', baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { value: 'kimi', label: 'Kimi', baseURL: 'https://api.moonshot.cn/v1' },
  { value: 'glm', label: 'GLM', baseURL: 'https://open.bigmodel.cn/api/paas/v4' },
  { value: 'minimax', label: 'MiniMax', baseURL: 'https://api.minimaxi.com/v1' },
  { value: 'mimo', label: 'MiMo', baseURL: 'https://api.xiaomimimo.com/v1' },
  { value: 'longcat', label: 'LongCat', baseURL: '' },
  { value: 'doubao', label: 'Doubao', baseURL: 'https://ark.cn-beijing.volces.com/api/v3' },
  { value: 'openrouter', label: 'OpenRouter', baseURL: 'https://openrouter.ai/api/v1' },
  { value: 'siliconflow', label: 'SiliconFlow', baseURL: 'https://api.siliconflow.cn/v1' },
  { value: 'ollama', label: 'Ollama', baseURL: 'http://localhost:11434/v1' }
];

export const providerCapabilityOptions = [
  { value: 'text', label: '文本' },
  { value: 'vision', label: '视觉' },
  { value: 'audio', label: '音频' },
  { value: 'image_generation', label: '生图' },
  { value: 'tool_calling', label: '工具调用' },
  { value: 'structured_output', label: '结构化输出' },
  { value: 'stream', label: '流式输出' },
  { value: 'deep_thinking', label: '深度思考' }
];

export const providerPurposeOptions: ProviderPurposeOption[] = [
  { value: 'general', label: '通用', requiredCapabilities: ['text'] },
  { value: 'diagnosis', label: '诊断', requiredCapabilities: ['text', 'tool_calling', 'stream'] },
  { value: 'inspection', label: '巡检', requiredCapabilities: ['text', 'tool_calling', 'structured_output'] },
  { value: 'workflow', label: '工作流', requiredCapabilities: ['text', 'tool_calling', 'structured_output'] }
];
