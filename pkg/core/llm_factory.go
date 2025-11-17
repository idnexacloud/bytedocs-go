package core

import "github.com/idnexacloud/bytedocs-go/pkg/ai"

// Aliases for backward compatibility - delegate to ai package
type LLMClientFactory = ai.ClientFactory

// RegisterLLMClientFactory registers a factory function for a specific provider
func RegisterLLMClientFactory(provider string, factory LLMClientFactory) {
    ai.RegisterClientFactory(provider, factory)
}

// NewLLMClient creates a new LLM client based on configuration  
func NewLLMClient(config *AIConfig) (LLMClient, error) {
    return ai.NewClient(config)
}