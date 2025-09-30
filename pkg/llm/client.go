package llm

import (
	"github.com/aibnuhibban/bytedocs/pkg/ai"
)

// init registers all LLM client factories
func init() {
	ai.RegisterClientFactory("openai", func(config *ai.AIConfig) (ai.Client, error) {
		return NewOpenAIClient(config)
	})
	ai.RegisterClientFactory("gemini", func(config *ai.AIConfig) (ai.Client, error) {
		return NewGeminiClient(config)
	})
	ai.RegisterClientFactory("openrouter", func(config *ai.AIConfig) (ai.Client, error) {
		return NewOpenRouterClient(config)
	})
}