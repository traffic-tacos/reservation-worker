# Project variables
BINARY_NAME=reservation-worker
MODULE_NAME=github.com/traffic-tacos/reservation-worker
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOLINT=golangci-lint

# Build flags
LDFLAGS=-ldflags "-w -s -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.CommitHash=$(COMMIT_HASH)"

# Directories
CMD_DIR=./cmd/reservation-worker
BUILD_DIR=./bin
COVERAGE_DIR=./coverage

# Docker parameters
DOCKER_REGISTRY?=ghcr.io
DOCKER_NAMESPACE?=traffic-tacos
DOCKER_IMAGE=$(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(BINARY_NAME)
DOCKER_TAG?=$(VERSION)

# Colors for terminal output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: all
all: clean fmt lint test build ## Run all tasks

.PHONY: help
help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

.PHONY: init
init: ## Initialize the project and install dependencies
	@echo "$(YELLOW)Initializing project...$(NC)"
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "$(GREEN)Project initialized successfully$(NC)"

.PHONY: fmt
fmt: ## Format the code
	@echo "$(YELLOW)Formatting code...$(NC)"
	$(GOFMT) ./...
	@echo "$(GREEN)Code formatted$(NC)"

.PHONY: lint
lint: ## Run linters
	@echo "$(YELLOW)Running linters...$(NC)"
	@if which $(GOLINT) > /dev/null 2>&1; then \
		$(GOLINT) run ./...; \
	else \
		echo "$(RED)golangci-lint not installed. Install with: make install-lint$(NC)"; \
	fi

.PHONY: install-lint
install-lint: ## Install golangci-lint
	@echo "$(YELLOW)Installing golangci-lint...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)golangci-lint installed$(NC)"

.PHONY: test
test: ## Run tests
	@echo "$(YELLOW)Running tests...$(NC)"
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	@echo "$(GREEN)Tests completed$(NC)"

.PHONY: test-coverage
test-coverage: test ## Run tests with coverage report
	@echo "$(YELLOW)Generating coverage report...$(NC)"
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)Coverage report generated at $(COVERAGE_DIR)/coverage.html$(NC)"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(YELLOW)Running integration tests...$(NC)"
	$(GOTEST) -v -tags=integration ./test/...
	@echo "$(GREEN)Integration tests completed$(NC)"

.PHONY: build
build: ## Build the binary
	@echo "$(YELLOW)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

.PHONY: build-linux
build-linux: ## Build for Linux (arm64)
	@echo "$(YELLOW)Building for Linux arm64...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64$(NC)"

.PHONY: run
run: ## Run the application locally
	@echo "$(YELLOW)Running $(BINARY_NAME)...$(NC)"
	@if [ -f .env.local ]; then \
		set -a; . ./.env.local; set +a; \
	fi; \
	$(GOCMD) run $(CMD_DIR)

.PHONY: run-with-env
run-with-env: ## Run with .env.local file
	@echo "$(YELLOW)Running with .env.local...$(NC)"
	@if [ ! -f .env.local ]; then \
		echo "$(RED).env.local not found. Copy from .env.example$(NC)"; \
		exit 1; \
	fi
	@set -a; . ./.env.local; set +a; $(GOCMD) run $(CMD_DIR)

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "$(YELLOW)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	@echo "$(YELLOW)Pushing Docker image...$(NC)"
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker image pushed: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

.PHONY: docker-run
docker-run: ## Run the application in Docker
	@echo "$(YELLOW)Running Docker container...$(NC)"
	docker run --rm -p 8040:8040 -p 8041:8041 \
		--env-file .env.local \
		--name $(BINARY_NAME) \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-run-with-aws
docker-run-with-aws: ## Run the application in Docker with AWS credentials
	@echo "$(YELLOW)Running Docker container with AWS credentials...$(NC)"
	docker run --rm -p 8040:8040 -p 8041:8041 \
		-v ~/.aws:/root/.aws:ro \
		-e AWS_PROFILE=tacos \
		-e AWS_REGION=ap-northeast-2 \
		-e SQS_QUEUE_URL=https://sqs.ap-northeast-2.amazonaws.com/137406935518/traffic-tacos-reservation-events \
		--name $(BINARY_NAME) \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: grpcui
grpcui: ## Launch grpcui for debugging (requires grpcui installed)
	@echo "$(YELLOW)Launching grpcui for gRPC debugging...$(NC)"
	@if which grpcui > /dev/null 2>&1; then \
		grpcui -plaintext localhost:8041; \
	else \
		echo "$(RED)grpcui not installed. Install with: go install github.com/fullstorydev/grpcui/cmd/grpcui@latest$(NC)"; \
	fi

.PHONY: install-grpcui
install-grpcui: ## Install grpcui
	@echo "$(YELLOW)Installing grpcui...$(NC)"
	go install github.com/fullstorydev/grpcui/cmd/grpcui@latest
	@echo "$(GREEN)grpcui installed$(NC)"

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR) $(COVERAGE_DIR)
	@echo "$(GREEN)Cleaned$(NC)"

.PHONY: verify
verify: fmt lint test ## Verify code quality (format, lint, test)
	@echo "$(GREEN)All checks passed!$(NC)"

.PHONY: deps
deps: ## Check and update dependencies
	@echo "$(YELLOW)Checking dependencies...$(NC)"
	$(GOMOD) verify
	$(GOMOD) tidy
	@echo "$(GREEN)Dependencies verified$(NC)"

.PHONY: proto
proto: ## Update proto-contracts dependency
	@echo "$(YELLOW)Updating proto-contracts...$(NC)"
	$(GOGET) -u github.com/traffic-tacos/proto-contracts@latest
	$(GOMOD) tidy
	@echo "$(GREEN)Proto contracts updated$(NC)"

.PHONY: localstack
localstack: ## Start LocalStack for local development
	@echo "$(YELLOW)Starting LocalStack...$(NC)"
	docker-compose -f docker-compose.localstack.yml up -d
	@echo "$(GREEN)LocalStack started$(NC)"

.PHONY: localstack-stop
localstack-stop: ## Stop LocalStack
	@echo "$(YELLOW)Stopping LocalStack...$(NC)"
	docker-compose -f docker-compose.localstack.yml down
	@echo "$(GREEN)LocalStack stopped$(NC)"

.PHONY: logs
logs: ## Show application logs
	@if [ -f ./reservation-worker.log ]; then \
		tail -f ./reservation-worker.log; \
	else \
		echo "$(RED)Log file not found$(NC)"; \
	fi

.PHONY: info
info: ## Show project information
	@echo "Project: $(MODULE_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Commit Hash: $(COMMIT_HASH)"
	@echo "Go Version: $(shell go version)"

# Default target
.DEFAULT_GOAL := help