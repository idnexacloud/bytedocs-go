package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/idnexacloud/bytedocs-go/pkg/core"
)

// GorillaMuxHandlerInfo holds parsed comment information for Gorilla-Mux handlers
type GorillaMuxHandlerInfo struct {
	Summary     string
	Description string
	Parameters  []core.Parameter
}

// parseGorillaMuxHandlerInfo parses handler comments to extract structured information
func parseGorillaMuxHandlerInfo(comments []string) GorillaMuxHandlerInfo {
	info := GorillaMuxHandlerInfo{
		Parameters: make([]core.Parameter, 0),
	}

	for _, line := range comments {
		if info.Summary == "" && !strings.HasPrefix(line, "@") {
			// First non-annotation line becomes summary
			info.Summary = line
		} else if !strings.HasPrefix(line, "@") && info.Description == "" {
			// Additional non-annotation lines become description
			info.Description = line
		}
	}

	return info
}

// GorillaMuxHandlerMetadata stores extracted documentation data for a Gorilla-Mux handler function.
type GorillaMuxHandlerMetadata struct {
	Info        GorillaMuxHandlerInfo
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// gorillaMuxAnalyzedHandler keeps track of metadata for an individual Gorilla-Mux handler within a package.
type gorillaMuxAnalyzedHandler struct {
	filePath     string
	funcName     string
	receiverName string
	startLine    int
	metadata     GorillaMuxHandlerMetadata
}

// gorillaMuxPackageAnalysis caches struct and handler information for a directory.
type gorillaMuxPackageAnalysis struct {
	handlers  map[string][]gorillaMuxAnalyzedHandler
	functions map[string][]functionSignature
}

var (
	gorillaMuxAnalysisCache = make(map[string]*gorillaMuxPackageAnalysis)
	gorillaMuxAnalysisMutex sync.RWMutex
)

// getGorillaMuxHandlerMetadataByName gets handler metadata by analyzing the function name from parsed files
func getGorillaMuxHandlerMetadataByName(funcName string, dir string) GorillaMuxHandlerMetadata {
	packageMeta := loadGorillaMuxPackageAnalysis(dir)
	if packageMeta == nil {
		return GorillaMuxHandlerMetadata{}
	}

	key := strings.ToLower(funcName)
	candidates := packageMeta.handlers[key]
	if len(candidates) == 0 {
		return GorillaMuxHandlerMetadata{}
	}

	if len(candidates) > 0 {
		return candidates[0].metadata
	}

	return GorillaMuxHandlerMetadata{}
}

func getGorillaMuxHandlerMetadata(handler http.Handler) GorillaMuxHandlerMetadata {
	if handler == nil {
		return GorillaMuxHandlerMetadata{}
	}

	var fn *runtime.Func
	var runtimeName string

	switch h := handler.(type) {
	case http.HandlerFunc:
		fn = runtime.FuncForPC(reflect.ValueOf(h).Pointer())
	case http.Handler:
		value := reflect.ValueOf(h)
		if value.Kind() == reflect.Func {
			fn = runtime.FuncForPC(value.Pointer())
		} else {
			method := value.MethodByName("ServeHTTP")
			if method.IsValid() {
				fn = runtime.FuncForPC(method.Pointer())
			}
		}
	default:
		value := reflect.ValueOf(handler)
		if value.Kind() == reflect.Func {
			fn = runtime.FuncForPC(value.Pointer())
		}
	}

	if fn == nil {
		return GorillaMuxHandlerMetadata{}
	}

	runtimeName = fn.Name()
	funcName := runtimeName
	if idx := strings.LastIndex(funcName, "."); idx != -1 {
		funcName = funcName[idx+1:]
	}

	entry := fn.Entry()
	file, _ := fn.FileLine(entry)
	if file == "" {
		return GorillaMuxHandlerMetadata{}
	}
	dir := filepath.Clean(filepath.Dir(file))

	return getGorillaMuxHandlerMetadataByName(funcName, dir)
}

// loadGorillaMuxPackageAnalysis parses and caches metadata for all Gorilla-Mux handlers within a directory.
func loadGorillaMuxPackageAnalysis(dir string) *gorillaMuxPackageAnalysis {
	gorillaMuxAnalysisMutex.RLock()
	if cached, ok := gorillaMuxAnalysisCache[dir]; ok {
		gorillaMuxAnalysisMutex.RUnlock()
		return cached
	}
	gorillaMuxAnalysisMutex.RUnlock()

	gorillaMuxAnalysisMutex.Lock()
	defer gorillaMuxAnalysisMutex.Unlock()

	if cached, ok := gorillaMuxAnalysisCache[dir]; ok {
		return cached
	}

	pkgAnalysis, err := analyzeGorillaMuxDirectory(dir)
	if err != nil {
		// Silently ignore analysis errors to avoid breaking docs generation.
		gorillaMuxAnalysisCache[dir] = nil
		return nil
	}

	gorillaMuxAnalysisCache[dir] = pkgAnalysis
	return pkgAnalysis
}

// analyzeGorillaMuxDirectory walks all Go files in a directory to extract Gorilla-Mux handler metadata.
func analyzeGorillaMuxDirectory(dir string) (*gorillaMuxPackageAnalysis, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info fs.FileInfo) bool {
		if info.IsDir() {
			return false
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".go") {
			return false
		}
		return !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	structs := collectStructDefinitions(pkgs)
	functions := collectFunctionSignatures(pkgs)
	handlers := collectGorillaMuxHandlerMetadata(fset, pkgs, structs, functions)

	return &gorillaMuxPackageAnalysis{
		handlers:  handlers,
		functions: functions,
	}, nil
}

// collectGorillaMuxHandlerMetadata extracts documentation metadata for Gorilla-Mux function declarations.
func collectGorillaMuxHandlerMetadata(fset *token.FileSet, pkgs map[string]*ast.Package, structs map[string]*ast.StructType, functions map[string][]functionSignature) map[string][]gorillaMuxAnalyzedHandler {
	handlers := make(map[string][]gorillaMuxAnalyzedHandler)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				// Check if this is likely a Gorilla-Mux handler (has http.ResponseWriter and *http.Request parameters)
				if !isGorillaMuxHandler(fn) {
					continue
				}

				var comments []string
				if fn.Doc != nil {
					comments = extractCommentsText(fn.Doc.List)
				}
				info := parseGorillaMuxHandlerInfo(comments)
				analysis := analyzeGorillaMuxHandlerDetails(fn, structs, functions)

				pos := fset.Position(fn.Pos())
				receiverName := receiverTypeName(fn.Recv)
				funcName := fn.Name.Name

				key := strings.ToLower(funcName)
				handlerEntry := gorillaMuxAnalyzedHandler{
					filePath:     pos.Filename,
					funcName:     funcName,
					receiverName: receiverName,
					startLine:    pos.Line,
					metadata: GorillaMuxHandlerMetadata{
						Info:        info,
						RequestBody: analysis.RequestBody,
						Responses:   analysis.Responses,
					},
				}

				handlers[key] = append(handlers[key], handlerEntry)
			}
		}
	}

	return handlers
}

