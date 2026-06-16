# Makefile for orchestrating Go builds and execution

.PHONY: run build tidy clean

# Default action when running make
all: run

# Run the project locally
run:
	go run cmd/bot/main.go

# Build production compiled binary
build:
	go build -o bin/bot.exe cmd/bot/main.go

# Format Go codebase files
fmt:
	go fmt ./...

# Tidy dependencies
tidy:
	go mod tidy

# Clean built binaries
clean:
	if exist bin rmdir /s /q bin