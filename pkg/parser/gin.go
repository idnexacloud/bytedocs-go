package parser

import (
	"go/ast"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/idnexacloud/bytedocs-go/pkg/core"
	"github.com/gin-gonic/gin"
)

var (
	globalDocs *core.APIDocs
	docsConfig *core.Config
	docsMutex  sync.RWMutex
)

type HandlerInfo struct {
	Summary     string
	Description string
	Parameters  []core.Parameter
}

func extractCommentsText(comments []*ast.Comment) []string {
	var lines []string
	for _, comment := range comments {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		text = strings.TrimSpace(strings.TrimPrefix(text, "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		if text != "" {
			lines = append(lines, text)
		}
	}
	return lines
}

func parseHandlerInfo(comments []string) HandlerInfo {
	info := HandlerInfo{
		Parameters: make([]core.Parameter, 0),
	}

	paramRegex := regexp.MustCompile(`@Param\s+(\w+)\s+(\w+)\s+(\w+)\s+(true|false)\s+"([^"]*)"`)

	for _, line := range comments {
		if matches := paramRegex.FindStringSubmatch(line); len(matches) == 6 {
			param := core.Parameter{
				Name:        matches[1],
				In:          matches[2],
				Type:        matches[3],
				Required:    matches[4] == "true",
				Description: matches[5],
			}
			info.Parameters = append(info.Parameters, param)
		} else if strings.HasPrefix(line, "@Param") {
			continue
		} else if info.Summary == "" && !strings.HasPrefix(line, "@") {
			info.Summary = line
		} else if !strings.HasPrefix(line, "@") && info.Description == "" {
			info.Description = line
		}
	}

	return info
}

func extractHandlerName(handler interface{}) string {
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

// SetupGinDocs sets up documentation for a Gin engine with auto-detection
func SetupGinDocs(engine *gin.Engine, config *core.Config) {
	if config == nil {
		config = &core.Config{
			Title:      "API Documentation",
			Version:    "1.0.0",
			DocsPath:   "/docs",
			AutoDetect: true,
		}
	}

	docsMutex.Lock()
	docsConfig = config
	globalDocs = core.New(config)
	docsMutex.Unlock()


	engine.Any(config.DocsPath+"/*path", func(c *gin.Context) {
		docsMutex.Lock()
		defer docsMutex.Unlock()

		endpointsCount := len(globalDocs.GetDocumentation().Endpoints)

		if endpointsCount == 0 && config.AutoDetect {
			routes := engine.Routes()

			for _, route := range routes {
				if strings.HasPrefix(route.Path, config.DocsPath) ||
					strings.Contains(route.Path, "/static") ||
					strings.Contains(route.Path, "/assets") {
					continue
				}

				metadata := getHandlerMetadata(route.HandlerFunc)

				routeInfo := core.RouteInfo{
					Method:      route.Method,
					Path:        route.Path,
					Handler:     route.HandlerFunc,
					Summary:     metadata.Info.Summary,
					Description: metadata.Info.Description,
					Parameters:  metadata.Info.Parameters,
					RequestBody: metadata.RequestBody,
					Responses:   metadata.Responses,
				}

				globalDocs.AddRouteInfo(routeInfo)
			}

			globalDocs.Generate()
		}

		globalDocs.ServeHTTP(c.Writer, c.Request)
	})
}
