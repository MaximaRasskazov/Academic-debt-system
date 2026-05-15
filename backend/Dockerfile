# --- Build stage ---
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache deps отдельно для быстрых rebuild'ов
COPY go.mod go.sum ./
RUN go mod download

# goose CLI собирается в этом же образе и попадает в runtime через COPY
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/bin/server \
    ./cmd/server

# --- Runtime stage ---
FROM alpine:3.20

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
RUN chmod +x /entrypoint.sh

USER app

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
