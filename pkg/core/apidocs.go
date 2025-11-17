package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	_ "github.com/aibnuhibban/bytedocs/pkg/llm"
	"gopkg.in/yaml.v3"
)

type APIDocs struct {
	config        *Config
	documentation *Documentation
	routes        []RouteInfo
	schemas       map[string]Schema
	llmClient     LLMClient
}

func convertPathToOpenAPI(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			param := strings.TrimPrefix(part, ":")
			parts[i] = "{" + param + "}"
		}
	}
	result := strings.Join(parts, "/")

	result = strings.ReplaceAll(result, "<", "{")
	result = strings.ReplaceAll(result, ">", "}")

	muxParamRegex := regexp.MustCompile(`\{([^{}:]+):[^{}]+\}`)
	result = muxParamRegex.ReplaceAllString(result, `{$1}`)

	result = strings.ReplaceAll(result, "{}/", "/")
	if strings.HasPrefix(result, "{}") {
		result = strings.TrimPrefix(result, "{}")
	}

	return result
}

func normalizeOpenAPIType(goType string) string {
	switch strings.ToLower(goType) {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "bool", "boolean":
		return "boolean"
	case "string", "":
		return "string"
	case "array", "slice", "[]string", "[]int":
		return "array"
	case "object", "map", "interface{}":
		return "object"
	default:
		return "string"
	}
}

func New(config *Config) *APIDocs {
	if config == nil {
		config = &Config{
			Title:       "API Documentation",
			Version:     "1.0.0",
			Description: "Auto-generated API documentation",
			DocsPath:    "/docs",
			AutoDetect:  true,
		}
	}

	var llmClient LLMClient
	if config.AIConfig != nil && config.AIConfig.Enabled {
		client, err := NewLLMClient(config.AIConfig)
		if err == nil {
			llmClient = client
		}
	}

	return &APIDocs{
		config:    config,
		routes:    make([]RouteInfo, 0),
		schemas:   make(map[string]Schema),
		llmClient: llmClient,
		documentation: &Documentation{
			Info: APIInfo{
				Title:       config.Title,
				Version:     config.Version,
				Description: config.Description,
				BaseURL:     config.BaseURL,
			},
			Endpoints: make([]EndpointSection, 0),
			Schemas:   make(map[string]Schema),
		},
	}
}

func (a *APIDocs) AddRouteInfo(route RouteInfo) {
	a.routes = append(a.routes, route)
}

func (a *APIDocs) GetConfig() *Config {
	return a.config
}

func (a *APIDocs) AddRoute(method, path string, handler interface{}, options ...RouteOption) {
	route := RouteInfo{
		Method:  strings.ToUpper(method),
		Path:    path,
		Handler: handler,
	}

	for _, option := range options {
		option(&route)
	}

	a.routes = append(a.routes, route)
}

type RouteOption func(*RouteInfo)

func (a *APIDocs) Generate() error {
	sections := make(map[string]*EndpointSection)

	for _, route := range a.routes {
		endpoint := a.processRoute(route)
		sectionName := a.extractSection(endpoint.Path)

		if sections[sectionName] == nil {
			sections[sectionName] = &EndpointSection{
				ID:          sectionName,
				Name:        a.formatSectionName(sectionName),
				Description: fmt.Sprintf("%s related endpoints", a.formatSectionName(sectionName)),
				Endpoints:   make([]Endpoint, 0),
			}
		}

		sections[sectionName].Endpoints = append(sections[sectionName].Endpoints, *endpoint)
	}

	a.documentation.Endpoints = make([]EndpointSection, 0, len(sections))
	for _, section := range sections {
		a.documentation.Endpoints = append(a.documentation.Endpoints, *section)
	}

	return nil
}

func (a *APIDocs) processRoute(route RouteInfo) *Endpoint {
	displayPath := convertPathToOpenAPI(route.Path)
	
	summary := route.Summary
	if summary == "" {
		summary = a.generateSummary(route.Method, displayPath)
	}

	description := route.Description
	if description == "" {
		description = summary
	}

	pathParams := a.extractParameters(route.Path, route.Handler)
	allParams := a.mergeParameters(pathParams, route.Parameters)

	requestBody := route.RequestBody
	if requestBody == nil {
		requestBody = a.extractRequestBody(route.Handler)
	}

	responses := route.Responses
	if len(responses) == 0 {
		responses = a.generateResponses(route.Handler)
	}

	endpoint := &Endpoint{
		ID:          a.generateID(route.Method, displayPath),
		Method:      route.Method,
		Path:        displayPath,
		Summary:     summary,
		Description: description,
		Parameters:  allParams,
		RequestBody: requestBody,
		Responses:   responses,
		Handler:     reflect.ValueOf(route.Handler),
	}

	return endpoint
}

