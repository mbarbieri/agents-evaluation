APP=hn-telegram-bot

.PHONY: build build-arm64 test coverage

build:
	mkdir -p bin
	go build -o bin/$(APP) .

build-arm64:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/$(APP)-linux-arm64 .

test:
	go test ./...

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
