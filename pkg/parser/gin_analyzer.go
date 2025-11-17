package parser

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/idnexacloud/bytedocs-go/pkg/core"
)

type HandlerMetadata struct {
	Info        HandlerInfo
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

// analyzedHandler keeps track of metadata for an individual handler within a package.
type analyzedHandler struct {
	filePath     string
	funcName     string
	receiverName string
	startLine    int
	metadata     HandlerMetadata
}

// packageAnalysis caches struct and handler information for a directory.
type packageAnalysis struct {
	handlers  map[string][]analyzedHandler
	functions map[string][]functionSignature
}

type functionSignature struct {
	receiver string
	results  []ast.Expr
}

var (
	analysisCache = make(map[string]*packageAnalysis)
	analysisMutex sync.RWMutex
)

// getHandlerMetadata analyzes a handler function and returns its documentation metadata.
func getHandlerMetadata(handler interface{}) HandlerMetadata {
	if handler == nil {
		return HandlerMetadata{}
	}

	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		return HandlerMetadata{}
	}

	fn := runtime.FuncForPC(handlerValue.Pointer())
	if fn == nil {
		return HandlerMetadata{}
	}

	entry := fn.Entry()
	file, line := fn.FileLine(entry)
	if file == "" {
		return HandlerMetadata{}
	}

	packageMeta := loadPackageAnalysis(filepath.Dir(file))
	if packageMeta == nil {
		return HandlerMetadata{}
	}

	runtimeName := fn.Name()
	funcName, receiverName := parseRuntimeFuncName(runtimeName)

	key := strings.ToLower(funcName)
	candidates := packageMeta.handlers[key]
	if len(candidates) == 0 {
		return HandlerMetadata{}
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
			return candidate.metadata
		}
	}

	return HandlerMetadata{}
}

// loadPackageAnalysis parses and caches metadata for all handlers within a directory.
func loadPackageAnalysis(dir string) *packageAnalysis {
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

	pkgAnalysis, err := analyzeDirectory(dir)
	if err != nil {
		// Silently ignore analysis errors to avoid breaking docs generation.
		analysisCache[dir] = nil
		return nil
	}

	analysisCache[dir] = pkgAnalysis
	return pkgAnalysis
}

// parseRuntimeFuncName extracts the function and receiver names from a runtime symbol.
func parseRuntimeFuncName(fullName string) (funcName string, receiverName string) {
	trimmed := fullName
	if idx := strings.LastIndex(trimmed, "/"); idx != -1 {
		trimmed = trimmed[idx+1:]
	}

	lastDot := strings.LastIndex(trimmed, ".")
	if lastDot == -1 {
		return trimmed, ""
	}

	funcName = trimmed[lastDot+1:]
	prefix := trimmed[:lastDot]

	receiverName = ""
	if prefix != "" {
		if idx := strings.LastIndex(prefix, "."); idx != -1 {
			candidate := prefix[idx+1:]
			if candidate != "" {
				if strings.HasPrefix(candidate, "(") {
					receiverName = normalizeReceiverName(candidate)
				} else if first := []rune(candidate)[0]; unicode.IsUpper(first) {
					receiverName = candidate
				}
			}
		} else {
			candidate := prefix
			if strings.HasPrefix(candidate, "(") {
				receiverName = normalizeReceiverName(candidate)
			} else if candidate != "" {
				if first := []rune(candidate)[0]; unicode.IsUpper(first) {
					receiverName = candidate
				}
			}
		}
	}

	return funcName, receiverName
}

func normalizeReceiverName(receiver string) string {
	receiver = strings.TrimSpace(receiver)
	if strings.HasPrefix(receiver, "(") {
		receiver = strings.TrimPrefix(receiver, "(")
	}
	if strings.HasSuffix(receiver, ")") {
		receiver = strings.TrimSuffix(receiver, ")")
	}
	return receiver
}

// analyzeDirectory walks all Go files in a directory to extract handler metadata.
func analyzeDirectory(dir string) (*packageAnalysis, error) {
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
	handlers := collectHandlerMetadata(fset, pkgs, structs, functions)

	return &packageAnalysis{
		handlers:  handlers,
		functions: functions,
	}, nil
}

func collectFunctionSignatures(pkgs map[string]*ast.Package) map[string][]functionSignature {
	functions := make(map[string][]functionSignature)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				receiver := receiverTypeName(fn.Recv)
				funcName := fn.Name.Name
				results := make([]ast.Expr, 0)
				if fn.Type.Results != nil {
					for _, result := range fn.Type.Results.List {
						if len(result.Names) == 0 {
							results = append(results, result.Type)
						} else {
							for range result.Names {
								results = append(results, result.Type)
							}
						}
					}
				}

				signature := functionSignature{
					receiver: receiver,
					results:  results,
				}

				key := funcName
				functions[key] = append(functions[key], signature)
				if receiver != "" {
					methodKey := receiver + "." + funcName
					functions[methodKey] = append(functions[methodKey], signature)
					trimmed := strings.TrimPrefix(receiver, "*")
					if trimmed != receiver {
						functions[trimmed+"."+funcName] = append(functions[trimmed+"."+funcName], signature)
					}
				}
			}
		}
	}

	return functions
}

