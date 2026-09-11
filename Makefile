.DEFAULT_GOAL := help

BINARY_NAME := notifier
MAIN_PKG := ./cmd/notifier
BINARY := bin/$(BINARY_NAME)

PORT ?= 8080
API_KEY ?= secret
LOG_LEVEL ?= info

.PHONY: build run test test-race lint clean docker-build docker-up docker-start docker-stop docker-down docker-logs help

build: ## собрать бинарник в bin/
	go build -o $(BINARY) $(MAIN_PKG)

run: build ## собрать и запустить сервис
	PORT=$(PORT) API_KEY=$(API_KEY) LOG_LEVEL=$(LOG_LEVEL) $(BINARY)

test: ## запустить тесты
	go test ./...

test-race: ## запустить тесты с race
	go test -race ./...

lint: ## прогнать golangci-lint
	golangci-lint run ./...

clean: ## удалить bin/
	rm -rf bin/

docker-build: ## собрать docker-образ сервиса
	docker compose build

docker-up: ## пересобрать и поднять сервис вместе с базой
	docker compose up -d --build

docker-start: ## запустить ранее остановленные контейнеры без пересборки
	docker compose start

docker-stop: ## остановить контейнеры без удаления
	docker compose stop

docker-down: ## остановить и удалить контейнеры
	docker compose down

docker-logs: ## смотреть логи сервиса
	docker compose logs -f notifier

help: ## подсказать
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'