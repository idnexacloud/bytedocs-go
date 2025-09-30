# ByteDocs Examples

This directory contains examples of how to use ByteDocs with different Go web frameworks. Each example demonstrates the same basic API with framework-specific implementations.

## 📁 Available Examples

All examples provide simple CRUD operations for Users and Products to demonstrate ByteDocs integration.

### 1. **Gin** (`gin/`)
- **Framework**: Gin
- **Features**: Users & Products CRUD
- **Complexity**: ⭐ (Beginner-friendly)

### 2. **Echo** (`echo/`)
- **Framework**: Echo
- **Features**: Users & Products CRUD
- **Complexity**: ⭐ (Beginner-friendly)

### 3. **Fiber** (`fiber/`)
- **Framework**: Fiber
- **Features**: Users & Products CRUD
- **Complexity**: ⭐ (Beginner-friendly)

### 4. **Gorilla Mux** (`gorilla-mux/`)
- **Framework**: Gorilla Mux
- **Features**: Users & Products CRUD
- **Complexity**: ⭐ (Beginner-friendly)

### 5. **Net/HTTP** (`net-http/`)
- **Framework**: Standard Library (net/http)
- **Features**: Users & Products CRUD
- **Complexity**: ⭐ (Beginner-friendly)

### 6. **Stdlib** (`stdlib/`)
- **Framework**: Standard Library (alternative implementation)
- **Features**: Users & Products CRUD
- **Complexity**: ⭐ (Beginner-friendly)

## 🚀 Quick Start

### 1. Choose an Example
Navigate to any example directory:
```bash
cd examples/gin/  # or echo/, fiber/, gorilla-mux/, net-http/, stdlib/
```

### 2. Configure Environment (Optional)
Copy and customize the environment file:
```bash
cp .env.example .env
```

Edit `.env` to customize:
- API title and description
- Authentication settings
- AI configuration (requires API key)
- Base URLs

### 3. Run the Example
```bash
go run main.go
```

### 4. Access Documentation
Open your browser to view the auto-generated docs:
```
http://localhost:8080/docs
```

Note: Port may vary based on your `.env` configuration.

## 🔑 Authentication (Optional)

All examples support optional authentication. Enable it in your `.env` file:

```env
BYTEDOCS_AUTH_ENABLED=true
BYTEDOCS_AUTH_TYPE=session
BYTEDOCS_AUTH_USERNAME=admin
BYTEDOCS_AUTH_PASSWORD=secret
```

## 🎯 What Each Example Provides

All examples include:
- ✅ **Users CRUD** - GET, POST, PUT, DELETE operations
- ✅ **Products CRUD** - GET, POST, PUT, DELETE operations
- ✅ **Auto-generated docs** - Beautiful UI at `/docs`
- ✅ **OpenAPI export** - JSON and YAML formats
- ✅ **Environment config** - Easy customization via `.env`
- ✅ **Framework-specific** - Shows best practices per framework

## 🔧 API Endpoints

Each example exposes these endpoints:

### Users API
- `GET /api/users` - List all users
- `GET /api/users/:id` - Get user by ID
- `POST /api/users` - Create new user
- `PUT /api/users/:id` - Update user
- `DELETE /api/users/:id` - Delete user

### Products API
- `GET /api/products` - List all products
- `GET /api/products/:id` - Get product by ID
- `POST /api/products` - Create new product
- `PUT /api/products/:id` - Update product
- `DELETE /api/products/:id` - Delete product

### Documentation
- `GET /docs` - ByteDocs UI
- `GET /docs/api-data.json` - API data
- `GET /docs/openapi.json` - OpenAPI 3.0 JSON
- `GET /docs/openapi.yaml` - OpenAPI 3.0 YAML

## 🤖 AI Integration (Optional)

Enable AI-powered documentation assistant in `.env`:

```env
BYTEDOCS_AI_ENABLED=true
BYTEDOCS_AI_PROVIDER=openai  # or gemini, claude, openrouter
BYTEDOCS_AI_API_KEY=your-api-key-here
BYTEDOCS_AI_MODEL=gpt-4o-mini
```

## 🎯 Framework Comparison

Choose the framework that fits your project:

| Framework | Routing | Middleware | Performance | Community |
|-----------|---------|------------|-------------|-----------|
| **Gin** | Simple | Built-in | Fast | Large |
| **Echo** | Simple | Built-in | Fast | Large |
| **Fiber** | Express-like | Built-in | Very Fast | Growing |
| **Gorilla Mux** | Flexible | Manual | Good | Mature |
| **Net/HTTP** | Manual | Manual | Standard | Official |
| **Stdlib** | Manual | Manual | Standard | Official |

## 🐛 Troubleshooting

### Common Issues

**Port already in use:**
```bash
# Check what's using the port
lsof -i :8080

# Kill the process or change port in .env
BYTEDOCS_PORT=8081
```

**Dependencies not found:**
```bash
# Install dependencies
go mod download
go mod tidy
```

**Configuration errors:**
```bash
# Verify your .env file exists and has correct values
cat .env

# Compare with example
diff .env .env.example
```

## 📝 Adding New Examples

Want to add a new framework example?

1. Create directory: `examples/your-framework/`
2. Add `main.go` implementing Users & Products CRUD
3. Include `.env.example` with configuration
4. Test it: `go run main.go`
5. Verify docs at `http://localhost:8080/docs`
6. Update this README

## 📞 Support

- 📖 Main docs: [../README.md](../README.md)
- 🐛 Issues: [GitHub Issues](https://github.com/aibnuhibban/bytedocs/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/aibnuhibban/bytedocs/discussions)