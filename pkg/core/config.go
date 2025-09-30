package core

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/aibnuhibban/bytedocs/pkg/ai"
)

// LoadConfigFromEnv loads configuration from environment variables
// Optionally loads from .env file if it exists
func LoadConfigFromEnv(envFile ...string) (*Config, error) {
	// Try to load .env file if specified or if .env exists
	envPath := ".env"
	if len(envFile) > 0 && envFile[0] != "" {
		envPath = envFile[0]
	}
	
	// Load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load(envPath)

	config := &Config{
		Title:       getEnvOrDefault("BYTEDOCS_TITLE", "API Documentation"),
		Version:     getEnvOrDefault("BYTEDOCS_VERSION", "1.0.0"),
		Description: getEnvOrDefault("BYTEDOCS_DESCRIPTION", "Auto-generated API documentation"),
		BaseURL:     getEnvOrDefault("BYTEDOCS_BASE_URL", "http://localhost:8080"),
		DocsPath:    getEnvOrDefault("BYTEDOCS_DOCS_PATH", "/docs"),
		AutoDetect:  getEnvBool("BYTEDOCS_AUTO_DETECT", true),
		ExcludePaths: getEnvSlice("BYTEDOCS_EXCLUDE_PATHS", []string{"_ignition", "debug", "health"}),
	}

	// Load multiple base URLs if provided
	if prodURL := os.Getenv("BYTEDOCS_PRODUCTION_URL"); prodURL != "" {
		config.BaseURLs = append(config.BaseURLs, BaseURLOption{
			Name: "Production",
			URL:  prodURL,
		})
	}
	if stagingURL := os.Getenv("BYTEDOCS_STAGING_URL"); stagingURL != "" {
		config.BaseURLs = append(config.BaseURLs, BaseURLOption{
			Name: "Staging", 
			URL:  stagingURL,
		})
	}
	if localURL := os.Getenv("BYTEDOCS_LOCAL_URL"); localURL != "" {
		config.BaseURLs = append(config.BaseURLs, BaseURLOption{
			Name: "Local",
			URL:  localURL,
		})
	}

	// Load authentication config
	if getEnvBool("BYTEDOCS_AUTH_ENABLED", false) {
		config.AuthConfig = &AuthConfig{
			Enabled:      true,
			Type:         getEnvOrDefault("BYTEDOCS_AUTH_TYPE", "session"),
			Username:     getEnvOrDefault("BYTEDOCS_AUTH_USERNAME", "admin"),
			Password:     getEnvOrDefault("BYTEDOCS_AUTH_PASSWORD", ""),
			APIKey:       getEnvOrDefault("BYTEDOCS_AUTH_API_KEY", ""),
			APIKeyHeader: getEnvOrDefault("BYTEDOCS_AUTH_API_KEY_HEADER", "X-API-Key"),
			Realm:        getEnvOrDefault("BYTEDOCS_AUTH_REALM", "ByteDocs API Documentation"),

			// Session auth configuration
			SessionExpire:        getEnvInt("BYTEDOCS_AUTH_SESSION_EXPIRE", 1440),
			IPBanEnabled:         getEnvBool("BYTEDOCS_AUTH_IP_BAN_ENABLED", true),
			IPBanMaxAttempts:     getEnvInt("BYTEDOCS_AUTH_IP_BAN_MAX_ATTEMPTS", 5),
			IPBanDuration:        getEnvInt("BYTEDOCS_AUTH_IP_BAN_DURATION", 60),
			AdminWhitelistIPs:    getEnvSlice("BYTEDOCS_AUTH_ADMIN_WHITELIST_IPS", []string{"127.0.0.1"}),
		}
	}

	// Load UI config
	if hasUIConfig() {
		config.UIConfig = &UIConfig{
			Theme:       getEnvOrDefault("BYTEDOCS_UI_THEME", "auto"),
			ShowTryIt:   getEnvBool("BYTEDOCS_UI_SHOW_TRY_IT", true),
			ShowSchemas: getEnvBool("BYTEDOCS_UI_SHOW_SCHEMAS", true),
			CustomCSS:   getEnvOrDefault("BYTEDOCS_UI_CUSTOM_CSS", ""),
			CustomJS:    getEnvOrDefault("BYTEDOCS_UI_CUSTOM_JS", ""),
			Favicon:     getEnvOrDefault("BYTEDOCS_UI_FAVICON", ""),
			Title:       getEnvOrDefault("BYTEDOCS_UI_TITLE", ""),
			Subtitle:    getEnvOrDefault("BYTEDOCS_UI_SUBTITLE", ""),
		}
	}

	// Load AI config
	if getEnvBool("BYTEDOCS_AI_ENABLED", false) {
		config.AIConfig = &ai.AIConfig{
			Enabled:  true,
			Provider: getEnvOrDefault("BYTEDOCS_AI_PROVIDER", "openai"),
			APIKey:   getEnvOrDefault("BYTEDOCS_AI_API_KEY", ""),
			Features: ai.AIFeatures{
				ChatEnabled:          getEnvBool("BYTEDOCS_AI_CHAT_ENABLED", true),
				DocGenerationEnabled: getEnvBool("BYTEDOCS_AI_DOC_GEN_ENABLED", false),
				Model:                getEnvOrDefault("BYTEDOCS_AI_MODEL", "gpt-4o-mini"),
				MaxTokens:            getEnvInt("BYTEDOCS_AI_MAX_TOKENS", 1000),
				MaxCompletionTokens:  getEnvInt("BYTEDOCS_AI_MAX_COMPLETION_TOKENS", 1000),
				Temperature:          getEnvFloat("BYTEDOCS_AI_TEMPERATURE", 0.7),
			},
			Settings: map[string]interface{}{
				"app_name": getEnvOrDefault("APP_NAME", "ByteDocs API"),
				"app_url":  getEnvOrDefault("APP_URL", "http://localhost:8080"),
				"base_url": getEnvOrDefault("BYTEDOCS_AI_BASE_URL", ""),
			},
		}
	}

	return config, nil
}