// isGorillaMuxHandler checks if a function is likely a Gorilla-Mux handler
func isGorillaMuxHandler(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}

	hasResponseWriter := false
	hasRequest := false

	for _, param := range fn.Type.Params.List {
		switch t := param.Type.(type) {
		case *ast.SelectorExpr:
			if t.Sel.Name == "ResponseWriter" {
				if ident, ok := t.X.(*ast.Ident); ok && ident.Name == "http" {
					hasResponseWriter = true
				}
			}
		case *ast.StarExpr:
			if sel, ok := t.X.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Request" {
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "http" {
						hasRequest = true
					}
				}
			}
		case *ast.Ident:
			if t.Name == "ResponseWriter" || t.Name == "Request" {
				if t.Name == "ResponseWriter" {
					hasResponseWriter = true
				}
				if t.Name == "Request" {
					hasRequest = true
				}
			}
		}
	}

	return hasResponseWriter && hasRequest
}

type gorillaMuxHandlerAnalysis struct {
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// analyzeGorillaMuxHandlerDetails inspects a Gorilla-Mux handler function to infer request bodies and responses.
func analyzeGorillaMuxHandlerDetails(fn *ast.FuncDecl, structs map[string]*ast.StructType, functions map[string][]functionSignature) gorillaMuxHandlerAnalysis {
	analysis := gorillaMuxHandlerAnalysis{
		Responses: make(map[string]core.Response),
	}

	if fn.Body == nil {
		return analysis
	}

	ctx := &analysisContext{
		structs:   structs,
		functions: functions,
		variables: make(map[string]ast.Expr),
		values:    make(map[string]ast.Expr),
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.DeclStmt:
			registerDeclarationTypes(node, ctx)
		case *ast.AssignStmt:
			registerAssignmentTypes(node, ctx)
		case *ast.RangeStmt:
			registerRangeTypes(node, ctx)
		case *ast.CallExpr:
			// Detect request body binding for Gorilla-Mux (json.Decoder)
			if analysis.RequestBody == nil && isGorillaMuxBindingCall(node) {
				if len(node.Args) > 0 {
					if resolved := resolveGorillaMuxRequestBody(node, node.Args[0], ctx); resolved != nil {
						analysis.RequestBody = resolved
					}
				}
			}

			// Detect response generation calls for Gorilla-Mux
			if contentType, statusExpr, dataExpr, ok := gorillaMuxResponseCallInfo(node, ctx); ok {
				statusCode := extractStatusCode(statusExpr, ctx)
				if statusCode == "" {
					statusCode = "200"
				}
				payloadExpr := resolveResponsePayloadExpr(dataExpr, ctx)
				schema, example := buildSchemaFromExpr(payloadExpr, ctx, make(map[string]bool))
				example = normalizeExampleWithSchema(schema, example)
				if example == nil {
					example = defaultExampleFromSchema(schema)
				}
				if contentType == "" {
					contentType = "application/json"
				}
				response := core.Response{
					Description: statusTextFromCode(statusCode),
					Example:     example,
					Schema:      schema,
					ContentType: contentType,
				}
				if response.Description == "" {
					response.Description = "Response"
				}
				analysis.Responses[statusCode] = response
			}
		}
		return true
	})

	return analysis
}