// collectStructDefinitions builds a lookup map of struct declarations in the parsed packages.
func collectStructDefinitions(pkgs map[string]*ast.Package) map[string]*ast.StructType {
	structs := make(map[string]*ast.StructType)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					structType, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}
					structs[typeSpec.Name.Name] = structType
				}
			}
		}
	}

	return structs
}

// collectHandlerMetadata extracts documentation metadata for function declarations.
func collectHandlerMetadata(fset *token.FileSet, pkgs map[string]*ast.Package, structs map[string]*ast.StructType, functions map[string][]functionSignature) map[string][]analyzedHandler {
	handlers := make(map[string][]analyzedHandler)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				var comments []string
				if fn.Doc != nil {
					comments = extractCommentsText(fn.Doc.List)
				}
				info := parseHandlerInfo(comments)
				analysis := analyzeHandlerDetails(fn, structs, functions)

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

// receiverTypeName returns a normalized receiver type ("" for functions).
func receiverTypeName(fieldList *ast.FieldList) string {
	if fieldList == nil || len(fieldList.List) == 0 {
		return ""
	}
	field := fieldList.List[0]
	return exprToString(field.Type)
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	}
	return ""
}

var httpStatusNameMap = map[string]int{
	"StatusContinue":                      http.StatusContinue,
	"StatusSwitchingProtocols":            http.StatusSwitchingProtocols,
	"StatusProcessing":                    http.StatusProcessing,
	"StatusEarlyHints":                    http.StatusEarlyHints,
	"StatusOK":                            http.StatusOK,
	"StatusCreated":                       http.StatusCreated,
	"StatusAccepted":                      http.StatusAccepted,
	"StatusNonAuthoritativeInfo":          http.StatusNonAuthoritativeInfo,
	"StatusNoContent":                     http.StatusNoContent,
	"StatusResetContent":                  http.StatusResetContent,
	"StatusPartialContent":                http.StatusPartialContent,
	"StatusMultiStatus":                   http.StatusMultiStatus,
	"StatusAlreadyReported":               http.StatusAlreadyReported,
	"StatusIMUsed":                        http.StatusIMUsed,
	"StatusMultipleChoices":               http.StatusMultipleChoices,
	"StatusMovedPermanently":              http.StatusMovedPermanently,
	"StatusFound":                         http.StatusFound,
	"StatusSeeOther":                      http.StatusSeeOther,
	"StatusNotModified":                   http.StatusNotModified,
	"StatusUseProxy":                      http.StatusUseProxy,
	"StatusTemporaryRedirect":             http.StatusTemporaryRedirect,
	"StatusPermanentRedirect":             http.StatusPermanentRedirect,
	"StatusBadRequest":                    http.StatusBadRequest,
	"StatusUnauthorized":                  http.StatusUnauthorized,
	"StatusPaymentRequired":               http.StatusPaymentRequired,
	"StatusForbidden":                     http.StatusForbidden,
	"StatusNotFound":                      http.StatusNotFound,
	"StatusMethodNotAllowed":              http.StatusMethodNotAllowed,
	"StatusNotAcceptable":                 http.StatusNotAcceptable,
	"StatusProxyAuthRequired":             http.StatusProxyAuthRequired,
	"StatusRequestTimeout":                http.StatusRequestTimeout,
	"StatusConflict":                      http.StatusConflict,
	"StatusGone":                          http.StatusGone,
	"StatusLengthRequired":                http.StatusLengthRequired,
	"StatusPreconditionFailed":            http.StatusPreconditionFailed,
	"StatusRequestEntityTooLarge":         http.StatusRequestEntityTooLarge,
	"StatusRequestURITooLong":             http.StatusRequestURITooLong,
	"StatusUnsupportedMediaType":          http.StatusUnsupportedMediaType,
	"StatusRequestedRangeNotSatisfiable":  http.StatusRequestedRangeNotSatisfiable,
	"StatusExpectationFailed":             http.StatusExpectationFailed,
	"StatusTeapot":                        http.StatusTeapot,
	"StatusMisdirectedRequest":            http.StatusMisdirectedRequest,
	"StatusUnprocessableEntity":           http.StatusUnprocessableEntity,
	"StatusLocked":                        http.StatusLocked,
	"StatusFailedDependency":              http.StatusFailedDependency,
	"StatusTooEarly":                      http.StatusTooEarly,
	"StatusUpgradeRequired":               http.StatusUpgradeRequired,
	"StatusPreconditionRequired":          http.StatusPreconditionRequired,
	"StatusTooManyRequests":               http.StatusTooManyRequests,
	"StatusRequestHeaderFieldsTooLarge":   http.StatusRequestHeaderFieldsTooLarge,
	"StatusUnavailableForLegalReasons":    http.StatusUnavailableForLegalReasons,
	"StatusInternalServerError":           http.StatusInternalServerError,
	"StatusNotImplemented":                http.StatusNotImplemented,
	"StatusBadGateway":                    http.StatusBadGateway,
	"StatusServiceUnavailable":            http.StatusServiceUnavailable,
	"StatusGatewayTimeout":                http.StatusGatewayTimeout,
	"StatusHTTPVersionNotSupported":       http.StatusHTTPVersionNotSupported,
	"StatusVariantAlsoNegotiates":         http.StatusVariantAlsoNegotiates,
	"StatusInsufficientStorage":           http.StatusInsufficientStorage,
	"StatusLoopDetected":                  http.StatusLoopDetected,
	"StatusNotExtended":                   http.StatusNotExtended,
	"StatusNetworkAuthenticationRequired": http.StatusNetworkAuthenticationRequired,
}

