package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"sync"

	"github.com/idnexacloud/bytedocs-go/pkg/core"
)

// EchoHandlerMetadata stores extracted documentation data for an Echo handler function.
type EchoHandlerMetadata struct {
	Info        EchoHandlerInfo
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// echoAnalyzedHandler keeps track of metadata for an individual Echo handler within a package.
type echoAnalyzedHandler struct {
	filePath     string
	funcName     string
	receiverName string
	startLine    int
	metadata     EchoHandlerMetadata
}

// echoPackageAnalysis caches struct and handler information for a directory.
type echoPackageAnalysis struct {
	handlers  map[string][]echoAnalyzedHandler
	functions map[string][]functionSignature
}

var (
	echoAnalysisCache = make(map[string]*echoPackageAnalysis)
	echoAnalysisMutex sync.RWMutex
)

// getEchoHandlerMetadataByName gets handler metadata by analyzing the function name from parsed files
func getEchoHandlerMetadataByName(funcName string, dir string) EchoHandlerMetadata {
	packageMeta := loadEchoPackageAnalysis(dir)
	if packageMeta == nil {
		return EchoHandlerMetadata{}
	}

	key := strings.ToLower(funcName)
	candidates := packageMeta.handlers[key]
	if len(candidates) == 0 {
		return EchoHandlerMetadata{}
	}

	if len(candidates) > 0 {
		return candidates[0].metadata
	}

	return EchoHandlerMetadata{}
}

// loadEchoPackageAnalysis parses and caches metadata for all Echo handlers within a directory.
func loadEchoPackageAnalysis(dir string) *echoPackageAnalysis {
	echoAnalysisMutex.RLock()
	if cached, ok := echoAnalysisCache[dir]; ok {
		echoAnalysisMutex.RUnlock()
		return cached
	}
	echoAnalysisMutex.RUnlock()

	echoAnalysisMutex.Lock()
	defer echoAnalysisMutex.Unlock()

	if cached, ok := echoAnalysisCache[dir]; ok {
		return cached
	}

	pkgAnalysis, err := analyzeEchoDirectory(dir)
	if err != nil {
		// Silently ignore analysis errors to avoid breaking docs generation.
		echoAnalysisCache[dir] = nil
		return nil
	}

	echoAnalysisCache[dir] = pkgAnalysis
	return pkgAnalysis
}

// analyzeEchoDirectory walks all Go files in a directory to extract Echo handler metadata.
func analyzeEchoDirectory(dir string) (*echoPackageAnalysis, error) {
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
	handlers := collectEchoHandlerMetadata(fset, pkgs, structs, functions)

	return &echoPackageAnalysis{
		handlers:  handlers,
		functions: functions,
	}, nil
}

// collectEchoHandlerMetadata extracts documentation metadata for Echo function declarations.
func collectEchoHandlerMetadata(fset *token.FileSet, pkgs map[string]*ast.Package, structs map[string]*ast.StructType, functions map[string][]functionSignature) map[string][]echoAnalyzedHandler {
	handlers := make(map[string][]echoAnalyzedHandler)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				// Check if this is likely an Echo handler (has echo.Context parameter)
				if !isEchoHandler(fn) {
					continue
				}

				var comments []string
				if fn.Doc != nil {
					comments = extractCommentsText(fn.Doc.List)
				}
				info := parseEchoHandlerInfo(comments)
				analysis := analyzeEchoHandlerDetails(fn, structs, functions)

				pos := fset.Position(fn.Pos())
				receiverName := receiverTypeName(fn.Recv)
				funcName := fn.Name.Name

				key := strings.ToLower(funcName)
				handlerEntry := echoAnalyzedHandler{
					filePath:     pos.Filename,
					funcName:     funcName,
					receiverName: receiverName,
					startLine:    pos.Line,
					metadata: EchoHandlerMetadata{
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

// isEchoHandler checks if a function is likely an Echo handler by looking for echo.Context parameter
func isEchoHandler(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}

	for _, param := range fn.Type.Params.List {
		switch t := param.Type.(type) {
		case *ast.SelectorExpr:
			if t.Sel.Name == "Context" {
				if ident, ok := t.X.(*ast.Ident); ok && ident.Name == "echo" {
					return true
				}
			}
		case *ast.Ident:
			if t.Name == "Context" {
				return true
			}
		}
	}
	return false
}

type echoHandlerAnalysis struct {
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// analyzeEchoHandlerDetails inspects an Echo handler function to infer request bodies and responses.
func analyzeEchoHandlerDetails(fn *ast.FuncDecl, structs map[string]*ast.StructType, functions map[string][]functionSignature) echoHandlerAnalysis {
	analysis := echoHandlerAnalysis{
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
			// Detect request body binding for Echo
			if analysis.RequestBody == nil && isEchoBindingCall(node) {
				if len(node.Args) > 0 {
					if resolved := resolveEchoRequestBody(node, node.Args[0], ctx); resolved != nil {
						analysis.RequestBody = resolved
					}
				}
			}

			// Detect response generation calls for Echo
			if contentType, statusExpr, dataExpr, ok := echoResponseCallInfo(node, ctx); ok {
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

var echoBindingMethods = map[string]string{
	"Bind": "auto",
}

func isEchoBindingCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	_, ok = echoBindingMethods[sel.Sel.Name]
	return ok
}

func resolveEchoRequestBody(call *ast.CallExpr, arg ast.Expr, ctx *analysisContext) *core.RequestBody {
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
			if mime, found := echoBindingMethods[sel.Sel.Name]; found && mime != "auto" {
				body.ContentType = mime
			}
		}
	}

	if body.ContentType == "" {
		body.ContentType = "application/json"
	}

	return body
}

func echoResponseCallInfo(call *ast.CallExpr, ctx *analysisContext) (contentType string, statusExpr ast.Expr, dataExpr ast.Expr, ok bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", nil, nil, false
	}

	method := sel.Sel.Name
	switch method {
	case "JSON", "JSONPretty":
		if len(call.Args) >= 2 {
			return "application/json", call.Args[0], call.Args[1], true
		}
	case "String":
		if len(call.Args) >= 2 {
			return "text/plain", call.Args[0], call.Args[1], true
		}
	case "XML", "XMLPretty":
		if len(call.Args) >= 2 {
			return "application/xml", call.Args[0], call.Args[1], true
		}
	case "HTML":
		if len(call.Args) >= 3 {
			return "text/html", call.Args[0], call.Args[2], true
		}
	case "Blob":
		if len(call.Args) >= 3 {
			ct := resolveContentType(call.Args[1], ctx)
			if ct == "" {
				ct = "application/octet-stream"
			}
			return ct, call.Args[0], call.Args[2], true
		}
	case "Stream":
		if len(call.Args) >= 3 {
			ct := resolveContentType(call.Args[1], ctx)
			if ct == "" {
				ct = "application/octet-stream"
			}
			return ct, call.Args[0], call.Args[2], true
		}
	case "File":
		if len(call.Args) >= 1 {
			return "application/octet-stream", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
		}
	case "Attachment":
		if len(call.Args) >= 2 {
			return "application/octet-stream", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
		}
	case "Inline":
		if len(call.Args) >= 2 {
			return "application/octet-stream", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
		}
	case "NoContent":
		if len(call.Args) >= 1 {
			return "", call.Args[0], &ast.BasicLit{Kind: 10, Value: `""`}, true
		}
	case "Redirect":
		if len(call.Args) >= 2 {
			return "text/html", call.Args[0], call.Args[1], true
		}
	}

	return "", nil, nil, false
}
