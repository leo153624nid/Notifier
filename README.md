# Notifier

Небольшой HTTP-сервис на Go для приёма и рассылки уведомлений по разным каналам (консоль, email, telegram — в текущей реализации это заглушки). Уведомления сохраняются в PostgreSQL, отправка выполняется асинхронно, а любой запрос на экспорт пишет аудит-лог на диск.

## Возможности

- REST API для создания, получения и листинга уведомлений
- gRPC API (`internal/transport/grpc`) — тот же сценарий создания уведомления, но для межсервисного вызова: им пользуется auth-сервис, чтобы отправить приветственное письмо при регистрации (см. «Интеграция с auth (gRPC)»)
- Kafka-консьюмер (`internal/transport/kafka`) — слушает событие об успешном логине от auth-сервиса и создаёт уведомление о новом входе (см. «Интеграция с auth (Kafka)»)
- Асинхронная отправка через набор `Sender` (`console`, `email`, `telegram`)
- Хранение в PostgreSQL, схема БД версионируется миграциями (`cmd/migrate`)
- Аутентификация по JWT (заголовок `Authorization: Bearer <token>`), токен выдаёт отдельный auth-сервис (`services/auth`) и подписывает общим `JWT_SECRET`
- Rate limiting по IP (token bucket, 10 запросов/сек, burst 20)
- Structured logging (`log/slog`, JSON), request ID на каждый запрос
- Graceful shutdown с ожиданием фоновых горутин отправки и остановкой обоих серверов (HTTP и gRPC)
- Экспорт уведомлений в аудит-лог (`audit.log`)
- Docker / docker compose для локального запуска вместе с Postgres

## Стек