func extractStatusCode(expr ast.Expr, ctx *analysisContext) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.INT {
			value := strings.ReplaceAll(e.Value, "_", "")
			if num, err := strconv.Atoi(value); err == nil {
				return strconv.Itoa(num)
			}
		}
	case *ast.SelectorExpr:
		if code, ok := httpStatusNameMap[e.Sel.Name]; ok {
			return strconv.Itoa(code)
		}
	case *ast.Ident:
		if ctx != nil {
			if typ, ok := ctx.variables[e.Name]; ok {
				return extractStatusCode(typ, ctx)
			}
		}
	}
	return ""
}

func statusTextFromCode(code string) string {
	if num, err := strconv.Atoi(code); err == nil {
		return http.StatusText(num)
	}
	return ""
}

func responseCallInfo(call *ast.CallExpr, ctx *analysisContext) (contentType string, statusExpr ast.Expr, dataExpr ast.Expr, ok bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", nil, nil, false
	}

	method := sel.Sel.Name
	switch method {
	case "JSON", "IndentedJSON", "PureJSON", "SecureJSON":
		if len(call.Args) >= 2 {
			return "application/json", call.Args[0], call.Args[1], true
		}
	case "AbortWithStatusJSON":
		if len(call.Args) >= 2 {
			return "application/json", call.Args[0], call.Args[1], true
		}
	case "Data":
		if len(call.Args) >= 3 {
			ct := resolveContentType(call.Args[1], ctx)
			if ct == "" {
				ct = "application/octet-stream"
			}
			return ct, call.Args[0], call.Args[2], true
		}
	case "String":
		if len(call.Args) >= 2 {
			return "text/plain", call.Args[0], call.Args[1], true
		}
	case "XML", "IndentedXML":
		if len(call.Args) >= 2 {
			return "application/xml", call.Args[0], call.Args[1], true
		}
	case "YAML":
		if len(call.Args) >= 2 {
			return "application/x-yaml", call.Args[0], call.Args[1], true
		}
	case "ProtoBuf":
		if len(call.Args) >= 2 {
			return "application/x-protobuf", call.Args[0], call.Args[1], true
		}
	case "JSONP":
		if len(call.Args) >= 2 {
			return "application/javascript", call.Args[0], call.Args[1], true
		}
	}

	return "", nil, nil, false
}

func resolveContentType(expr ast.Expr, ctx *analysisContext) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if value, err := strconv.Unquote(e.Value); err == nil {
				return value
			}
			return strings.Trim(e.Value, "\"")
		}
	case *ast.Ident:
		if ctx != nil {
			if valExpr, ok := ctx.values[e.Name]; ok && valExpr != nil {
				if resolved := resolveContentType(valExpr, ctx); resolved != "" {
					return resolved
				}
			}
			if typExpr, ok := ctx.variables[e.Name]; ok && typExpr != nil {
				if resolved := resolveContentType(typExpr, ctx); resolved != "" {
					return resolved
				}
			}
		}
	case *ast.CallExpr:
		if len(e.Args) > 0 {
			return resolveContentType(e.Args[0], ctx)
		}
	case *ast.StarExpr:
		return resolveContentType(e.X, ctx)
	case *ast.SelectorExpr:
		return exprToString(e)
	}
	return ""
}

func resolveResponsePayloadExpr(expr ast.Expr, ctx *analysisContext) ast.Expr {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return resolveResponsePayloadExpr(e.X, ctx)
		}
	case *ast.CompositeLit:
		return e
	case *ast.Ident:
		if ctx != nil {
			if origin, ok := ctx.values[e.Name]; ok && origin != nil {
				if call, ok := origin.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						full := exprToString(sel)
						if full == "json.Marshal" || full == "json.MarshalIndent" {
							if len(call.Args) > 0 {
								return resolveResponsePayloadExpr(call.Args[0], ctx)
							}
						}
					}
				}
				if lit, ok := origin.(*ast.CompositeLit); ok {
					return lit
				}
			}
			if typ, ok := ctx.variables[e.Name]; ok {
				return typ
			}
		}
		return e
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			full := exprToString(sel)
			if full == "json.Marshal" || full == "json.MarshalIndent" {
				if len(e.Args) > 0 {
					return resolveResponsePayloadExpr(e.Args[0], ctx)
				}
			}
			receiverType := resolveTypeFromArg(sel.X, ctx)
			receiverName := exprToString(receiverType)
			if results := lookupFunctionResult(ctx, receiverName, sel.Sel.Name); len(results) > 0 {
				return results[0]
			}
		}
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if ident.Name == "new" && len(e.Args) == 1 {
				return &ast.StarExpr{X: e.Args[0]}
			}
			if results := lookupFunctionResult(ctx, "", ident.Name); len(results) > 0 {
				return results[0]
			}
		}
		return e
	case *ast.SelectorExpr:
		return e
	case *ast.BasicLit:
		return e
	}
	return expr
}

