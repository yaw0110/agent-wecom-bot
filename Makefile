.PHONY: build run clean test

BINARY=wecom-bot

build:
	go build -o $(BINARY) ./cmd/wecom-bot/

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	go clean

test:
	go test ./... -v

fmt:
	go fmt ./...

lint:
	go vet ./...
