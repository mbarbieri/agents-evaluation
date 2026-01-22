.PHONY: build build-arm64 test test-coverage clean run

BIN_NAME=hn-bot

build:
	go build -o $(BIN_NAME) .

build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BIN_NAME)-arm64 .

test:
	go test ./... -v

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -f $(BIN_NAME)
	rm -f $(BIN_NAME)-arm64
	rm -f coverage.out coverage.html

run:
	go run .

deps:
	go mod tidy
	go mod download