type handlerAnalysis struct {
	RequestBody *core.RequestBody
	Responses   map[string]core.Response
}

type analysisContext struct {
	structs   map[string]*ast.StructType
	functions map[string][]functionSignature
	variables map[string]ast.Expr
	values    map[string]ast.Expr
}

// analyzeHandlerDetails inspects a handler function to infer request bodies and responses.
func analyzeHandlerDetails(fn *ast.FuncDecl, structs map[string]*ast.StructType, functions map[string][]functionSignature) handlerAnalysis {
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
		case *ast.RangeStmt:
			registerRangeTypes(node, ctx)
		case *ast.CallExpr:
			// Detect request body binding
			if analysis.RequestBody == nil && isBindingCall(node) {
				if len(node.Args) > 0 {
					if resolved := resolveRequestBody(node, node.Args[0], ctx); resolved != nil {
						analysis.RequestBody = resolved
					}
				}
			}

			// Detect response generation calls
			if contentType, statusExpr, dataExpr, ok := responseCallInfo(node, ctx); ok {
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

var bindingMethods = map[string]string{
	"Bind":               "auto",
	"MustBind":           "auto",
	"ShouldBind":         "auto",
	"BindJSON":           "application/json",
	"MustBindWith":       "auto",
	"ShouldBindJSON":     "application/json",
	"BindXML":            "application/xml",
	"ShouldBindXML":      "application/xml",
	"BindYAML":           "application/x-yaml",
	"ShouldBindYAML":     "application/x-yaml",
	"BindProto":          "application/protobuf",
	"BindProtobuf":       "application/protobuf",
	"ShouldBindProto":    "application/protobuf",
	"ShouldBindProtoBuf": "application/protobuf",
	"BindProtoBuf":       "application/protobuf",
	"BindBodyWith":       "auto",
	"ShouldBindBodyWith": "auto",
}

func isBindingCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	_, ok = bindingMethods[sel.Sel.Name]
	return ok
}

func resolveRequestBody(call *ast.CallExpr, arg ast.Expr, ctx *analysisContext) *core.RequestBody {
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
			if mime, found := bindingMethods[sel.Sel.Name]; found && mime != "auto" {
				body.ContentType = mime
			}
		}
	}

	if body.ContentType == "" {
		body.ContentType = "application/json"
	}

	return body
}

func registerDeclarationTypes(decl *ast.DeclStmt, ctx *analysisContext) {
	if ctx == nil {
		return
	}
	genDecl, ok := decl.Decl.(*ast.GenDecl)
	if !ok || (genDecl.Tok != token.VAR && genDecl.Tok != token.CONST) {
		return
	}

	for _, spec := range genDecl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		for idx, name := range valueSpec.Names {
			if name == nil || name.Name == "_" {
				continue
			}
			if _, exists := ctx.variables[name.Name]; exists {
				continue
			}

			var (
				inferred  ast.Expr
				valueExpr ast.Expr
			)
			if valueSpec.Type != nil {
				inferred = valueSpec.Type
				if len(valueSpec.Values) > 0 {
					targetIdx := idx
					if len(valueSpec.Values) == 1 {
						targetIdx = 0
					}
					if targetIdx < len(valueSpec.Values) {
						valueExpr = valueSpec.Values[targetIdx]
					}
				}
			} else if len(valueSpec.Values) > 0 {
				targetIdx := idx
				if len(valueSpec.Values) == 1 {
					targetIdx = 0
				}
				if targetIdx < len(valueSpec.Values) {
					valueExpr = valueSpec.Values[targetIdx]
					inferred = inferTypeFromExpr(valueExpr, ctx)
				}
			}

			if inferred != nil {
				ctx.variables[name.Name] = inferred
				ctx.values[name.Name] = valueExpr
			}
		}
	}
}

func registerAssignmentTypes(assign *ast.AssignStmt, ctx *analysisContext) {
	if ctx == nil || assign.Tok != token.DEFINE {
		return
	}

	for idx, name := range assign.Lhs {
		ident, ok := name.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		if _, exists := ctx.variables[ident.Name]; exists {
			continue
		}
		if idx >= len(assign.Rhs) {
			continue
		}

		inferred := inferTypeFromExpr(assign.Rhs[idx], ctx)
		if inferred != nil {
			ctx.variables[ident.Name] = inferred
			ctx.values[ident.Name] = assign.Rhs[idx]
		}
	}
}

func registerRangeTypes(rng *ast.RangeStmt, ctx *analysisContext) {
	if ctx == nil || rng.Tok != token.DEFINE {
		return
	}

	collectionType := inferTypeFromExpr(rng.X, ctx)
	if collectionType == nil {
		return
	}

	if valueIdent, ok := rng.Value.(*ast.Ident); ok && valueIdent.Name != "_" {
		if _, exists := ctx.variables[valueIdent.Name]; !exists {
			switch col := collectionType.(type) {
			case *ast.ArrayType:
				ctx.variables[valueIdent.Name] = col.Elt
				ctx.values[valueIdent.Name] = nil
			case *ast.MapType:
				ctx.variables[valueIdent.Name] = col.Value
				ctx.values[valueIdent.Name] = nil
			}
		}
	}
}

