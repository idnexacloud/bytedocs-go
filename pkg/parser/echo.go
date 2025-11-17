package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/idnexacloud/bytedocs-go/pkg/core"
)

// Global registry for Echo route tracking
var (
	globalEchoDocs   *core.APIDocs
	echoDocsConfig   *core.Config
	echoDocsMutex    sync.RWMutex
	echoFuncComments map[string][]string
)

func init() {
	echoFuncComments = make(map[string][]string)
}

// EchoHandlerInfo holds parsed comment information for Echo handlers
type EchoHandlerInfo struct {
	Summary     string
	Description string
	Parameters  []core.Parameter
}

// parseEchoHandlerComments parses Go source files to extract Echo handler comments
func parseEchoHandlerComments(filePaths ...string) map[string]EchoHandlerInfo {
	handlerInfos := make(map[string]EchoHandlerInfo)

	// If no file paths provided, try to find main.go
	if len(filePaths) == 0 {
		filePaths = []string{"main.go", "examples/echo/main.go"}
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
				handlerInfos[funcName] = parseEchoHandlerInfo(comments)
			}
		}
	}

	return handlerInfos
}

// parseEchoHandlerInfo parses handler comments to extract structured information
func parseEchoHandlerInfo(comments []string) EchoHandlerInfo {
	info := EchoHandlerInfo{
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

// extractEchoHandlerName extracts function name from Echo handler function
func extractEchoHandlerName(handler echo.HandlerFunc) string {
	if handler == nil {
		return ""
	}

	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		return ""
	}

	funcName := runtime.FuncForPC(handlerValue.Pointer()).Name()

	parts := strings.Split(funcName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

// EchoRoute represents an Echo route for documentation
type EchoRoute struct {
	Method string
	Path   string
	Name   string
}

// getEchoRoutes extracts routes from Echo instance using reflection
func getEchoRoutes(e *echo.Echo) []EchoRoute {
	var routes []EchoRoute

	// Use Echo's Routes method to get all registered routes
	echoRoutes := e.Routes()

	for _, route := range echoRoutes {
		echoRoute := EchoRoute{
			Method: route.Method,
			Path:   route.Path,
			Name:   route.Name,
		}
		routes = append(routes, echoRoute)
	}

	return routes
}


// SetupEchoDocs sets up documentation for an Echo instance with auto-detection
func SetupEchoDocs(e *echo.Echo, config *core.Config) {
	if config == nil {
		config = &core.Config{
			Title:      "API Documentation",
			Version:    "1.0.0",
			DocsPath:   "/docs",
			AutoDetect: true,
		}
	}

	echoDocsMutex.Lock()
	echoDocsConfig = config
	globalEchoDocs = core.New(config)
	echoDocsMutex.Unlock()

	// Set up the docs route that does auto-detection
	docsHandler := func(c echo.Context) error {
		echoDocsMutex.Lock()
		defer echoDocsMutex.Unlock()

		endpointsCount := len(globalEchoDocs.GetDocumentation().Endpoints)

		if endpointsCount == 0 && config.AutoDetect {
			routes := getEchoRoutes(e)

			for _, route := range routes {
				if strings.HasPrefix(route.Path, config.DocsPath) ||
					strings.Contains(route.Path, "/static") ||
					strings.Contains(route.Path, "/assets") {
					continue
				}

				var metadata EchoHandlerMetadata
				var funcName string

				if strings.Contains(route.Name, ".") {
					parts := strings.Split(route.Name, ".")
					funcName = parts[len(parts)-1]
				} else {
					funcName = route.Name
				}

				if funcName != "" {
					metadata = getEchoHandlerMetadataByName(funcName, ".")
				}

				if metadata.Info.Summary == "" && metadata.Info.Description == "" {
					handlerInfos := parseEchoHandlerComments("main.go", "examples/echo/main.go")
					if handlerInfo, exists := handlerInfos[funcName]; exists {
						metadata.Info = handlerInfo
					}
				}

				routeInfo := core.RouteInfo{
					Method:      route.Method,
					Path:        route.Path,
					Handler:     nil,
					Summary:     metadata.Info.Summary,
					Description: metadata.Info.Description,
					Parameters:  metadata.Info.Parameters,
					RequestBody: metadata.RequestBody,
					Responses:   metadata.Responses,
				}

				globalEchoDocs.AddRouteInfo(routeInfo)
			}

			globalEchoDocs.Generate()
		}

		globalEchoDocs.ServeHTTP(c.Response().Writer, c.Request())
		return nil
	}

	// Register the docs routes
	e.Any(config.DocsPath, docsHandler)
	e.Any(config.DocsPath+"/*path", docsHandler)
}

// EchoMiddleware creates Echo middleware for automatic route documentation
func EchoMiddleware(config *core.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// This middleware can be used to automatically capture route information
			// For now, it just passes through to the next handler
			return next(c)
		}
	}
}