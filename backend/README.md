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
├── cmd/server/                # main: config + pool + сборка сервисов + router + graceful shutdown
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
│   │   ├── rbac/              # HasPermission, AssignRole с privilege-escalation guard
│   │   ├── audit/             # AuditService — события безопасности в audit_log
│   │   ├── changelog/         # ChangeLogService — before/after JSONB для истории и undo
│   │   ├── discipline/        # CRUD дисциплин + привязка teacher/student
│   │   └── debt/              # академические долги (создание, оценка, отмена, выборки по ролям)
│   └── transport/http/
│       ├── dto/               # API-схемы (auth, discipline, debt) без password_hash и pgtype.*
│       ├── handler/           # /api/auth/*, /api/disciplines/*, /api/debts/*, /api/me/disciplines/*
│       ├── middleware/        # Auth, RequirePermission, CORS, SecurityHeaders
│       └── router.go          # сборка chi.Router + mountDisciplines/mountDebts
├── sql/
│   ├── migrations/            # 00001-00019 — auth, RBAC, audit, changelog + доменные таблицы
│   └── seeds/                 # 00001_seed_dev_accounts — для SEED_DEV_ACCOUNTS=true
├── Dockerfile                 # multi-stage + goose CLI + entrypoint.sh
├── entrypoint.sh              # wait-for-postgres → goose up → seeds → exec server
├── Makefile                   # читает ../.env (единый источник конфига)
├── sqlc.yaml
└── go.mod
```

## API endpoints

Все защищённые endpoint'ы требуют `Authorization: Bearer <jwt>` и проходят RBAC-проверки на нужный permission. Полный список permission'ов — в миграции `00010_seed_rbac.sql`.

### Authentication
| Метод | Путь | Permission | Назначение |
|---|---|---|---|
| POST | `/api/auth/register` | — | Регистрация (выдаётся роль `student`) |
| POST | `/api/auth/login` | — | Логин по email + password |
| POST | `/api/auth/refresh` | — | Обмен refresh-cookie на новый access |
| POST | `/api/auth/logout` | Auth | Закрытие сессии |
| GET | `/api/auth/me` | Auth | Профиль + roles + permissions |

### Disciplines
| Метод | Путь | Permission | Назначение |
|---|---|---|---|
| GET | `/api/disciplines` | Auth | Список справочника (доступен всем авторизованным) |
| GET | `/api/disciplines/:id` | Auth | Одна дисциплина |
| GET | `/api/disciplines/:id/teachers` | Auth | Преподаватели дисциплины |
| GET | `/api/disciplines/:id/students` | Auth | Студенты дисциплины |
| POST | `/api/disciplines` | `disciplines.create` | Создать (admin/dean) |
| PATCH | `/api/disciplines/:id` | `disciplines.update` | Обновить (admin/dean) |
| DELETE | `/api/disciplines/:id` | `disciplines.delete` | Soft-delete (admin) |
| POST | `/api/disciplines/:id/restore` | `disciplines.delete` | Восстановить (admin) |
| POST | `/api/disciplines/:id/teachers` | `disciplines.update` | Привязать преподавателя |
| DELETE | `/api/disciplines/:id/teachers/:user_id` | `disciplines.update` | Снять |
| POST | `/api/disciplines/:id/students` | `disciplines.update` | Привязать студента (с academic_year+semester) |
| DELETE | `/api/disciplines/:id/students/:user_id` | `disciplines.update` | Снять |

### Дисциплины пользователя
| Метод | Путь | Permission | Назначение |
|---|---|---|---|
| GET | `/api/me/disciplines/student` | Auth | На каких дисциплинах я учусь |
| GET | `/api/me/disciplines/teacher` | Auth | Какие веду |
| GET | `/api/users/:id/disciplines/student` | `users.view` | Просмотр чужих (admin/dean) |
| GET | `/api/users/:id/disciplines/teacher` | `users.view` | Просмотр чужих (admin/dean) |

### Debts (академические долги)
| Метод | Путь | Permission | Назначение |
|---|---|---|---|
| GET | `/api/debts/my` | `debts.view.own` | Свои долги (студент) |
| GET | `/api/debts/by-discipline` | `debts.view.by_discipline` | Долги по моим дисциплинам (преподаватель) |
| GET | `/api/debts` | `debts.view.all` | Все долги (dean/admin) |
| GET | `/api/debts/summary` | `debts.view.all` | Сводка по дисциплинам (open/graded count) |
| GET | `/api/debts/:id` | `debts.view.all` | Один долг (dean/admin) |
| POST | `/api/debts` | `debts.create` | Создать долг (преподаватель, проверка teacher_disciplines + student_disciplines) |
| PATCH | `/api/debts/:id/grade` | `debts.update` | Выставить оценку 2..5 (open → graded) |
| PATCH | `/api/debts/:id/cancel` | `debts.delete` | Отменить долг (open → cancelled) |

### Системные
| Метод | Путь | Назначение |
|---|---|---|
| GET | `/health` | Liveness/readiness check (проверяет pool.Ping) |

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

## Демо-аккаунты для разработки

При `SEED_DEV_ACCOUNTS=true` (по умолчанию в `.env.example`) после `make setup` доступны 7 аккаунтов с паролем `password`:

| Email | Роль |
|---|---|
| `admin@academic.local` | admin |
| `dean@academic.local` | dean |
| `teacher1@academic.local` | teacher |
| `teacher2@academic.local` | teacher |
| `student1@academic.local` | student (группа БСБО-01-22) |
| `student2@academic.local` | student (БСБО-01-22) |
| `student3@academic.local` | student (БСБО-02-22) |

## Архитектурные инварианты

Несколько свойств, которые гарантируются на уровне БД и сервисов — стоит знать при работе с системой:

- **UoW во всех мутациях.** Любая мутация (создание/обновление/удаление) идёт через `repo.Store.RunInTx` с записью в `audit_log` и `change_logs` в той же транзакции. Лог не может оторваться от данных.
- **One-use refresh с replay-detection.** При попытке использовать refresh-токен второй раз `TokenService.Rotate` ревокует ВСЕ access-токены пользователя — разрывает сессию атакующего.
- **Privilege escalation guard.** `RBACService.AssignRole` не позволяет actor'у выдать роль с level больше своего. Декан (level 700) не может назначить admin (1000) даже с `roles.assign`.
- **UNIQUE-индексы как бизнес-правила.** `debts (student_id, discipline_id) WHERE status='open'` — нельзя 2 открытых долга на одну дисциплину. `teacher_role_requests (requested_by) WHERE status='pending'` — спам заявок запрещён.
- **Soft-delete везде где есть зависимые сущности.** `disciplines`, `roles`, `permissions` — soft-delete, потому что на них ссылаются `debts`/`role_user`/`permission_role`. Hard-delete сломал бы FK.
- **CHECK-constraints на статусах.** `debts.status='graded'` ⇒ `final_grade + graded_at + graded_by` обязательны. `retakes.kind='commission'` ⇒ `min_teachers ≥ 3`. БД гарантирует согласованность даже если сервис ошибётся.

## Текущий статус

### Готово (в `develop`)

- [x] **Auth-foundation** (PR #10): users, sessions, refresh, RBAC, audit, change_logs, миграции 00001-00010
- [x] **DX one-click setup** (PR #11): `make setup`, автомиграции, дев-сиды
- [x] **Доменная схема** (PR #12): миграции 00011-00019 + sqlc-репозитории для всех доменных таблиц + AuditService + ChangeLogService
- [x] **Disciplines** (PR #13): DisciplineService + handler'ы `/api/disciplines/*`

### Готово (в PR, ждёт мерджа)

- [ ] **Debts** (`feat/backend-debts`): DebtService + handler'ы `/api/debts/*` + `/api/me/disciplines/*`

### В работе / следующие PR (см. [project-rent/backend_roadmap_tasks.md](../../project-rent/backend_roadmap_tasks.md))

| Задача | Содержание |
|---|---|
| BACK-03 | Retakes — пересдачи + участники + переходы статусов |
| BACK-04 | RetakeChangeRequest — заявки на изменение пересдачи |
| BACK-05 | TeacherRoleRequest + RBAC HTTP API (закрытие RBAC на 100%) |
| BACK-06 | NotifyService: email через gomail/v2 + Mailpit + WebSocket |
| BACK-07 | Шедулер `time.Ticker` + синхронизация с внешней системой |
| BACK-08 | Сводные отчёты + экспорт XLSX/CSV (excelize) |
| BACK-09 | Swagger через swaggo/swag |
| BACK-10 | CI GitHub Actions + rate-limit на `/api/auth/*` |

### Покрытие тестами

Интеграционные тесты против реального Postgres (запускаются через `make test-integration`). Текущее покрытие по сервисам:

| Пакет | Coverage | Тестов |
|---|---|---|
| audit | 90.5% | 4 |
| repo | 84.6% | 4 |
| rbac | 83.9% | 6 |
| auth | 76.7% | 7 |
| token | 77.8% | 7 |
| changelog | 77.8% | 5 |
| debt | ~80% | 10 |
| discipline | 58.0% | 8 |
| **Всего** | | **51 тест** |
