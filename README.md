# Notifier

Небольшой HTTP-сервис на Go для приёма и рассылки уведомлений по разным каналам (консоль, email, telegram — в текущей реализации это заглушки). Уведомления сохраняются в PostgreSQL, отправка выполняется асинхронно, а любой запрос на экспорт пишет аудит-лог на диск.

## Возможности

- REST API для создания, получения и листинга уведомлений
- Асинхронная отправка через набор `Sender` (`console`, `email`, `telegram`)
- Хранение в PostgreSQL, схема БД версионируется миграциями (`cmd/migrate`)
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

Поднимутся три контейнера: `postgres`, разовый `migrate` (применяет миграции схемы и завершается) и `notifier`, который стартует только после его успешного завершения (`depends_on: condition: service_completed_successfully`). Сервис будет доступен на `http://localhost:8080`.

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
make migrate-up
make run
```

`make migrate-up` применит миграции схемы к БД, `make run` соберёт бинарник в `bin/notifier` и запустит его, подхватив переменные из `.env`. Приложение больше не создаёт и не меняет таблицы самостоятельно — без применённых миграций оно упадёт на первом же запросе к БД.

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
| `RD_HOST` | Хост Redis | `localhost` |
| `RD_PORT` | Порт Redis | `6379` |
| `RD_PASSWORD` | Пароль Redis | `notifier` |
| `APP_HOST_PORT` | Порт сервиса, публикуемый на хосте (только docker compose) | `8080` |
| `POSTGRES_HOST_PORT` | Порт Postgres, публикуемый на хосте (только docker compose) | `5432` |
| `REDIS_HOST_PORT` | Порт Redis, публикуемый на хосте (только docker compose) | `6379` |

Внутри docker-сети адрес БД и Redis всегда `postgres:5432` и `redis:6379` соответственно — это топология compose, а не настраиваемый параметр.

## Кэширование (Redis)

`GetById`/`GetList` в `internal/repository/cachedRepository.go` реализуют cache-aside поверх Postgres: сначала читают Redis, при промахе идут в БД и кладут результат в кэш с TTL 30 секунд; `Save`/`UpdateStatus` инвалидируют кэш затронутого уведомления.

Эксплуатационные решения, зафиксированные в `docker-compose.yml`:

- **Redis — не источник истины.** Вся логика читает и пишет в Postgres напрямую при недоступном кэше (fail-open): ошибки Redis только логируются, не прерывают запрос. При падении Redis сервис продолжает работать медленнее (без кэша), а не отказывает.
- **Персистентность отключена** (`--save "" --appendonly no`, без volume под `/data`) — раз ни один ключ не единственный источник данных, RDB/AOF только тратят диск и I/O; после рестарта контейнера кэш просто прогреется заново по мере запросов.
- **`maxmemory 256mb` + `allkeys-lru`** — инстанс не может расти без предела; при нехватке памяти вытесняются наименее используемые ключи вместо OOM.
- **Пароль обязателен** (`--requirepass`), порт наружу в проде публиковать не рекомендуется — см. комментарий в `.env.prod.example`.
- **Прогрев кэша не делается намеренно** — короткий TTL и cache-aside означают, что кэш самопрогревается первыми же запросами; отдельный warmup-шаг добавил бы сложность без заметной пользы при текущей нагрузке.
- **Один инстанс Redis, без Sentinel/Cluster** — сознательный выбор под текущий масштаб; раз Redis не источник истины, его недоступность не требует HA — сервис просто продолжит работать через Postgres.

## Миграции БД

Схема БД версионируется SQL-файлами в `internal/migrations/` (embed в бинарник, `golang-migrate`) и применяется отдельным инструментом `cmd/migrate`, а не самим `notifier` — так безопаснее при нескольких репликах сервиса и позволяет откатывать схему независимо от релизов кода. Каждая миграция — пара файлов `<версия>_<имя>.up.sql` / `<версия>_<имя>.down.sql`; применённые версии golang-migrate хранит в служебной таблице `schema_migrations` (колонки `version`, `dirty`).

```bash
make migrate-up       # применить все непринятые миграции
make migrate-down     # откатить последнюю применённую миграцию
make migrate-version  # показать текущую версию схемы
make migrate-create name=add_foo  # создать новую пару файлов миграции
```

В docker compose это отдельный сервис `migrate`, который применяет миграции и завершается до старта `notifier` (см. выше). DSN для миграций берётся из тех же `PG_*` переменных, что и для самого приложения.

Если `migrate up` упал с `Dirty database version N`, значит предыдущий прогон миграции N оборвался на середине (например, БД разорвала соединение) — прежде чем повторить `up`, нужно вручную проверить фактическое состояние схемы и, если всё применилось корректно, сбросить флаг: `bin/migrate force N` (либо откатить миграцию руками и попробовать снова).

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

Слоистая архитектура: транспорт знает про сервис, сервис — про репозиторий и домен, домен не знает ни о ком.

```
cmd/notifier/              composition root: конфигурация, wiring зависимостей, запуск/graceful shutdown
internal/transport/http/   HTTP-транспорт: роутинг, middleware, хендлеры, DTO запросов/ответов
internal/service/          бизнес-логика: валидация, оркестрация отправки, экспорт, health-check
internal/repository/       доступ к данным (Postgres и in-memory реализации интерфейса Repository)
internal/notification/     доменная модель уведомления и валидация
internal/sender/           отправители уведомлений по каналам (console/email/telegram)
internal/audit/            запись аудит-лога на диск
internal/config/           загрузка конфигурации из окружения
```
