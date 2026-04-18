.PHONY: build run test dev

build:
	go build -o bin/portal ./cmd/portal

run: build
	./bin/portal

test:
	go test ./...

dev:
	go run ./cmd/portal
