.PHONY: build test clean fmt vet lint help

# Variables
BINARY_NAME=msa-core
GO_VERSION=1.23.5

help: ## Hiển thị help
	@echo "Các lệnh có sẵn:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build project
	@echo "Building..."
	go build ./...

test: ## Chạy tests
	@echo "Running tests..."
	go test -v ./...

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

vet: ## Chạy go vet
	@echo "Running go vet..."
	go vet ./...

clean: ## Xóa build artifacts
	@echo "Cleaning..."
	go clean
	rm -f $(BINARY_NAME)

deps: ## Tải dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

check: fmt vet test ## Chạy format, vet và test

install: deps build ## Install dependencies và build

