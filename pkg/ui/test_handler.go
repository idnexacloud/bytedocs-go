package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TestRequest represents a test request
type TestRequest struct {
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Auth       TestAuthConfig    `json:"auth,omitempty"`
	Timeout    int               `json:"timeout,omitempty"`
}

// TestAuthConfig represents authentication for test requests
type TestAuthConfig struct {
	Type     string `json:"type"`     // "none", "bearer", "basic", "apikey"
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Header   string `json:"header,omitempty"`
}

// TestResponse represents a test response
type TestResponse struct {
	StatusCode   int                    `json:"status_code"`
	Headers      map[string][]string    `json:"headers"`
	Body         string                 `json:"body"`
	Duration     int64                  `json:"duration_ms"`
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
	RequestInfo  TestRequest            `json:"request_info"`
	ResponseSize int64                  `json:"response_size"`
	Timestamp    time.Time              `json:"timestamp"`
}

// serveTestEndpoint handles test execution requests
func (h *Handler) serveTestEndpoint(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var testReq TestRequest
	if err := json.NewDecoder(r.Body).Decode(&testReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Execute test request
	response := h.executeTestRequest(testReq)

	json.NewEncoder(w).Encode(response)
}

// executeTestRequest executes a test request and returns the response
func (h *Handler) executeTestRequest(testReq TestRequest) TestResponse {
	startTime := time.Now()

	response := TestResponse{
		RequestInfo: testReq,
		Timestamp:   startTime,
		Success:     false,
	}

	// Validate URL
	if testReq.URL == "" {
		response.Error = "URL is required"
		response.Duration = time.Since(startTime).Milliseconds()
		return response
	}

	// Build full URL with parameters
	fullURL := testReq.URL
	if len(testReq.Parameters) > 0 {
		// Add query parameters
		params := make([]string, 0, len(testReq.Parameters))
		for key, value := range testReq.Parameters {
			if value != "" {
				params = append(params, fmt.Sprintf("%s=%s", key, value))
			}
		}
		if len(params) > 0 {
			separator := "?"
			if strings.Contains(fullURL, "?") {
				separator = "&"
			}
			fullURL += separator + strings.Join(params, "&")
		}
	}

	// Create HTTP request
	var bodyReader io.Reader
	if testReq.Body != "" && (testReq.Method == "POST" || testReq.Method == "PUT" || testReq.Method == "PATCH") {
		bodyReader = strings.NewReader(testReq.Body)
	}

	req, err := http.NewRequest(testReq.Method, fullURL, bodyReader)
	if err != nil {
		response.Error = fmt.Sprintf("Failed to create request: %v", err)
		response.Duration = time.Since(startTime).Milliseconds()
		return response
	}

	// Set headers
	if len(testReq.Headers) > 0 {
		for key, value := range testReq.Headers {
			req.Header.Set(key, value)
		}
	}

	// Set Content-Type for requests with body
	if testReq.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set authentication
	h.setAuthentication(req, testReq.Auth)

	// Set timeout
	timeout := time.Duration(30) * time.Second // Default 30 seconds
	if testReq.Timeout > 0 {
		timeout = time.Duration(testReq.Timeout) * time.Millisecond
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		response.Error = fmt.Sprintf("Request failed: %v", err)
		response.Duration = time.Since(startTime).Milliseconds()
		return response
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		response.Error = fmt.Sprintf("Failed to read response: %v", err)
		response.Duration = time.Since(startTime).Milliseconds()
		return response
	}

	// Build response
	response.StatusCode = resp.StatusCode
	response.Headers = resp.Header
	response.Body = string(bodyBytes)
	response.ResponseSize = int64(len(bodyBytes))
	response.Duration = time.Since(startTime).Milliseconds()
	response.Success = resp.StatusCode >= 200 && resp.StatusCode < 400

	// Pretty format JSON response if possible
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		var jsonData interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
			if prettyJSON, err := json.MarshalIndent(jsonData, "", "  "); err == nil {
				response.Body = string(prettyJSON)
			}
		}
	}

	return response
}

// setAuthentication sets authentication headers based on auth config
func (h *Handler) setAuthentication(req *http.Request, auth TestAuthConfig) {
	switch auth.Type {
	case "bearer":
		if auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		}
	case "basic":
		if auth.Username != "" && auth.Password != "" {
			req.SetBasicAuth(auth.Username, auth.Password)
		}
	case "apikey":
		if auth.APIKey != "" {
			header := auth.Header
			if header == "" {
				header = "X-API-Key"
			}
			req.Header.Set(header, auth.APIKey)
		}
	}
}

// serveScenarioExecution handles scenario execution
func (h *Handler) serveScenarioExecution(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse scenario ID from URL
	path := strings.TrimPrefix(r.URL.Path, h.config.DocsPath+"/scenarios/")
	scenarioID := strings.TrimSuffix(path, "/execute")

	scenario, exists := scenarios[scenarioID]
	if !exists {
		http.Error(w, "Scenario not found", http.StatusNotFound)
		return
	}

	// Execute scenario
	results := h.executeScenario(scenario)

	json.NewEncoder(w).Encode(results)
}

