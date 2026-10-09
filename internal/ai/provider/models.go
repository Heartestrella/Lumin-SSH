package provider

import "strings"

func normalizeProviderProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "compatible":
		return "Compatible"
	case "responses":
		return "Responses"
	case "messages":
		return "Messages"
	default:
		return "Compatible"
	}
}

func normalizeReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "disable":
		return "disable"
	case "none":
		return "none"
	case "minimal":
		return "minimal"
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	case "xhigh":
		return "xhigh"
	case "max":
		return "max"
	default:
		return "disable"
	}
}

const (
	AIProviderReasoningModeNone   = "none"
	AIProviderReasoningModeBinary = "binary"
	AIProviderReasoningModeEffort = "effort"
	AIProviderReasoningModeBudget = "budget"
)

type AIProviderModelCapability struct {
	Provider                string   `json:"provider,omitempty"`
	ModelID                 string   `json:"modelId,omitempty"`
	Known                   bool     `json:"known"`
	SupportsPromptCache     bool     `json:"supportsPromptCache"`
	PromptCacheRetention    string   `json:"promptCacheRetention,omitempty"`
	SupportsReasoningBinary bool     `json:"supportsReasoningBinary"`
	SupportsReasoningBudget bool     `json:"supportsReasoningBudget"`
	RequiredReasoningBudget bool     `json:"requiredReasoningBudget"`
	SupportsReasoningEffort []string `json:"supportsReasoningEffort,omitempty"`
	RequiredReasoningEffort bool     `json:"requiredReasoningEffort"`
	ReasoningEffort         string   `json:"reasoningEffort,omitempty"`
	ReasoningMode           string   `json:"reasoningMode,omitempty"`
	MaxTokens               int      `json:"maxTokens,omitempty"`
	MaxThinkingTokens       int      `json:"maxThinkingTokens,omitempty"`
	SupportsTemperature     bool     `json:"supportsTemperature"`
	// ContextWindow 是模型的上下文窗口保守估计（含输入与输出），供主动压缩等
	// 压力判断使用；允许偏小，不允许偏大（溢出兜底仍由失败恢复路径负责）。
	ContextWindow int `json:"contextWindow,omitempty"`
}

type aiProviderModelCapabilityRule struct {
	MatchExact    string
	MatchPrefix   string
	MatchContains string
	Capability    AIProviderModelCapability
}

var codexBridgeCompatibleModelCapability = AIProviderModelCapability{
	Known:                true,
	SupportsPromptCache:  true,
	PromptCacheRetention: "24h",
	SupportsReasoningEffort: []string{
		"low",
		"medium",
		"high",
	},
	ReasoningEffort:     "medium",
	ReasoningMode:       AIProviderReasoningModeEffort,
	SupportsTemperature: false,
}

var compatibleProviderModelCapabilityRules = []aiProviderModelCapabilityRule{
	// Codex App Server 网关模型使用统一能力约束；后缀规则兼容网关侧的版本化模型名。
	{MatchPrefix: "gpt-5.6", Capability: codexBridgeCompatibleModelCapability},
	{MatchPrefix: "gpt-5.5", Capability: codexBridgeCompatibleModelCapability},
	{MatchPrefix: "gpt-6", Capability: codexBridgeCompatibleModelCapability},
	{MatchContains: "-sol", Capability: codexBridgeCompatibleModelCapability},
	{MatchContains: "-terra", Capability: codexBridgeCompatibleModelCapability},
	{MatchContains: "-luna", Capability: codexBridgeCompatibleModelCapability},
	{MatchContains: "-astra", Capability: codexBridgeCompatibleModelCapability},
	{
		MatchPrefix: "gpt-5.4",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "24h",
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
				"xhigh",
			},
			ReasoningEffort:     "xhigh",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchPrefix: "gpt-5.2",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "24h",
			SupportsReasoningEffort: []string{
				"none",
				"low",
				"medium",
				"high",
				"xhigh",
			},
			ReasoningEffort:     "medium",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchPrefix: "gpt-5.1",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "24h",
			SupportsReasoningEffort: []string{
				"none",
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "medium",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchPrefix: "gpt-5-chat",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "24h",
			ReasoningMode:        AIProviderReasoningModeNone,
			SupportsTemperature:  false,
		},
	},
	{
		MatchExact: "gpt-5",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "24h",
			ReasoningMode:        AIProviderReasoningModeNone,
			SupportsTemperature:  false,
		},
	},
	{
		MatchContains: "codex",
		Capability:    codexBridgeCompatibleModelCapability,
	},
	{
		MatchExact: "o4-mini-high",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "high",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchExact: "o4-mini-low",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "low",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchPrefix: "o4-mini",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "medium",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchExact: "o3-mini-high",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "high",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchExact: "o3-mini-low",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "low",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchPrefix: "o3-mini",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "medium",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchExact: "o3-low",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "low",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchPrefix: "o3",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "medium",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
	{
		MatchPrefix: "o1",
		Capability: AIProviderModelCapability{
			Known:               true,
			SupportsPromptCache: true,
			SupportsReasoningEffort: []string{
				"low",
				"medium",
				"high",
			},
			ReasoningEffort:     "high",
			ReasoningMode:       AIProviderReasoningModeEffort,
			SupportsTemperature: false,
		},
	},
}

