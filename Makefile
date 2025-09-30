.PHONY: build dev test clean install release

# Build Go binary  
build-go: 
	@echo "🚀 Building Go binary..."
	go build -ldflags "-s -w" -o bin/bytedocs ./cmd/bytedocs

# Development mode
dev:
	@echo "🔧 Starting development server..."
	cd web && npm run dev &
	go run ./examples/gin/main.go

# Test the package
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/ web/dist/ web/node_modules/

# Install dependencies
install:
	go mod download
	cd web && npm install

# Quick test with Gin example
example:
	@echo "🚀 Running Gin example..."
	@echo "Visit http://localhost:8080/docs after server starts"
	go run ./examples/gin/main.go

# Release builds for different platforms
release: build-ui
	@echo "📦 Building release binaries..."
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/bytedocs-linux ./cmd/bytedocs
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/bytedocs-windows.exe ./cmd/bytedocs  
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o bin/bytedocs-darwin ./cmd/bytedocs
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o bin/bytedocs-darwin-arm64 ./cmd/bytedocs
	
	@echo "✅ Release binaries built in bin/"