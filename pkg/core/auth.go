package core

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

func AuthMiddleware(config *AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config == nil || !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			if config.Type == "session" {
				sessionAuth, err := NewSessionAuthMiddleware(config)
				if err != nil {
					http.Error(w, "Failed to initialize session auth", http.StatusInternalServerError)
					return
				}
				sessionAuth.ServeHTTP(w, r, next)
				return
			}

			if err := authenticateRequest(r, config); err != nil {
				handleAuthError(w, r, config, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func authenticateRequest(r *http.Request, config *AuthConfig) error {
	switch config.Type {
	case "basic":
		return authenticateBasic(r, config)
	case "api_key":
		return authenticateAPIKey(r, config)
	case "bearer":
		return authenticateBearer(r, config)
	default:
		return fmt.Errorf("unsupported auth type: %s", config.Type)
	}
}

func authenticateBasic(r *http.Request, config *AuthConfig) error {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return fmt.Errorf("missing Authorization header")
	}

	if !strings.HasPrefix(auth, "Basic ") {
		return fmt.Errorf("invalid Authorization header format")
	}

	payload, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return fmt.Errorf("invalid base64 in Authorization header")
	}

	credentials := strings.SplitN(string(payload), ":", 2)
	if len(credentials) != 2 {
		return fmt.Errorf("invalid credential format")
	}

	username, password := credentials[0], credentials[1]

	if subtle.ConstantTimeCompare([]byte(username), []byte(config.Username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(config.Password)) != 1 {
		return fmt.Errorf("invalid credentials")
	}

	return nil
}

func authenticateAPIKey(r *http.Request, config *AuthConfig) error {
	headerName := config.APIKeyHeader
	if headerName == "" {
		headerName = "X-API-Key"
	}

	apiKey := r.Header.Get(headerName)
	if apiKey == "" {
		return fmt.Errorf("missing %s header", headerName)
	}

	if subtle.ConstantTimeCompare([]byte(apiKey), []byte(config.APIKey)) != 1 {
		return fmt.Errorf("invalid API key")
	}

	return nil
}

func authenticateBearer(r *http.Request, config *AuthConfig) error {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return fmt.Errorf("missing Authorization header")
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return fmt.Errorf("invalid Authorization header format")
	}

	token := strings.TrimSpace(auth[7:])
	if token == "" {
		return fmt.Errorf("missing bearer token")
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(config.APIKey)) != 1 {
		return fmt.Errorf("invalid bearer token")
	}

	return nil
}

func handleAuthError(w http.ResponseWriter, r *http.Request, config *AuthConfig, err error) {
	switch config.Type {
	case "basic":
		realm := config.Realm
		if realm == "" {
			realm = "ByteDocs API Documentation"
		}
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
	case "bearer":
		w.Header().Set("WWW-Authenticate", `Bearer realm="ByteDocs API Documentation"`)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	errorResponse := map[string]interface{}{
		"error": "Authentication required",
		"message": "Access to this resource requires authentication",
		"type": config.Type,
	}

	switch config.Type {
	case "basic":
		errorResponse["hint"] = "Use HTTP Basic Authentication with username and password"
	case "api_key":
		headerName := config.APIKeyHeader
		if headerName == "" {
			headerName = "X-API-Key"
		}
		errorResponse["hint"] = fmt.Sprintf("Provide API key in %s header", headerName)
	case "bearer":
		errorResponse["hint"] = "Use Authorization: Bearer <token> header"
	}

	w.Write([]byte(fmt.Sprintf(`{
		"error": "%s",
		"message": "%s", 
		"type": "%s",
		"hint": "%s"
	}`, 
		errorResponse["error"], 
		errorResponse["message"],
		errorResponse["type"],
		errorResponse["hint"],
	)))
}

func GinAuthMiddleware(config *AuthConfig) func(c interface{}) {
	return func(c interface{}) {
	}
}