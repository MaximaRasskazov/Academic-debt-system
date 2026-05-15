#!/bin/sh
# Entrypoint бэкенда. Запускается при старте контейнера.
#
# Шаги:
#   1. Ждём готовности Postgres (max 60 секунд). depends_on c healthcheck
#      в docker-compose уже даёт первую гарантию, но дополнительный wait
#      делает образ переносимым (k8s, ручной docker run без compose).
#   2. Прогоняем goose-миграции (idempotently: если current_version
#      совпадает с последней — goose сразу выходит).
#   3. Опционально накатываем dev-сиды (SEED_DEV_ACCOUNTS=true).
#   4. exec'аем основной бинарь сервера.
#
# Любой шаг вернувший ненулевой код останавливает запуск — контейнер
# не должен стартовать со сломанной БД.

set -e

: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"

DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

echo "[entrypoint] waiting for postgres at ${DB_HOST}:${DB_PORT}..."
for i in $(seq 1 60); do
    if pg_isready -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" >/dev/null 2>&1; then
        echo "[entrypoint] postgres is ready (after ${i}s)"
        break
    fi
    if [ "${i}" = "60" ]; then
        echo "[entrypoint] postgres did not become ready in 60s, aborting" >&2
        exit 1
    fi
    sleep 1
done

echo "[entrypoint] running migrations..."
goose -dir /app/sql/migrations postgres "${DB_URL}" up

# Dev-сиды: 1 админ + 1 деканат + 2 преподавателя + 3 студента.
# Запускаются отдельно от основной серии миграций, чтобы их легко было
# не катить на проде (просто SEED_DEV_ACCOUNTS=false).
if [ "${SEED_DEV_ACCOUNTS:-false}" = "true" ]; then
    echo "[entrypoint] seeding dev accounts (SEED_DEV_ACCOUNTS=true)..."
    # GOOSE_TABLE=goose_seed_version — отдельная таблица версий,
    # чтобы seed-цепочка не пересекалась с основной sql/migrations.
    GOOSE_TABLE=goose_seed_version goose -dir /app/sql/seeds postgres "${DB_URL}" up
fi

echo "[entrypoint] starting server..."
exec /app/server
