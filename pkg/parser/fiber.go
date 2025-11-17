package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/aibnuhibban/bytedocs/pkg/core"
	"github.com/gofiber/fiber/v2"
)

// Global registry for Fiber route tracking
var (
	globalFiberDocs   *core.APIDocs
	fiberDocsConfig   *core.Config
	fiberDocsMutex    sync.RWMutex
	fiberFuncComments map[string][]string
)

func init() {
	fiberFuncComments = make(map[string][]string)
}

// FiberHandlerInfo holds parsed comment information for Fiber handlers
type FiberHandlerInfo struct {
	Summary     string
	Description string
	Parameters  []core.Parameter
}

// parseFiberHandlerComments parses Go source files to extract Fiber handler comments
func parseFiberHandlerComments(filePaths ...string) map[string]FiberHandlerInfo {
	handlerInfos := make(map[string]FiberHandlerInfo)

	// If no file paths provided, try to find main.go
	if len(filePaths) == 0 {
		filePaths = []string{"main.go", "examples/fiber/main.go"}
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
				handlerInfos[funcName] = parseFiberHandlerInfo(comments)
			}
		}
	}

	return handlerInfos
}

// parseFiberHandlerInfo parses handler comments to extract structured information
func parseFiberHandlerInfo(comments []string) FiberHandlerInfo {
	info := FiberHandlerInfo{
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

// extractFiberHandlerName extracts function name from Fiber handler function
func extractFiberHandlerName(handler interface{}) string {
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

// FiberRoute represents a Fiber route for documentation
type FiberRoute struct {
	Method  string
	Path    string
	Handler fiber.Handler
}

// getFiberRoutes extracts routes from Fiber app using reflection
func getFiberRoutes(app *fiber.App) []FiberRoute {
	var routes []FiberRoute
	seen := make(map[string]struct{})

	// Fiber automatically registers HEAD routes alongside GET. They pollute the docs sidebar
	// with duplicated endpoints, so walk the flattened route list and skip unwanted entries.
	for _, route := range app.GetRoutes(true) {
		method := strings.TrimSpace(strings.ToUpper(route.Method))
		path := strings.TrimSpace(route.Path)

		if method == "" || path == "" {
			continue
		}
		if method == fiber.MethodHead {
			continue
		}

		key := method + " " + path
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		if len(route.Handlers) == 0 {
			continue
		}

		fiberRoute := FiberRoute{
			Method:  method,
			Path:    path,
			Handler: route.Handlers[len(route.Handlers)-1],
		}

		routes = append(routes, fiberRoute)
	}

	return routes
}

// SetupFiberDocs sets up documentation for a Fiber app with auto-detection
func SetupFiberDocs(app *fiber.App, config *core.Config) {
	if config == nil {
		config = &core.Config{
			Title:      "API Documentation",
			Version:    "1.0.0",
			DocsPath:   "/docs",
			AutoDetect: true,
		}
	}

	fiberDocsMutex.Lock()
	fiberDocsConfig = config
	globalFiberDocs = core.New(config)
	fiberDocsMutex.Unlock()

	// Set up the docs route that does auto-detection
	docsHandler := func(c *fiber.Ctx) error {
		fiberDocsMutex.Lock()
		defer fiberDocsMutex.Unlock()

		endpointsCount := len(globalFiberDocs.GetDocumentation().Endpoints)

		if endpointsCount == 0 && config.AutoDetect {
			routes := getFiberRoutes(app)

			for _, route := range routes {
				if strings.HasPrefix(route.Path, config.DocsPath) ||
					strings.Contains(route.Path, "/static") ||
					strings.Contains(route.Path, "/assets") {
					continue
				}

				var metadata FiberHandlerMetadata
				handlerName := extractFiberHandlerName(route.Handler)

				if handlerName != "" {
					metadata = getFiberHandlerMetadataByName(handlerName, ".")
				}

				if metadata.Info.Summary == "" && metadata.Info.Description == "" {
					handlerInfos := parseFiberHandlerComments("main.go", "examples/fiber/main.go")
					if handlerInfo, exists := handlerInfos[handlerName]; exists {
						metadata.Info = handlerInfo
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

				globalFiberDocs.AddRouteInfo(routeInfo)
			}

			globalFiberDocs.Generate()
		}

		// Serve documentation directly using Fiber's response writer
		// Convert Fiber request to standard HTTP request
		uri := c.Request().URI()
		req := &http.Request{
			Method: c.Method(),
			URL: &url.URL{
				Scheme:   string(uri.Scheme()),
				Host:     string(uri.Host()),
				Path:     string(uri.Path()),
				RawQuery: string(uri.QueryString()),
			},
			Header: make(http.Header),
		}

		// Copy headers from Fiber to standard HTTP
		c.Request().Header.VisitAll(func(key, value []byte) {
			req.Header.Set(string(key), string(value))
		})

		if c.Method() == "POST" {
			body := c.Body()
			if len(body) > 0 {
				req.Body = &bodyReader{data: body}
				req.ContentLength = int64(len(body))
			}
		}

		// Create a simple response writer that wraps Fiber context
		w := &simpleFiberResponseWriter{ctx: c}

		// Serve documentation
		globalFiberDocs.ServeHTTP(w, req)
		return nil
	}

	// Register the docs routes
	app.All(config.DocsPath, docsHandler)
	app.All(config.DocsPath+"/*", docsHandler)
}

// bodyReader implements io.ReadCloser for request body
type bodyReader struct {
	data []byte
	pos  int
}

func (r *bodyReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *bodyReader) Close() error {
	return nil
}

// simpleFiberResponseWriter wraps Fiber context to implement http.ResponseWriter
type simpleFiberResponseWriter struct {
	ctx     *fiber.Ctx
	header  http.Header
	written bool
}

func (w *simpleFiberResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *simpleFiberResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.WriteHeader(200)
	}
	return w.ctx.Write(data)
}

func (w *simpleFiberResponseWriter) WriteHeader(statusCode int) {
	if w.written {
		return
	}
	w.written = true

	// Set status code
	w.ctx.Status(statusCode)

	// Copy headers from our buffer to Fiber context
	for key, values := range w.header {
		for _, value := range values {
			w.ctx.Set(key, value)
		}
	}
}
