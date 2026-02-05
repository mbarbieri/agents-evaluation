.PHONY: build build-arm64 test coverage clean

build:
	go build -o hn-telegram-bot ./cmd/bot

build-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o hn-telegram-bot-arm64 ./cmd/bot

test:
	go test ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -f hn-telegram-bot hn-telegram-bot-arm64 coverage.out
