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

	"github.com/idnexacloud/bytedocs-go/pkg/core"
)

// Global registry for net/http route tracking
var (
	globalNetHTTPDocs   *core.APIDocs
	netHTTPDocsConfig   *core.Config
	netHTTPDocsMutex    sync.RWMutex
	netHTTPFuncComments map[string][]string
)

func init() {
	netHTTPFuncComments = make(map[string][]string)
}

// NetHTTPHandlerInfo holds parsed comment information for net/http handlers
type NetHTTPHandlerInfo struct {
	Summary     string
	Description string
	Parameters  []core.Parameter
}

// NetHTTPHandlerMetadata stores extracted documentation data for a net/http handler function.
type NetHTTPHandlerMetadata struct {
	Info        NetHTTPHandlerInfo
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// getNetHTTPHandlerMetadataByName gets handler metadata by analyzing the function name from parsed files
func getNetHTTPHandlerMetadataByName(funcName string, dir string) NetHTTPHandlerMetadata {
	// For net/http, we'll reuse the analysis logic from gorilla_mux_analyzer.go
	// since both use the same writeJSON pattern
	gorillaMeta := getGorillaMuxHandlerMetadataByName(funcName, dir)

	return NetHTTPHandlerMetadata{
		Info: NetHTTPHandlerInfo{
			Summary:     gorillaMeta.Info.Summary,
			Description: gorillaMeta.Info.Description,
			Parameters:  gorillaMeta.Info.Parameters,
		},
		RequestBody: gorillaMeta.RequestBody,
		Responses:   gorillaMeta.Responses,
	}
}

// parseNetHTTPHandlerComments parses Go source files to extract net/http handler comments
func parseNetHTTPHandlerComments(filePaths ...string) map[string]NetHTTPHandlerInfo {
	handlerInfos := make(map[string]NetHTTPHandlerInfo)

	// If no file paths provided, try to find main.go
	if len(filePaths) == 0 {
		filePaths = []string{"main.go", "examples/net-http/main.go"}
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
				handlerInfos[funcName] = parseNetHTTPHandlerInfo(comments)
			}
		}
	}

	return handlerInfos
}

