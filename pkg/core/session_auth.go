package core

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SessionAuthMiddleware represents session-based authentication middleware
type SessionAuthMiddleware struct {
	config    *AuthConfig
	templates map[string]*template.Template
	sessions  map[string]int64 // session ID -> auth time
	ipBans    map[string]int64 // IP -> ban expiry time
	attempts  map[string]int   // IP -> attempt count
	mutex     sync.RWMutex
}

// SessionData represents template data for auth views
type SessionData struct {
	Error           string
	ErrorTitle      string
	ErrorMessage    string
	ErrorDetails    []string
	MaxAttempts     int
	BanDuration     int
	ClientIP        string
	BlockedAt       string
}

// NewSessionAuthMiddleware creates a new session auth middleware
func NewSessionAuthMiddleware(config *AuthConfig) (*SessionAuthMiddleware, error) {
	if config == nil || config.Type != "session" {
		return nil, fmt.Errorf("invalid config for session auth")
	}

	middleware := &SessionAuthMiddleware{
		config:    config,
		templates: make(map[string]*template.Template),
		sessions:  make(map[string]int64),
		ipBans:    make(map[string]int64),
		attempts:  make(map[string]int),
	}

	// Load templates
	if err := middleware.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load auth templates: %w", err)
	}

	// Start cleanup routine
	go middleware.cleanupRoutine()

	return middleware, nil
}

// loadTemplates loads HTML templates for auth pages
func (m *SessionAuthMiddleware) loadTemplates() error {
	templatePaths := map[string]string{
		"login":        "pkg/ui/templates/auth/login.html",
		"banned":       "pkg/ui/templates/auth/banned.html",
		"config-error": "pkg/ui/templates/auth/config-error.html",
	}

	for name, path := range templatePaths {
		tmpl, err := template.ParseFiles(path)
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", name, err)
		}
		m.templates[name] = tmpl
	}

	return nil
}

// ServeHTTP implements http.Handler for session auth
func (m *SessionAuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.Handler) {
	// Skip auth if disabled
	if !m.config.Enabled {
		next.ServeHTTP(w, r)
		return
	}

	// Validate that password is configured
	if m.config.Password == "" {
		m.renderConfigError(w, r)
		return
	}

	ip := getClientIP(r)
	sessionID := m.getSessionID(r)

	// Check if IP is banned
	if m.isIPBanned(ip) {
		m.renderBanned(w, r, ip)
		return
	}

	// Check if already authenticated
	if m.isAuthenticated(sessionID) {
		next.ServeHTTP(w, r)
		return
	}

	if r.Method == "POST" && r.FormValue("password") != "" {
		m.handleLogin(w, r, next, ip, sessionID)
		return
	}

	// Show login form
	m.renderLogin(w, r, "")
}

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// getSessionID extracts session ID from cookie
func (m *SessionAuthMiddleware) getSessionID(r *http.Request) string {
	cookie, err := r.Cookie("bytedocs_session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// isIPBanned checks if IP is currently banned
func (m *SessionAuthMiddleware) isIPBanned(ip string) bool {
	if !m.config.IPBanEnabled {
		return false
	}

	// Check if IP is whitelisted
	for _, whitelistIP := range m.config.AdminWhitelistIPs {
		if ip == whitelistIP {
			return false
		}
	}

	m.mutex.RLock()
	banExpiry, exists := m.ipBans[ip]
	m.mutex.RUnlock()

	if !exists {
		return false
	}

	// Check if ban has expired
	if time.Now().Unix() > banExpiry {
		m.mutex.Lock()
		delete(m.ipBans, ip)
		delete(m.attempts, ip)
		m.mutex.Unlock()
		return false
	}

	return true
}

// isAuthenticated checks if session is valid
func (m *SessionAuthMiddleware) isAuthenticated(sessionID string) bool {
	if sessionID == "" {
		return false
	}

	m.mutex.RLock()
	authTime, exists := m.sessions[sessionID]
	m.mutex.RUnlock()

	if !exists {
		return false
	}

	// Check session expiration
	expirationTime := authTime + int64(m.config.SessionExpire*60)
	if time.Now().Unix() > expirationTime {
		m.mutex.Lock()
		delete(m.sessions, sessionID)
		m.mutex.Unlock()
		return false
	}

	return true
}

// handleLogin processes login form submission
func (m *SessionAuthMiddleware) handleLogin(w http.ResponseWriter, r *http.Request, next http.Handler, ip, sessionID string) {
	password := r.FormValue("password")

	// Check password
	if subtle.ConstantTimeCompare([]byte(password), []byte(m.config.Password)) == 1 {
		// Success - clear attempts and set session
		m.mutex.Lock()
		delete(m.attempts, ip)

		// Generate session ID if not exists
		if sessionID == "" {
			sessionID = generateSessionID()
		}

		m.sessions[sessionID] = time.Now().Unix()
		m.mutex.Unlock()

		// Set session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "bytedocs_session",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			MaxAge:   m.config.SessionExpire * 60,
		})

		// Clear any error cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "bytedocs_auth_error",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		next.ServeHTTP(w, r)
		return
	}

	// Failed login - increment attempts
	m.mutex.Lock()
	attempts := m.attempts[ip] + 1
	m.attempts[ip] = attempts
	m.mutex.Unlock()

	// Ban IP if max attempts reached (unless whitelisted)
	if attempts >= m.config.IPBanMaxAttempts && m.config.IPBanEnabled {
		isWhitelisted := false
		for _, whitelistIP := range m.config.AdminWhitelistIPs {
			if ip == whitelistIP {
				isWhitelisted = true
				break
			}
		}

		if !isWhitelisted {
			banExpiry := time.Now().Add(time.Duration(m.config.IPBanDuration) * time.Minute).Unix()
			m.mutex.Lock()
			m.ipBans[ip] = banExpiry
			delete(m.attempts, ip)
			m.mutex.Unlock()

			m.renderBanned(w, r, ip)
			return
		} else {
			// If IP is whitelisted, just reset attempts instead of banning
			m.mutex.Lock()
			delete(m.attempts, ip)
			m.mutex.Unlock()
		}
	}

	// Show error
	remainingAttempts := m.config.IPBanMaxAttempts - attempts
	errorMessage := fmt.Sprintf("Password salah. Sisa percobaan: %d", remainingAttempts)

	// Set error cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "bytedocs_auth_error",
		Value:    errorMessage,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   300, // 5 minutes
	})

	m.renderLogin(w, r, errorMessage)
}

