package core

import (
	"reflect"

	"github.com/idnexacloud/bytedocs-go/pkg/ai"
)

// APIInfo represents basic API information
type APIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
	BaseURL     string `json:"baseUrl"`
}

// EndpointSection groups related endpoints
type EndpointSection struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Endpoints   []Endpoint `json:"endpoints"`
}

// Endpoint represents a single API endpoint
type Endpoint struct {
	ID          string              `json:"id"`
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Summary     string              `json:"summary"`
	Description string              `json:"description"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
	Tags        []string            `json:"tags,omitempty"`
	Handler     reflect.Value       `json:"-"` // Internal use
}

// Parameter represents endpoint parameter
type Parameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"` // "path", "query", "header", "cookie"
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Description string      `json:"description"`
	Example     interface{} `json:"example,omitempty"`
}

// RequestBody represents request body schema
type RequestBody struct {
	ContentType string      `json:"contentType"`
	Schema      interface{} `json:"schema"`
	Example     interface{} `json:"example,omitempty"`
	Required    bool        `json:"required"`
}

// Response represents endpoint response
type Response struct {
	Description string      `json:"description"`
	Example     interface{} `json:"example,omitempty"`
	Schema      interface{} `json:"schema,omitempty"`
	ContentType string      `json:"contentType,omitempty"`
}

// Documentation represents complete API documentation
type Documentation struct {
	Info      APIInfo           `json:"info"`
	Endpoints []EndpointSection `json:"endpoints"`
	Schemas   map[string]Schema `json:"schemas,omitempty"`
}

// Schema represents data structure schema
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
	Example    interface{}         `json:"example,omitempty"`
}

// Property represents schema property
type Property struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Example     interface{} `json:"example,omitempty"`
	Format      string      `json:"format,omitempty"`
}

// Config represents apidocs configuration
type Config struct {
	Title        string           `json:"title"`
	Version      string           `json:"version"`
	Description  string           `json:"description"`
	BaseURL      string           `json:"baseUrl"`  // Backward compatibility - single URL
	BaseURLs     []BaseURLOption  `json:"baseUrls"` // New field - multiple URLs
	DocsPath     string           `json:"docsPath"`
	AutoDetect   bool             `json:"autoDetect"`
	IncludeTypes []reflect.Type   `json:"-"`
	ExcludePaths []string         `json:"excludePaths"`
	Middlewares  []MiddlewareFunc `json:"-"`
	AuthConfig   *AuthConfig      `json:"authConfig,omitempty"`
	UIConfig     *UIConfig        `json:"uiConfig,omitempty"`
	AIConfig     *ai.AIConfig     `json:"aiConfig,omitempty"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled      bool   `json:"enabled"`
	Type         string `json:"type"`         // "basic", "api_key", "bearer", "session"
	Username     string `json:"username"`     // For basic auth
	Password     string `json:"password"`     // For basic auth / simple password / session password
	APIKey       string `json:"apiKey"`       // For API key auth
	APIKeyHeader string `json:"apiKeyHeader"` // Header name for API key (default: "X-API-Key")
	Realm        string `json:"realm"`        // Basic auth realm

	// Session-based auth configuration (Laravel-style)
	SessionExpire     int      `json:"sessionExpire"`     // Session expiration in minutes (default: 1440)
	IPBanEnabled      bool     `json:"ipBanEnabled"`      // Enable IP banning (default: true)
	IPBanMaxAttempts  int      `json:"ipBanMaxAttempts"`  // Max failed attempts before ban (default: 5)
	IPBanDuration     int      `json:"ipBanDuration"`     // Ban duration in minutes (default: 60)
	AdminWhitelistIPs []string `json:"adminWhitelistIPs"` // IPs that cannot be banned (default: ["127.0.0.1"])
}

// BaseURLOption represents a selectable base URL option
type BaseURLOption struct {
	Name string `json:"name"` // Display name like "Production", "Staging"
	URL  string `json:"url"`  // The actual URL
}

// UIConfig represents UI customization options
type UIConfig struct {
	Theme       string `json:"theme"` // "light", "dark", "auto"
	ShowTryIt   bool   `json:"showTryIt"`
	ShowSchemas bool   `json:"showSchemas"`
	CustomCSS   string `json:"customCss"`
	CustomJS    string `json:"customJs"`
	Favicon     string `json:"favicon"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
}

// MiddlewareFunc represents middleware function
type MiddlewareFunc func(endpoint *Endpoint) *Endpoint

// RouteInfo represents route information from framework
type RouteInfo struct {
	Method      string
	Path        string
	Handler     interface{}
	Middlewares []interface{}
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses,omitempty"`
}

// Type aliases for backward compatibility
type AIConfig = ai.AIConfig
type AIFeatures = ai.AIFeatures
type ChatRequest = ai.ChatRequest
type ChatResponse = ai.ChatResponse
type LLMClient = ai.Client