// parseNetHTTPHandlerInfo parses handler comments to extract structured information
func parseNetHTTPHandlerInfo(comments []string) NetHTTPHandlerInfo {
	info := NetHTTPHandlerInfo{
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

// extractNetHTTPHandlerName extracts function name from net/http handler function
func extractNetHTTPHandlerName(handler http.Handler) string {
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

// NetHTTPRoute represents a net/http route for documentation
type NetHTTPRoute struct {
	Method  string
	Path    string
	Handler http.Handler
}

// NetHTTPMuxWrapper wraps http.ServeMux to track registered routes for net/http
type NetHTTPMuxWrapper struct {
	*http.ServeMux
	routes []NetHTTPRoute
	mutex  sync.RWMutex
}

// NewNetHTTPMuxWrapper creates a new wrapper for http.ServeMux specifically for net/http examples
func NewNetHTTPMuxWrapper() *NetHTTPMuxWrapper {
	return &NetHTTPMuxWrapper{
		ServeMux: http.NewServeMux(),
		routes:   make([]NetHTTPRoute, 0),
	}
}

func (m *NetHTTPMuxWrapper) Handle(pattern string, handler http.Handler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Parse method and path from pattern
	method := "GET" // Default method
	path := pattern

	if parts := strings.SplitN(pattern, " ", 2); len(parts) == 2 {
		method = parts[0]
		path = parts[1]
	}

	route := NetHTTPRoute{
		Method:  method,
		Path:    path,
		Handler: handler,
	}
	m.routes = append(m.routes, route)

	// Call original Handle method
	m.ServeMux.Handle(pattern, handler)
}

func (m *NetHTTPMuxWrapper) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.Handle(pattern, http.HandlerFunc(handler))
}

// GetRoutes returns all registered routes
func (m *NetHTTPMuxWrapper) GetRoutes() []NetHTTPRoute {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Make a copy to avoid race conditions
	routes := make([]NetHTTPRoute, len(m.routes))
	copy(routes, m.routes)
	return routes
}

// SetupNetHTTPDocs sets up documentation for a net/http ServeMux with auto-detection
func SetupNetHTTPDocs(mux *NetHTTPMuxWrapper, config *core.Config) {
	if config == nil {
		config = &core.Config{
			Title:      "API Documentation",
			Version:    "1.0.0",
			DocsPath:   "/docs",
			AutoDetect: true,
		}
	}

	netHTTPDocsMutex.Lock()
	netHTTPDocsConfig = config
	globalNetHTTPDocs = core.New(config)
	netHTTPDocsMutex.Unlock()

	// Set up the docs route that does auto-detection
	mux.HandleFunc(config.DocsPath+"/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("🚀 Net/HTTP docs handler called for path: %s\n", r.URL.Path)
		netHTTPDocsMutex.Lock()
		defer netHTTPDocsMutex.Unlock()

		// Check if we need to detect routes
		endpointsCount := len(globalNetHTTPDocs.GetDocumentation().Endpoints)
		fmt.Printf("🔍 Current endpoints count: %d, AutoDetect: %t\n", endpointsCount, config.AutoDetect)

		if endpointsCount == 0 && config.AutoDetect {
			// Parse handler comments first
			fmt.Printf("📝 Parsing net/http handler comments...\n")
			handlerInfos := parseNetHTTPHandlerComments("main.go", "examples/net-http/main.go")
			fmt.Printf("📝 Found %d handlers with comments\n", len(handlerInfos))

			// Auto-detect all routes
			routes := mux.GetRoutes()
			fmt.Printf("🔍 Detecting net/http routes, found: %d\n", len(routes))

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
				handlerName := extractNetHTTPHandlerName(route.Handler)
				handlerInfo := handlerInfos[handlerName]

				// Perform AST analysis to get metadata (request/response structures)
				metadata := getNetHTTPHandlerMetadataByName(handlerName, ".")

				// Create route info from net/http route with AST-analyzed data
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

				fmt.Printf("✅ Adding net/http route: %s %s (handler: %s)\n", route.Method, route.Path, handlerName)
				if handlerInfo.Summary != "" {
					fmt.Printf("   📄 Summary: %s\n", handlerInfo.Summary)
				}
				if len(handlerInfo.Parameters) > 0 {
					fmt.Printf("   🔧 Parameters: %d\n", len(handlerInfo.Parameters))
				}

				// Show AST analysis results
				if metadata.RequestBody != nil || len(metadata.Responses) > 0 {
					fmt.Printf("   ✅ AST analysis successful for %s\n", handlerName)
				}
				if metadata.RequestBody != nil {
					fmt.Printf("   📦 Request body detected (content-type: %s)\n", metadata.RequestBody.ContentType)
				}
				if len(metadata.Responses) > 0 {
					fmt.Printf("   📤 Responses detected: %d\n", len(metadata.Responses))
					for statusCode, response := range metadata.Responses {
						fmt.Printf("   📤   %s: %s (%s)\n", statusCode, response.Description, response.ContentType)
					}
				}

				// Add to documentation
				globalNetHTTPDocs.AddRouteInfo(routeInfo)
			}

			fmt.Printf("📚 Generating net/http documentation...\n")
			// Generate documentation
			globalNetHTTPDocs.Generate()

			fmt.Printf("📊 Final endpoints count: %d\n", len(globalNetHTTPDocs.GetDocumentation().Endpoints))
		}

		// Serve documentation
		globalNetHTTPDocs.ServeHTTP(w, r)
	})

}

// NetHTTPMiddleware creates net/http middleware for automatic route documentation
func NetHTTPMiddleware(config *core.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This middleware can be used to automatically capture route information
			// For now, it just passes through to the next handler
			next.ServeHTTP(w, r)
		})
	}
}