- Go 1.27, монорепозиторий из трёх модулей, объединённых [Go workspace](https://go.dev/ref/mod#workspaces) (`go.work`):
  - `notifier` — этот сервис (модуль в корне репозитория)
  - `services/auth` (модуль `authservice`) — регистрация, логин, выдача JWT
  - `contracts` — общие контракты notifier ↔ auth: gRPC (`.proto` + сгенерированный код) и Kafka-события (`events/`), отдельный модуль, чтобы auth не зависел от всего модуля `notifier` целиком
- PostgreSQL (драйвер `jackc/pgx/v5`)
- gRPC (`google.golang.org/grpc`) и Protocol Buffers — межсервисный вызов auth → notifier
- Kafka (`github.com/segmentio/kafka-go`) — событие об успешном логине, auth → notifier
- `golang.org/x/time/rate` — rate limiting

## Быстрый старт

### Через Docker Compose (рекомендуется)

```bash
cp .env.example .env
make docker-up
```

Поднимутся `postgres`, разовый `migrate` (применяет миграции схемы и завершается), `kafka` и `notifier`, который стартует только после успешного завершения `migrate` (`depends_on: condition: service_completed_successfully`) и готовности `kafka`, а также зеркальный набор для auth-сервиса — `postgres_auth`, `auth_migrate`, `auth`. Notifier будет доступен на `http://localhost:8080` (HTTP) и `localhost:9090` (gRPC), auth — на `http://localhost:8081`. Оба сервиса используют общий `JWT_SECRET`: auth подписывает им токены, notifier проверяет подпись. Между `auth` и `notifier` намеренно нет `depends_on` — недоступность notifier в момент старта auth не критична, см. «Интеграция с auth (gRPC)»; оба сервиса, однако, ждут готовности `kafka`, так как продюсер и консьюмер устанавливают соединение с брокером при старте.

> Образы `auth`/`auth_migrate` собираются из **корня репозитория** (`context: .` в `docker-compose.yml`), а не из `services/auth` — потому что `authservice` импортирует пакет из модуля `contracts` через `go.work`, а Go workspace требует, чтобы все перечисленные в нём модули физически лежали рядом на диске в момент сборки. Если правите `services/auth/Dockerfile`, помните, что пути внутри него — относительно корня репозитория, а не `services/auth`.

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
| `GRPC_PORT` | Порт gRPC-сервера (см. «Интеграция с auth (gRPC)») | `9090` |
| `JWT_SECRET` | Общий с auth-сервисом секрет для проверки подписи JWT (обязателен) | — |
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
| `KAFKA_BROKERS` | Адреса брокеров Kafka через запятую (см. «Интеграция с auth (Kafka)») | `kafka:9092` |
| `KAFKA_LOGIN_TOPIC` | Топик, из которого notifier читает событие логина; переопределяет дефолт из `authevents.TopicUserLoggedIn` | `auth.user.logged_in` |
| `KAFKA_LOGIN_GROUP_ID` | Consumer group для `KAFKA_LOGIN_TOPIC`; переопределяет дефолт из `authevents.GroupIDLoginConsumer` | `notifier-login-consumer` |
| `KAFKA_DLQ_TOPIC` | Dead-letter топик для необработанных сообщений (см. «Dead-letter топик» ниже); переопределяет дефолт из `authevents.TopicNotifierDLQ` | `notifier.dlq` |
| `APP_HOST_PORT` | Порт сервиса, публикуемый на хосте (только docker compose) | `8080` |
| `NOTIFIER_GRPC_PORT` | gRPC-порт внутри docker-сети — им же auth находит notifier как `notifier:$NOTIFIER_GRPC_PORT` (только docker compose) | `9090` |
| `NOTIFIER_GRPC_HOST_PORT` | gRPC-порт, публикуемый на хосте — удобно для проверки через `grpcurl` (только docker compose) | `9090` |
| `POSTGRES_HOST_PORT` | Порт Postgres, публикуемый на хосте (только docker compose) | `5432` |
| `REDIS_HOST_PORT` | Порт Redis, публикуемый на хосте (только docker compose) | `6379` |
| `KAFKA_HOST_PORT` | Порт Kafka, публикуемый на хосте (только docker compose) | `9092` |

Внутри docker-сети адрес БД, Redis и Kafka всегда `postgres:5432`, `redis:6379` и `kafka:9092` соответственно — это топология compose, а не настраиваемый параметр. Значение по умолчанию `KAFKA_BROKERS=kafka:9092` рассчитано именно на запуск внутри docker-сети; для `make run` на хосте задайте в `.env` `KAFKA_BROKERS=localhost:9092` (см. `.env.example`).

## Интеграция с auth (gRPC)

При успешной регистрации нового пользователя auth-сервис по gRPC вызывает `NotificationService/CreateNotification` у notifier, чтобы отправить приветственное письмо. Notifier обслуживает этот вызов на отдельном порту (`GRPC_PORT`, по умолчанию `9090`) параллельно с HTTP — оба сервера запускаются и останавливаются независимо в `cmd/notifier/main.go`.

**Контракт.** `.proto`-файл и сгенерированный Go-код лежат в `contracts/` — это отдельный Go-модуль (не пакет внутри `notifier`), подключённый к обоим сервисам через `go.work`. Так `services/auth` зависит только от контракта, а не от всего модуля `notifier`.

```
contracts/
  proto/notifications/v1/notification.proto   # исходный контракт
  gen/notifications/v1/                       # сгенерированный код (protoc-gen-go, protoc-gen-go-grpc)
```

Перегенерировать код после правки `.proto`: `make proto` (требует `protoc`, `protoc-gen-go` и `protoc-gen-go-grpc` в `PATH` — см. `Makefile`, цель `proto`).

**Реализация:**

| Сторона | Код |
|---|---|
| gRPC-сервер (notifier) | `internal/transport/grpc/server.go` — тонкий адаптер, переводит вызов в `service.NotificationService.Create`, как HTTP-хендлер `POST /api/v1/notifications` |
| gRPC-клиент (auth) | `services/auth/internal/client/notifierclient/` — реализует порт `service.NotifierClient` (`services/auth/internal/service/notifier_client.go`), поэтому `AuthService` не знает о деталях транспорта |
| Точка вызова | `AuthService.Register` (`services/auth/internal/service/auth_service.go`) — после успешного создания пользователя |

**Ключевое архитектурное решение: отправка письма не блокирует и не может провалить регистрацию.** `Register` запускает вызов `Notify` в фоновой горутине с собственным контекстом и таймаутом (не привязанным к контексту HTTP-запроса, который может завершиться раньше). Если notifier недоступен или вернул ошибку — она только логируется auth-сервисом; клиент всё равно получает `201 Created`. Это осознанный trade-off: письмо — сайд-эффект, а не часть транзакции регистрации. Оборотная сторона — если процесс auth убьют в узкое окно между сохранением пользователя и завершением этой горутины, письмо будет потеряно молча; при штатном завершении процесс дожидается фоновых вызовов (`AuthService.Wait()` в `cmd/auth/main.go`, как и `NotificationService.Wait()` у notifier).

**Проверить вручную** (без auth, напрямую через [grpcurl](https://github.com/fullstorydev/grpcurl)):

```bash
grpcurl -plaintext -d '{"recipient":"test@mail.com","subject":"hi","body":"hi","channel":"email"}' \
  localhost:9090 notifications.v1.NotificationService/CreateNotification
```

## Интеграция с auth (Kafka)

При успешном логине auth-сервис публикует в Kafka событие `auth.user.logged_in`, а notifier асинхронно читает его и создаёт уведомление о новом входе в аккаунт. В отличие от интеграции при регистрации (прямой gRPC-вызов, см. выше), здесь сервисы не знают друг о друге напрямую — оба общаются только с брокером Kafka, поэтому auth может публиковать событие, даже если notifier в этот момент не работает: сообщение просто подождёт в топике.

**Контракт.** Топик и структура события — Go-пакет `contracts/events/auth/v1` (не protobuf: событие сериализуется в JSON, что проще читать при отладке через `kafka-ui`/консольные консьюмеры и не требует Schema Registry для одного потребителя):

```go
const TopicUserLoggedIn = "auth.user.logged_in"
const TopicNotifierDLQ = "notifier.dlq"
const GroupIDLoginConsumer = "notifier-login-consumer"

type UserLoggedIn struct {
    EventID    uuid.UUID `json:"event_id"`
    UserID     uuid.UUID `json:"user_id"`
    Email      string    `json:"email"`
    OccurredAt time.Time `json:"occurred_at"`
}
```

**Реализация:**

| Сторона | Код |
|---|---|
| Kafka-продюсер (auth) | `services/auth/internal/client/kafkaproducer/` — реализует порт `service.EventPublisher` (`services/auth/internal/service/event_publisher.go`), поэтому `AuthService` не знает о деталях транспорта |
| Kafka-консьюмер (notifier) | `internal/transport/kafka/login_consumer.go` — читает топик через consumer group `notifier-login-consumer`, десериализует событие и вызывает `service.NotificationService.Create`, как и HTTP/gRPC-хендлеры |
| Точка вызова | `AuthService.Login` (`services/auth/internal/service/auth_service.go`) — после выпуска пары токенов |

Сообщение партиционируется по `UserID` (`kafka.Hash` балансировщик в продюсере) — это гарантирует, что события одного пользователя обрабатываются в том порядке, в котором были опубликованы.

**Consumer group.** `GroupID: "notifier-login-consumer"` в `kafka.ReaderConfig` — ключевой момент: Kafka сохраняет offset (позицию последнего прочитанного сообщения) для этой группы. Если notifier перезапустится, он продолжит чтение с того места, где остановился, а не прочитает всё заново и не потеряет события, пришедшие, пока сервис был недоступен. При масштабировании notifier до нескольких реплик Kafka автоматически распределит партиции топика между ними так, что каждое сообщение обработает только одна реплика.

**Тот же trade-off, что и с письмом при регистрации.** Публикация события в `AuthService.publishLoginEvent` выполняется в фоновой горутине со своим контекстом и таймаутом, не блокируя ответ `Login`; ошибка публикации только логируется. Симметрично, консьюмер в notifier не роняет процесс при ошибке разбора одного сообщения (`json.Unmarshal`) или при сбое `NotificationService.Create` — он не даёт одному «плохому» событию заблокировать партицию (см. «Dead-letter топик» ниже).

### Dead-letter топик

Если `handleMessage` не смог обработать сообщение (невалидный JSON или ошибка `NotificationService.CreateIdempotent`, отличная от «уже обработано» — она не считается ошибкой и просто коммитится), сообщение не отбрасывается и не ретраится вечно на месте, а публикуется как есть в топик `notifier.dlq` (константа `authevents.TopicNotifierDLQ`, `contracts/events/auth/v1/login.go`) с заголовками `error` (текст ошибки) и `source_topic` (имя исходного топика) — после этого оффсет исходного топика коммитится, и консьюмер идёт дальше.

Это разделяет два разных отказа:

- **Сообщение необрабатываемо** (poison pill: битый JSON, доменная ошибка) — уходит в DLQ, оффсет двигается, партиция не блокируется.
- **Сама Kafka недоступна** (не удалось записать в DLQ) — оффсет **не** коммитится, `Run()` переходит к следующей итерации `FetchMessage` и на следующем цикле получит то же сообщение снова — данные не теряются молча.

**Один DLQ-топик на всё приложение**, а не по одному на консьюмер: сейчас есть только `LoginConsumer`, но и при появлении новых консьюмеров они будут писать сюда же, различая источник через заголовок `source_topic`. Это меньше топиков и consumer group'ов для администрирования; разбор сообщения начинается с чтения этого заголовка, а не с выбора, в какой из множества DLQ-топиков смотреть.

**Создание топиков.** `auto.create.topics.enable` у брокера выключен (`docker-compose.yml`, сервис `kafka`) — без этого брокер создал бы отсутствующий топик сам при первом обращении, с дефолтными параметрами (обычно 1 партиция), что не даёт никаких гарантий по числу партиций/репликации. Вместо этого топики создаёт отдельный сервис `kafka-init`:

```
cmd/kafkatopics/        печатает "<topic>:<partitions>:<replication-factor>" по одной строке на топик;
                        имена топиков берёт из internal/config (единственный источник —
                        она сама берёт дефолты из contracts/events/auth/v1)
scripts/kafka-init.sh   вызывает /kafkatopics и создаёт то, что он вывел, через kafka-topics.sh
Dockerfile.kafkainit    собирает kafkatopics и кладёт его поверх образа apache/kafka
                        вместе со scripts/kafka-init.sh
```

Список топиков не продублирован ни в YAML, ни в shell-скрипте — единственное место, где перечислены имена, это Go-код (`contracts` → `internal/config` → `cmd/kafkatopics`). `notifier` и `auth` в `docker-compose.yml` стартуют только после `kafka-init: service_completed_successfully`, поэтому оба топика (`auth.user.logged_in` и `notifier.dlq`) гарантированно существуют до первого подключения консьюмера/продюсера.

Читать сообщения из DLQ и разбираться с ними — отдельная задача, пока не реализованная (см. план в PR/issue, если завели).

**Проверить вручную:**

```bash
# залогиниться под существующим пользователем
curl -s -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "supersecret"}'

# посмотреть, что notifier создал уведомление о новом входе
curl http://localhost:8080/api/v1/notifications -H "Authorization: Bearer $TOKEN"
```

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

Все эндпоинты, кроме `/health`, требуют заголовок `Authorization: Bearer <token>` с JWT, выданным auth-сервисом (`POST /api/v1/auth/login`, см. `services/auth`), и подчиняются rate limiting по IP.

### `GET /health`

Проверка живости сервиса и доступности БД. Без аутентификации.

```bash
curl http://localhost:8080/health
```

### `POST /api/v1/notifications`

Создать уведомление. Отправка выполняется асинхронно; ответ возвращается сразу со статусом `pending`.

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $TOKEN" \
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
curl http://localhost:8080/api/v1/notifications -H "Authorization: Bearer $TOKEN"
```

### `GET /api/v1/notifications/{id}`

Получить уведомление по ID.

```bash
curl http://localhost:8080/api/v1/notifications/1 -H "Authorization: Bearer $TOKEN"
```

### `GET /api/v1/notifications/export`

Выгрузить все уведомления в аудит-лог (`AUDIT_LOG_PATH`) и вернуть количество экспортированных записей.

```bash
curl http://localhost:8080/api/v1/notifications/export -H "Authorization: Bearer $TOKEN"
```

`$TOKEN` — значение `access_token`, полученное от auth-сервиса:

```bash
TOKEN=$(curl -s -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "supersecret"}' | jq -r .access_token)
```

## Разработка

```bash
make build      # собрать бинарник в bin/
make test       # запустить тесты
make test-race  # запустить тесты с флагом -race
make lint       # прогнать golangci-lint
make clean      # удалить bin/
make proto      # перегенерировать gRPC-код из contracts/proto после правки .proto
```

Список всех доступных команд:

```bash
make help
```

## Структура проекта

Репозиторий — монорепо из трёх Go-модулей, объединённых `go.work` (см. «Стек»):

```
.                 этот сервис — корень репозитория и есть модуль notifier
services/auth/    auth-сервис — модуль authservice, регистрация/логин/JWT
contracts/        общие контракты notifier ↔ auth (gRPC и Kafka-события) — модуль contracts
```

Каждый модуль собирается и тестируется независимо (`go build`/`go test` внутри своей директории), но `go.work` позволяет `authservice` импортировать `contracts` напрямую, без публикации в отдельный реестр модулей — см. «Интеграция с auth (gRPC)» и «Интеграция с auth (Kafka)».

Внутри `notifier` — слоистая архитектура: транспорт знает про сервис, сервис — про репозиторий и домен, домен не знает ни о ком.

```
cmd/notifier/              composition root: конфигурация, wiring зависимостей, запуск/graceful shutdown
cmd/kafkatopics/           печатает список Kafka-топиков notifier для kafka-init (см. «Dead-letter топик»)
internal/transport/http/   HTTP-транспорт: роутинг, middleware, хендлеры, DTO запросов/ответов
internal/transport/grpc/   gRPC-транспорт: сервер NotificationService для межсервисных вызовов (см. auth)
internal/transport/kafka/  Kafka-консьюмер: чтение события auth.user.logged_in (см. auth)
internal/service/          бизнес-логика: валидация, оркестрация отправки, экспорт, health-check
internal/repository/       доступ к данным (Postgres и in-memory реализации интерфейса Repository)
internal/notification/     доменная модель уведомления и валидация
internal/sender/           отправители уведомлений по каналам (console/email/telegram)
internal/audit/            запись аудит-лога на диск
internal/config/           загрузка конфигурации из окружения
scripts/kafka-init.sh      создаёт Kafka-топики notifier при старте docker compose
```
