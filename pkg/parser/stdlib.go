package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/aibnuhibban/bytedocs/pkg/core"
)

// Global registry for stdlib route tracking
var (
	globalStdlibDocs   *core.APIDocs
	stdlibDocsConfig   *core.Config
	stdlibDocsMutex    sync.RWMutex
	stdlibFuncComments map[string][]string
)

func init() {
	stdlibFuncComments = make(map[string][]string)
}

// StdlibHandlerInfo holds parsed comment information for stdlib handlers
type StdlibHandlerInfo struct {
	Summary     string
	Description string
	Parameters  []core.Parameter
}

// parseStdlibHandlerComments parses Go source files to extract stdlib handler comments
func parseStdlibHandlerComments(filePaths ...string) map[string]StdlibHandlerInfo {
	handlerInfos := make(map[string]StdlibHandlerInfo)

	// If no file paths provided, try to find main.go
	if len(filePaths) == 0 {
		filePaths = []string{"main.go", "examples/stdlib/main.go", "examples/net-http/main.go"}
	}

	for _, filePath := range filePaths {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		// Extract function comments
		for _, decl := range node.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Doc != nil {
				funcName := fn.Name.Name
				comments := extractCommentsText(fn.Doc.List)
				handlerInfos[funcName] = parseStdlibHandlerInfo(comments)
			}
		}
	}

	return handlerInfos
}

// parseStdlibHandlerInfo parses handler comments to extract structured information
func parseStdlibHandlerInfo(comments []string) StdlibHandlerInfo {
	info := StdlibHandlerInfo{
		Parameters: make([]core.Parameter, 0),
	}

	paramRegex := regexp.MustCompile(`@Param\s+(\w+)\s+(\w+)\s+(\w+)\s+(true|false)\s+"([^"]*)"`)

	for _, line := range comments {
		// Parse @Param annotations
		if matches := paramRegex.FindStringSubmatch(line); len(matches) == 6 {
			param := core.Parameter{
				Name:        matches[1],
				In:          matches[2], // path, query, header, etc.
				Type:        matches[3],
				Required:    matches[4] == "true",
				Description: matches[5],
			}
			info.Parameters = append(info.Parameters, param)
		} else if strings.HasPrefix(line, "@Param") {
			continue
		} else if info.Summary == "" && !strings.HasPrefix(line, "@") {
			// First non-annotation line becomes summary
			info.Summary = line
		} else if !strings.HasPrefix(line, "@") && info.Description == "" {
			// Additional non-annotation lines become description
			info.Description = line
		}
	}

	return info
}

// extractStdlibHandlerName extracts function name from stdlib handler function
func extractStdlibHandlerName(handler http.Handler) string {
	if handler == nil {
		return ""
	}

	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		if handlerFunc, ok := handler.(http.HandlerFunc); ok {
			handlerValue = reflect.ValueOf(handlerFunc)
		} else {
			return ""
		}
	}

	funcName := runtime.FuncForPC(handlerValue.Pointer()).Name()

	parts := strings.Split(funcName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

// StdlibRoute represents a stdlib route for documentation
type StdlibRoute struct {
	Method  string
	Path    string
	Handler http.Handler
}

// StdlibMuxWrapper wraps http.ServeMux to track registered routes
type StdlibMuxWrapper struct {
	*http.ServeMux
	routes []StdlibRoute
	mutex  sync.RWMutex
}

// NewStdlibMuxWrapper creates a new wrapper for http.ServeMux
func NewStdlibMuxWrapper() *StdlibMuxWrapper {
	return &StdlibMuxWrapper{
		ServeMux: http.NewServeMux(),
		routes:   make([]StdlibRoute, 0),
	}
}

func (m *StdlibMuxWrapper) Handle(pattern string, handler http.Handler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Parse method and path from pattern
	method := "GET" // Default method
	path := pattern

	if parts := strings.SplitN(pattern, " ", 2); len(parts) == 2 {
		method = parts[0]
		path = parts[1]
	}

	route := StdlibRoute{
		Method:  method,
		Path:    path,
		Handler: handler,
	}
	m.routes = append(m.routes, route)

	// Call original Handle method
	m.ServeMux.Handle(pattern, handler)
}

func (m *StdlibMuxWrapper) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.Handle(pattern, http.HandlerFunc(handler))
}

