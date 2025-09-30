package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/aibnuhibban/bytedocs/pkg/ai"
	"google.golang.org/genai"
)

// GeminiClient implements the Client interface for Google Gemini
type GeminiClient struct {
	client *genai.Client
	model  string
	config *ai.AIConfig
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(config *ai.AIConfig) (*GeminiClient, error) {
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is required (set APIKey in config or GEMINI_API_KEY environment variable)")
	}

	// Create Gemini client with API key
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %v", err)
	}

	// Default model
	model := "gemini-2.5-flash"
	if config.Features.Model != "" {
		model = config.Features.Model
	}

	return &GeminiClient{
		client: client,
		model:  model,
		config: config,
	}, nil
}

// Chat implements the Chat method for Gemini
func (c *GeminiClient) Chat(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	// Build the prompt by combining system prompt and user message
	fullPrompt := c.buildSystemPrompt(request) + "\n\nUser: " + request.Message

	// Make API call using the official genai library
	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(fullPrompt),
		nil,
	)
	if err != nil {
		return &ai.ChatResponse{
			Error:    err.Error(),
			Provider: c.GetProvider(),
			Model:    c.model,
		}, err
	}

	// Extract response text
	responseText := result.Text()
	if responseText == "" {
		return &ai.ChatResponse{
			Error:    "No response content returned",
			Provider: c.GetProvider(),
			Model:    c.model,
		}, fmt.Errorf("no response content")
	}

	// Get tokens used (Gemini API might not always provide this)
	tokensUsed := 0
	// Note: Usage information might not be available in this version of the API

	return &ai.ChatResponse{
		Response:   responseText,
		Provider:   c.GetProvider(),
		Model:      c.model,
		TokensUsed: tokensUsed,
	}, nil
}

// GetProvider returns the provider name
func (c *GeminiClient) GetProvider() string {
	return "gemini"
}

// GetModel returns the current model
func (c *GeminiClient) GetModel() string {
	return c.model
}

// buildSystemPrompt creates a system prompt based on the request context
func (c *GeminiClient) buildSystemPrompt(request ai.ChatRequest) string {
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

