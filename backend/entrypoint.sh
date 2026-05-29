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

# Dev-сиды: admin + dean (без них в свежей БД некому залогиниться,
# а в эмуляторе деканата таких ролей нет). Студенты и преподаватели
# приходят целиком из эмулятора через sync.Service.
#
# Сиды выполняются прямо через psql, без goose-чекпойнта: скрипт
# идемпотентен (ON CONFLICT (id) DO NOTHING), и нам не нужна отдельная
# таблица версий. Раньше пробовали `GOOSE_TABLE=goose_seed_version
# goose ... up` — в goose v3.21+ эту env-переменную не уважают и
# сиды молча не накатывались (goose проверял общую goose_db_version
# и видел "current version: 22, no migrations to run").
#
# `+goose Up/StatementBegin/StatementEnd` для psql — обычные комментарии,
# а вот секцию `+goose Down` нужно отрезать, иначе psql её тоже выполнит
# и удалит то, что только что вставил.
if [ "${SEED_DEV_ACCOUNTS:-false}" = "true" ]; then
    echo "[entrypoint] seeding dev accounts (SEED_DEV_ACCOUNTS=true)..."
    SEED_UP=$(sed '/-- +goose Down/,$d' /app/sql/seeds/00001_seed_dev_accounts.sql)
    echo "${SEED_UP}" | PGPASSWORD="${DB_PASSWORD}" psql \
        -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" \
        -v ON_ERROR_STOP=1 \
        --quiet
fi

echo "[entrypoint] starting server..."
exec /app/server