var responsesProviderModelCapabilityRules = compatibleProviderModelCapabilityRules

var messagesProviderModelCapabilityRules = []aiProviderModelCapabilityRule{
	{
		MatchExact: "claude-opus-4-8",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "in_memory",
			SupportsReasoningEffort: []string{
				"minimal",
				"low",
				"medium",
				"high",
				"xhigh",
			},
			RequiredReasoningEffort: false,
			ReasoningEffort:         "medium",
			ReasoningMode:           AIProviderReasoningModeEffort,
			MaxTokens:               16384,
			MaxThinkingTokens:       8192,
			SupportsTemperature:     true,
		},
	},
	{
		MatchContains: "claude-opus-4",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "in_memory",
			SupportsReasoningEffort: []string{
				"minimal",
				"low",
				"medium",
				"high",
				"xhigh",
			},
			RequiredReasoningEffort: false,
			ReasoningEffort:         "medium",
			ReasoningMode:           AIProviderReasoningModeEffort,
			MaxTokens:               16384,
			MaxThinkingTokens:       8192,
			SupportsTemperature:     true,
		},
	},
	{
		MatchContains: "claude-sonnet-4",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "in_memory",
			SupportsReasoningEffort: []string{
				"minimal",
				"low",
				"medium",
				"high",
				"xhigh",
			},
			RequiredReasoningEffort: false,
			ReasoningEffort:         "medium",
			ReasoningMode:           AIProviderReasoningModeEffort,
			MaxTokens:               16384,
			MaxThinkingTokens:       8192,
			SupportsTemperature:     true,
		},
	},
	{
		MatchContains: "claude-3.7-sonnet",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "in_memory",
			SupportsReasoningEffort: []string{
				"minimal",
				"low",
				"medium",
				"high",
				"xhigh",
			},
			RequiredReasoningEffort: false,
			ReasoningEffort:         "medium",
			ReasoningMode:           AIProviderReasoningModeEffort,
			MaxTokens:               16384,
			MaxThinkingTokens:       8192,
			SupportsTemperature:     true,
		},
	},
	{
		MatchContains: "claude",
		Capability: AIProviderModelCapability{
			Known:                true,
			SupportsPromptCache:  true,
			PromptCacheRetention: "in_memory",
			SupportsReasoningEffort: []string{
				"minimal",
				"low",
				"medium",
				"high",
				"xhigh",
			},
			RequiredReasoningEffort: false,
			ReasoningEffort:         "medium",
			ReasoningMode:           AIProviderReasoningModeEffort,
			MaxTokens:               16384,
			MaxThinkingTokens:       8192,
			SupportsTemperature:     true,
		},
	},
}

func normalizeAIProviderModelReasoningEffortOptions(values []string) []string {
	if values == nil {
		return []string{}
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		nextValue := strings.ToLower(strings.TrimSpace(value))
		switch nextValue {
		case "disable", "none", "minimal", "low", "medium", "high", "xhigh", "max":
		default:
			continue
		}
		if _, exists := seen[nextValue]; exists {
			continue
		}
		seen[nextValue] = struct{}{}
		normalized = append(normalized, nextValue)
	}
	return normalized
}

