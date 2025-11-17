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

// FiberHandlerMetadata stores extracted documentation data for a Fiber handler function.
type FiberHandlerMetadata struct {
	Info        FiberHandlerInfo
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// fiberAnalyzedHandler keeps track of metadata for an individual Fiber handler within a package.
type fiberAnalyzedHandler struct {
	filePath     string
	funcName     string
	receiverName string
	startLine    int
	metadata     FiberHandlerMetadata
}

// fiberPackageAnalysis caches struct and handler information for a directory.
type fiberPackageAnalysis struct {
	handlers  map[string][]fiberAnalyzedHandler
	functions map[string][]functionSignature
}

var (
	fiberAnalysisCache = make(map[string]*fiberPackageAnalysis)
	fiberAnalysisMutex sync.RWMutex
)

// getFiberHandlerMetadataByName gets handler metadata by analyzing the function name from parsed files
func getFiberHandlerMetadataByName(funcName string, dir string) FiberHandlerMetadata {
	packageMeta := loadFiberPackageAnalysis(dir)
	if packageMeta == nil {
		return FiberHandlerMetadata{}
	}

	key := strings.ToLower(funcName)
	candidates := packageMeta.handlers[key]
	if len(candidates) == 0 {
		return FiberHandlerMetadata{}
	}

	if len(candidates) > 0 {
		return candidates[0].metadata
	}

	return FiberHandlerMetadata{}
}

// loadFiberPackageAnalysis parses and caches metadata for all Fiber handlers within a directory.
func loadFiberPackageAnalysis(dir string) *fiberPackageAnalysis {
	fiberAnalysisMutex.RLock()
	if cached, ok := fiberAnalysisCache[dir]; ok {
		fiberAnalysisMutex.RUnlock()
		return cached
	}
	fiberAnalysisMutex.RUnlock()

	fiberAnalysisMutex.Lock()
	defer fiberAnalysisMutex.Unlock()

	if cached, ok := fiberAnalysisCache[dir]; ok {
		return cached
	}

	pkgAnalysis, err := analyzeFiberDirectory(dir)
	if err != nil {
		// Silently ignore analysis errors to avoid breaking docs generation.
		fiberAnalysisCache[dir] = nil
		return nil
	}

	fiberAnalysisCache[dir] = pkgAnalysis
	return pkgAnalysis
}

// analyzeFiberDirectory walks all Go files in a directory to extract Fiber handler metadata.
func analyzeFiberDirectory(dir string) (*fiberPackageAnalysis, error) {
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
	handlers := collectFiberHandlerMetadata(fset, pkgs, structs, functions)

	return &fiberPackageAnalysis{
		handlers:  handlers,
		functions: functions,
	}, nil
}

// collectFiberHandlerMetadata extracts documentation metadata for Fiber function declarations.
func collectFiberHandlerMetadata(fset *token.FileSet, pkgs map[string]*ast.Package, structs map[string]*ast.StructType, functions map[string][]functionSignature) map[string][]fiberAnalyzedHandler {
	handlers := make(map[string][]fiberAnalyzedHandler)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				// Check if this is likely a Fiber handler (has *fiber.Ctx parameter)
				if !isFiberHandler(fn) {
					continue
				}

				var comments []string
				if fn.Doc != nil {
					comments = extractCommentsText(fn.Doc.List)
				}
				info := parseFiberHandlerInfo(comments)
				analysis := analyzeFiberHandlerDetails(fn, structs, functions)

				pos := fset.Position(fn.Pos())
				receiverName := receiverTypeName(fn.Recv)
				funcName := fn.Name.Name

				key := strings.ToLower(funcName)
				handlerEntry := fiberAnalyzedHandler{
					filePath:     pos.Filename,
					funcName:     funcName,
					receiverName: receiverName,
					startLine:    pos.Line,
					metadata: FiberHandlerMetadata{
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

// isFiberHandler checks if a function is likely a Fiber handler by looking for *fiber.Ctx parameter
func isFiberHandler(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}

	for _, param := range fn.Type.Params.List {
		switch t := param.Type.(type) {
		case *ast.StarExpr:
			if sel, ok := t.X.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Ctx" {
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fiber" {
						return true
					}
				}
			}
		case *ast.SelectorExpr:
			if t.Sel.Name == "Ctx" {
				if ident, ok := t.X.(*ast.Ident); ok && ident.Name == "fiber" {
					return true
				}
			}
		case *ast.Ident:
			if t.Name == "Ctx" {
				return true
			}
		}
	}
	return false
}

type fiberHandlerAnalysis struct {
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// analyzeFiberHandlerDetails inspects a Fiber handler function to infer request bodies and responses.
func analyzeFiberHandlerDetails(fn *ast.FuncDecl, structs map[string]*ast.StructType, functions map[string][]functionSignature) fiberHandlerAnalysis {
	analysis := fiberHandlerAnalysis{
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
			// Detect request body binding for Fiber
			if analysis.RequestBody == nil && isFiberBindingCall(node) {
				if len(node.Args) > 0 {
					if resolved := resolveFiberRequestBody(node, node.Args[0], ctx); resolved != nil {
						analysis.RequestBody = resolved
					}
				}
			}

			// Detect response generation calls for Fiber
			if contentType, statusExpr, dataExpr, ok := fiberResponseCallInfo(node, ctx); ok {
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

var fiberBindingMethods = map[string]string{
	"BodyParser": "auto",
}

func isFiberBindingCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	_, ok = fiberBindingMethods[sel.Sel.Name]
	return ok
}

func resolveFiberRequestBody(call *ast.CallExpr, arg ast.Expr, ctx *analysisContext) *core.RequestBody {
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
			if mime, found := fiberBindingMethods[sel.Sel.Name]; found && mime != "auto" {
				body.ContentType = mime
			}
		}
	}

	if body.ContentType == "" {
		body.ContentType = "application/json"
	}

	return body
}

func fiberResponseCallInfo(call *ast.CallExpr, ctx *analysisContext) (contentType string, statusExpr ast.Expr, dataExpr ast.Expr, ok bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", nil, nil, false
	}

	method := sel.Sel.Name
	switch method {
	case "JSON":
		if len(call.Args) >= 1 {
			return "application/json", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
		}
	case "Status":
		// Check if this is chained with JSON like c.Status(201).JSON(user)
		return "", nil, nil, false
	case "String":
		if len(call.Args) >= 1 {
			return "text/plain", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
		}
	case "XML":
		if len(call.Args) >= 1 {
			return "application/xml", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
		}
	case "SendFile":
		if len(call.Args) >= 1 {
			return "application/octet-stream", &ast.BasicLit{Kind: 9, Value: "200"}, call.Args[0], true
		}
	case "SendStatus":
		if len(call.Args) >= 1 {
			return "", call.Args[0], &ast.BasicLit{Kind: 10, Value: `""`}, true
		}
	}

	return "", nil, nil, false
}
