
BINARY_NAME := notifier
MAIN_PKG := ./cmd/notifier
BINARY := bin/$(BINARY_NAME)

PORT ?= 8080
API_KEY ?= secret123
LOG_LEVEL ?= info

.PHONY: build run test test-race clean

build:
	go build -o $(BINARY) $(MAIN_PKG)

run: build
	PORT=$(PORT) API_KEY=$(API_KEY) LOG_LEVEL=$(LOG_LEVEL) $(BINARY)

test:
	go test ./...

test-race:
	go test -race ./...

clean:
	rm -rf bin/