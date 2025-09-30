package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Scenario represents a test scenario
type Scenario struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Requests    []ScenarioRequest      `json:"requests"`
	Config      ScenarioConfig         `json:"config"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ScenarioRequest represents a request in a scenario
type ScenarioRequest struct {
	ID           string                 `json:"id"`
	Method       string                 `json:"method"`
	URL          string                 `json:"url"`
	Headers      map[string]string      `json:"headers,omitempty"`
	Body         string                 `json:"body,omitempty"`
	Config       RequestConfig          `json:"config"`
	Variables    map[string]string      `json:"variables,omitempty"`
	Tests        []string               `json:"tests,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
}

// ScenarioConfig represents scenario configuration
type ScenarioConfig struct {
	ExecutionMode  string            `json:"execution_mode"` // "sequential" or "parallel"
	ContinueOnFail bool              `json:"continue_on_fail"`
	Timeout        int               `json:"timeout"`
	BaseURL        string            `json:"base_url"`
	Auth           AuthConfig        `json:"auth"`
	Environment    map[string]string `json:"environment,omitempty"`
}

// RequestConfig represents request-specific configuration
type RequestConfig struct {
	UseExampleBody bool              `json:"use_example_body"`
	Body           map[string]interface{} `json:"body,omitempty"`
	Timeout        int               `json:"timeout"`
	FollowRedirect bool              `json:"follow_redirect"`
}

// AuthConfig represents authentication configuration for scenarios
type AuthConfig struct {
	Type     string `json:"type"`     // "none", "bearer", "basic", "apikey"
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Header   string `json:"header,omitempty"`
}

// In-memory storage for scenarios (in production, use database)
var scenarios = make(map[string]*Scenario)
var scenarioCounter = 0

// generateScenarioID generates a unique scenario ID
func generateScenarioID() string {
	scenarioCounter++
	return fmt.Sprintf("scenario_%d_%d", time.Now().Unix(), scenarioCounter)
}

