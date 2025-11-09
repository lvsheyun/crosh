.PHONY: build clean install test release

# Binary name
BINARY_NAME=crosh
VERSION?=$(shell cat VERSION 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Build directory
BUILD_DIR=build
DIST_DIR=dist

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/crosh
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	@echo "Clean complete"

# Install to local system
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Install complete"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Cross-compile for multiple platforms
release: clean deps
	@echo "Building release binaries..."
	@mkdir -p $(DIST_DIR)
	
	# Linux AMD64
	@echo "Building for linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/crosh
	
	# Linux ARM64
	@echo "Building for linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/crosh
	
	# macOS AMD64
	@echo "Building for darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/crosh
	
	# macOS ARM64 (Apple Silicon)
	@echo "Building for darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/crosh
	
	# Windows AMD64
	@echo "Building for windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/crosh
	
	# Create checksums
	@echo "Generating checksums..."
	cd $(DIST_DIR) && sha256sum * > checksums.txt
	
	@echo "Release build complete!"
	@echo "Binaries available in $(DIST_DIR)/"

# Create offline installation packages
package: release
	@echo "Creating offline installation packages..."
	@mkdir -p $(DIST_DIR)/packages
	
	# Linux AMD64
	@echo "Packaging for linux/amd64..."
	@mkdir -p $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64
	@cp $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/$(BINARY_NAME)
	@cp scripts/install.sh $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/
	@echo "#!/bin/bash" > $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo "# Offline installation script for crosh" >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo "set -e" >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo 'INSTALL_DIR="$${INSTALL_DIR:-/usr/local/bin}"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo 'echo "Installing crosh to $$INSTALL_DIR..."' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo 'if [ -w "$$INSTALL_DIR" ]; then' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo '    cp crosh "$$INSTALL_DIR/"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo '    chmod +x "$$INSTALL_DIR/crosh"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo 'else' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo '    sudo cp crosh "$$INSTALL_DIR/"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo '    sudo chmod +x "$$INSTALL_DIR/crosh"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo 'fi' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@echo 'echo "Installation complete! Run: crosh help"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@chmod +x $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh
	@tar czf $(DIST_DIR)/packages/crosh-$(VERSION)-linux-amd64.tar.gz -C $(DIST_DIR)/tmp crosh-$(VERSION)-linux-amd64
	
	# Linux ARM64
	@echo "Packaging for linux/arm64..."
	@mkdir -p $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-arm64
	@cp $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-arm64/$(BINARY_NAME)
	@cp scripts/install.sh $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-arm64/
	@cp $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-arm64/
	@tar czf $(DIST_DIR)/packages/crosh-$(VERSION)-linux-arm64.tar.gz -C $(DIST_DIR)/tmp crosh-$(VERSION)-linux-arm64
	
	# macOS AMD64
	@echo "Packaging for darwin/amd64..."
	@mkdir -p $(DIST_DIR)/tmp/crosh-$(VERSION)-darwin-amd64
	@cp $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(DIST_DIR)/tmp/crosh-$(VERSION)-darwin-amd64/$(BINARY_NAME)
	@cp scripts/install.sh $(DIST_DIR)/tmp/crosh-$(VERSION)-darwin-amd64/
	@cp $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh $(DIST_DIR)/tmp/crosh-$(VERSION)-darwin-amd64/
	@tar czf $(DIST_DIR)/packages/crosh-$(VERSION)-darwin-amd64.tar.gz -C $(DIST_DIR)/tmp crosh-$(VERSION)-darwin-amd64
	
	# macOS ARM64
	@echo "Packaging for darwin/arm64..."
	@mkdir -p $(DIST_DIR)/tmp/crosh-$(VERSION)-darwin-arm64
	@cp $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(DIST_DIR)/tmp/crosh-$(VERSION)-darwin-arm64/$(BINARY_NAME)
	@cp scripts/install.sh $(DIST_DIR)/tmp/crosh-$(VERSION)-darwin-arm64/
	@cp $(DIST_DIR)/tmp/crosh-$(VERSION)-linux-amd64/install-offline.sh $(DIST_DIR)/tmp/crosh-$(VERSION)-darwin-arm64/
	@tar czf $(DIST_DIR)/packages/crosh-$(VERSION)-darwin-arm64.tar.gz -C $(DIST_DIR)/tmp crosh-$(VERSION)-darwin-arm64
	
	# Windows AMD64
	@echo "Packaging for windows/amd64..."
	@mkdir -p $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64
	@cp $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/$(BINARY_NAME).exe
	@echo "@echo off" > $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/install.bat
	@echo "REM Offline installation script for crosh on Windows" >> $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/install.bat
	@echo 'set "INSTALL_DIR=%ProgramFiles%\crosh"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/install.bat
	@echo 'echo Installing crosh to %INSTALL_DIR%...' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/install.bat
	@echo 'if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/install.bat
	@echo 'copy crosh.exe "%INSTALL_DIR%\"' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/install.bat
	@echo 'echo Installation complete!' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/install.bat
	@echo 'echo Please add %INSTALL_DIR% to your PATH' >> $(DIST_DIR)/tmp/crosh-$(VERSION)-windows-amd64/install.bat
	@cd $(DIST_DIR)/tmp && zip -r ../packages/crosh-$(VERSION)-windows-amd64.zip crosh-$(VERSION)-windows-amd64
	
	# Clean up temp directory
	@rm -rf $(DIST_DIR)/tmp
	
	@echo "Offline packages created in $(DIST_DIR)/packages/"
	@ls -lh $(DIST_DIR)/packages/

# Build for current platform and run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# Show help
help:
	@echo "Makefile commands:"
	@echo "  make build      - Build for current platform"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make install    - Install to /usr/local/bin"
	@echo "  make test       - Run tests"
	@echo "  make deps       - Download dependencies"
	@echo "  make release    - Cross-compile for all platforms"
	@echo "  make package    - Create offline installation packages"
	@echo "  make run        - Build and run"
	@echo "  make fmt        - Format code"
	@echo "  make lint       - Run linter"
	@echo "  make help       - Show this help message"

