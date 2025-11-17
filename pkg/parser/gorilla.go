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
	"github.com/gorilla/mux"
)

// Global registry for Gorilla Mux route tracking
var (
	globalGorillaDocs   *core.APIDocs
	gorillaDocsConfig   *core.Config
	gorillaDocsMutex    sync.RWMutex
	gorillaFuncComments map[string][]string
)

func init() {
	gorillaFuncComments = make(map[string][]string)
}

// GorillaHandlerInfo holds parsed comment information for Gorilla Mux handlers
type GorillaHandlerInfo struct {
	Summary     string
	Description string
	Parameters  []core.Parameter
}

// parseGorillaHandlerComments parses Go source files to extract Gorilla Mux handler comments
func parseGorillaHandlerComments(filePaths ...string) map[string]GorillaHandlerInfo {
	handlerInfos := make(map[string]GorillaHandlerInfo)

	// If no file paths provided, try to find main.go
	if len(filePaths) == 0 {
		filePaths = []string{"main.go", "examples/gorilla-mux/main.go"}
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
				handlerInfos[funcName] = parseGorillaHandlerInfo(comments)
			}
		}
	}

	return handlerInfos
}

// parseGorillaHandlerInfo parses handler comments to extract structured information
func parseGorillaHandlerInfo(comments []string) GorillaHandlerInfo {
	info := GorillaHandlerInfo{
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

// extractGorillaHandlerName extracts function name from Gorilla Mux handler function
func extractGorillaHandlerName(handler http.Handler) string {
	if handler == nil {
		return ""
	}

	handlerValue := reflect.ValueOf(handler)

	switch h := handler.(type) {
	case http.HandlerFunc:
		handlerValue = reflect.ValueOf(h)
	default:
		if handlerValue.Kind() != reflect.Func {
			// Try to extract from method
			if handlerValue.Kind() == reflect.Ptr && handlerValue.Elem().Kind() == reflect.Struct {
				return ""
			}
			return ""
		}
	}

	if !handlerValue.IsValid() || handlerValue.Kind() != reflect.Func {
		return ""
	}

	funcPtr := handlerValue.Pointer()
	if funcPtr == 0 {
		return ""
	}

	fn := runtime.FuncForPC(funcPtr)
	if fn == nil {
		return ""
	}

	funcName := fn.Name()
	if funcName == "" {
		return ""
	}

	parts := strings.Split(funcName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

// inferHandlerNameFromRoute tries to infer handler name from HTTP method and path
func inferHandlerNameFromRoute(method, path string) string {
	// Extract the resource name from the path
	// Examples:
	// GET /api/v1/users -> GetUsers
	// GET /api/v1/products -> GetProducts
	// GET /api/v1/users/{id} -> GetUser
	// POST /api/v1/users -> CreateUser

	// Remove path parameters and clean the path
	cleanPath := strings.Split(path, "{")[0]
	cleanPath = strings.TrimSuffix(cleanPath, "/")

	// Extract the last segment as the resource name
	segments := strings.Split(cleanPath, "/")
	if len(segments) == 0 {
		return ""
	}

	resource := segments[len(segments)-1]
	if resource == "" {
		return ""
	}

	// Convert to title case
	resource = strings.Title(resource)

	// Generate handler name based on method and whether it's a collection or item
	hasIDParam := strings.Contains(path, "{")

	switch method {
	case "GET":
		if hasIDParam {
			// GET /users/{id} -> GetUser (singular)
			if strings.HasSuffix(resource, "s") {
				resource = resource[:len(resource)-1] // Remove 's'
			}
			return "Get" + resource
		} else {
			// GET /users -> GetUsers (plural)
			return "Get" + resource
		}
	case "POST":
		// POST /users -> CreateUser (singular)
		if strings.HasSuffix(resource, "s") {
			resource = resource[:len(resource)-1]
		}
		return "Create" + resource
	case "PUT":
		// PUT /users/{id} -> UpdateUser (singular)
		if strings.HasSuffix(resource, "s") {
			resource = resource[:len(resource)-1]
		}
		return "Update" + resource
	case "PATCH":
		// PATCH /users/{id} -> PatchUser (singular)
		if strings.HasSuffix(resource, "s") {
			resource = resource[:len(resource)-1]
		}
		return "Patch" + resource
	case "DELETE":
		// DELETE /users/{id} -> DeleteUser (singular)
		if strings.HasSuffix(resource, "s") {
			resource = resource[:len(resource)-1]
		}
		return "Delete" + resource
	}

	return ""
}

// GorillaRoute represents a Gorilla Mux route for documentation
type GorillaRoute struct {
	Method  string
	Path    string
	Handler http.Handler
}

// GorillaMuxWrapper wraps mux.Router to track registered routes
type GorillaMuxWrapper struct {
	*mux.Router
	routes []GorillaRoute
	mutex  sync.RWMutex
}

// RouteBuilder helps with method chaining for route registration
type RouteBuilder struct {
	wrapper *GorillaMuxWrapper
	route   *mux.Route
}

// Methods sets the HTTP methods for the route and updates the wrapper's tracking
func (rb *RouteBuilder) Methods(methods ...string) *mux.Route {
	// Update the wrapper's tracking with the correct method
	if rb.wrapper != nil && len(methods) > 0 {
		rb.wrapper.mutex.Lock()
		if len(rb.wrapper.routes) > 0 {
			rb.wrapper.routes[len(rb.wrapper.routes)-1].Method = methods[0]
		}
		rb.wrapper.mutex.Unlock()
	}

	// Call the original Methods on the mux.Route
	return rb.route.Methods(methods...)
}

// NewGorillaMuxWrapper creates a new wrapper for mux.Router
func NewGorillaMuxWrapper() *GorillaMuxWrapper {
	return &GorillaMuxWrapper{
		Router: mux.NewRouter(),
		routes: make([]GorillaRoute, 0),
	}
}

func (m *GorillaMuxWrapper) Handle(path string, handler http.Handler) *mux.Route {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	route := GorillaRoute{
		Method:  "GET", // Default method, will be overridden by Methods()
		Path:    path,
		Handler: handler,
	}
	m.routes = append(m.routes, route)

	// Call original Handle method
	return m.Router.Handle(path, handler)
}

func (m *GorillaMuxWrapper) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) *RouteBuilder {
	route := m.Handle(path, http.HandlerFunc(handler))
	return &RouteBuilder{
		wrapper: m,
		route:   route,
	}
}

// Methods wraps the route with specific HTTP methods
func (m *GorillaMuxWrapper) Methods(methods ...string) *GorillaMuxWrapper {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Update the last added route with the specified methods
	if len(m.routes) > 0 && len(methods) > 0 {
		m.routes[len(m.routes)-1].Method = methods[0] // Use first method for simplicity
	}

	return m
}

// GetRoutes returns all registered routes
func (m *GorillaMuxWrapper) GetRoutes() []GorillaRoute {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Also try to extract routes using mux.Router's Walk method
	var allRoutes []GorillaRoute

	// Add manually tracked routes
	allRoutes = append(allRoutes, m.routes...)

	// Try to extract additional routes from mux router
	m.Router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		methods, _ := route.GetMethods()
		pathTemplate, _ := route.GetPathTemplate()

		if pathTemplate != "" {
			method := "GET" // Default
			if len(methods) > 0 {
				method = methods[0]
			}

			// Check if this route is already tracked to avoid duplicates
			exists := false
			for _, existing := range allRoutes {
				if existing.Method == method && existing.Path == pathTemplate {
					exists = true
					break
				}
			}

			if !exists {
				gorillaRoute := GorillaRoute{
					Method:  method,
					Path:    pathTemplate,
					Handler: route.GetHandler(),
				}
				allRoutes = append(allRoutes, gorillaRoute)
			}
		}
		return nil
	})

	return allRoutes
}

// SetupGorillaMuxDocs sets up documentation for a Gorilla Mux router with auto-detection
func SetupGorillaMuxDocs(router *GorillaMuxWrapper, config *core.Config) {
	if config == nil {
		config = &core.Config{
			Title:      "API Documentation",
			Version:    "1.0.0",
			DocsPath:   "/docs",
			AutoDetect: true,
		}
	}

	gorillaDocsMutex.Lock()
	gorillaDocsConfig = config
	globalGorillaDocs = core.New(config)
	gorillaDocsMutex.Unlock()

	// Set up the docs route that does auto-detection
	router.HandleFunc(config.DocsPath+"/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("🚀 Gorilla Mux docs handler called for path: %s\n", r.URL.Path)
		gorillaDocsMutex.Lock()
		defer gorillaDocsMutex.Unlock()

		// Check if we need to detect routes
		endpointsCount := len(globalGorillaDocs.GetDocumentation().Endpoints)
		fmt.Printf("🔍 Current endpoints count: %d, AutoDetect: %t\n", endpointsCount, config.AutoDetect)

		if endpointsCount == 0 && config.AutoDetect {
			// Parse handler metadata first
			fmt.Printf("📝 Parsing Gorilla Mux handler metadata...\n")

			// Auto-detect all routes
			routes := router.GetRoutes()
			fmt.Printf("🔍 Detecting Gorilla Mux routes, found: %d\n", len(routes))

			for _, route := range routes {
				fmt.Printf("📍 Route: %s %s\n", route.Method, route.Path)

				// Skip docs routes and static files
				if strings.HasPrefix(route.Path, config.DocsPath) ||
					strings.Contains(route.Path, "/static") ||
					strings.Contains(route.Path, "/assets") {
					fmt.Printf("⏭️  Skipping route: %s\n", route.Path)
					continue
				}

				// Parse handler metadata using AST analysis
				metadata := getGorillaMuxHandlerMetadata(route.Handler)
				handlerName := extractGorillaHandlerName(route.Handler)

				// Fallback: if handler name is empty, try to infer from path and method
				if handlerName == "" {
					handlerName = inferHandlerNameFromRoute(route.Method, route.Path)
					// Try to get metadata using the inferred name
					if handlerName != "" {
						metadata = getGorillaMuxHandlerMetadataByName(handlerName, ".")
					}
				}

				fmt.Printf("   🔍 Analyzing handler: %s\n", handlerName)

				if metadata.Info.Summary != "" || metadata.RequestBody != nil || len(metadata.Responses) > 0 {
					fmt.Printf("   ✅ AST analysis successful for %s\n", handlerName)
				}

				// Fallback to comment parsing if AST analysis didn't work
				if metadata.Info.Summary == "" && metadata.Info.Description == "" {
					handlerInfos := parseGorillaHandlerComments("main.go", "examples/gorilla-mux/main.go")
					if handlerInfo, exists := handlerInfos[handlerName]; exists {
						metadata.Info = GorillaMuxHandlerInfo{
							Summary:     handlerInfo.Summary,
							Description: handlerInfo.Description,
							Parameters:  handlerInfo.Parameters,
						}
						fmt.Printf("   ✅ Comment parsing successful for %s\n", handlerName)
					}
				}

				routeInfo := core.RouteInfo{
					Method:      route.Method,
					Path:        route.Path,
					Handler:     route.Handler,
					Summary:     metadata.Info.Summary,
					Description: metadata.Info.Description,
					Parameters:  metadata.Info.Parameters,
					RequestBody: metadata.RequestBody,
					Responses:   metadata.Responses,
				}

				fmt.Printf("✅ Adding Gorilla Mux route: %s %s (handler: %s)\n", route.Method, route.Path, handlerName)
				if metadata.Info.Summary != "" {
					fmt.Printf("   📄 Summary: %s\n", metadata.Info.Summary)
				}
				if len(metadata.Info.Parameters) > 0 {
					fmt.Printf("   🔧 Parameters: %d\n", len(metadata.Info.Parameters))
				}
				if metadata.RequestBody != nil {
					fmt.Printf("   📦 Request body detected (content-type: %s)\n", metadata.RequestBody.ContentType)
					if metadata.RequestBody.Example != nil {
						fmt.Printf("   📦 Request body example: %+v\n", metadata.RequestBody.Example)
					}
				}
				if len(metadata.Responses) > 0 {
					fmt.Printf("   📤 Responses detected: %d\n", len(metadata.Responses))
					for code, resp := range metadata.Responses {
						fmt.Printf("   📤   %s: %s (%s)\n", code, resp.Description, resp.ContentType)
					}
				}

				// Add to documentation
				globalGorillaDocs.AddRouteInfo(routeInfo)
			}

			fmt.Printf("📚 Generating Gorilla Mux documentation...\n")
			// Generate documentation
			globalGorillaDocs.Generate()

			fmt.Printf("📊 Final endpoints count: %d\n", len(globalGorillaDocs.GetDocumentation().Endpoints))
		}

		// Serve documentation
		globalGorillaDocs.ServeHTTP(w, r)
	})

	router.PathPrefix(config.DocsPath + "/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalGorillaDocs.ServeHTTP(w, r)
	})
}
