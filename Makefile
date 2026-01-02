.PHONY: all build run test clean docker-up docker-down docker-logs lint help

# Application
APP_NAME := taskchat
MAIN_PATH := ./cmd/server

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GORUN := $(GOCMD) run
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOLINT := golangci-lint

# Build output
BUILD_DIR := ./bin
BINARY := $(BUILD_DIR)/$(APP_NAME)

# Default target
all: lint build

## build: Build the application binary
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BINARY) $(MAIN_PATH)
	@echo "Binary created at $(BINARY)"

## run: Run the application locally
run:
	$(GORUN) $(MAIN_PATH)/main.go

## test: Run all tests
test:
	$(GOTEST) -v -race ./...

## test-coverage: Run tests with coverage report
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linter
lint:
	@if command -v $(GOLINT) > /dev/null; then \
		$(GOLINT) run ./...; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Done"

## deps: Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## up: Start all services with Docker Compose
up:
	docker-compose up -d --build

## down: Stop all Docker services
down:
	docker-compose down

## docker-logs: View Docker logs
docker-logs:
	docker-compose logs -f

## clean: Stop services and remove volumes
clean:
	docker-compose down -v --rmi local

## api-test: Quick API smoke test (requires running server)
api-test:
	@echo "Testing health endpoint..."
	@curl -s http://localhost:8080/health | jq .
	@echo "\nRegistering test user..."
	@curl -s -X POST http://localhost:8080/signup \
		-H "Content-Type: application/json" \
		-d '{"email":"test@example.com","password":"test123456"}' | jq .
	@echo "\nLogging in..."
	@curl -s -X POST http://localhost:8080/login \
		-H "Content-Type: application/json" \
		-d '{"email":"test@example.com","password":"test123456"}' | jq .

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
