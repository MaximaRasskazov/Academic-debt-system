# Backend (Go)

Backend сервис системы учёта академических задолженностей.

## Стек

- Go 1.26
- chi v5 — HTTP-роутер
- pgx v5 — PostgreSQL-драйвер
- sqlc — генерация типобезопасного Go-кода из SQL
- goose — миграции БД
- slog (stdlib) — структурированное логирование

## Требования к окружению

- Go 1.26+
- PostgreSQL 17 (или Docker)
- goose CLI: `go install github.com/pressly/goose/v3/cmd/goose@latest`
- sqlc CLI: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`

## Быстрый старт

```bash
# 1. Поднять Postgres + Mailpit (из корня репо)
docker compose up -d postgres mailpit

# 2. Скопировать .env (из backend/)
cp .env.example .env

# 3. Применить миграции (когда появятся)
make migrate-up

# 4. Запустить сервер
make run
```

Сервер поднимется на `http://localhost:8080`.

Проверка живости:
```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

## Структура проекта

```
backend/
├── cmd/server/             # Точка входа
├── internal/
│   ├── config/             # Загрузка конфигурации из env
│   ├── domain/             # Бизнес-сущности (будут добавлены)
│   ├── repo/               # Репозитории + sqlc-generated
│   ├── service/            # Бизнес-логика
│   └── transport/http/     # HTTP-слой (handler/middleware/dto)
├── sql/
│   ├── migrations/         # goose-миграции (.sql)
│   └── queries/            # sqlc-запросы (.sql)
├── Dockerfile
├── Makefile
├── go.mod
└── sqlc.yaml
```

## Команды Makefile

```
make help            # Список команд
make run             # Локальный запуск
make build           # Сборка бинарника
make test            # Тесты с race detector
make lint            # gofmt + go vet
make tidy            # go mod tidy
make migrate-up      # Применить миграции
make migrate-down    # Откатить последнюю миграцию
make migrate-status  # Статус миграций
make migrate-create name=foo  # Создать новую миграцию
make sqlc            # Перегенерировать код из sql/queries
make docker-up       # Поднять postgres + mailpit + backend
make docker-down     # Остановить всё
```

## Переменные окружения

См. `.env.example`. Основные:

| Переменная | Описание | По умолчанию |
|---|---|---|
| `APP_ENV` | Окружение (development/production) | `development` |
| `APP_PORT` | Порт HTTP-сервера | `8080` |
| `LOG_LEVEL` | Уровень логов | `info` |
| `DB_HOST` | Хост Postgres | `127.0.0.1` |
| `DB_PORT` | Порт Postgres | `5432` |
| `DB_NAME` | Имя БД | `academic_debts` |
| `DB_USER` | Пользователь БД | `academic` |
| `DB_PASSWORD` | Пароль | — |

## Текущий статус

Foundation-скелет. Готово:
- [x] Структура папок
- [x] Конфигурация из env
- [x] Подключение к PostgreSQL (pgxpool)
- [x] Базовые middleware (RequestID, RealIP, Recoverer, Timeout)
- [x] `/health` endpoint
- [x] Graceful shutdown
- [x] Dockerfile (multi-stage)
- [x] docker-compose с postgres + mailpit
- [x] Makefile с базовыми командами
- [x] sqlc.yaml + папки под миграции/запросы

Следующие PR:
- [ ] Auth (users + JWT + register/login/refresh)
- [ ] RBAC (roles + permissions + middleware)
- [ ] Audit log + change history
- [ ] Доменные сущности (debts, retakes, disciplines)
- [ ] WebSocket для real-time уведомлений
- [ ] Email-сервис
- [ ] Шедулер (переход пересдач в "Завершена")
- [ ] Синхронизация с внешней системой
- [ ] Экспорт отчётов (XLSX/CSV)
- [ ] Swagger
