package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/aibnuhibban/bytedocs/pkg/ai"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// OpenRouterClient implements the Client interface for OpenRouter
type OpenRouterClient struct {
	client *openai.Client
	model  string
	config *ai.AIConfig
}

// NewOpenRouterClient creates a new OpenRouter client
func NewOpenRouterClient(config *ai.AIConfig) (*OpenRouterClient, error) {
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenRouter API key is required (set APIKey in config or OPENROUTER_API_KEY environment variable)")
	}

	// Create OpenAI-compatible client with OpenRouter base URL
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://openrouter.ai/api/v1"),
	)

	// Default model
	model := "openai/gpt-3.5-turbo"
	if config.Features.Model != "" {
		model = config.Features.Model
	}

	return &OpenRouterClient{
		client: &client,
		model:  model,
		config: config,
	}, nil
}

// Chat implements the Chat method for OpenRouter
func (c *OpenRouterClient) Chat(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	// Build messages
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(c.buildSystemPrompt(request)),
		openai.UserMessage(request.Message),
	}

	// Prepare parameters
	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    openai.ChatModel(c.model),
	}

	// Set max tokens based on config
	if c.config.Features.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(c.config.Features.MaxTokens))
	}
	if c.config.Features.MaxCompletionTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(c.config.Features.MaxCompletionTokens))
	}

	// Set temperature if specified
	if c.config.Features.Temperature > 0 {
		params.Temperature = openai.Float(c.config.Features.Temperature)
	}

	// Make API call
	chatCompletion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return &ai.ChatResponse{
			Error:    err.Error(),
			Provider: c.GetProvider(),
			Model:    c.model,
		}, err
	}

	// Extract response content
	if len(chatCompletion.Choices) == 0 {
		return &ai.ChatResponse{
			Error:    "No response choices returned",
			Provider: c.GetProvider(),
			Model:    c.model,
		}, fmt.Errorf("no response choices")
	}

	// Get tokens used
	tokensUsed := 0
	if chatCompletion.Usage.TotalTokens > 0 {
		tokensUsed = int(chatCompletion.Usage.TotalTokens)
	}

	return &ai.ChatResponse{
		Response:   chatCompletion.Choices[0].Message.Content,
		Provider:   c.GetProvider(),
		Model:      string(chatCompletion.Model),
		TokensUsed: tokensUsed,
	}, nil
}

// GetProvider returns the provider name
func (c *OpenRouterClient) GetProvider() string {
	return "openrouter"
}

// GetModel returns the current model
func (c *OpenRouterClient) GetModel() string {
	return c.model
}

// buildSystemPrompt creates a system prompt based on the request context
func (c *OpenRouterClient) buildSystemPrompt(request ai.ChatRequest) string {
	basePrompt := `You are an API documentation assistant. You MUST ONLY provide information about the exact API endpoints defined in the OpenAPI specification provided below.

CRITICAL RULES:
1. NEVER mention endpoints that are not in the OpenAPI specification
2. NEVER invent or assume endpoints, parameters, or responses
3. ONLY use the exact paths, methods, and schemas from the provided OpenAPI JSON
4. If an endpoint doesn't exist in the spec, explicitly say "This endpoint does not exist in the API"
5. Always reference the actual OpenAPI specification as your single source of truth

When answering:
- Check the OpenAPI "paths" section for available endpoints
- Use only the exact path names, HTTP methods, and parameters documented
- Show actual request/response schemas from the "components" section
- Provide curl examples using only documented endpoints
- If asked about non-existent endpoints, clearly state they don't exist
- Be very concise; provide only the information requested and nothing extraneous.
- Match the user's language (respond in Indonesian if the user wrote in Indonesian).
- For code or curl examples, include only minimal, runnable snippets.
- Do not speculate, infer, or answer beyond what the OpenAPI spec and the user's query require.`

	// Add the full API context (OpenAPI JSON)
	if request.Context != "" {
		basePrompt += fmt.Sprintf("\n\n%s", request.Context)
	}

	// Add specific endpoint context if provided
	if request.Endpoint != nil {
		basePrompt += "\n\n=== CURRENT FOCUSED ENDPOINT ===\nThe user is currently viewing a specific endpoint. Please provide contextual responses about this endpoint and the API in general."
	}

	return basePrompt
}