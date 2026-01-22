.PHONY: build build-arm64 test coverage clean

BINARY_NAME=hn-bot
GO=go

build:
	$(GO) build -o $(BINARY_NAME) ./main.go

build-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -o $(BINARY_NAME)-arm64 ./main.go

test:
	$(GO) test -v ./...

coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	$(GO) tool cover -func=coverage.out

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-arm64 coverage.out coverage.html