// renderLogin renders the login page
func (m *SessionAuthMiddleware) renderLogin(w http.ResponseWriter, r *http.Request, error string) {
	// Check for error in cookie if not provided
	if error == "" {
		if cookie, err := r.Cookie("bytedocs_auth_error"); err == nil {
			error = cookie.Value
		}
	}

	data := SessionData{
		Error: error,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	m.templates["login"].Execute(w, data)
}

// renderBanned renders the banned page
func (m *SessionAuthMiddleware) renderBanned(w http.ResponseWriter, r *http.Request, ip string) {
	data := SessionData{
		MaxAttempts: m.config.IPBanMaxAttempts,
		BanDuration: m.config.IPBanDuration,
		ClientIP:    ip,
		BlockedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	m.templates["banned"].Execute(w, data)
}

// renderConfigError renders the configuration error page
func (m *SessionAuthMiddleware) renderConfigError(w http.ResponseWriter, r *http.Request) {
	data := SessionData{
		ErrorTitle:   "Authentication Not Configured",
		ErrorMessage: "ByteDocs authentication is enabled but no password is configured.",
		ErrorDetails: []string{
			"Please set BYTEDOCS_AUTH_PASSWORD in your environment variables",
			"Or disable authentication by setting BYTEDOCS_AUTH_ENABLED=false",
			"Check your configuration settings",
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	m.templates["config-error"].Execute(w, data)
}

// cleanupRoutine periodically cleans up expired sessions and bans
func (m *SessionAuthMiddleware) cleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().Unix()

		m.mutex.Lock()

		// Clean up expired sessions
		for sessionID, authTime := range m.sessions {
			if now > authTime+int64(m.config.SessionExpire*60) {
				delete(m.sessions, sessionID)
			}
		}

		// Clean up expired bans
		for ip, banExpiry := range m.ipBans {
			if now > banExpiry {
				delete(m.ipBans, ip)
				delete(m.attempts, ip)
			}
		}

		m.mutex.Unlock()
	}
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess_%d_%d", time.Now().UnixNano(), time.Now().Unix())
	}
	return base64.URLEncoding.EncodeToString(b)
}