var gorillaMuxBindingMethods = map[string]string{
	"Decode": "application/json",
}

func isGorillaMuxBindingCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	_, ok = gorillaMuxBindingMethods[sel.Sel.Name]
	return ok
}

func resolveGorillaMuxRequestBody(call *ast.CallExpr, arg ast.Expr, ctx *analysisContext) *core.RequestBody {
	typeExpr := resolveTypeFromArg(arg, ctx)
	if typeExpr == nil {
		return nil
	}

	body := buildRequestBodyFromExpr(typeExpr, ctx)
	if body == nil {
		return nil
	}

	body.Required = true

	if body.ContentType == "" {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if mime, found := gorillaMuxBindingMethods[sel.Sel.Name]; found && mime != "auto" {
				body.ContentType = mime
			}
		}
	}

	if body.ContentType == "" {
		body.ContentType = "application/json"
	}

	return body
}

func gorillaMuxResponseCallInfo(call *ast.CallExpr, ctx *analysisContext) (contentType string, statusExpr ast.Expr, dataExpr ast.Expr, ok bool) {
	// Check for writeJSON helper function first (plain ident call)
	if ident, ok := call.Fun.(*ast.Ident); ok {
		if ident.Name == "writeJSON" && len(call.Args) >= 3 {
			return "application/json", call.Args[1], call.Args[2], true
		}
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", nil, nil, false
	}

	// Check for json.NewEncoder().Encode() pattern
	if sel.Sel.Name == "Encode" {
		if callExpr, ok := sel.X.(*ast.CallExpr); ok {
			if newEncSel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if newEncSel.Sel.Name == "NewEncoder" {
					if len(call.Args) >= 1 {
						return "application/json", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
					}
				}
			}
		}
	}

	// Check for w.WriteHeader() and w.Write() patterns
	method := sel.Sel.Name
	switch method {
	case "WriteHeader":
		if len(call.Args) >= 1 {
			return "", call.Args[0], &ast.BasicLit{Kind: 10, Value: `""`}, true
		}
	case "Write":
		if len(call.Args) >= 1 {
			return "text/plain", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
		}
	}

	return "", nil, nil, false
}
