# Pupervisor Makefile

# Variables
APP_NAME := pupervisor
MAIN_PATH := ./cmd/server
BUILD_DIR := ./build
CONFIG_FILE := pupervisor.yaml
DB_FILE := pupervisor.db

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GORUN := $(GOCMD) run
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build flags
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags="-w -s -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Docker variables
DOCKER_IMAGE := $(APP_NAME)
DOCKER_TAG := latest

.PHONY: all build run run-dev test clean lint fmt vet \
        docker-build docker-up docker-down docker-logs docker-clean \
        deps install help

.DEFAULT_GOAL := help

##@ Development

build: ## Build the binary
	$(GOBUILD) $(LDFLAGS) -o $(APP_NAME) $(MAIN_PATH)

run: build ## Build and run the application
	./$(APP_NAME) --config $(CONFIG_FILE) --db $(DB_FILE)

run-dev: ## Run without building (faster for development)
	$(GORUN) $(MAIN_PATH) --config $(CONFIG_FILE) --db $(DB_FILE)

watch: ## Run with auto-reload (requires: go install github.com/air-verse/air@latest)
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air

##@ Testing & Quality

test: ## Run tests
	$(GOTEST) -v ./...

test-cover: ## Run tests with coverage
	$(GOTEST) -v -cover -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linter (requires: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

fmt: ## Format code
	$(GOFMT) ./...

vet: ## Run go vet
	$(GOVET) ./...

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

##@ Build Variants

build-linux: ## Build for Linux (amd64)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_PATH)

build-linux-arm: ## Build for Linux (arm64)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 $(MAIN_PATH)

build-darwin: ## Build for macOS (amd64)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 $(MAIN_PATH)

build-darwin-arm: ## Build for macOS (arm64/M1)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 $(MAIN_PATH)

build-windows: ## Build for Windows (amd64)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe $(MAIN_PATH)

build-all: ## Build for all platforms
	@mkdir -p $(BUILD_DIR)
	@$(MAKE) build-linux
	@$(MAKE) build-linux-arm
	@$(MAKE) build-darwin
	@$(MAKE) build-darwin-arm
	@$(MAKE) build-windows
	@echo "Binaries built in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/

##@ Docker

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-up: ## Start with docker-compose
	docker-compose up -d

docker-down: ## Stop docker-compose
	docker-compose down

docker-logs: ## Show docker-compose logs
	docker-compose logs -f

docker-restart: docker-down docker-up ## Restart docker-compose

docker-rebuild: ## Rebuild and restart
	docker-compose up -d --build

docker-clean: ## Remove Docker image and volumes
	docker-compose down -v
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true

docker-shell: ## Open shell in running container
	docker-compose exec pupervisor /bin/sh

##@ Dependencies

deps: ## Download dependencies
	$(GOMOD) download

tidy: ## Tidy dependencies
	$(GOMOD) tidy

update: ## Update dependencies
	$(GOMOD) tidy
	$(GOCMD) get -u ./...
	$(GOMOD) tidy

##@ Cleanup

clean: ## Remove build artifacts
	rm -f $(APP_NAME)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	$(GOCMD) clean

clean-all: clean ## Remove all generated files including database
	rm -f $(DB_FILE) $(DB_FILE)-journal $(DB_FILE)-shm $(DB_FILE)-wal

##@ Installation

install: build ## Install binary to $GOPATH/bin
	cp $(APP_NAME) $(GOPATH)/bin/

uninstall: ## Remove binary from $GOPATH/bin
	rm -f $(GOPATH)/bin/$(APP_NAME)

##@ Help

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n\033[1mPupervisor - Process Supervisor\033[0m\n\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

info: ## Show build info
	@echo "App:       $(APP_NAME)"
	@echo "Version:   $(VERSION)"
	@echo "Build:     $(BUILD_TIME)"
	@echo "Go:        $(shell go version)"
