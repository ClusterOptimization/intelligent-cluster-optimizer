# Intelligent Cluster Optimizer - Makefile
# Build, test, and manage the project

# Variables
BINARY_NAME=intelligent-cluster-optimizer
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Directories
BIN_DIR=bin
CMD_DIR=cmd

# Default target
.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: all
all: clean test build ## Clean, test, and build all binaries

.PHONY: build
build: build-controller build-collector build-optctl ## Build all binaries

.PHONY: build-controller
build-controller: ## Build the controller binary
	@echo "Building controller..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/controller ./$(CMD_DIR)/controller

.PHONY: build-collector
build-collector: ## Build the collector binary
	@echo "Building collector..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/collector ./$(CMD_DIR)/collector

.PHONY: build-optctl
build-optctl: ## Build the optctl CLI binary
	@echo "Building optctl..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/optctl ./$(CMD_DIR)/optctl

.PHONY: install
install: build ## Install binaries to $GOPATH/bin
	@echo "Installing binaries..."
	cp $(BIN_DIR)/controller $(GOPATH)/bin/
	cp $(BIN_DIR)/collector $(GOPATH)/bin/
	cp $(BIN_DIR)/optctl $(GOPATH)/bin/

##@ Testing

.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GOTEST) -race -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: test-short
test-short: ## Run short tests only
	$(GOTEST) -short -v ./...

.PHONY: bench
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

##@ Code Quality

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

.PHONY: check
check: fmt vet test ## Run all checks (fmt, vet, test)

##@ Dependencies

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

.PHONY: tidy
tidy: ## Tidy go.mod
	@echo "Tidying go.mod..."
	$(GOMOD) tidy

.PHONY: verify
verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GOMOD) verify

##@ Build Variants

.PHONY: build-linux
build-linux: ## Build for Linux amd64
	@echo "Building for Linux amd64..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/controller-linux-amd64 ./$(CMD_DIR)/controller
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/collector-linux-amd64 ./$(CMD_DIR)/collector
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/optctl-linux-amd64 ./$(CMD_DIR)/optctl

.PHONY: build-linux-arm64
build-linux-arm64: ## Build for Linux arm64
	@echo "Building for Linux arm64..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/controller-linux-arm64 ./$(CMD_DIR)/controller
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/collector-linux-arm64 ./$(CMD_DIR)/collector
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/optctl-linux-arm64 ./$(CMD_DIR)/optctl

.PHONY: build-darwin
build-darwin: ## Build for macOS amd64
	@echo "Building for macOS amd64..."
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/controller-darwin-amd64 ./$(CMD_DIR)/controller
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/collector-darwin-amd64 ./$(CMD_DIR)/collector
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/optctl-darwin-amd64 ./$(CMD_DIR)/optctl

.PHONY: build-all-platforms
build-all-platforms: build-linux build-linux-arm64 build-darwin ## Build for all platforms

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

.PHONY: docker-push
docker-push: ## Push Docker image (requires REGISTRY env var)
	@echo "Pushing Docker image..."
	docker tag $(BINARY_NAME):$(VERSION) $(REGISTRY)/$(BINARY_NAME):$(VERSION)
	docker push $(REGISTRY)/$(BINARY_NAME):$(VERSION)

##@ Kubernetes

.PHONY: install-crd
install-crd: ## Install CRD to cluster
	@echo "Installing CRD..."
	kubectl apply -f config/crd/optimizerconfig-crd.yaml

.PHONY: uninstall-crd
uninstall-crd: ## Uninstall CRD from cluster
	@echo "Uninstalling CRD..."
	kubectl delete -f config/crd/optimizerconfig-crd.yaml --ignore-not-found

##@ Cleanup

.PHONY: clean
clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

.PHONY: clean-all
clean-all: clean ## Remove build artifacts and cached data
	$(GOCMD) clean -cache -testcache

##@ Release

.PHONY: release-dry-run
release-dry-run: ## Dry run of release (requires goreleaser)
	@echo "Running release dry-run..."
	@which goreleaser > /dev/null || (echo "goreleaser not installed" && exit 1)
	goreleaser release --snapshot --clean

.PHONY: version
version: ## Print version information
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Built:   $(BUILD_TIME)"