// GetRoutes returns all registered routes
func (m *StdlibMuxWrapper) GetRoutes() []StdlibRoute {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Make a copy to avoid race conditions
	routes := make([]StdlibRoute, len(m.routes))
	copy(routes, m.routes)
	return routes
}

// SetupStdlibDocs sets up documentation for a stdlib ServeMux with auto-detection
func SetupStdlibDocs(mux *StdlibMuxWrapper, config *core.Config) {
	if config == nil {
		config = &core.Config{
			Title:      "API Documentation",
			Version:    "1.0.0",
			DocsPath:   "/docs",
			AutoDetect: true,
		}
	}

	stdlibDocsMutex.Lock()
	stdlibDocsConfig = config
	globalStdlibDocs = core.New(config)
	stdlibDocsMutex.Unlock()

	// Set up the docs route that does auto-detection
	mux.HandleFunc(config.DocsPath+"/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("🚀 Stdlib docs handler called for path: %s\n", r.URL.Path)
		stdlibDocsMutex.Lock()
		defer stdlibDocsMutex.Unlock()

		// Check if we need to detect routes
		endpointsCount := len(globalStdlibDocs.GetDocumentation().Endpoints)
		fmt.Printf("🔍 Current endpoints count: %d, AutoDetect: %t\n", endpointsCount, config.AutoDetect)

		if endpointsCount == 0 && config.AutoDetect {
			// Parse handler comments first
			fmt.Printf("📝 Parsing stdlib handler comments...\n")
			handlerInfos := parseStdlibHandlerComments("main.go", "examples/stdlib/main.go", "examples/net-http/main.go")
			fmt.Printf("📝 Found %d handlers with comments\n", len(handlerInfos))

			// Auto-detect all routes
			routes := mux.GetRoutes()
			fmt.Printf("🔍 Detecting stdlib routes, found: %d\n", len(routes))

			for _, route := range routes {
				fmt.Printf("📍 Route: %s %s\n", route.Method, route.Path)

				// Skip docs routes and static files
				if strings.HasPrefix(route.Path, config.DocsPath) ||
					strings.Contains(route.Path, "/static") ||
					strings.Contains(route.Path, "/assets") {
					fmt.Printf("⏭️  Skipping route: %s\n", route.Path)
					continue
				}

				// Extract handler function name from the route
				handlerName := extractStdlibHandlerName(route.Handler)
				handlerInfo := handlerInfos[handlerName]

				// Get detailed metadata using analyzer
				metadata := getStdlibHandlerMetadata(route.Handler)

				routeInfo := core.RouteInfo{
					Method:      route.Method,
					Path:        route.Path,
					Handler:     route.Handler,
					Summary:     handlerInfo.Summary,
					Description: handlerInfo.Description,
					Parameters:  handlerInfo.Parameters,
					RequestBody: metadata.RequestBody,
					Responses:   metadata.Responses,
				}

				fmt.Printf("✅ Adding stdlib route: %s %s (handler: %s)\n", route.Method, route.Path, handlerName)
				if handlerInfo.Summary != "" {
					fmt.Printf("   📄 Summary: %s\n", handlerInfo.Summary)
				}
				if len(handlerInfo.Parameters) > 0 {
					fmt.Printf("   🔧 Parameters: %d\n", len(handlerInfo.Parameters))
				}

				// Add to documentation
				globalStdlibDocs.AddRouteInfo(routeInfo)
			}

			fmt.Printf("📚 Generating stdlib documentation...\n")
			// Generate documentation
			globalStdlibDocs.Generate()

			fmt.Printf("📊 Final endpoints count: %d\n", len(globalStdlibDocs.GetDocumentation().Endpoints))
		}

		// Serve documentation
		globalStdlibDocs.ServeHTTP(w, r)
	})
}

// SetupStdlibHTTPDocs is an alias for SetupStdlibDocs for net/http compatibility
func SetupStdlibHTTPDocs(mux *StdlibMuxWrapper, config *core.Config) {
	SetupStdlibDocs(mux, config)
}