.PHONY: build build-arm64 test coverage clean

# Build binary for current platform
build:
	go build -o hn-bot .

# Cross-compile for Linux ARM64 (Raspberry Pi)
build-arm64:
	GOOS=linux GOARCH=arm64 go build -o hn-bot-arm64 .

# Run all tests
test:
	go test -v ./...

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

# Clean build artifacts
clean:
	rm -f hn-bot hn-bot-arm64 coverage.out coverage.html
