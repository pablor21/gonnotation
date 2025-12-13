.PHONY: dev prod build test clean

# Default to development mode
dev:
	@echo "Switching to development mode..."
	@sed -i '/^replace.*=> \.\./d' go.mod
	@cat go.mod.dev >> go.mod
	@go mod tidy

# Production mode (removes replace directives)
prod:
	@echo "Switching to production mode..."
	@sed -i '/^replace.*=> \.\./d' go.mod
	@go mod tidy

# Build for development
build-dev: dev
	go build ./...

# Build for production/CI
build-prod: prod
	go build ./...

# Test with current mode
test:
	go test ./...

# Clean up any backup files (if they exist)
clean:
	rm -f go.mod.bak go.mod.backup

# Setup development environment
setup-dev: dev
	@echo "Development environment ready\!"
	@echo "Use 'make build-dev' for development builds"
	@echo "Use 'make build-prod' for production/CI builds"

publish:
	@echo "Publishing module..."
	@go clean -modcache
	@go mod tidy 
	@make prod 
	@./publish.sh