func buildConservativeAIProviderModelCapability(provider string, modelID string) AIProviderModelCapability {
	return AIProviderModelCapability{
		Provider:            normalizeProviderProtocol(provider),
		ModelID:             strings.TrimSpace(modelID),
		Known:               false,
		ReasoningMode:       AIProviderReasoningModeNone,
		SupportsTemperature: true,
	}
}

func finalizeAIProviderModelCapability(provider string, modelID string, capability AIProviderModelCapability) AIProviderModelCapability {
	capability.Provider = normalizeProviderProtocol(provider)
	capability.ModelID = strings.TrimSpace(modelID)
	capability.ReasoningMode = strings.TrimSpace(capability.ReasoningMode)
	if capability.ReasoningMode == "" {
		switch {
		case capability.SupportsReasoningBudget || capability.RequiredReasoningBudget:
			capability.ReasoningMode = AIProviderReasoningModeBudget
		case capability.SupportsReasoningBinary:
			capability.ReasoningMode = AIProviderReasoningModeBinary
		case len(capability.SupportsReasoningEffort) > 0:
			capability.ReasoningMode = AIProviderReasoningModeEffort
		default:
			capability.ReasoningMode = AIProviderReasoningModeNone
		}
	}
	capability.SupportsReasoningEffort = normalizeAIProviderModelReasoningEffortOptions(capability.SupportsReasoningEffort)
	capability.ReasoningEffort = strings.ToLower(strings.TrimSpace(capability.ReasoningEffort))
	return capability
}

func matchesAIProviderModelCapabilityRule(rule aiProviderModelCapabilityRule, normalizedModelID string) bool {
	switch {
	case rule.MatchExact != "":
		return normalizedModelID == strings.ToLower(strings.TrimSpace(rule.MatchExact))
	case rule.MatchPrefix != "":
		return strings.HasPrefix(normalizedModelID, strings.ToLower(strings.TrimSpace(rule.MatchPrefix)))
	case rule.MatchContains != "":
		return strings.Contains(normalizedModelID, strings.ToLower(strings.TrimSpace(rule.MatchContains)))
	default:
		return false
	}
}

func getAIProviderModelCapabilityRules(provider string) []aiProviderModelCapabilityRule {
	switch normalizeProviderProtocol(provider) {
	case "Responses":
		return responsesProviderModelCapabilityRules
	case "Messages":
		return messagesProviderModelCapabilityRules
	default:
		return compatibleProviderModelCapabilityRules
	}
}

func ResolveModelCapability(provider string, modelID string) AIProviderModelCapability {
	normalizedModelID := strings.ToLower(strings.TrimSpace(modelID))
	if normalizedModelID == "" {
		return buildConservativeAIProviderModelCapability(provider, modelID)
	}
	for _, rule := range getAIProviderModelCapabilityRules(provider) {
		if matchesAIProviderModelCapabilityRule(rule, normalizedModelID) {
			return finalizeAIProviderModelCapability(provider, modelID, rule.Capability)
		}
	}
	return buildConservativeAIProviderModelCapability(provider, modelID)
}

func providerSupportsAIQuickEditPromptCache(provider string) bool {
	switch normalizeProviderProtocol(provider) {
	case "Compatible", "Responses", "Messages":
		return true
	default:
		return false
	}
}

func CanBeDedicatedWebSearchCandidate(provider string) bool {
	switch normalizeProviderProtocol(provider) {
	case "Compatible", "Responses":
		return true
	default:
		return false
	}
}

// DefaultAIModelContextWindow 是未知模型的上下文窗口保守估计。
const DefaultAIModelContextWindow = 131072

