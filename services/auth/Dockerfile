FROM golang:1.27.0 AS builder

WORKDIR /build

# Билд-контекст — корень репозитория (см. docker-compose.yml: build.context),
# а не services/auth: authservice импортирует contracts/ (сгенерированный
# gRPC-код) через go.work, а go.work требует, чтобы все перечисленные в нём
# модули (., ./services/auth, ./contracts) физически лежали рядом на диске.
COPY . .

WORKDIR /build/services/auth
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /auth ./cmd/auth
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /migrate ./cmd/migrate

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /auth /auth
COPY --from=builder /migrate /migrate

# непривилегированный пользователь: в scratch нет /etc/passwd,
# но числовой UID/GID ядру достаточен и без записи в passwd
USER 65532:65532

EXPOSE 8081
ENTRYPOINT [ "/auth" ]