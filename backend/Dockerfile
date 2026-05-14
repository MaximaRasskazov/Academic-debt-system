# --- Build stage ---
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache deps separately for faster rebuilds
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/bin/server \
    ./cmd/server

# --- Runtime stage ---
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /app/bin/server /app/server

USER app

EXPOSE 8080

ENTRYPOINT ["/app/server"]