func inferTypeFromExpr(expr ast.Expr, ctx *analysisContext) ast.Expr {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		return e.Type
	case *ast.CallExpr:
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if ident.Name == "new" && len(e.Args) == 1 {
				return &ast.StarExpr{X: e.Args[0]}
			}
			if results := lookupFunctionResult(ctx, "", ident.Name); len(results) > 0 {
				return results[0]
			}
		}
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			full := exprToString(sel)
			if full == "json.Marshal" || full == "json.MarshalIndent" {
				return &ast.ArrayType{Elt: &ast.Ident{Name: "byte"}}
			}
			receiverType := resolveTypeFromArg(sel.X, ctx)
			receiverName := exprToString(receiverType)
			if results := lookupFunctionResult(ctx, receiverName, sel.Sel.Name); len(results) > 0 {
				return results[0]
			}
		}
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			switch x := e.X.(type) {
			case *ast.CompositeLit:
				return x.Type
			case *ast.Ident:
				if ctx != nil {
					if typ, ok := ctx.variables[x.Name]; ok {
						return typ
					}
				}
			}
		}
	case *ast.Ident, *ast.SelectorExpr, *ast.ArrayType, *ast.MapType, *ast.StructType, *ast.StarExpr:
		return e
	case *ast.BasicLit:
		return e
	}
	return nil
}

func resolveTypeFromArg(expr ast.Expr, ctx *analysisContext) ast.Expr {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			switch target := e.X.(type) {
			case *ast.Ident:
				if ctx != nil {
					if typ, ok := ctx.variables[target.Name]; ok {
						return typ
					}
				}
			case *ast.CompositeLit:
				return target.Type
			}
		}
	case *ast.Ident:
		if ctx != nil {
			if typ, ok := ctx.variables[e.Name]; ok {
				return typ
			}
		}
	case *ast.CallExpr:
		return inferTypeFromExpr(e, ctx)
	case *ast.CompositeLit:
		return e.Type
	case *ast.SelectorExpr:
		return e
	}
	return expr
}

func lookupFunctionResult(ctx *analysisContext, receiver, name string) []ast.Expr {
	if ctx == nil {
		return nil
	}

	if receiver != "" {
		if sigs, ok := ctx.functions[receiver+"."+name]; ok && len(sigs) > 0 {
			return sigs[0].results
		}
		trimmed := strings.TrimPrefix(receiver, "*")
		if sigs, ok := ctx.functions[trimmed+"."+name]; ok && len(sigs) > 0 {
			return sigs[0].results
		}
	}

	if sigs, ok := ctx.functions[name]; ok && len(sigs) > 0 {
		return sigs[0].results
	}

	return nil
}

func buildRequestBodyFromExpr(expr ast.Expr, ctx *analysisContext) *core.RequestBody {
	if expr == nil {
		return nil
	}

	schema, example := buildSchemaFromExpr(expr, ctx, make(map[string]bool))
	if schema == nil {
		return nil
	}

	body := &core.RequestBody{
		Schema:   schema,
		Example:  example,
		Required: true,
	}

	return body
}

