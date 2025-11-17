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

	"github.com/idnexacloud/bytedocs-go/pkg/core"
)

// StdlibHandlerMetadata stores extracted documentation data for a stdlib handler function.
type StdlibHandlerMetadata struct {
	Info        StdlibHandlerInfo
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// getStdlibHandlerMetadata analyzes a stdlib handler function and returns its documentation metadata.
func getStdlibHandlerMetadata(handler interface{}) StdlibHandlerMetadata {
	if handler == nil {
		return StdlibHandlerMetadata{}
	}

	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		if handlerFunc, ok := handler.(http.HandlerFunc); ok {
			handlerValue = reflect.ValueOf(handlerFunc)
		} else {
			return StdlibHandlerMetadata{}
		}
	}

	fn := runtime.FuncForPC(handlerValue.Pointer())
	if fn == nil {
		return StdlibHandlerMetadata{}
	}

	entry := fn.Entry()
	file, line := fn.FileLine(entry)
	if file == "" {
		return StdlibHandlerMetadata{}
	}

	packageMeta := loadStdlibPackageAnalysis(filepath.Dir(file))
	if packageMeta == nil {
		return StdlibHandlerMetadata{}
	}

	runtimeName := fn.Name()
	funcName, receiverName := parseRuntimeFuncName(runtimeName)

	key := strings.ToLower(funcName)
	candidates := packageMeta.handlers[key]
	if len(candidates) == 0 {
		return StdlibHandlerMetadata{}
	}

	normalizedFile := filepath.Clean(file)
	for _, candidate := range candidates {
		if filepath.Clean(candidate.filePath) != normalizedFile {
			continue
		}
		// Receiver names must match; empty receiver matches standalone functions.
		if candidate.receiverName != receiverName {
			continue
		}
		if line >= candidate.startLine {
			return StdlibHandlerMetadata{
				Info: StdlibHandlerInfo{
					Summary:     candidate.metadata.Info.Summary,
					Description: candidate.metadata.Info.Description,
					Parameters:  candidate.metadata.Info.Parameters,
				},
				RequestBody: candidate.metadata.RequestBody,
				Responses:   candidate.metadata.Responses,
			}
		}
	}

	return StdlibHandlerMetadata{}
}

// loadStdlibPackageAnalysis parses and caches metadata for all handlers within a directory.
func loadStdlibPackageAnalysis(dir string) *packageAnalysis {
	analysisMutex.RLock()
	if cached, ok := analysisCache[dir]; ok {
		analysisMutex.RUnlock()
		return cached
	}
	analysisMutex.RUnlock()

	analysisMutex.Lock()
	defer analysisMutex.Unlock()

	if cached, ok := analysisCache[dir]; ok {
		return cached
	}

	pkgAnalysis, err := analyzeStdlibDirectory(dir)
	if err != nil {
		// Silently ignore analysis errors to avoid breaking docs generation.
		analysisCache[dir] = nil
		return nil
	}

	analysisCache[dir] = pkgAnalysis
	return pkgAnalysis
}

// analyzeStdlibDirectory walks all Go files in a directory to extract stdlib handler metadata.
func analyzeStdlibDirectory(dir string) (*packageAnalysis, error) {
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
	handlers := collectStdlibHandlerMetadata(fset, pkgs, structs, functions)

	return &packageAnalysis{
		handlers:  handlers,
		functions: functions,
	}, nil
}

