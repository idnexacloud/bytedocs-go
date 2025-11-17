package ui

import (
	"net/http"

	"github.com/idnexacloud/bytedocs-go/pkg/core"
)

// NewAuthenticatedHandler creates a UI handler with authentication middleware
func NewAuthenticatedHandler(docs *core.APIDocs, config *core.Config) http.Handler {
	// Create the base UI handler
	uiHandler := NewHandler(docs, config)

	// If auth is not enabled, return the handler as-is
	if config.AuthConfig == nil || !config.AuthConfig.Enabled {
		return uiHandler
	}

	// Create auth middleware
	authMiddleware := core.AuthMiddleware(config.AuthConfig)

	return authMiddleware(uiHandler)
}

// AuthenticatedHandlerFunc returns an http.HandlerFunc with authentication
func AuthenticatedHandlerFunc(docs *core.APIDocs, config *core.Config) http.HandlerFunc {
	handler := NewAuthenticatedHandler(docs, config)
	return handler.ServeHTTP
}