// ValidateConfig validates the configuration
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate required fields
	if config.Title == "" {
		return fmt.Errorf("title is required")
	}
	if config.Version == "" {
		return fmt.Errorf("version is required")
	}
	if config.DocsPath == "" {
		return fmt.Errorf("docs path is required")
	}
	if !strings.HasPrefix(config.DocsPath, "/") {
		return fmt.Errorf("docs path must start with /")
	}

	// Validate auth config
	if config.AuthConfig != nil && config.AuthConfig.Enabled {
		if err := validateAuthConfig(config.AuthConfig); err != nil {
			return fmt.Errorf("auth config validation failed: %w", err)
		}
	}

	// Validate AI config
	if config.AIConfig != nil && config.AIConfig.Enabled {
		if err := validateAIConfig(config.AIConfig); err != nil {
			return fmt.Errorf("ai config validation failed: %w", err)
		}
	}

	// Validate base URLs
	if config.BaseURL == "" && len(config.BaseURLs) == 0 {
		return fmt.Errorf("at least one base URL must be provided")
	}

	return nil
}

// validateAuthConfig validates authentication configuration
func validateAuthConfig(auth *AuthConfig) error {
	if !auth.Enabled {
		return nil
	}

	switch auth.Type {
	case "basic":
		if auth.Username == "" || auth.Password == "" {
			return fmt.Errorf("basic auth requires both username and password")
		}
	case "api_key", "bearer":
		if auth.APIKey == "" {
			return fmt.Errorf("%s auth requires API key", auth.Type)
		}
		if auth.APIKeyHeader == "" {
			auth.APIKeyHeader = "X-API-Key" // Set default
		}
	case "session":
		if auth.Password == "" {
			return fmt.Errorf("session auth requires password")
		}
		// Set defaults for session auth
		if auth.SessionExpire <= 0 {
			auth.SessionExpire = 1440 // 24 hours
		}
		if auth.IPBanMaxAttempts <= 0 {
			auth.IPBanMaxAttempts = 5
		}
		if auth.IPBanDuration <= 0 {
			auth.IPBanDuration = 60 // 1 hour
		}
		if len(auth.AdminWhitelistIPs) == 0 {
			auth.AdminWhitelistIPs = []string{"127.0.0.1"}
		}
	default:
		return fmt.Errorf("unsupported auth type: %s (supported: basic, api_key, bearer, session)", auth.Type)
	}

	return nil
}

// validateAIConfig validates AI configuration
func validateAIConfig(ai *ai.AIConfig) error {
	if !ai.Enabled {
		return nil
	}

	if ai.APIKey == "" {
		return fmt.Errorf("AI API key is required when AI is enabled")
	}

	supportedProviders := []string{"openai", "gemini", "openrouter", "claude"}
	isSupported := false
	for _, provider := range supportedProviders {
		if ai.Provider == provider {
			isSupported = true
			break
		}
	}
	if !isSupported {
		return fmt.Errorf("unsupported AI provider: %s (supported: %s)", ai.Provider, strings.Join(supportedProviders, ", "))
	}

	if ai.Features.MaxTokens < 1 {
		return fmt.Errorf("max tokens must be greater than 0")
	}
	if ai.Features.Temperature < 0 || ai.Features.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	return nil
}

// Helper functions for environment variable parsing
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func hasUIConfig() bool {
	uiKeys := []string{
		"BYTEDOCS_UI_THEME",
		"BYTEDOCS_UI_SHOW_TRY_IT", 
		"BYTEDOCS_UI_SHOW_SCHEMAS",
		"BYTEDOCS_UI_CUSTOM_CSS",
		"BYTEDOCS_UI_CUSTOM_JS",
		"BYTEDOCS_UI_FAVICON",
		"BYTEDOCS_UI_TITLE",
		"BYTEDOCS_UI_SUBTITLE",
	}

	for _, key := range uiKeys {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}