.PHONY: build build-arm64 test coverage clean

BINARY_NAME := hn-telegram-bot

build:
	go build -trimpath -o bin/$(BINARY_NAME) ./cmd/hn-telegram-bot

build-arm64:
	GOOS=linux GOARCH=arm64 go build -trimpath -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/hn-telegram-bot

test:
	go test ./...

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

clean:
	rm -rf bin coverage.out