// modelContextWindowRules 按"先专用后通用"的顺序匹配模型 ID，给出上下文窗口的保守估计。
// 估计值允许偏小（只是提前触发主动压缩），不允许偏大（会失去溢出兜底前的压缩机会）。
var modelContextWindowRules = []aiProviderModelCapabilityRule{
	// 窗口后缀是最明确的声明（如 moonshot-v1-32k、doubao-1-5-pro-32k），排在所有家族规则之前
	{MatchContains: "256k", Capability: AIProviderModelCapability{ContextWindow: 262144}},
	{MatchContains: "128k", Capability: AIProviderModelCapability{ContextWindow: 131072}},
	{MatchContains: "32k", Capability: AIProviderModelCapability{ContextWindow: 32768}},
	{MatchContains: "8k", Capability: AIProviderModelCapability{ContextWindow: 8192}},
	// OpenAI：gpt-5-chat 走普通 completions 通道，窗口更小，需排在 gpt-5 前缀之前
	{MatchPrefix: "gpt-5-chat", Capability: AIProviderModelCapability{ContextWindow: 128000}},
	{MatchPrefix: "gpt-5", Capability: AIProviderModelCapability{ContextWindow: 400000}},
	{MatchPrefix: "gpt-6", Capability: AIProviderModelCapability{ContextWindow: 400000}},
	{MatchContains: "-sol", Capability: AIProviderModelCapability{ContextWindow: 400000}},
	{MatchContains: "-terra", Capability: AIProviderModelCapability{ContextWindow: 400000}},
	{MatchContains: "-luna", Capability: AIProviderModelCapability{ContextWindow: 400000}},
	{MatchContains: "-astra", Capability: AIProviderModelCapability{ContextWindow: 400000}},
	{MatchContains: "codex", Capability: AIProviderModelCapability{ContextWindow: 400000}},
	{MatchPrefix: "o3", Capability: AIProviderModelCapability{ContextWindow: 200000}},
	{MatchPrefix: "o4", Capability: AIProviderModelCapability{ContextWindow: 200000}},
	// OpenAI o1：o1-mini / o1-preview 实际窗口 128k；o1-pro 已停产 200k
	{MatchPrefix: "o1-mini", Capability: AIProviderModelCapability{ContextWindow: 128000}},
	{MatchPrefix: "o1-preview", Capability: AIProviderModelCapability{ContextWindow: 128000}},
	{MatchPrefix: "o1", Capability: AIProviderModelCapability{ContextWindow: 200000}},
	// Anthropic
	{MatchContains: "claude", Capability: AIProviderModelCapability{ContextWindow: 200000}},
	// Google
	{MatchContains: "gemini", Capability: AIProviderModelCapability{ContextWindow: 1000000}},
	// 智谱 GLM
	{MatchPrefix: "glm-4.6", Capability: AIProviderModelCapability{ContextWindow: 200000}},
	{MatchContains: "glm", Capability: AIProviderModelCapability{ContextWindow: 131072}},
	// Kimi / Moonshot
	{MatchContains: "kimi-k2", Capability: AIProviderModelCapability{ContextWindow: 262144}},
	{MatchContains: "kimi", Capability: AIProviderModelCapability{ContextWindow: 262144}},
	// MiniMax
	{MatchContains: "minimax-m1", Capability: AIProviderModelCapability{ContextWindow: 1000000}},
	{MatchContains: "minimax", Capability: AIProviderModelCapability{ContextWindow: 200000}},
	// DeepSeek
	{MatchContains: "deepseek", Capability: AIProviderModelCapability{ContextWindow: 131072}},
	// 火山方舟豆包
	{MatchContains: "doubao-seed-1-6", Capability: AIProviderModelCapability{ContextWindow: 262144}},
	{MatchContains: "doubao", Capability: AIProviderModelCapability{ContextWindow: 131072}},
	// 阿里云 Qwen
	{MatchPrefix: "qwen3", Capability: AIProviderModelCapability{ContextWindow: 262144}},
	{MatchContains: "qwen", Capability: AIProviderModelCapability{ContextWindow: 131072}},
}

// GetModelContextWindow 返回模型上下文窗口的保守估计（token 数）。
// 按模型家族规则依次匹配，未知模型取默认 128k；允许偏小，不允许偏大。
func GetModelContextWindow(provider string, modelID string) int {
	normalizedModelID := strings.ToLower(strings.TrimSpace(modelID))
	if normalizedModelID != "" {
		for _, rule := range modelContextWindowRules {
			if matchesAIProviderModelCapabilityRule(rule, normalizedModelID) {
				if rule.Capability.ContextWindow > 0 {
					return rule.Capability.ContextWindow
				}
			}
		}
	}
	return DefaultAIModelContextWindow
}
