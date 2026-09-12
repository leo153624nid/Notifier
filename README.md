# Notifier

Небольшой HTTP-сервис на Go для приёма и рассылки уведомлений по разным каналам (консоль, email, telegram — в текущей реализации это заглушки). Уведомления сохраняются в PostgreSQL, отправка выполняется асинхронно, а любой запрос на экспорт пишет аудит-лог на диск.

## Возможности

- REST API для создания, получения и листинга уведомлений
- Асинхронная отправка через набор `Sender` (`console`, `email`, `telegram`)
- Хранение в PostgreSQL (таблица создаётся автоматически при старте)
- Аутентификация по API-ключу (заголовок `X-API-KEY`)
- Rate limiting по IP (token bucket, 10 запросов/сек, burst 20)
- Structured logging (`log/slog`, JSON), request ID на каждый запрос
- Graceful shutdown с ожиданием фоновых горутин отправки
- Экспорт уведомлений в аудит-лог (`audit.log`)
- Docker / docker compose для локального запуска вместе с Postgres

## Стек

- Go 1.27
- PostgreSQL (драйвер `jackc/pgx/v5`)
- `golang.org/x/time/rate` — rate limiting

## Быстрый старт

### Через Docker Compose (рекомендуется)

```bash
cp .env.example .env
make docker-up
```

Поднимутся два контейнера: `postgres` и `notifier`. Сервис будет доступен на `http://localhost:8080`.

Другие команды для работы с Docker:

```bash
make docker-build   # собрать образ
make docker-start   # запустить остановленные контейнеры
make docker-stop    # остановить без удаления
make docker-down    # остановить и удалить
make docker-logs    # смотреть логи сервиса
```

### Локальный запуск

Нужен доступный Postgres (например, поднятый через `docker compose`, см. выше) и Go 1.27+.

```bash
cp .env.example .env
make run
```

`make run` соберёт бинарник в `bin/notifier` и запустит его, подхватив переменные из `.env`.

### Прод-окружение

```bash
cp .env.prod.example .env.prod
# заполнить реальными значениями
make docker-up ENV_FILE=.env.prod
```

`.env.prod` не коммитится — храните секреты отдельно.

## Конфигурация

Все переменные читаются из файла окружения (`.env` по умолчанию, задаётся через `ENV_FILE`) и Makefile прокидывает их дальше — и в `make run`, и в `docker compose`.

| Переменная | Описание | По умолчанию |
|---|---|---|
| `PORT` | Порт HTTP-сервера | `8080` |
| `API_KEY` | Ключ для аутентификации запросов (обязателен) | — |
| `LOG_LEVEL` | Уровень логирования (`debug`/`info`/`warn`/`error`) | `info` |
| `AUDIT_LOG_PATH` | Путь к файлу аудит-лога | `audit.log` |
| `PG_HOST` | Хост Postgres | `localhost` |
| `PG_PORT` | Порт Postgres | `5432` |
| `PG_USER` | Пользователь Postgres | `notifier` |
| `PG_PASSWORD` | Пароль Postgres | `notifier` |
| `PG_DBNAME` | Имя базы данных | `notifier` |
| `PG_SSLMODE` | Режим SSL для подключения к Postgres | `disable` |
| `APP_HOST_PORT` | Порт сервиса, публикуемый на хосте (только docker compose) | `8080` |
| `POSTGRES_HOST_PORT` | Порт Postgres, публикуемый на хосте (только docker compose) | `5432` |

Внутри docker-сети адрес БД сервиса всегда `postgres:5432` — это топология compose, а не настраиваемый параметр.

## API

Все эндпоинты, кроме `/health`, требуют заголовок `X-API-KEY` со значением `API_KEY` и подчиняются rate limiting по IP.

### `GET /health`

Проверка живости сервиса и доступности БД. Без аутентификации.

```bash
curl http://localhost:8080/health
```

### `POST /api/v1/notifications`

Создать уведомление. Отправка выполняется асинхронно; ответ возвращается сразу со статусом `pending`.

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "X-API-KEY: secret" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "user@example.com",
    "subject": "Hello",
    "body": "World",
    "channel": "email",
    "urgent": false
  }'
```

Поддерживаемые значения `channel`: `console`, `email`, `telegram`.

### `GET /api/v1/notifications`

Список всех уведомлений.

```bash
curl http://localhost:8080/api/v1/notifications -H "X-API-KEY: secret"
```

### `GET /api/v1/notifications/{id}`

Получить уведомление по ID.

```bash
curl http://localhost:8080/api/v1/notifications/1 -H "X-API-KEY: secret"
```

### `GET /api/v1/notifications/export`

Выгрузить все уведомления в аудит-лог (`AUDIT_LOG_PATH`) и вернуть количество экспортированных записей.

```bash
curl http://localhost:8080/api/v1/notifications/export -H "X-API-KEY: secret"
```

## Разработка

```bash
make build      # собрать бинарник в bin/
make test       # запустить тесты
make test-race  # запустить тесты с флагом -race
make lint       # прогнать golangci-lint
make clean      # удалить bin/
```

Список всех доступных команд:

```bash
make help
```

## Структура проекта

```
cmd/notifier/          точка входа, HTTP-хендлеры, middleware, аудит-лог
internal/config/       загрузка конфигурации из окружения
internal/notification/ доменная модель уведомления и валидация
internal/sender/       отправители уведомлений по каналам (console/email/telegram)
internal/store/        хранилище (Postgres и in-memory реализации)
```