// ScenarioExecutionResult represents the results of executing a scenario
type ScenarioExecutionResult struct {
	ScenarioID     string                  `json:"scenario_id"`
	Status         string                  `json:"status"` // "running", "completed", "failed"
	StartedAt      time.Time               `json:"started_at"`
	CompletedAt    *time.Time              `json:"completed_at,omitempty"`
	Duration       int64                   `json:"duration_ms"`
	TotalRequests  int                     `json:"total_requests"`
	Successful     int                     `json:"successful"`
	Failed         int                     `json:"failed"`
	Results        []ScenarioRequestResult `json:"results"`
	Variables      map[string]string       `json:"variables,omitempty"`
	Error          string                  `json:"error,omitempty"`
}

// ScenarioRequestResult represents the result of a single request in a scenario
type ScenarioRequestResult struct {
	RequestID    string      `json:"request_id"`
	Method       string      `json:"method"`
	URL          string      `json:"url"`
	StatusCode   int         `json:"status_code"`
	Duration     int64       `json:"duration_ms"`
	Success      bool        `json:"success"`
	Response     interface{} `json:"response,omitempty"`
	Error        string      `json:"error,omitempty"`
	Variables    map[string]string `json:"variables,omitempty"`
	Tests        []TestResult      `json:"tests,omitempty"`
}

// TestResult represents the result of a test assertion
type TestResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// executeScenario executes a complete scenario
func (h *Handler) executeScenario(scenario *Scenario) ScenarioExecutionResult {
	startTime := time.Now()
	result := ScenarioExecutionResult{
		ScenarioID:    scenario.ID,
		Status:        "running",
		StartedAt:     startTime,
		TotalRequests: len(scenario.Requests),
		Results:       make([]ScenarioRequestResult, 0, len(scenario.Requests)),
		Variables:     make(map[string]string),
	}

	// Initialize variables from scenario config
	for key, value := range scenario.Config.Environment {
		result.Variables[key] = value
	}

	successful := 0
	failed := 0

	// Execute requests based on execution mode
	if scenario.Config.ExecutionMode == "parallel" {
		// TODO: Implement parallel execution
		result.Error = "Parallel execution not yet implemented"
		result.Status = "failed"
	} else {
		// Sequential execution
		for _, scenarioReq := range scenario.Requests {
			requestResult := h.executeScenarioRequest(scenarioReq, scenario.Config, result.Variables)
			result.Results = append(result.Results, requestResult)

			if requestResult.Success {
				successful++
			} else {
				failed++
				if !scenario.Config.ContinueOnFail {
					break
				}
			}

			// Update variables from response
			for key, value := range requestResult.Variables {
				result.Variables[key] = value
			}
		}
	}

	// Complete execution
	completedAt := time.Now()
	result.CompletedAt = &completedAt
	result.Duration = completedAt.Sub(startTime).Milliseconds()
	result.Successful = successful
	result.Failed = failed

	if result.Error == "" {
		if failed == 0 {
			result.Status = "completed"
		} else {
			result.Status = "completed_with_errors"
		}
	}

	return result
}

// executeScenarioRequest executes a single request within a scenario
func (h *Handler) executeScenarioRequest(scenarioReq ScenarioRequest, config ScenarioConfig, variables map[string]string) ScenarioRequestResult {
	result := ScenarioRequestResult{
		RequestID: scenarioReq.ID,
		Method:    scenarioReq.Method,
		URL:       scenarioReq.URL,
		Success:   false,
		Variables: make(map[string]string),
	}

	// Build test request from scenario request
	testReq := TestRequest{
		Method:  scenarioReq.Method,
		URL:     h.replaceVariables(scenarioReq.URL, variables),
		Headers: scenarioReq.Headers,
		Body:    h.replaceVariables(scenarioReq.Body, variables),
		Auth: TestAuthConfig{
			Type:     config.Auth.Type,
			Token:    config.Auth.Token,
			Username: config.Auth.Username,
			Password: config.Auth.Password,
			APIKey:   config.Auth.APIKey,
			Header:   config.Auth.Header,
		},
		Timeout: config.Timeout,
	}

	// Use example body if configured
	if scenarioReq.Config.UseExampleBody && len(scenarioReq.Config.Body) > 0 {
		if bodyJSON, err := json.Marshal(scenarioReq.Config.Body); err == nil {
			testReq.Body = string(bodyJSON)
		}
	}

	// Execute the request
	testResponse := h.executeTestRequest(testReq)

	// Map test response to scenario result
	result.StatusCode = testResponse.StatusCode
	result.Duration = testResponse.Duration
	result.Success = testResponse.Success
	result.Error = testResponse.Error

	// Parse response for variable extraction
	if testResponse.Success && testResponse.Body != "" {
		var responseData interface{}
		if err := json.Unmarshal([]byte(testResponse.Body), &responseData); err == nil {
			result.Response = responseData
			// TODO: Extract variables from response based on scenario configuration
		} else {
			result.Response = testResponse.Body
		}
	}

	// TODO: Execute test assertions
	// For now, just basic status code check
	result.Tests = []TestResult{
		{
			Name:   "Status code is 2xx",
			Passed: testResponse.StatusCode >= 200 && testResponse.StatusCode < 300,
			Message: fmt.Sprintf("Expected 2xx, got %d", testResponse.StatusCode),
		},
	}

	return result
}

// replaceVariables replaces {{variable}} placeholders with actual values
func (h *Handler) replaceVariables(text string, variables map[string]string) string {
	result := text
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}