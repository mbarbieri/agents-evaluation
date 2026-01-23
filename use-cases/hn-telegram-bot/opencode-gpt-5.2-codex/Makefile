MODULE=hn-telegram-bot

.PHONY: test coverage build build-arm64

test:
	go test ./...

coverage:
	go test ./... -coverprofile=coverage.out

build:
	go build ./cmd/hn-bot

build-arm64:
	GOOS=linux GOARCH=arm64 go build ./cmd/hn-bot
