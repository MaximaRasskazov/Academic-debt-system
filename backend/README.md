# Backend (Go)

Backend-сервис системы учёта академических задолженностей.

Для **быстрого запуска всего стека** используй корневой [Quick Start](../README.md#-quick-start) — `make setup` поднимет всё одной командой.

Этот документ — для тех, кто работает с Go-кодом локально, без docker.

## Стек

- Go 1.26
- chi v5 — HTTP-роутер
- pgx v5 + pgxpool — драйвер Postgres
- sqlc — генерация типобезопасных репозиториев из SQL
- goose — миграции (`backend/sql/migrations`) и dev-сиды (`backend/sql/seeds`)
- golang-jwt/jwt/v5 — JWT access-токены
- bcrypt — хеш паролей
- slog (stdlib) — структурированное логирование

## Структура

```
backend/
├── cmd/server/                # main: config + pool + router + graceful shutdown
├── internal/
│   ├── config/                # загрузка .env, валидация JWT_SECRET
│   ├── pgutil/                # interop google/uuid ↔ pgtype.*
│   ├── repo/
│   │   ├── queries/           # sqlc-generated, НЕ редактировать вручную
│   │   ├── store.go           # *Store + RunInTx (UoW)
│   │   └── store_test.go
│   ├── service/
│   │   ├── token/             # JWT-генерация, валидация, refresh-replay-detection
│   │   ├── auth/              # Register / Login / Refresh / Logout / Me
│   │   └── rbac/              # HasPermission, AssignRole с privilege-escalation guard
│   └── transport/http/
│       ├── dto/               # API-схемы (без password_hash и pgtype.*)
│       ├── handler/           # /api/auth/*
│       ├── middleware/        # Auth, RBAC, CORS, SecurityHeaders
│       └── router.go          # сборка chi.Router
├── sql/
│   ├── migrations/            # 00001-00010 — auth, RBAC, audit, changelog
│   └── seeds/                 # 00001_seed_dev_accounts — для SEED_DEV_ACCOUNTS=true
├── Dockerfile                 # multi-stage + goose CLI + entrypoint.sh
├── entrypoint.sh              # wait-for-postgres → goose up → seeds → exec server
├── Makefile                   # читает ../.env (единый источник конфига)
├── sqlc.yaml
└── go.mod
```

## Локальная разработка (без docker)

```bash
# Из корня репо: создай .env (это нужно сделать один раз)
make setup     # либо вручную: cp .env.example .env && подставить JWT_SECRET

# Подними только Postgres из docker, всё остальное локально:
docker compose up -d postgres mailpit

# Накати миграции с хоста (goose читает DB_HOST/DB_PORT из ../.env)
cd backend
make migrate-up

# Запусти сервер локально
make run
```

## Команды Makefile

```
make help                     # Список таргетов
make run                      # go run ./cmd/server
make build                    # сборка бинарника в bin/server
make test                     # unit-тесты (интеграционные скипаются)
make test-integration         # все тесты против поднятого postgres
make lint                     # gofmt + go vet
make tidy                     # go mod tidy
make migrate-up               # goose up на хостовой БД (DB_HOST из ../.env)
make migrate-down             # откатить последнюю миграцию
make migrate-status           # статус миграций
make migrate-create name=foo  # создать новую миграцию
make sqlc                     # перегенерировать internal/repo/queries из sql/queries
```

## Конфиг

См. корневой `.env.example`. Backend читает env-переменные через `internal/config`.

| Группа | Переменные |
|---|---|
| Application | `APP_ENV`, `APP_PORT`, `LOG_LEVEL` |
| Postgres | `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD` |
| JWT | `JWT_SECRET` (обязателен, ≥32 байт), `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL` |
| Cookie | `COOKIE_SECURE`, `COOKIE_DOMAIN`, `COOKIE_PATH`, `COOKIE_SAMESITE` |
| CORS | `ALLOWED_ORIGINS` (через запятую) |
| Dev | `SEED_DEV_ACCOUNTS` — катить ли сиды демо-аккаунтов |

## Текущий статус

Auth-foundation готов и смержен в `develop` (PR #10):

- [x] 10 миграций (users, sessions, refresh_tokens, RBAC, audit, change_logs + сиды RBAC)
- [x] sqlc-репозитории (44 типобезопасных метода)
- [x] Store + UoW поверх pgxpool
- [x] TokenService с JWT и one-use refresh + replay-detection
- [x] AuthService (Register/Login/Refresh/Logout/Me) с bcrypt
- [x] RBACService с защитой от privilege escalation
- [x] Middleware: Auth, RequirePermission, CORS, SecurityHeaders
- [x] Handler'ы /api/auth/* (5 endpoints)
- [x] DX: автомиграции в Docker, один `make setup`, дев-сиды
- [x] Coverage 76-85% по сервисным пакетам

В работе:
- [ ] Доменные сущности (debts, retakes, disciplines, teacher_requests)
- [ ] WebSocket-хаб для real-time уведомлений
- [ ] Email-сервис (gomail/v2 + Mailpit)
- [ ] Шедулер пересдач (`time.Ticker`)
- [ ] Синхронизация с внешней системой успеваемости
- [ ] Экспорт отчётов (XLSX/CSV)
- [ ] Swagger (swaggo/swag)
