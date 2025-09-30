# Contributing to ByteDocs

Thanks for contributing! Here's how to get started.

## Quick Start

1. Fork and clone the repository
2. Install dependencies: `go mod download`
3. Make your changes
4. Run tests: `make test`
5. Build the project: `make build`
6. Submit a pull request

## Prerequisites

- Go 1.23 or higher
- Make (optional, for convenience)

## Development Workflow

1. Create a feature branch: `git checkout -b feature/your-feature`
2. Make changes and add tests for new functionality
3. Run tests to ensure everything works: `make test` or `go test ./...`
4. Build the project: `make build` or `go build`
5. Test with examples: `cd examples/gin && go run main.go`
6. Use conventional commits: `feat:`, `fix:`, `docs:`, etc.
7. Submit a pull request

## What We're Looking For

- Framework parsers for new Go web frameworks
- Additional AI/LLM provider integrations
- Bug fixes and performance improvements
- Documentation improvements

## Code Guidelines

- Follow Go conventions (`gofmt`, `golint`, `go vet`)
- Add tests for new functionality
- Document public APIs
- Handle errors properly
- Use meaningful variable names

## Pull Request Checklist

- [ ] Tests pass (`make test`)
- [ ] Build succeeds (`make build`)
- [ ] Documentation updated if needed
- [ ] Conventional commit messages used
- [ ] No API keys or secrets committed