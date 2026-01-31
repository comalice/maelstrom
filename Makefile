# Makefile for Maelstrom
# Run from project root (maelstrom/)

.PHONY: all build run test lint tidy swagger clean install-deps test-e2e

all: tidy lint test swagger build

run:
	LISTEN_ADDR=:8090 go run ./cmd/server/maelstrom.go

dev: run

test:
	go test ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

swagger:
	swag init -g ./cmd/server/maelstrom.go

build: swagger
	go build -o bin/server ./cmd/server

clean:
	rm -rf bin/ docs/swagger*.json docs/swagger*.go

install-deps:
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

test-e2e: build
	./e2e_tests.sh