func (a *APIDocs) extractParameters(path string, handler interface{}) []Parameter {
	params := make([]Parameter, 0)

	pathParams := extractPathParams(path)
	for _, param := range pathParams {
		params = append(params, Parameter{
			Name:     param,
			In:       "path",
			Type:     "string",
			Required: true,
		})
	}

	if handler != nil {
		queryParams := a.analyzeHandlerParameters(handler)
		params = append(params, queryParams...)
	}

	return params
}

func (a *APIDocs) analyzeHandlerParameters(handler interface{}) []Parameter {
	params := make([]Parameter, 0)

	if handler == nil {
		return params
	}

	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() != reflect.Func {
		return params
	}

	return params
}

func (a *APIDocs) mergeParameters(pathParams, providedParams []Parameter) []Parameter {
	paramMap := make(map[string]Parameter)

	for _, param := range pathParams {
		key := param.Name + ":" + param.In
		paramMap[key] = param
	}

	for _, param := range providedParams {
		key := param.Name + ":" + param.In
		paramMap[key] = param
	}

	result := make([]Parameter, 0, len(paramMap))
	for _, param := range paramMap {
		result = append(result, param)
	}

	return result
}

func (a *APIDocs) extractRequestBody(handler interface{}) *RequestBody {
	return nil
}

func (a *APIDocs) generateResponses(handler interface{}) map[string]Response {
	responses := make(map[string]Response)

	responses["200"] = Response{
		Description: "Success",
		Example: map[string]interface{}{
			"status": "success",
		},
	}
	responses["400"] = Response{Description: "Bad Request"}
	responses["404"] = Response{Description: "Not Found"}
	responses["500"] = Response{Description: "Internal Server Error"}

	return responses
}

func (a *APIDocs) extractSection(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")

	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if part != "" && !strings.HasPrefix(part, ":") && !strings.Contains(part, "{") {
			if part != "api" && !strings.HasPrefix(part, "v") {
				return part
			}
		}
	}

	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "default"
}

func (a *APIDocs) formatSectionName(section string) string {
	return strings.Title(section)
}

func (a *APIDocs) generateID(method, path string) string {
	return fmt.Sprintf("%s-%s", strings.ToLower(method),
		strings.ReplaceAll(strings.ReplaceAll(path, "/", "-"), ":", ""))
}

func (a *APIDocs) generateSummary(method, path string) string {
	section := a.extractSection(path)
	action := a.inferAction(method, path)
	return fmt.Sprintf("%s %s", action, section)
}

func (a *APIDocs) inferAction(method, path string) string {
	switch strings.ToUpper(method) {
	case "GET":
		hasParam := strings.Contains(path, ":") || strings.Contains(path, "{")
		if hasParam {
			return "Get"
		}
		return "List"
	case "POST":
		return "Create"
	case "PUT", "PATCH":
		return "Update"
	case "DELETE":
		return "Delete"
	default:
		return method
	}
}

func (a *APIDocs) GetDocumentation() *Documentation {
	return a.documentation
}

func (a *APIDocs) GetOpenAPIJSON() (map[string]interface{}, error) {
	if err := a.Generate(); err != nil {
		return nil, err
	}

	openAPI := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       a.documentation.Info.Title,
			"version":     a.documentation.Info.Version,
			"description": a.documentation.Info.Description,
		},
		"servers": []map[string]interface{}{},
		"paths":   map[string]interface{}{},
		"components": map[string]interface{}{
			"schemas": a.documentation.Schemas,
		},
	}

	if a.config.BaseURL != "" {
		openAPI["servers"] = []map[string]interface{}{
			{"url": a.config.BaseURL},
		}
	}
	if len(a.config.BaseURLs) > 0 {
		servers := make([]map[string]interface{}, 0)
		for _, baseURL := range a.config.BaseURLs {
			servers = append(servers, map[string]interface{}{
				"url":         baseURL.URL,
				"description": baseURL.Name,
			})
		}
		openAPI["servers"] = servers
	}

	paths := make(map[string]interface{})
	for _, section := range a.documentation.Endpoints {
		for _, endpoint := range section.Endpoints {
			pathKey := convertPathToOpenAPI(endpoint.Path)
			if paths[pathKey] == nil {
				paths[pathKey] = make(map[string]interface{})
			}

			pathItem := paths[pathKey].(map[string]interface{})
			methodKey := strings.ToLower(endpoint.Method)

			operation := map[string]interface{}{
				"summary":     endpoint.Summary,
				"description": endpoint.Description,
				"tags":        []string{section.Name},
				"operationId": endpoint.ID,
				"parameters":  []map[string]interface{}{},
				"responses":   map[string]interface{}{},
			}

			if len(endpoint.Parameters) > 0 {
				params := make([]map[string]interface{}, 0)
				for _, param := range endpoint.Parameters {
					params = append(params, map[string]interface{}{
						"name":        param.Name,
						"in":          param.In,
						"required":    param.Required,
						"description": param.Description,
						"schema": map[string]interface{}{
							"type": normalizeOpenAPIType(param.Type),
						},
						"example": param.Example,
					})
				}
				operation["parameters"] = params
			}

			if endpoint.RequestBody != nil {
				contentType := endpoint.RequestBody.ContentType
				if contentType == "" {
					contentType = "application/json"
				}
				operation["requestBody"] = map[string]interface{}{
					"required": endpoint.RequestBody.Required,
					"content": map[string]interface{}{
						contentType: map[string]interface{}{
							"schema":  endpoint.RequestBody.Schema,
							"example": endpoint.RequestBody.Example,
						},
					},
				}
			}

			responses := make(map[string]interface{})
			for statusCode, response := range endpoint.Responses {
				respContentType := response.ContentType
				if respContentType == "" {
					respContentType = "application/json"
				}
				responses[statusCode] = map[string]interface{}{
					"description": response.Description,
					"content": map[string]interface{}{
						respContentType: map[string]interface{}{
							"schema":  response.Schema,
							"example": response.Example,
						},
					},
				}
			}
			operation["responses"] = responses

			pathItem[methodKey] = operation
			paths[pathKey] = pathItem
		}
	}

	openAPI["paths"] = paths
	return openAPI, nil
}

