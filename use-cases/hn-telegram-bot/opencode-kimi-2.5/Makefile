.PHONY: all build build-arm64 test coverage clean help

# Binary name
BINARY_NAME=hn-telegram-bot

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-s -w"

# Default target
all: test build

## build: Build binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

## build-arm64: Cross-compile for Linux ARM64 (Raspberry Pi)
build-arm64:
	@echo "Building $(BINARY_NAME) for Linux ARM64..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-arm64 .

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## coverage: Generate coverage report
coverage:
	@echo "Generating coverage report..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-arm64
	rm -f coverage.out
	rm -f coverage.html
	rm -f *.db

## deps: Download and verify dependencies
deps:
	$(GOMOD) download
	$(GOMOD) verify

## tidy: Tidy go modules
tidy:
	$(GOMOD) tidy

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## //g'