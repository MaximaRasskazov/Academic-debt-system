# 🎓 Система учета академических задолженностей

Добро пожаловать в основной репозиторий проекта! 🚀 

Это вводный документ для знакомства с разрабатываемой системой. Наша главная цель — создать удобную, интуитивно понятную и быструю платформу для управления академическими долгами, которая объединит студентов, преподавателей и деканат в едином цифровом пространстве.

## 🛠 Стек технологий

* **Backend:** Python, FastAPI
* **Frontend:** Vue.js
* **Инфраструктура:** Docker

## ✨ Кратко о проекте

Система автоматизирует рутинные процессы учета успеваемости:
* **👨‍🎓 Студенты:** отслеживают свои долги и видят актуальное расписание назначенных пересдач.
* **👨‍🏫 Преподаватели:** просматривают списки должников, меняют статусы долгов на оценки, подают заявки на обычные и комиссионные пересдачи.
* **🏛 Деканат:** выступает главным модератором — назначает пересдачи, одобряет заявки преподавателей и выгружает сводную аналитику по потокам.

## 👥 Команда разработчиков

* **Максим Рассказов** — Project Manager / System Analyst / Backend Developer
* **Саян** — Frontend Developer (Vue.js) / UI/UX Designer
* **Андрей** — Backend Developer (FastAPI) / DevOps
* **Виталя** — Backend Developer

## 🚀 Quick Start

Полное поднятие проекта одной командой:

```bash
make setup
```

Что делает:
1. Создаёт `.env` из `.env.example`, если его ещё нет.
2. Генерирует случайный `JWT_SECRET` (через `openssl rand -hex 32`).
3. Поднимает `docker compose up -d --build` — postgres, mailpit и backend.
4. Внутри backend-контейнера `entrypoint.sh` дожидается готовности postgres, накатывает миграции из `backend/sql/migrations/` и (если `SEED_DEV_ACCOUNTS=true`) демо-аккаунты из `backend/sql/seeds/`.

После запуска доступны:
- **Backend:** http://localhost:8080/health
- **Mailpit UI:** http://localhost:8025
- **Postgres:** localhost:15432, БД `academic_debts`, пользователь `academic` / `academic`

### Демо-аккаунты для разработки (пароль у всех `password`)

| Email | Роль |
|---|---|
| `admin@academic.local`    | admin   |
| `dean@academic.local`     | dean    |
| `teacher1@academic.local` | teacher |
| `teacher2@academic.local` | teacher |
| `student1@academic.local` | student (группа БСБО-01-22) |
| `student2@academic.local` | student (группа БСБО-01-22) |
| `student3@academic.local` | student (группа БСБО-02-22) |

Чтобы отключить дев-сиды в prod — `SEED_DEV_ACCOUNTS=false` в `.env`.

### Полезные команды

```bash
make help        # Список таргетов
make logs        # Tail-логи всех сервисов
make psql        # psql внутри контейнера postgres
make down        # Остановить (volumes остаются)
make reset       # Полный сброс с подтверждением (volumes + .env)
make test        # Интеграционные тесты backend против поднятого postgres
```

Backend-специфичные команды (запуск из `backend/` или через `make -C backend ...`):
см. [backend/README.md](backend/README.md).

### Требования

- Docker + docker compose
- `make`, `openssl` (для генерации JWT_SECRET)
- Для локальной разработки backend без docker: Go 1.26+, `goose`, `sqlc`
