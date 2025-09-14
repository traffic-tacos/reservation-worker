.PHONY: build test clean docker-build docker-run lint fmt vet deps dev

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Main package
MAIN_PATH=./cmd/reservation-worker
BINARY_NAME=reservation-worker
BINARY_UNIX=$(BINARY_NAME)_unix

# Build the project
build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PATH)

# Build for linux/arm64
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PATH)

# Test
test:
	$(GOTEST) -v ./...

# Test coverage
test-coverage:
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f coverage.out coverage.html

# Run
run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PATH)
	./$(BINARY_NAME)

# Dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	$(GOFMT) ./...

# Vet code
vet:
	$(GOVET) ./...

# Lint (requires golangci-lint)
lint:
	golangci-lint run

# Docker build
docker-build:
	docker build -t $(BINARY_NAME):latest .

# Docker run
docker-run:
	docker run --rm $(BINARY_NAME):latest

# Development setup
dev: deps fmt vet lint test build

# Local development with air (hot reload)
dev-air:
	air

# Install development tools
install-tools:
	$(GOCMD) install github.com/cosmtrek/air@latest
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install github.com/vektra/mockery/v2@latest

# Generate mocks
mocks:
	mockery --all --output ./test/mocks

# CI pipeline
ci: deps fmt vet lint test build-linux