func (a *APIDocs) GetOpenAPIYAML() ([]byte, error) {
	openAPIMap, err := a.GetOpenAPIJSON()
	if err != nil {
		return nil, err
	}

	yamlBytes, err := yaml.Marshal(openAPIMap)
	if err != nil {
		return nil, err
	}

	return yamlBytes, nil
}

func (a *APIDocs) GetAPIContext() (string, error) {
	openAPIJSON, err := a.GetOpenAPIJSON()
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.MarshalIndent(openAPIJSON, "", "  ")
	if err != nil {
		return "", err
	}

	context := fmt.Sprintf(`
=== API SPECIFICATION FOR YOUR REFERENCE ===

API Title: %s
Version: %s
Description: %s
Base URLs: %v

=== COMPLETE OPENAPI JSON SPECIFICATION ===
%s

=== STRICT INSTRUCTIONS ===
- ONLY answer programming or API-related questions about the OpenAPI JSON specification above.
- DO NOT answer questions outside the context of this API or its OpenAPI spec.
- DO NOT provide information unrelated to the API, its endpoints, or usage.
- ONLY use the provided OpenAPI JSON as your source of truth.
- Give code examples, endpoint usage, and parameter details strictly based on the OpenAPI spec.
- Be precise about required/optional parameters and show real request/response JSON from the spec.
- DO NOT speculate or invent endpoints, parameters, or behaviors not present in the OpenAPI JSON.
`,
		a.documentation.Info.Title,
		a.documentation.Info.Version,
		a.documentation.Info.Description,
		a.config.BaseURLs,
		string(jsonBytes))

	return context, nil
}

func (a *APIDocs) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, a.config.DocsPath)
	if strings.HasPrefix(path, "/openapi.json") || strings.HasPrefix(path, "/openapi.yaml") || strings.HasPrefix(path, "/openapi.yml") {
		a.serveDocs(w, r)
		return
	}

	if a.config.AuthConfig != nil && a.config.AuthConfig.Enabled {
		authMiddleware := AuthMiddleware(a.config.AuthConfig)

		docsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a.serveDocs(w, r)
		})

		authMiddleware(docsHandler).ServeHTTP(w, r)
		return
	}

	a.serveDocs(w, r)
}

func (a *APIDocs) serveDocs(w http.ResponseWriter, r *http.Request) {

	if len(a.documentation.Endpoints) == 0 {
		a.Generate()
	}

	path := strings.TrimPrefix(r.URL.Path, a.config.DocsPath)
	if path == "" {
		path = "/"
	} else if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	switch {
	case path == "" || path == "/":
		a.serveReactApp(w, r)
	case path == "/api-data.json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(a.documentation)
	case path == "/chat":
		a.serveChat(w, r)
	case path == "/openapi.json":
		a.serveOpenAPI(w, r)
	case path == "/openapi.yaml" || path == "/openapi.yml":
		a.serveOpenAPIYAML(w, r)
	case strings.HasPrefix(path, "/assets/"):
		a.serveAsset(w, r, path)
	default:
		a.serveReactApp(w, r)
	}
}

