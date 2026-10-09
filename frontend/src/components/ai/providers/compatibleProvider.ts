// 桥接模块（自 .js 收编后类型化）：Compatible 供应商模型能力表
import type { ModelCapability } from './messagesProvider.ts'

const VALID_REASONING_EFFORTS = new Set(['disable', 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])

const CONSERVATIVE_CAPABILITY: ModelCapability = {
  known: false,
  supportsReasoningBinary: false,
  supportsReasoningBudget: false,
  requiredReasoningBudget: false,
  supportsReasoningEffort: [],
  requiredReasoningEffort: false,
  reasoningEffort: 'disable',
  reasoningMode: 'none',
  maxTokens: 0,
  maxThinkingTokens: 0,
  supportsTemperature: true,
}

interface CapabilityRule {
  matchExact?: string
  matchPrefix?: string
  matchContains?: string
  capability: Partial<ModelCapability>
}

const capabilityRules: CapabilityRule[] = [
  // Codex App Server 网关模型使用统一能力约束；后缀规则兼容网关侧的版本化模型名。
  ...['gpt-5.6', 'gpt-5.5', 'gpt-6'].map((matchPrefix) => ({
    matchPrefix,
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'medium',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  })),
  ...['-sol', '-terra', '-luna', '-astra'].map((matchContains) => ({
    matchContains,
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'medium',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  })),
  {
    matchPrefix: 'gpt-5.4',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high', 'xhigh'],
      reasoningEffort: 'xhigh',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchPrefix: 'gpt-5.2',
    capability: {
      known: true,
      supportsReasoningEffort: ['none', 'low', 'medium', 'high', 'xhigh'],
      reasoningEffort: 'medium',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchPrefix: 'gpt-5.1',
    capability: {
      known: true,
      supportsReasoningEffort: ['none', 'low', 'medium', 'high'],
      reasoningEffort: 'medium',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchPrefix: 'gpt-5-chat',
    capability: {
      known: true,
      reasoningMode: 'none',
      supportsTemperature: false,
    },
  },
  {
    matchExact: 'gpt-5',
    capability: {
      known: true,
      reasoningMode: 'none',
      supportsTemperature: false,
    },
  },
  {
    matchContains: 'codex',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'medium',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchExact: 'o4-mini-high',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'high',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchExact: 'o4-mini-low',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'low',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchPrefix: 'o4-mini',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'medium',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchExact: 'o3-mini-high',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'high',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchExact: 'o3-mini-low',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'low',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchPrefix: 'o3-mini',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'medium',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchExact: 'o3-low',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'low',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchPrefix: 'o3',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'medium',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
  {
    matchPrefix: 'o1',
    capability: {
      known: true,
      supportsReasoningEffort: ['low', 'medium', 'high'],
      reasoningEffort: 'high',
      reasoningMode: 'effort',
      supportsTemperature: false,
    },
  },
]

function normalizeReasoningEffortOptions(values: unknown): string[] {
  if (!Array.isArray(values)) {
    return []
  }
  const seen = new Set<string>()
  const normalized: string[] = []
  values.forEach((value) => {
    const nextValue = typeof value === 'string' ? value.trim().toLowerCase() : ''
    if (!VALID_REASONING_EFFORTS.has(nextValue) || seen.has(nextValue)) {
      return
    }
    seen.add(nextValue)
    normalized.push(nextValue)
  })
  return normalized
}

function buildCapability(modelId: unknown, patch: Partial<ModelCapability> = {}): ModelCapability {
  return {
    ...CONSERVATIVE_CAPABILITY,
    modelId: typeof modelId === 'string' ? modelId.trim() : '',
    ...patch,
    supportsReasoningEffort: normalizeReasoningEffortOptions(patch.supportsReasoningEffort),
    reasoningEffort: typeof patch.reasoningEffort === 'string' ? patch.reasoningEffort.trim().toLowerCase() : CONSERVATIVE_CAPABILITY.reasoningEffort,
  }
}

function matchesRule(rule: CapabilityRule, normalizedModelId: string): boolean {
  if (rule.matchExact) {
    return normalizedModelId === rule.matchExact.toLowerCase()
  }
  if (rule.matchPrefix) {
    return normalizedModelId.startsWith(rule.matchPrefix.toLowerCase())
  }
  if (rule.matchContains) {
    return normalizedModelId.includes(rule.matchContains.toLowerCase())
  }
  return false
}

function getModelCapability(modelId: unknown): ModelCapability {
  const normalizedModelId = typeof modelId === 'string' ? modelId.trim().toLowerCase() : ''
  if (!normalizedModelId) {
    return buildCapability(modelId)
  }
  const matchedRule = capabilityRules.find((rule) => matchesRule(rule, normalizedModelId))
  return matchedRule ? buildCapability(modelId, matchedRule.capability) : buildCapability(modelId)
}

export const compatibleProvider = {
  value: 'Compatible',
  label: 'Compatible',
  defaultModel: '',
  initialModels: [],
  supportsPromptCacheSettings: true,
  supportsWebSearch: true,
  supportsDedicatedWebSearchCandidate: true,
  getModelCapability,
}