func buildSchemaFromExpr(expr ast.Expr, ctx *analysisContext, visited map[string]bool) (interface{}, interface{}) {
	if expr == nil {
		return nil, nil
	}

	switch e := expr.(type) {
	case *ast.CompositeLit:
		return buildSchemaFromCompositeLiteral(e, ctx, visited)
	case *ast.StarExpr:
		return buildSchemaFromExpr(e.X, ctx, visited)
	case *ast.BasicLit:
		switch e.Kind {
		case token.STRING:
			value, err := strconv.Unquote(e.Value)
			if err != nil {
				value = strings.Trim(e.Value, "\"")
			}
			return map[string]interface{}{"type": "string"}, value
		case token.INT:
			parsed := strings.ReplaceAll(e.Value, "_", "")
			if num, err := strconv.ParseInt(parsed, 10, 64); err == nil {
				return map[string]interface{}{"type": "integer"}, num
			}
		case token.FLOAT:
			parsed := strings.ReplaceAll(e.Value, "_", "")
			if num, err := strconv.ParseFloat(parsed, 64); err == nil {
				return map[string]interface{}{"type": "number"}, num
			}
		case token.CHAR:
			value, err := strconv.Unquote(e.Value)
			if err != nil {
				value = e.Value
			}
			return map[string]interface{}{"type": "string"}, value
		}
		return map[string]interface{}{"type": "string"}, e.Value
	case *ast.Ident:
		if ctx != nil {
			if valExpr, ok := ctx.values[e.Name]; ok && valExpr != nil {
				schema, example := buildSchemaFromExpr(valExpr, ctx, visited)
				if schema != nil {
					return schema, example
				}
			}
		}
		if ctx != nil {
			if typ, ok := ctx.variables[e.Name]; ok {
				return buildSchemaFromExpr(typ, ctx, visited)
			}
		}
		if schema, example := primitiveSchemaForIdent(e.Name); schema != nil {
			return schema, example
		}
		if ctx != nil {
			if structType, ok := ctx.structs[e.Name]; ok {
				if visited[e.Name] {
					return map[string]interface{}{"type": "object"}, map[string]interface{}{}
				}
				visited[e.Name] = true
				schema, example := buildStructSchema(structType, ctx, visited)
				visited[e.Name] = false
				return schema, example
			}
		}
		return map[string]interface{}{"type": "string"}, ""
	case *ast.ArrayType:
		itemSchema, itemExample := buildSchemaFromExpr(e.Elt, ctx, visited)
		if itemSchema == nil {
			return nil, nil
		}
		schema := map[string]interface{}{"type": "array", "items": itemSchema}
		example := []interface{}{}
		if itemExample != nil {
			example = append(example, itemExample)
		}
		return schema, example
	case *ast.MapType:
		valueSchema, valueExample := buildSchemaFromExpr(e.Value, ctx, visited)
		schema := map[string]interface{}{"type": "object"}
		if valueSchema != nil {
			schema["additionalProperties"] = valueSchema
		}
		example := map[string]interface{}{}
		if valueExample != nil {
			example["key"] = valueExample
		}
		return schema, example
	case *ast.InterfaceType:
		schema := map[string]interface{}{"type": "object"}
		return schema, map[string]interface{}{}
	case *ast.StructType:
		return buildStructSchema(e, ctx, visited)
	case *ast.SelectorExpr:
		fullName := exprToString(e)
		if schema, example := schemaForSelector(fullName); schema != nil {
			return schema, example
		}
		if fullName == "gin.H" {
			return map[string]interface{}{"type": "object"}, map[string]interface{}{}
		}
		return map[string]interface{}{"type": "string"}, ""
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			full := exprToString(sel)
			if full == "json.Marshal" || full == "json.MarshalIndent" {
				if len(e.Args) > 0 {
					return buildSchemaFromExpr(e.Args[0], ctx, visited)
				}
			}
			receiverType := resolveTypeFromArg(sel.X, ctx)
			receiverName := exprToString(receiverType)
			if results := lookupFunctionResult(ctx, receiverName, sel.Sel.Name); len(results) > 0 {
				return buildSchemaFromExpr(results[0], ctx, visited)
			}
		}
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if ident.Name == "new" && len(e.Args) == 1 {
				return buildSchemaFromExpr(&ast.StarExpr{X: e.Args[0]}, ctx, visited)
			}
			if results := lookupFunctionResult(ctx, "", ident.Name); len(results) > 0 {
				return buildSchemaFromExpr(results[0], ctx, visited)
			}
		}
		return map[string]interface{}{"type": "object"}, map[string]interface{}{}
	}

	return nil, nil
}

func buildSchemaFromCompositeLiteral(lit *ast.CompositeLit, ctx *analysisContext, visited map[string]bool) (interface{}, interface{}) {
	if lit == nil {
		return map[string]interface{}{"type": "object"}, map[string]interface{}{}
	}

	if lit.Type == nil {
		return buildMapLiteralSchema(lit, ctx, visited)
	}

	switch t := lit.Type.(type) {
	case *ast.StructType:
		return buildStructSchema(t, ctx, visited)
	case *ast.Ident:
		if ctx != nil {
			if structType, ok := ctx.structs[t.Name]; ok {
				schema, example := buildStructSchema(structType, ctx, visited)
				if literalExample := buildStructLiteralExample(lit, structType, ctx, visited); len(literalExample) > 0 {
					if example == nil {
						example = make(map[string]interface{})
					}
					for key, value := range literalExample {
						example[key] = value
					}
				}
				return schema, example
			}
		}
		return buildSchemaFromExpr(t, ctx, visited)
	case *ast.MapType:
		return buildMapLiteralSchema(lit, ctx, visited)
	case *ast.ArrayType:
		itemSchema, _ := buildSchemaFromExpr(t.Elt, ctx, visited)
		schema := map[string]interface{}{"type": "array", "items": itemSchema}
		examples := make([]interface{}, 0, len(lit.Elts))
		for _, elt := range lit.Elts {
			_, ex := buildSchemaFromExpr(elt, ctx, visited)
			if ex == nil {
				ex = defaultExampleFromSchema(itemSchema)
			}
			if ex != nil {
				examples = append(examples, ex)
			}
		}
		if len(examples) == 0 {
			if ex := defaultExampleFromSchema(itemSchema); ex != nil {
				examples = append(examples, ex)
			}
		}
		return schema, examples
	case *ast.SelectorExpr:
		typeName := exprToString(t)
		if typeName == "gin.H" {
			return buildMapLiteralSchema(lit, ctx, visited)
		}
		return buildSchemaFromExpr(t, ctx, visited)
	default:
		return buildSchemaFromExpr(lit.Type, ctx, visited)
	}
}

func buildMapLiteralSchema(lit *ast.CompositeLit, ctx *analysisContext, visited map[string]bool) (interface{}, interface{}) {
	schema := map[string]interface{}{"type": "object"}
	properties := make(map[string]interface{})
	example := make(map[string]interface{})

	if lit != nil {
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key := literalKeyToString(kv.Key)
			if key == "" {
				continue
			}
			valueSchema, valueExample := buildSchemaFromExpr(kv.Value, ctx, visited)
			if valueSchema != nil {
				properties[key] = valueSchema
			}
			if valueExample == nil {
				valueExample = defaultExampleFromSchema(valueSchema)
			}
			if valueExample != nil {
				example[key] = valueExample
			}
		}
	}

	if len(properties) > 0 {
		schema["properties"] = properties
	} else {
		schema["additionalProperties"] = map[string]interface{}{"type": "string"}
	}

	if len(example) == 0 {
		return schema, map[string]interface{}{}
	}

	return schema, example
}

func literalKeyToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			value, err := strconv.Unquote(e.Value)
			if err != nil {
				return strings.Trim(e.Value, "\"")
			}
			return value
		}
	case *ast.Ident:
		return e.Name
	}
	return ""
}

func primitiveSchemaForIdent(name string) (map[string]interface{}, interface{}) {
	lower := strings.ToLower(name)
	switch lower {
	case "string", "rune":
		return map[string]interface{}{"type": "string"}, "string"
	case "bool":
		return map[string]interface{}{"type": "boolean"}, true
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		schema := map[string]interface{}{"type": "integer"}
		if lower == "int32" || lower == "uint32" {
			schema["format"] = "int32"
		}
		if lower == "int64" || lower == "uint64" {
			schema["format"] = "int64"
		}
		return schema, 0
	case "float32", "float64":
		schema := map[string]interface{}{"type": "number"}
		if lower == "float32" {
			schema["format"] = "float"
		}
		if lower == "float64" {
			schema["format"] = "double"
		}
		return schema, 0.0
	case "byte":
		schema := map[string]interface{}{"type": "integer", "format": "int32"}
		return schema, 0
	case "interface{}":
		schema := map[string]interface{}{"type": "object"}
		return schema, map[string]interface{}{}
	}
	return nil, nil
}

func schemaForSelector(fullName string) (map[string]interface{}, interface{}) {
	switch fullName {
	case "time.Time":
		return map[string]interface{}{"type": "string", "format": "date-time"}, "2024-01-01T00:00:00Z"
	case "uuid.UUID", "guuid.UUID", "github.com/google/uuid.UUID":
		return map[string]interface{}{"type": "string", "format": "uuid"}, "123e4567-e89b-12d3-a456-426614174000"
	default:
		return nil, nil
	}
}

func buildStructSchema(structType *ast.StructType, ctx *analysisContext, visited map[string]bool) (map[string]interface{}, map[string]interface{}) {
	properties := make(map[string]interface{})
	example := make(map[string]interface{})
	requiredFields := make([]string, 0)

	if structType.Fields == nil {
		return map[string]interface{}{"type": "object", "properties": properties}, example
	}

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			schema, nestedExample := buildSchemaFromExpr(field.Type, ctx, visited)
			if schemaMap, ok := schema.(map[string]interface{}); ok {
				if props, ok := schemaMap["properties"].(map[string]interface{}); ok {
					for key, val := range props {
						properties[key] = val
					}
				}
				if reqList, ok := schemaMap["required"].([]string); ok {
					requiredFields = append(requiredFields, reqList...)
				}
			}
			if nestedMap, ok := nestedExample.(map[string]interface{}); ok {
				for key, val := range nestedMap {
					example[key] = val
				}
			}
			continue
		}

		for _, name := range field.Names {
			if name == nil || name.Name == "" {
				continue
			}

			jsonName, skip := resolveJSONFieldName(name.Name, getStructTag(field, "json"))
			if skip {
				continue
			}

			bindingTag := getStructTag(field, "binding")
			validateTag := getStructTag(field, "validate")
			required := isFieldRequired(getStructTag(field, "json"), bindingTag, validateTag)

			schema, fieldExample := buildSchemaFromExpr(field.Type, ctx, visited)
			if schema == nil {
				continue
			}

			if description := fieldComment(field); description != "" {
				if schemaMap, ok := schema.(map[string]interface{}); ok {
					schemaMap["description"] = description
				}
			}

			if tagExample := getStructTag(field, "example"); tagExample != "" {
				fieldExample = convertExampleValue(tagExample, schema, fieldExample)
			}

			if fieldExample == nil {
				fieldExample = defaultExampleFromSchema(schema)
			}

			properties[jsonName] = schema
			if required {
				requiredFields = append(requiredFields, jsonName)
			}
			if fieldExample != nil {
				example[jsonName] = fieldExample
			}
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(requiredFields) > 0 {
		schema["required"] = requiredFields
	}

	return schema, example
}

func buildStructLiteralExample(lit *ast.CompositeLit, structType *ast.StructType, ctx *analysisContext, visited map[string]bool) map[string]interface{} {
	if lit == nil || structType == nil || structType.Fields == nil {
		return nil
	}

	fieldLookup := make(map[string]*ast.Field)
	for _, field := range structType.Fields.List {
		if field == nil {
			continue
		}
		for _, name := range field.Names {
			if name == nil || name.Name == "" {
				continue
			}
			fieldLookup[name.Name] = field
		}
	}

	if len(fieldLookup) == 0 {
		return nil
	}

	example := make(map[string]interface{})
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		fieldIdent, ok := kv.Key.(*ast.Ident)
		if !ok || fieldIdent.Name == "" {
			continue
		}

		field, ok := fieldLookup[fieldIdent.Name]
		if !ok {
			continue
		}

		jsonName, skip := resolveJSONFieldName(fieldIdent.Name, getStructTag(field, "json"))
		if skip || jsonName == "" {
			continue
		}

		_, fieldExample := buildSchemaFromExpr(kv.Value, ctx, visited)
		if fieldExample == nil {
			valueSchema, _ := buildSchemaFromExpr(field.Type, ctx, visited)
			fieldExample = defaultExampleFromSchema(valueSchema)
		}

		if fieldExample != nil {
			example[jsonName] = fieldExample
		}
	}

	if len(example) == 0 {
		return nil
	}
	return example
}

