# Gin Example with Auto-Configuration

This example demonstrates how to use ByteDocs with Gin framework using **environment-based configuration** instead of hardcoding values in `main.go`.

## 🚀 Quick Start

1. **Copy environment file**:
   ```bash
   cp .env.example .env
   ```

2. **Edit configuration** (optional):
   ```bash
   nano .env
   ```

3. **Run the application**:
   ```bash
   go run main.go
   ```

4. **Access documentation**:
   - Open: http://localhost:8083/docs
   - Password: `your-secure-password` (from .env)

## 📁 Files

- **`main.go`** - Clean code with minimal hardcoding
- **`.env.example`** - All configuration options with examples
- **`.env`** - Your actual configuration (copy from .env.example)

## 🔧 Configuration Options

### Authentication
```env
# Laravel-style Session Authentication
BYTEDOCS_AUTH_ENABLED=true
BYTEDOCS_AUTH_TYPE=session
BYTEDOCS_AUTH_PASSWORD=your-secure-password

# Session settings
BYTEDOCS_AUTH_SESSION_EXPIRE=1440  # 24 hours
BYTEDOCS_AUTH_IP_BAN_ENABLED=true
BYTEDOCS_AUTH_IP_BAN_MAX_ATTEMPTS=5
BYTEDOCS_AUTH_IP_BAN_DURATION=60   # 60 minutes
```

### API Information
```env
BYTEDOCS_TITLE=Gin API Documentation
BYTEDOCS_VERSION=1.0.0
BYTEDOCS_DESCRIPTION=Your API description
BYTEDOCS_DOCS_PATH=/docs
```

### AI Features (Optional)
```env
BYTEDOCS_AI_ENABLED=true
BYTEDOCS_AI_PROVIDER=openrouter
OPENROUTER_API_KEY=sk-or-your-key
BYTEDOCS_AI_MODEL=anthropic/claude-3-haiku
```

## 🎯 Benefits

- ✅ **No hardcoding** in main.go
- ✅ **Environment-based** configuration
- ✅ **Laravel-style UI** authentication
- ✅ **Auto-detection** of API endpoints
- ✅ **Beautiful documentation** with session auth
- ✅ **Easy deployment** - just set environment variables

## 🔐 Authentication Features

- **Session-based auth** like Laravel
- **Beautiful login page** with dark/light mode
- **IP banning** after failed attempts
- **Session expiration** control
- **Admin IP whitelist** protection

## 🤖 AI Chat (Optional)

If AI is enabled, users can chat with AI about your API:
- AI knows all your endpoints automatically
- Contextual responses based on your API spec
- Multiple AI providers supported

## 📋 OpenAPI Spec

Auto-generated OpenAPI specification available at:
- http://localhost:8083/docs/openapi.json