.PHONY: build test coverage clean arm64

build:
	go build -o hn-bot ./cmd/hn-bot

test:
	go test ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

arm64:
	GOOS=linux GOARCH=arm64 go build -o hn-bot-arm64 ./cmd/hn-bot

clean:
	rm -f hn-bot hn-bot-arm64 coverage.out
