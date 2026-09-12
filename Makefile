.DEFAULT_GOAL := help

BINARY_NAME := notifier
MAIN_PKG := ./cmd/notifier
BINARY := bin/$(BINARY_NAME)

# ---- единая точка конфигурации ----
# ENV_FILE выбирает окружение: .env (дев, по умолчанию) или .env.prod (прод).
# Пример: make docker-up ENV_FILE=.env.prod
# Переменные из файла подхватываются здесь и экспортируются (см. `export`
# ниже) во все команды — и в локальный `run`, и в `docker compose`.
ENV_FILE ?= .env
-include $(ENV_FILE)

# запасные значения на случай, если $(ENV_FILE) отсутствует или не
# задаёт переменную (например, на чистом чекауте без cp .env.example .env)
PORT ?= 8080
API_KEY ?= secret
LOG_LEVEL ?= info

export

.PHONY: build run test test-race lint clean docker-build docker-up docker-start docker-stop docker-down docker-logs release ci help

build: ## собрать бинарник в bin/
	go build -o $(BINARY) $(MAIN_PKG)

run: build ## собрать и запустить сервис локально (переменные из $(ENV_FILE))
	$(BINARY)

test: ## запустить тесты
	go test ./...

test-race: ## запустить тесты с race
	go test -race ./...

lint: ## прогнать golangci-lint
	golangci-lint run ./...

clean: ## удалить bin/
	rm -rf bin/

docker-build: ## собрать docker-образ сервиса
	docker compose --env-file $(ENV_FILE) build

docker-up: ## пересобрать и поднять сервис вместе с базой
	docker compose --env-file $(ENV_FILE) up -d --build

docker-start: ## запустить ранее остановленные контейнеры без пересборки
	docker compose --env-file $(ENV_FILE) start

docker-stop: ## остановить контейнеры без удаления
	docker compose --env-file $(ENV_FILE) stop

docker-down: ## остановить и удалить контейнеры
	docker compose --env-file $(ENV_FILE) down

docker-logs: ## смотреть логи сервиса
	docker compose --env-file $(ENV_FILE) logs -f notifier

release: ## Собрать релизные бинарники в dist/
	@mkdir -p dist
	@for OSARCH in linux/amd64 linux/arm64 darwin/arm64; do \
		GOOS=$${OSARCH%/*}; GOARCH=$${OSARCH#*/}; \
		echo "building notifier-$$GOOS-$$GOARCH"; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH \
			go build -ldflags='-s -w' -o dist/notifier-$$GOOS-$$GOARCH ./cmd/notifier; \
	done

ci: lint test-race build ## Прогнать все проверки CI локально

help: ## подсказать
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'