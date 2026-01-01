# csd-devtrack Makefile

.PHONY: all build clean run test deps

# Variables
GO = go
BINARY_NAME = csd-devtrack
CLI_DIR = cli
BUILD_DIR = targets
LDFLAGS = -ldflags="-s -w"

# Platforms
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Default target
all: build

# Install dependencies
deps:
	cd $(CLI_DIR) && $(GO) mod download
	cd $(CLI_DIR) && $(GO) mod tidy

# Build for current platform
build: deps
	cd $(CLI_DIR) && $(GO) build $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME) ./csd-devtrack.go

# Build for all platforms
release: deps
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(BUILD_DIR)/$(BINARY_NAME)-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then output=$$output.exe; fi; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(LDFLAGS) -C $(CLI_DIR) -o ../$$output ./csd-devtrack.go; \
	done

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run with TUI
ui: build
	./$(BUILD_DIR)/$(BINARY_NAME) ui

# Run shell mode
shell: build
	./$(BUILD_DIR)/$(BINARY_NAME) shell

# Run tests
test:
	cd $(CLI_DIR) && $(GO) test -v ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	cd $(CLI_DIR) && $(GO) clean

# Development: build and run
dev: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Format code
fmt:
	cd $(CLI_DIR) && $(GO) fmt ./...

# Vet code
vet:
	cd $(CLI_DIR) && $(GO) vet ./...

# Help
help:
	@echo "csd-devtrack Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make deps     - Download dependencies"
	@echo "  make build    - Build for current platform"
	@echo "  make release  - Build for all platforms"
	@echo "  make run      - Build and run"
	@echo "  make ui       - Build and run TUI"
	@echo "  make shell    - Build and run shell mode"
	@echo "  make test     - Run tests"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make fmt      - Format code"
	@echo "  make vet      - Vet code"