func resolveJSONFieldName(fieldName, tag string) (string, bool) {
	if tag == "-" {
		return "", true
	}
	if tag != "" {
		parts := strings.Split(tag, ",")
		if parts[0] != "" {
			return parts[0], false
		}
	}
	if fieldName == "" {
		return "", true
	}
	return lowerFirst(fieldName), false
}

func isFieldRequired(jsonTag, bindingTag, validateTag string) bool {
	if strings.Contains(jsonTag, "omitempty") {
		return false
	}
	if strings.Contains(bindingTag, "omitempty") {
		return false
	}
	if strings.Contains(bindingTag, "required") {
		return true
	}
	if strings.Contains(validateTag, "required") {
		return true
	}
	return false
}

func getStructTag(field *ast.Field, key string) string {
	if field.Tag == nil {
		return ""
	}
	value := field.Tag.Value
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		unquoted = strings.Trim(value, "`")
	}
	if unquoted == "" {
		return ""
	}
	return reflect.StructTag(unquoted).Get(key)
}

func convertExampleValue(raw string, schema interface{}, fallback interface{}) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}

	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var val interface{}
		if err := json.Unmarshal([]byte(trimmed), &val); err == nil {
			return val
		}
	}

	if schemaMap, ok := schema.(map[string]interface{}); ok {
		if kind, ok := schemaMap["type"].(string); ok {
			switch kind {
			case "integer":
				if num, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
					return num
				}
			case "number":
				if num, err := strconv.ParseFloat(trimmed, 64); err == nil {
					return num
				}
			case "boolean":
				if b, err := strconv.ParseBool(trimmed); err == nil {
					return b
				}
			}
		}
	}

	return raw
}

func defaultExampleFromSchema(schema interface{}) interface{} {
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil
	}
	kind, _ := schemaMap["type"].(string)
	switch kind {
	case "string":
		if format, ok := schemaMap["format"].(string); ok {
			switch format {
			case "date-time":
				return "2024-01-01T00:00:00Z"
			case "uuid":
				return "123e4567-e89b-12d3-a456-426614174000"
			}
		}
		return "string"
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		return false
	case "array":
		if items, ok := schemaMap["items"].(map[string]interface{}); ok {
			itemExample := defaultExampleFromSchema(items)
			if itemExample != nil {
				return []interface{}{itemExample}
			}
		}
		return []interface{}{}
	case "object":
		if props, ok := schemaMap["properties"].(map[string]interface{}); ok {
			example := make(map[string]interface{})
			for key, val := range props {
				if propSchema, ok := val.(map[string]interface{}); ok {
					example[key] = defaultExampleFromSchema(propSchema)
				} else {
					example[key] = nil
				}
			}
			return example
		}
		return map[string]interface{}{}
	}
	return nil
}

func normalizeExampleWithSchema(schema interface{}, example interface{}) interface{} {
	schemaMap, ok := schema.(map[string]interface{})
	if !ok || example == nil {
		return example
	}

	kind, _ := schemaMap["type"].(string)
	switch kind {
	case "array":
		itemsSchema := schemaMap["items"]
		slice, ok := example.([]interface{})
		if !ok {
			return example
		}
		normalized := make([]interface{}, 0, len(slice))
		for _, item := range slice {
			normalized = append(normalized, normalizeExampleWithSchema(itemsSchema, item))
		}
		return normalized
	case "object":
		exMap, ok := example.(map[string]interface{})
		if !ok {
			return example
		}
		props, _ := schemaMap["properties"].(map[string]interface{})
		if len(props) == 0 {
			return exMap
		}
		normalized := make(map[string]interface{}, len(exMap))
		usedKeys := make(map[string]bool)
		for propName, propSchema := range props {
			var value interface{}
			if v, ok := exMap[propName]; ok {
				value = v
				usedKeys[propName] = true
			} else {
				for exKey, exVal := range exMap {
					if strings.EqualFold(exKey, propName) {
						value = exVal
						usedKeys[exKey] = true
						break
					}
				}
			}
			if value != nil {
				normalized[propName] = normalizeExampleWithSchema(propSchema, value)
			}
		}
		for exKey, exVal := range exMap {
			if !usedKeys[exKey] {
				normalized[exKey] = exVal
			}
		}
		return normalized
	default:
		return example
	}
}

func fieldComment(field *ast.Field) string {
	if field.Comment != nil {
		var parts []string
		for _, comment := range field.Comment.List {
			parts = append(parts, strings.TrimSpace(strings.TrimPrefix(comment.Text, "//")))
		}
		return strings.Join(parts, " ")
	}
	if field.Doc != nil {
		var parts []string
		for _, comment := range field.Doc.List {
			parts = append(parts, strings.TrimSpace(strings.TrimPrefix(comment.Text, "//")))
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