// serveScenarios handles scenario management endpoints
func (h *Handler) serveScenarios(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse URL path to get scenario ID if present
	path := strings.TrimPrefix(r.URL.Path, h.config.DocsPath+"/scenarios")
	if path == "" {
		path = "/"
	}

	switch {
	case path == "/" && r.Method == "GET":
		h.listScenarios(w, r)
	case path == "/" && r.Method == "POST":
		h.createScenario(w, r)
	case path == "/export" && r.Method == "GET":
		h.exportScenarios(w, r)
	case path == "/import" && r.Method == "POST":
		h.importScenarios(w, r)
	case strings.HasPrefix(path, "/") && r.Method == "GET":
		scenarioID := strings.TrimPrefix(path, "/")
		h.getScenario(w, r, scenarioID)
	case strings.HasPrefix(path, "/") && r.Method == "PUT":
		scenarioID := strings.TrimPrefix(path, "/")
		h.updateScenario(w, r, scenarioID)
	case strings.HasPrefix(path, "/") && r.Method == "DELETE":
		scenarioID := strings.TrimPrefix(path, "/")
		h.deleteScenario(w, r, scenarioID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listScenarios returns all scenarios
func (h *Handler) listScenarios(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	scenarioList := make([]*Scenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		scenarioList = append(scenarioList, scenario)
	}

	response := map[string]interface{}{
		"scenarios": scenarioList,
		"count":     len(scenarioList),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode scenarios", http.StatusInternalServerError)
	}
}

// createScenario creates a new scenario
func (h *Handler) createScenario(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var scenario Scenario
	if err := json.NewDecoder(r.Body).Decode(&scenario); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Generate ID and timestamps
	scenario.ID = generateScenarioID()
	scenario.CreatedAt = time.Now()
	scenario.UpdatedAt = time.Now()

	// Validate required fields
	if scenario.Name == "" {
		http.Error(w, "Scenario name is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if scenario.Config.ExecutionMode == "" {
		scenario.Config.ExecutionMode = "sequential"
	}
	if scenario.Config.Timeout == 0 {
		scenario.Config.Timeout = 30000 // 30 seconds
	}

	scenarios[scenario.ID] = &scenario

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(scenario)
}

// getScenario returns a specific scenario
func (h *Handler) getScenario(w http.ResponseWriter, r *http.Request, scenarioID string) {
	w.Header().Set("Content-Type", "application/json")

	scenario, exists := scenarios[scenarioID]
	if !exists {
		http.Error(w, "Scenario not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(scenario)
}

// updateScenario updates an existing scenario
func (h *Handler) updateScenario(w http.ResponseWriter, r *http.Request, scenarioID string) {
	w.Header().Set("Content-Type", "application/json")

	scenario, exists := scenarios[scenarioID]
	if !exists {
		http.Error(w, "Scenario not found", http.StatusNotFound)
		return
	}

	var updates Scenario
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Update fields while preserving ID and created timestamp
	updates.ID = scenario.ID
	updates.CreatedAt = scenario.CreatedAt
	updates.UpdatedAt = time.Now()

	// Validate required fields
	if updates.Name == "" {
		http.Error(w, "Scenario name is required", http.StatusBadRequest)
		return
	}

	scenarios[scenarioID] = &updates

	json.NewEncoder(w).Encode(updates)
}

// deleteScenario deletes a scenario
func (h *Handler) deleteScenario(w http.ResponseWriter, r *http.Request, scenarioID string) {
	_, exists := scenarios[scenarioID]
	if !exists {
		http.Error(w, "Scenario not found", http.StatusNotFound)
		return
	}

	delete(scenarios, scenarioID)
	w.WriteHeader(http.StatusNoContent)
}

// exportScenarios exports all scenarios to JSON
func (h *Handler) exportScenarios(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=scenarios.json")

	scenarioList := make([]*Scenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		scenarioList = append(scenarioList, scenario)
	}

	exportData := map[string]interface{}{
		"scenarios":     scenarioList,
		"exported_at":   time.Now(),
		"exported_by":   "ByteDocs",
		"format_version": "1.0",
	}

	if err := json.NewEncoder(w).Encode(exportData); err != nil {
		http.Error(w, "Failed to export scenarios", http.StatusInternalServerError)
	}
}

// importScenarios imports scenarios from JSON
func (h *Handler) importScenarios(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var importData struct {
		Scenarios []Scenario `json:"scenarios"`
		ReplaceAll bool      `json:"replace_all"`
	}

	if err := json.NewDecoder(r.Body).Decode(&importData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// If replace_all is true, clear existing scenarios
	if importData.ReplaceAll {
		scenarios = make(map[string]*Scenario)
	}

	imported := 0
	skipped := 0
	errors := []string{}

	for _, scenario := range importData.Scenarios {
		// Validate required fields
		if scenario.Name == "" {
			errors = append(errors, fmt.Sprintf("Scenario missing name: %s", scenario.ID))
			continue
		}

		// Generate new ID if not exists or conflicts
		if scenario.ID == "" || scenarios[scenario.ID] != nil {
			scenario.ID = generateScenarioID()
		}

		// Update timestamps
		scenario.UpdatedAt = time.Now()
		if scenario.CreatedAt.IsZero() {
			scenario.CreatedAt = time.Now()
		}

		scenarios[scenario.ID] = &scenario
		imported++
	}

	response := map[string]interface{}{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errors,
		"total":    len(importData.Scenarios),
	}

	json.NewEncoder(w).Encode(response)
}

// executeScenario executes a scenario and returns results
func (h *Handler) executeScenario(w http.ResponseWriter, r *http.Request, scenarioID string) {
	w.Header().Set("Content-Type", "application/json")

	scenario, exists := scenarios[scenarioID]
	if !exists {
		http.Error(w, "Scenario not found", http.StatusNotFound)
		return
	}

	// TODO: Implement actual scenario execution logic
	// For now, return mock execution results

	results := map[string]interface{}{
		"scenario_id":  scenario.ID,
		"status":       "completed",
		"started_at":   time.Now(),
		"completed_at": time.Now().Add(time.Second * 5),
		"duration_ms":  5000,
		"total_requests": len(scenario.Requests),
		"successful":   len(scenario.Requests),
		"failed":       0,
		"results":      []map[string]interface{}{},
	}

	// Mock individual request results
	for i, req := range scenario.Requests {
		result := map[string]interface{}{
			"request_id":   req.ID,
			"method":       req.Method,
			"url":         req.URL,
			"status_code": 200,
			"duration_ms": 100 + i*50,
			"success":     true,
			"response":    map[string]interface{}{"status": "ok"},
		}
		results["results"] = append(results["results"].([]map[string]interface{}), result)
	}

	json.NewEncoder(w).Encode(results)
}