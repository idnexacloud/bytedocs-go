# ByteDocs for Go

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-bytedocs-blue.svg)](https://github.com/aibnuhibban/bytedocs)

**ByteDocs** is a modern alternative to Swagger with better design, auto-detection, and AI integration for Go web applications. It automatically generates beautiful API documentation from your routes with zero configuration required.

## Features

- 🚀 **Auto Route Detection** - Automatically discovers routes, parameters, and data structures from your Go framework
- 🎨 **Beautiful Modern UI** - Clean, responsive interface with dark mode support
- 🤖 **AI Integration** - Built-in AI assistant (OpenAI, Gemini, Claude, OpenRouter) to help users understand your API
- 📱 **Mobile Responsive** - Works perfectly on all device sizes
- 🔍 **Interactive Testing** - Try out endpoints directly from the documentation
- 📊 **OpenAPI Compatible** - Exports standard OpenAPI 3.0.3 specification in JSON and YAML formats
- 🔐 **Built-in Authentication** - Support for Basic Auth, API Key, Bearer Token, and Session authentication
- ⚡ **Zero Configuration** - Works out of the box with sensible defaults
- 🔧 **Multi-Framework Support** - Gin, Echo, Fiber, Gorilla Mux, and standard net/http
- 🌍 **Environment Config** - Full `.env` file support with validation

## Quick Start

### 1. Install ByteDocs

```bash
go get github.com/idnexacloud/bytedocs-go
```

### 2. Add One Line to Your Code

```go
package main

import (
    "github.com/aibnuhibban/bytedocs/pkg/core"
    "github.com/aibnuhibban/bytedocs/pkg/parser"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()
    
    parser.SetupGinDocs(r, &core.Config{
        Title:       "My API",
        Version:     "1.0.0",
        Description: "Auto-generated API documentation",
        DocsPath:    "/docs",
        AutoDetect:  true,
    })
    
    r.GET("/users", getUsers)
    r.POST("/users", createUser)
    
    r.Run(":8080")
}
```

### 3. Visit Your Documentation

Open http://localhost:8080/docs and enjoy your auto-generated API documentation!

## API Endpoints

Once installed, ByteDocs provides these endpoints:

- `GET /docs` - Main documentation interface with beautiful UI
- `GET /docs/api-data.json` - Raw documentation data
- `GET /docs/openapi.json` - OpenAPI 3.0.3 specification (JSON format)
- `GET /docs/openapi.yaml` - OpenAPI 3.0.3 specification (YAML format)
- `POST /docs/chat` - AI chat endpoint (if AI is enabled)

## Configuration

### Basic Configuration

```go
config := &core.Config{
    Title:       "My API Documentation",
    Version:     "1.0.0",
    Description: "Comprehensive API for my application",
    DocsPath:    "/docs",
    AutoDetect:  true,

    BaseURLs: []core.BaseURLOption{
        {Name: "Production", URL: "https://api.myapp.com"},
        {Name: "Staging", URL: "https://staging-api.myapp.com"},
        {Name: "Local", URL: "http://localhost:8080"},
    },
}
```

### AI Integration

Enable AI assistance for your API documentation:

```go
config := &core.Config{
    Title:   "My API",
    Version: "1.0.0",
    AIConfig: &ai.AIConfig{
        Enabled:  true,
        Provider: "openai", // openai, gemini, claude, openrouter
        APIKey:   "your-api-key", // or use env var
        Features: ai.AIFeatures{
            ChatEnabled:          true,
            DocGenerationEnabled: false,
            Model:                "gpt-4o-mini",
            MaxTokens:            1000,
            Temperature:          0.7,
        },
    },
}
```

**Supported AI Providers:**

### OpenAI
```go
AIConfig: &ai.AIConfig{
    Provider: "openai",
    APIKey:   os.Getenv("OPENAI_API_KEY"),
    Features: ai.AIFeatures{
        Model: "gpt-4o-mini", // or gpt-4, gpt-3.5-turbo
    },
}
```

### Google Gemini
```go
AIConfig: &ai.AIConfig{
    Provider: "gemini",
    APIKey:   os.Getenv("GEMINI_API_KEY"),
    Features: ai.AIFeatures{
        Model: "gemini-1.5-flash", // or gemini-1.5-pro
    },
}
```

### Anthropic Claude
```go
AIConfig: &ai.AIConfig{
    Provider: "claude",
    APIKey:   os.Getenv("ANTHROPIC_API_KEY"),
    Features: ai.AIFeatures{
        Model: "claude-3-sonnet-20240229",
    },
}
```

### OpenRouter
```go
AIConfig: &ai.AIConfig{
    Provider: "openrouter",
    APIKey:   os.Getenv("OPENROUTER_API_KEY"),
    Features: ai.AIFeatures{
        Model: "openai/gpt-4o-mini", // Any OpenRouter model
    },
}
```

### Authentication

Protect your documentation with built-in authentication:

```go
config := &core.Config{
    AuthConfig: &core.AuthConfig{
        Enabled:  true,
        Type:     "session", // basic, api_key, bearer, session
        Username: "admin",
        Password: "secret",
        Realm:    "API Documentation",

        // Session-specific settings
        SessionExpire:     1440, // minutes
        IPBanEnabled:      true,
        IPBanMaxAttempts:  5,
        IPBanDuration:     60, // minutes
        AdminWhitelistIPs: []string{"127.0.0.1"},
    },
}
```

**Authentication Types:**
- `basic` - HTTP Basic Authentication
- `api_key` - API Key authentication via custom header
- `bearer` - Bearer token authentication
- `session` - Session-based authentication with login page

### Environment Configuration

Create a `.env` file for easy configuration:

```bash
# Basic Settings
BYTEDOCS_TITLE="My API Documentation"
BYTEDOCS_VERSION="1.0.0"
BYTEDOCS_DESCRIPTION="Comprehensive API for my application"
BYTEDOCS_DOCS_PATH="/docs"
BYTEDOCS_AUTO_DETECT=true

# Multiple Environment URLs
BYTEDOCS_PRODUCTION_URL="https://api.myapp.com"
BYTEDOCS_STAGING_URL="https://staging-api.myapp.com"
BYTEDOCS_LOCAL_URL="http://localhost:8080"

# Authentication
BYTEDOCS_AUTH_ENABLED=true
BYTEDOCS_AUTH_TYPE=session
BYTEDOCS_AUTH_USERNAME=admin
BYTEDOCS_AUTH_PASSWORD=your-secret-password
BYTEDOCS_AUTH_SESSION_EXPIRE=1440
BYTEDOCS_AUTH_IP_BAN_ENABLED=true
BYTEDOCS_AUTH_IP_BAN_MAX_ATTEMPTS=5

# AI Configuration
BYTEDOCS_AI_ENABLED=true
BYTEDOCS_AI_PROVIDER=openai
BYTEDOCS_AI_API_KEY=sk-your-openai-key
BYTEDOCS_AI_MODEL=gpt-4o-mini
BYTEDOCS_AI_MAX_TOKENS=1000
BYTEDOCS_AI_TEMPERATURE=0.7

# UI Customization
BYTEDOCS_UI_THEME=auto
BYTEDOCS_UI_SHOW_TRY_IT=true
BYTEDOCS_UI_SHOW_SCHEMAS=true
```

Then load it in your code:
```go
config, err := core.LoadConfigFromEnv()
if err != nil {
    log.Fatal(err)
}

// Validate configuration
if err := core.ValidateConfig(config); err != nil {
    log.Fatal(err)
}

parser.SetupGinDocs(r, config)
```

## Framework Support

ByteDocs supports all major Go web frameworks with simple one-line setup:

### Gin
```go
import (
    "github.com/aibnuhibban/bytedocs/pkg/core"
    "github.com/aibnuhibban/bytedocs/pkg/parser"
    "github.com/gin-gonic/gin"
)

r := gin.Default()
parser.SetupGinDocs(r, config)
```

### Echo
```go
import (
    "github.com/aibnuhibban/bytedocs/pkg/core"
    "github.com/aibnuhibban/bytedocs/pkg/parser"
    "github.com/labstack/echo/v4"
)

e := echo.New()
parser.SetupEchoDocs(e, config)
```

### Fiber
```go
import (
    "github.com/aibnuhibban/bytedocs/pkg/core"
    "github.com/aibnuhibban/bytedocs/pkg/parser"
    "github.com/gofiber/fiber/v2"
)

app := fiber.New()
parser.SetupFiberDocs(app, config)
```

### Gorilla Mux
```go
import (
    "github.com/aibnuhibban/bytedocs/pkg/core"
    "github.com/aibnuhibban/bytedocs/pkg/parser"
    "github.com/gorilla/mux"
)

r := mux.NewRouter()
parser.SetupMuxDocs(r, config)
```

### Standard net/http
```go
import (
    "github.com/aibnuhibban/bytedocs/pkg/core"
    "github.com/aibnuhibban/bytedocs/pkg/parser"
    "net/http"
)

mux := http.NewServeMux()
parser.SetupHTTPDocs(mux, config)
```

## Advanced Usage

### Manual Route Registration

```go
import "github.com/aibnuhibban/bytedocs/pkg/core"

docs := core.New(config)

// Manually add route information
docs.AddRoute("GET", "/api/custom-endpoint", nil,
    core.WithSummary("Custom endpoint"),
    core.WithDescription("This is a manually registered endpoint"),
    core.WithParameters([]core.Parameter{
        {
            Name:        "id",
            In:          "path",
            Type:        "integer",
            Required:    true,
            Description: "Record ID",
        },
    }),
)

// Generate documentation
docs.Generate()
```

### Export OpenAPI Specifications

```go
// Get OpenAPI JSON
openAPIJSON, err := docs.GetOpenAPIJSON()
if err != nil {
    log.Fatal(err)
}

// Get OpenAPI YAML
openAPIYAML, err := docs.GetOpenAPIYAML()
if err != nil {
    log.Fatal(err)
}

// Save to file
ioutil.WriteFile("openapi.yaml", openAPIYAML, 0644)
```

## Requirements

- Go 1.23 or higher
- Supported frameworks: Gin, Echo, Fiber, Gorilla Mux, or standard net/http

## Development

### Prerequisites
- Go 1.23 or higher
- Node.js 18+ (for UI development)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/aibnuhibban/bytedocs.git
cd bytedocs

# Install dependencies
make install

# Build everything
make build

# Run example
make example
# Visit http://localhost:8080/docs
```

### Development Mode

```bash
# Start development server with hot reload
make dev
```

### Available Commands

```bash
make build      # Build everything (UI + Go binary)
make dev        # Development mode with hot reload
make test       # Run tests
make clean      # Clean build artifacts
make example    # Run Gin example
make release    # Build release binaries for all platforms
```

## Project Structure

```
bytedocs/
  cmd/
    bytedocs/           # CLI application
    apidocs/            # API documentation generator
  pkg/
    core/               # Core functionality
    parser/             # Framework parsers
    llm/                # AI/LLM integration
    ui/                 # Web UI components
    web/                # React frontend
    examples/           # Framework examples
        gin/
        echo/
        fiber/
        gorilla-mux/
        net-http/
    docs/               # Documentation
    scripts/            # Build scripts
```

## Examples

Check out the `examples/` directory for complete implementations with different frameworks:

- **Gin**: `examples/gin/main.go` - Full-featured Gin example with routes
- **Echo**: `examples/echo/main.go` - Echo framework integration
- **Fiber**: `examples/fiber/main.go` - Fiber framework with ByteDocs
- **Gorilla Mux**: `examples/gorilla-mux/main.go` - Gorilla Mux router example
- **Standard HTTP**: `examples/net-http/main.go` - Standard library example
- **Stdlib**: `examples/stdlib/main.go` - Alternative stdlib implementation

Each example demonstrates:
- Basic setup and configuration
- Route auto-detection
- AI integration (optional)
- Authentication setup
- Custom documentation

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests if applicable
5. Run `make test` to ensure everything works
6. Commit your changes (`git commit -m 'Add some amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 📖 [Documentation](https://github.com/aibnuhibban/bytedocs)
- 🐛 [Report Issues](https://github.com/aibnuhibban/bytedocs/issues)
- 💬 [GitHub Discussions](https://github.com/aibnuhibban/bytedocs/discussions)

---

**Made with ❤️ for the Go community**