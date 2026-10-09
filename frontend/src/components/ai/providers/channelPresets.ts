// 内置渠道预设：选择渠道后自动填充协议、基础 URL 与模型列表，用户只需填写 API 密钥。
// 预设名称为品牌专名不参与 i18n（与 Compatible/Responses/Messages 协议标签同策略）。
export interface AIChannelPreset {
  value: string
  label: string
  provider: string
  baseUrl: string
  defaultModel: string
  models: string[]
  cacheStrategy: string
  keyUrl?: string
  apiKeyRequired?: boolean
}

export const customChannelPresetValue = '__custom__'

const GLM_PRESET_KEY_URL = 'https://open.bigmodel.cn/usercenter/apikeys'
const OPENAI_PRESET_KEY_URL = 'https://platform.openai.com/api-keys'
const ANTHROPIC_PRESET_KEY_URL = 'https://console.anthropic.com/settings/keys'
const GEMINI_PRESET_KEY_URL = 'https://aistudio.google.com/app/apikey'
const MOONSHOT_PRESET_KEY_URL = 'https://platform.moonshot.cn/console/api-keys'
const MINIMAX_PRESET_KEY_URL = 'https://platform.minimaxi.com/user-center/basic-information/interface-key'
const DOUBAO_PRESET_KEY_URL = 'https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey'
const DEEPSEEK_PRESET_KEY_URL = 'https://platform.deepseek.com/api_keys'
const QWEN_PRESET_KEY_URL = 'https://bailian.console.aliyun.com/?apiKey=1#/api-key'

export const aiChannelPresets: AIChannelPreset[] = [
  {
    value: 'codex-app-server',
    label: 'Codex (App Server Bridge)',
    provider: 'Compatible',
    baseUrl: 'http://127.0.0.1:5050/v1',
    defaultModel: 'gpt-5.6-sol',
    models: ['gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna', 'gpt-6-sol', 'gpt-6-astra', 'gpt-6-luna', 'gpt-5.5'],
    cacheStrategy: 'model',
    apiKeyRequired: false,
  },
  {
    value: 'openai',
    label: 'OpenAI',
    provider: 'Responses',
    baseUrl: 'https://api.openai.com/v1',
    defaultModel: 'gpt-5.2',
    models: ['gpt-5.4', 'gpt-5.2', 'gpt-5.1', 'gpt-5-chat', 'o4-mini'],
    cacheStrategy: 'model',
    keyUrl: OPENAI_PRESET_KEY_URL,
  },
  {
    value: 'claude',
    label: 'Anthropic Claude',
    provider: 'Messages',
    baseUrl: 'https://api.anthropic.com',
    defaultModel: 'claude-sonnet-5',
    models: ['claude-sonnet-5', 'claude-opus-5', 'claude-opus-4-8', 'claude-sonnet-4-5', 'claude-haiku-4-5'],
    cacheStrategy: '5m',
    keyUrl: ANTHROPIC_PRESET_KEY_URL,
  },
  {
    value: 'gemini',
    label: 'Google Gemini',
    provider: 'Compatible',
    baseUrl: 'https://generativelanguage.googleapis.com/v1beta/openai',
    defaultModel: 'gemini-2.5-pro',
    models: ['gemini-2.5-pro', 'gemini-2.5-flash', 'gemini-2.5-flash-lite', 'gemini-2.0-flash'],
    cacheStrategy: 'model',
    keyUrl: GEMINI_PRESET_KEY_URL,
  },
  {
    value: 'glm',
    label: '智谱 GLM',
    provider: 'Compatible',
    baseUrl: 'https://open.bigmodel.cn/api/paas/v4',
    defaultModel: 'glm-4.6',
    models: ['glm-4.6', 'glm-4.5', 'glm-4.5-air', 'glm-4.5-flash'],
    cacheStrategy: 'model',
    keyUrl: GLM_PRESET_KEY_URL,
  },
  {
    value: 'kimi',
    label: '月之暗面 Kimi',
    provider: 'Compatible',
    baseUrl: 'https://api.moonshot.cn/v1',
    defaultModel: 'kimi-k2-0905-preview',
    models: ['kimi-k2-0905-preview', 'kimi-k2-turbo-preview', 'kimi-latest', 'kimi-thinking-preview', 'moonshot-v1-128k'],
    cacheStrategy: 'model',
    keyUrl: MOONSHOT_PRESET_KEY_URL,
  },
  {
    value: 'minimax',
    label: 'MiniMax',
    provider: 'Compatible',
    baseUrl: 'https://api.minimaxi.com/v1',
    defaultModel: 'MiniMax-M2',
    models: ['MiniMax-M2', 'MiniMax-M1', 'abab6.5s-chat'],
    cacheStrategy: 'model',
    keyUrl: MINIMAX_PRESET_KEY_URL,
  },
  {
    value: 'doubao',
    label: '字节跳动 豆包',
    provider: 'Compatible',
    baseUrl: 'https://ark.cn-beijing.volces.com/api/v3',
    defaultModel: 'doubao-seed-1-6',
    models: ['doubao-seed-1-6', 'doubao-seed-1-6-flash', 'doubao-seed-1-6-thinking', 'doubao-1-5-pro-32k', 'doubao-pro-32k'],
    cacheStrategy: 'model',
    keyUrl: DOUBAO_PRESET_KEY_URL,
  },
  {
    value: 'deepseek',
    label: 'DeepSeek',
    provider: 'Compatible',
    baseUrl: 'https://api.deepseek.com/v1',
    defaultModel: 'deepseek-chat',
    models: ['deepseek-chat', 'deepseek-reasoner'],
    cacheStrategy: 'model',
    keyUrl: DEEPSEEK_PRESET_KEY_URL,
  },
  {
    value: 'qwen',
    label: '阿里云 Qwen',
    provider: 'Compatible',
    baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    defaultModel: 'qwen3-max',
    models: ['qwen3-max', 'qwen3-coder-plus', 'qwen-plus', 'qwen-turbo', 'qwen-max'],
    cacheStrategy: 'model',
    keyUrl: QWEN_PRESET_KEY_URL,
  },
]

export function getAIChannelPreset(value: unknown): AIChannelPreset | null {
  const nextValue = typeof value === 'string' ? value.trim() : ''
  if (!nextValue || nextValue === customChannelPresetValue) {
    return null
  }
  return aiChannelPresets.find((preset) => preset.value === nextValue) || null
}

export function matchAIChannelPresetByBaseUrl(baseUrl: unknown): AIChannelPreset | null {
  const normalized = typeof baseUrl === 'string' ? baseUrl.trim().replace(/\/+$/, '').toLowerCase() : ''
  if (!normalized) {
    return null
  }
  return aiChannelPresets.find((preset) => preset.baseUrl.replace(/\/+$/, '').toLowerCase() === normalized) || null
}
