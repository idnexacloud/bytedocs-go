package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aibnuhibban/bytedocs/pkg/ai"
	"github.com/aibnuhibban/bytedocs/pkg/core"
)

//go:embed template.html
var staticFiles embed.FS

type Handler struct {
	docs      *core.APIDocs
	config    *core.Config
	template  *template.Template
	llmClient ai.Client
}

// NewHandler creates a new UI handler
func NewHandler(docs *core.APIDocs, config *core.Config) *Handler {
	// Read the template from embedded file
	templateContent, err := staticFiles.ReadFile("template.html")
	if err != nil {
		panic(fmt.Sprintf("Failed to read template.html: %v", err))
	}

	// Parse the HTML template
	tmpl := template.Must(template.New("index").Parse(string(templateContent)))

	// Initialize LLM client if AI is enabled
	var llmClient ai.Client
	if config.AIConfig != nil && config.AIConfig.Enabled {
		client, err := ai.NewClient(config.AIConfig)
		if err == nil {
			llmClient = client
		}
	}

	return &Handler{
		docs:      docs,
		config:    config,
		template:  tmpl,
		llmClient: llmClient,
	}
}

// ServeHTTP serves the documentation UI
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Remove docs path prefix
	path := strings.TrimPrefix(r.URL.Path, h.config.DocsPath)
	if path == "" {
		path = "/"
	}

	switch {
	case path == "/" || path == "/index.html":
		h.serveIndex(w, r)
	case path == "/api-data.json":
		h.serveAPIData(w, r)
	case path == "/chat":
		h.serveChat(w, r)
	case path == "/openapi.json":
		h.serveOpenAPI(w, r)
	case strings.HasPrefix(path, "/scenarios") && strings.HasSuffix(path, "/execute"):
		h.serveScenarioExecution(w, r)
	case strings.HasPrefix(path, "/scenarios"):
		h.serveScenarios(w, r)
	case path == "/test":
		h.serveTestEndpoint(w, r)
	case strings.HasPrefix(path, "/static/"):
		h.serveStatic(w, r, path)
	default:
		h.serveIndex(w, r)
	}
}

// serveIndex serves the main HTML page with embedded React app
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	// Generate documentation data
	if err := h.docs.Generate(); err != nil {
		http.Error(w, "Failed to generate documentation", http.StatusInternalServerError)
		return
	}

	// Read the built index.html file from embedded FS
	indexFile, err := staticFiles.Open("../../web/dist/index.html")
	if err != nil {
		// Fallback to embedded template
		h.serveEmbeddedTemplate(w, r)
		return
	}
	defer indexFile.Close()

	// Read content
	content, err := io.ReadAll(indexFile)
	if err != nil {
		h.serveEmbeddedTemplate(w, r)
		return
	}

	// Inject API data into the HTML
	docs := h.docs.GetDocumentation()
	docsJSON, _ := json.Marshal(docs)

	htmlContent := string(content)

	// Inject the API data script before closing </body>
	injection := fmt.Sprintf(`<script>window.__API_DOCS_DATA__ = %s;</script>
    <script>window.__API_DOCS_CONFIG__ = %s;</script>
</body>`, string(docsJSON), mustMarshalJSON(h.config))

	htmlContent = strings.Replace(htmlContent, "</body>", injection, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlContent))
}

// serveEmbeddedTemplate serves the fallback template
func (h *Handler) serveEmbeddedTemplate(w http.ResponseWriter, r *http.Request) {
	docs := h.docs.GetDocumentation()
	docsJSON, _ := json.Marshal(docs)
	configJSON, _ := json.Marshal(h.config)

	data := struct {
		Title        string
		DocsDataJSON string
		ConfigJSON   string
		Config       *core.Config
	}{
		Title:        h.config.Title,
		DocsDataJSON: string(docsJSON),
		ConfigJSON:   string(configJSON),
		Config:       h.config,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.template.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func mustMarshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// serveAPIData serves the API documentation data as JSON
func (h *Handler) serveAPIData(w http.ResponseWriter, r *http.Request) {
	if err := h.docs.Generate(); err != nil {
		http.Error(w, "Failed to generate documentation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // For development

	if err := json.NewEncoder(w).Encode(h.docs.GetDocumentation()); err != nil {
		http.Error(w, "Failed to encode documentation", http.StatusInternalServerError)
		return
	}
}

// serveStatic serves static files from embedded filesystem
func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request, path string) {
	var filePath string
	if strings.HasPrefix(path, "/assets/") {
		filePath = "../../web/dist" + path
	} else {
		// Remove /static/ prefix for other static files
		filePath = strings.TrimPrefix(path, "/static/")
		filePath = "../../web/dist/" + filePath
	}

	// Try to serve from embedded files
	file, err := staticFiles.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "File stat error", http.StatusInternalServerError)
		return
	}

	// Set content type based on file extension
	ext := filepath.Ext(filePath)
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	// Serve the file
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file.(io.ReadSeeker))
}

// serveChat handles chat requests to the AI assistant
func (h *Handler) serveChat(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for development
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
	fmt.Println(h.llmClient)
	// Check if AI is enabled and client is available
	if h.llmClient == nil {
		w.Header().Set("Content-Type", "application/json")
		response := ai.ChatResponse{
			Error:    "AI chat is not enabled or configured",
			Provider: "none",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse the request
	var chatRequest ai.ChatRequest
	err := json.NewDecoder(r.Body).Decode(&chatRequest)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := ai.ChatResponse{
			Error:    fmt.Sprintf("Invalid request: %v", err),
			Provider: h.llmClient.GetProvider(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate request
	if chatRequest.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		response := ai.ChatResponse{
			Error:    "Message is required",
			Provider: h.llmClient.GetProvider(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Automatically include API context if not already provided
	if chatRequest.Context == "" {
		apiContext, err := h.docs.GetAPIContext()
		if err == nil {
			chatRequest.Context = apiContext
		}
	}

	// Call the LLM
	chatResponse, err := h.llmClient.Chat(r.Context(), chatRequest)
	if err != nil {
		// Error response is already included in chatResponse
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse)
}

// serveOpenAPI serves the OpenAPI specification JSON
func (h *Handler) serveOpenAPI(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
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

	// Generate OpenAPI JSON
	openAPIJSON, err := h.docs.GetOpenAPIJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate OpenAPI JSON: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(openAPIJSON)
}
