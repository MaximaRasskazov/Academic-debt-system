# syntax=docker/dockerfile:1.7

# --- Build stage ---
FROM golang:1.26.4-alpine3.23@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder

WORKDIR /app

# Cache deps отдельно для быстрых rebuild'ов
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# goose CLI собирается в этом же образе и попадает в runtime через COPY
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/pressly/goose/v3/cmd/goose@v3.21.1

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-s -w" \
        -o /app/bin/server \
        ./cmd/server

# --- Runtime stage ---
FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc

# postgresql-client нужен для pg_isready в entrypoint.sh.
# ca-certificates — для исходящих https-запросов (синхронизация с
# внешней системой успеваемости в будущем). tzdata — корректное время в логах.
RUN apk --no-cache add ca-certificates tzdata postgresql-client && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /app/bin/server /app/server
COPY --from=builder /go/bin/goose /usr/local/bin/goose
COPY sql/migrations /app/sql/migrations
COPY sql/seeds /app/sql/seeds
COPY entrypoint.sh /entrypoint.sh
# Windows-разработчики коммитят .sh с CRLF — alpine /bin/sh падает с
# "exec format error" на \r в shebang. Срезаем \r и даём +x.
# Это безопасно для LF-файлов: sed просто ничего не найдёт.
RUN sed -i 's/\r$//' /entrypoint.sh && chmod +x /entrypoint.sh

USER app

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
