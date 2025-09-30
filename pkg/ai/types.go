package ai

import (
    "context"
    "fmt"
)

type AIConfig struct {
    Provider string                 `json:"provider"`     // LLM provider (openai, gemini, claude, etc.)
    APIKey   string                 `json:"apiKey"`
    Enabled  bool                   `json:"enabled"`
    Features AIFeatures             `json:"features"`
    Settings map[string]interface{} `json:"settings"`
}

type AIFeatures struct {
    ChatEnabled          bool    `json:"chatEnabled"`
    DocGenerationEnabled bool    `json:"docGenerationEnabled"`
    Model                string  `json:"model"`
    MaxTokens            int     `json:"maxTokens"`
    MaxCompletionTokens  int     `json:"maxCompletionTokens"`
    Temperature          float64 `json:"temperature"` 
}

type ChatRequest struct {
    Message     string                 `json:"message"`
    Context     string                 `json:"context,omitempty"`
    Endpoint    interface{}            `json:"endpoint,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ChatResponse struct {
    Response    string `json:"response"`
    Provider    string `json:"provider"`
    Model       string `json:"model,omitempty"`
    TokensUsed  int    `json:"tokensUsed,omitempty"`
    Error       string `json:"error,omitempty"`
}

type Client interface {
    Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error)
    GetProvider() string
    GetModel() string
}

type ClientFactory func(config *AIConfig) (Client, error)

var clientFactories = make(map[string]ClientFactory)

func RegisterClientFactory(provider string, factory ClientFactory) {
    clientFactories[provider] = factory
}

func NewClient(config *AIConfig) (Client, error) {
    if config == nil || !config.Enabled {
        return nil, fmt.Errorf("AI configuration is not enabled")
    }

    factory, exists := clientFactories[config.Provider]
    if !exists {
        return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
    }

    return factory(config)
}