func (a *APIDocs) serveReactApp(w http.ResponseWriter, r *http.Request) {
	docsJSON, _ := json.Marshal(a.documentation)
	configJSON, _ := json.Marshal(a.config)

	templatePath := filepath.Join("..", "..", "pkg", "ui", "template.html")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		a.serveBasicTemplate(w, r)
		return
	}

	tmpl, err := template.New("docs").Parse(string(templateContent))
	if err != nil {
		http.Error(w, "Template parsing error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Title      string
		DocsPath   string
		DocsJSON   string
		ConfigJSON string
		Config     *Config
	}{
		Title:      a.config.Title,
		DocsPath:   a.config.DocsPath,
		DocsJSON:   string(docsJSON),
		ConfigJSON: string(configJSON),
		Config:     a.config,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *APIDocs) serveAsset(w http.ResponseWriter, r *http.Request, path string) {
	http.NotFound(w, r)
}

func (a *APIDocs) serveBasicTemplate(w http.ResponseWriter, r *http.Request) {
	docsJSON, _ := json.Marshal(a.documentation)
	configJSON, _ := json.Marshal(a.config)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: Inter, system-ui, sans-serif; background: #fff; color: #333; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        .header { border-bottom: 1px solid #e5e7eb; padding-bottom: 20px; margin-bottom: 30px; }
        .endpoint { background: #f9fafb; padding: 15px; margin: 10px 0; border-radius: 8px; border: 1px solid #e5e7eb; cursor: pointer; }
        .method { display: inline-block; padding: 4px 12px; border-radius: 4px; font-weight: 600; margin-right: 12px; font-size: 12px; }
        .get { background: #dcfce7; color: #166534; }
        .post { background: #dbeafe; color: #1e40af; }
        .put { background: #fef3c7; color: #92400e; }
        .delete { background: #fecaca; color: #991b1b; }
        .sidebar { width: 300px; background: #f9fafb; padding: 20px; border-right: 1px solid #e5e7eb; }
        .main { flex: 1; padding: 20px; }
        .layout { display: flex; min-height: 100vh; }
        .section { margin-bottom: 20px; }
        .section h3 { margin-bottom: 10px; font-size: 18px; }
        .no-endpoints { text-align: center; color: #6b7280; padding: 40px; }
    </style>
</head>
<body>
    <div class="layout">
        <div class="sidebar">
            <h2>%s</h2>
            <p style="color: #6b7280; margin-bottom: 20px;">%s</p>
            <div style="background: #e5e7eb; padding: 10px; border-radius: 6px; margin-bottom: 20px;">
                <div style="font-size: 12px; color: #6b7280;">Base URL:</div>
                <code style="font-size: 12px;">%s</code>
            </div>
            <div id="sidebar-content"></div>
        </div>
        <div class="main">
            <div id="main-content">
                <div class="no-endpoints">
                    <h3>Template Loading</h3>
                    <p>Using basic fallback template - full template not found</p>
                </div>
            </div>
        </div>
    </div>
    
    <script>
        window.__API_DOCS_DATA__ = %s;
        window.__API_DOCS_CONFIG__ = %s;
        console.log("Basic template loaded, data:", window.__API_DOCS_DATA__);
    </script>
</body>
</html>`,
		a.config.Title, a.config.Title, a.config.Description, a.config.BaseURL,
		string(docsJSON), string(configJSON))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func extractPathParams(path string) []string {
	params := make([]string, 0)
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			params = append(params, strings.TrimPrefix(part, ":"))
		}
		
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			param := strings.Trim(part, "{}")
			if strings.Contains(param, ":") {
				param = strings.Split(param, ":")[0]
			}
			params = append(params, param)
		}
	}

	return params
}

func (a *APIDocs) serveChat(w http.ResponseWriter, r *http.Request) {
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

	if a.llmClient == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChatResponse{
			Error:    "AI chat is not enabled or configured",
			Provider: "none",
		})
		return
	}

	var chatRequest ChatRequest
	err := json.NewDecoder(r.Body).Decode(&chatRequest)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := ChatResponse{
			Error:    fmt.Sprintf("Invalid request: %v", err),
			Provider: a.llmClient.GetProvider(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if chatRequest.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		response := ChatResponse{
			Error:    "Message is required",
			Provider: a.llmClient.GetProvider(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if chatRequest.Context == "" {
		apiContext, err := a.GetAPIContext()
		if err == nil {
			chatRequest.Context = apiContext
		}
	}

	chatResponse, err := a.llmClient.Chat(r.Context(), chatRequest)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse)
}

func (a *APIDocs) serveOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	openAPIJSON, err := a.GetOpenAPIJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate OpenAPI JSON: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(openAPIJSON)
}

func (a *APIDocs) serveOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/yaml")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	openAPIYAML, err := a.GetOpenAPIYAML()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate OpenAPI YAML: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(openAPIYAML); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write OpenAPI YAML: %v", err), http.StatusInternalServerError)
	}
}