// collectStdlibHandlerMetadata extracts documentation metadata for stdlib function declarations.
func collectStdlibHandlerMetadata(fset *token.FileSet, pkgs map[string]*ast.Package, structs map[string]*ast.StructType, functions map[string][]functionSignature) map[string][]analyzedHandler {
	handlers := make(map[string][]analyzedHandler)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				// Only analyze functions that look like HTTP handlers (have http.ResponseWriter and *http.Request params)
				if !isStdlibHTTPHandler(fn) {
					continue
				}

				var comments []string
				if fn.Doc != nil {
					comments = extractCommentsText(fn.Doc.List)
				}
				info := parseStdlibHandlerInfo(comments)
				analysis := analyzeStdlibHandlerDetails(fn, structs, functions)

				pos := fset.Position(fn.Pos())
				receiverName := receiverTypeName(fn.Recv)
				funcName := fn.Name.Name

				key := strings.ToLower(funcName)
				handlerEntry := analyzedHandler{
					filePath:     pos.Filename,
					funcName:     funcName,
					receiverName: receiverName,
					startLine:    pos.Line,
					metadata: HandlerMetadata{
						Info:        HandlerInfo{
							Summary:     info.Summary,
							Description: info.Description,
							Parameters:  info.Parameters,
						},
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

// isStdlibHTTPHandler checks if a function is an HTTP handler by looking at its parameters
func isStdlibHTTPHandler(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) < 2 {
		return false
	}

	params := fn.Type.Params.List
	hasResponseWriter := false
	hasRequest := false

	for _, param := range params {
		paramType := exprToString(param.Type)

		// Check for http.ResponseWriter
		if strings.Contains(paramType, "ResponseWriter") {
			hasResponseWriter = true
		}

		// Check for *http.Request
		if strings.Contains(paramType, "Request") && strings.Contains(paramType, "*") {
			hasRequest = true
		}
	}

	return hasResponseWriter && hasRequest
}

// analyzeStdlibHandlerDetails inspects a stdlib handler function to infer request bodies and responses.
func analyzeStdlibHandlerDetails(fn *ast.FuncDecl, structs map[string]*ast.StructType, functions map[string][]functionSignature) handlerAnalysis {
	analysis := handlerAnalysis{
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
			registerRegularAssignmentTypes(node, ctx)
		case *ast.RangeStmt:
			registerRangeTypes(node, ctx)
		case *ast.CallExpr:
			// Detect request body binding for stdlib (json.NewDecoder, etc.)
			if analysis.RequestBody == nil && isStdlibBindingCall(node) {
				if resolved := resolveStdlibRequestBody(node, ctx); resolved != nil {
					analysis.RequestBody = resolved
				}
			}

			// Detect response generation calls for stdlib
			if contentType, statusExpr, dataExpr, ok := stdlibResponseCallInfo(node, ctx); ok {
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

// isStdlibBindingCall detects stdlib JSON binding calls
func isStdlibBindingCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	method := sel.Sel.Name
	// Look for json.NewDecoder().Decode() pattern
	if method == "Decode" {
		if callExpr, ok := sel.X.(*ast.CallExpr); ok {
			if innerSel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				return innerSel.Sel.Name == "NewDecoder"
			}
		}
	}

	return false
}

// resolveStdlibRequestBody resolves request body for stdlib patterns
func resolveStdlibRequestBody(call *ast.CallExpr, ctx *analysisContext) *core.RequestBody {
	// Look for json.NewDecoder(r.Body).Decode(&struct{})
	if len(call.Args) > 0 {
		if unaryExpr, ok := call.Args[0].(*ast.UnaryExpr); ok && unaryExpr.Op == token.AND {
			typeExpr := resolveTypeFromArg(unaryExpr.X, ctx)
			if typeExpr != nil {
				body := buildRequestBodyFromExpr(typeExpr, ctx)
				if body != nil {
					body.Required = true
					body.ContentType = "application/json"
					return body
				}
			}
		}
	}
	return nil
}

// stdlibResponseCallInfo detects stdlib response calls like json.NewEncoder().Encode() or writeJSON()
func stdlibResponseCallInfo(call *ast.CallExpr, ctx *analysisContext) (contentType string, statusExpr ast.Expr, dataExpr ast.Expr, ok bool) {
	// First check for direct function calls like writeJSON(w, status, data)
	if ident, ok := call.Fun.(*ast.Ident); ok {
		switch ident.Name {
		case "writeJSON":
			// writeJSON(w, status, data) - our custom function
			if len(call.Args) >= 3 {
				return "application/json", call.Args[1], call.Args[2], true
			}
		case "writeError":
			// writeError(w, status, message, errorMsg) - our custom error function
			if len(call.Args) >= 4 {
				// Create ErrorResponse struct
				errorStruct := &ast.CompositeLit{
					Type: &ast.Ident{Name: "ErrorResponse"},
				}
				return "application/json", call.Args[1], errorStruct, true
			}
		}
	}

	// Then check for method calls
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		method := sel.Sel.Name
		switch method {
		case "Encode":
			// json.NewEncoder(w).Encode(data)
			if len(call.Args) >= 1 {
				if callExpr, ok := sel.X.(*ast.CallExpr); ok {
					if innerSel, ok := callExpr.Fun.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "NewEncoder" {
						// Default status is 200, data is the argument
						return "application/json", &ast.BasicLit{Kind: token.INT, Value: "200"}, call.Args[0], true
					}
				}
			}
		case "WriteHeader":
			// w.WriteHeader(status)
			if len(call.Args) >= 1 {
				return "application/json", call.Args[0], nil, true
			}
		case "JSON":
			// For compatibility with any JSON method calls
			if len(call.Args) >= 2 {
				return "application/json", call.Args[0], call.Args[1], true
			}
		}
	}

	return "", nil, nil, false
}

// registerRegularAssignmentTypes handles regular assignments (=) not just short declarations (:=)
func registerRegularAssignmentTypes(assign *ast.AssignStmt, ctx *analysisContext) {
	if ctx == nil || assign.Tok != token.ASSIGN {
		return
	}

	for idx, name := range assign.Lhs {
		ident, ok := name.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		if idx >= len(assign.Rhs) {
			continue
		}

		// Track this variable assignment
		inferred := inferTypeFromExpr(assign.Rhs[idx], ctx)
		if inferred != nil {
			ctx.variables[ident.Name] = inferred
			ctx.values[ident.Name] = assign.Rhs[idx]
		}
	}
}