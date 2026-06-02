# Система уведомлений

## Обзор

Уведомления доставляются по двум каналам одновременно:

| Канал | Технология | Поведение |
|-------|-----------|-----------|
| Real-time | WebSocket (`coder/websocket`) | Best-effort: если пользователь офлайн — просто нет push |
| Email | `gopkg.in/gomail.v2` + SMTP | Best-effort: SMTP-ошибки логируются, не прерывают запрос |
| История | PostgreSQL `notifications` | **Обязательно**: при ошибке — весь `Notify` возвращает ошибку |

Запись в БД всегда идёт первой. WebSocket и email — лишь каналы доставки поверх неё.

---

## Структура файлов

```
backend/internal/service/notify/
├── notify.go       — Service, Event, Notification, Kind-константы
├── store.go        — dbNotifier: запись в таблицу notifications
├── hub.go          — WebSocket Hub: Register / Unregister / broadcast
├── email.go        — emailNotifier: gomail + go:embed HTML-шаблоны
├── multi.go        — multiNotifier: fan-out по каналам
├── templates/
│   ├── retake_scheduled.html
│   ├── retake_updated.html
│   └── retake_cancelled.html
└── service_test.go — 7 интеграционных тестов

backend/internal/transport/http/
├── handler/notifications.go  — REST: List, MarkRead, UnreadCount
├── handler/ws.go             — WebSocket upgrade + Hub + burst
└── dto/notifications.go      — NotificationResponse, UnreadCountResponse

backend/internal/config/config.go  — добавлены SMTP-поля
```

---

## Типы событий (Kind)

| Константа | Значение | Email-шаблон |
|-----------|---------|-------------|
| `KindRetakeScheduled` | `retake_scheduled` | `retake_scheduled.html` |
| `KindRetakeUpdated` | `retake_updated` | `retake_updated.html` |
| `KindRetakeCancelled` | `retake_cancelled` | `retake_cancelled.html` |
| `KindRetakeGradeReceived` | `retake_grade_received` | нет (только DB + WS) |
| `KindRetakeRequestApproved` | `retake_request_approved` | нет — заявка преподавателя на создание пересдачи одобрена |
| `KindRetakeRequestRejected` | `retake_request_rejected` | нет — заявка на создание отклонена |
| `KindRetakeChangeApproved` | `retake_change_approved` | нет — заявка на изменение одобрена (автору) |
| `KindRetakeChangeRejected` | `retake_change_rejected` | нет — заявка на изменение отклонена |
| `KindTeacherRequestApproved` | `teacher_request_approved` | нет — заявка на роль преподавателя |
| `KindTeacherRequestRejected` | `teacher_request_rejected` | нет — заявка на роль преподавателя |

> При одобрении заявки на изменение (`changerequest`) автор получает
> `retake_change_approved` («вам одобрили изменение»), а остальные участники
> (студенты, другие преподаватели) — `retake_updated` («вам изменили
> пересдачу»), как при прямом изменении деканатом.

Для типов без шаблона email не отправляется — `emailNotifier.send` тихо возвращает `nil`.

---

## Поток данных при `Notify`

```
Caller (напр. retake.Assign)
    │
    ▼
notify.Service.Notify(ctx, Event)
    │
    ▼
multiNotifier.notify
    │
    ├─ 1. dbNotifier.persist ──► INSERT notifications → возвращает Notification с ID
    │        (если ошибка — весь Notify возвращает err, дальше не идёт)
    │
    ├─ 2. Hub.broadcast ──► wsjson.Write ко всем WS-коннектам userID
    │        (если пользователь офлайн — ноль итераций, нет ошибки)
    │
    └─ 3. emailNotifier.send
             ├─ cfg.Host == "" → skip (dev без SMTP)
             ├─ нет шаблона для kind → skip
             └─ GetUserByID → render template → gomail.DialAndSend
```

---

## API

Все `/api/notifications/*` требуют `Authorization: Bearer <access_token>`.

### GET /api/notifications

Список уведомлений пользователя, новые первые.

**Query params:** `limit` (default 20), `offset` (default 0)

**Response 200:**
```json
{
  "items": [
    {
      "id": "uuid",
      "kind": "retake_scheduled",
      "payload": { "discipline": "Математика", "scheduled_at": "..." },
      "read_at": null,
      "created_at": "2026-06-15T10:00:00Z"
    }
  ]
}
```

---

### GET /api/notifications/unread-count

Количество непрочитанных — для badge на иконке колокольчика.

**Response 200:**
```json
{ "count": 3 }
```

---

### POST /api/notifications/:id/read

Отметить уведомление прочитанным. Идемпотентно (повторный вызов — OK).
Защита: SQL-запрос содержит `WHERE user_id = $actorID` — нельзя прочитать чужое.

**Response 204** (без тела)

---

### GET /ws/notifications?token=\<access_token\>

WebSocket-эндпоинт. JWT передаётся через query-параметр, так как браузер не может устанавливать заголовки при WS-апгрейде.

**Поведение при подключении:**
1. Сервер валидирует `token`
2. Регистрирует соединение в Hub
3. **Burst**: сразу отправляет все непрочитанные уведомления из БД (догоняет пропущенное после reconnect)
4. Держит соединение открытым

**Формат push-сообщения** (идентичен `NotificationResponse`):
```json
{
  "id": "uuid",
  "kind": "retake_scheduled",
  "payload": { "discipline": "Математика", "scheduled_at": "15 июня 2026, 10:00", "building": "Корп. 3", "room": "301" },
  "read_at": null,
  "created_at": "2026-06-15T07:30:00Z"
}
```

**Важно:** маршрут `/ws/notifications` намеренно вынесен за пределы `chi.Timeout(30s)` — глобальный таймаут убивал бы соединения через 30 секунд.

---

## Email-шаблоны

Шаблоны встроены в бинарь через `//go:embed` и не требуют файловой системы в проде.

Шаблон получает `Event.Payload` как map — ключи зависят от типа события:

| Kind | Ожидаемые ключи в Payload |
|------|--------------------------|
| `retake_scheduled` | `discipline`, `scheduled_at`, `building`, `room`, `duration_minutes` |
| `retake_updated` | `discipline`, `scheduled_at`, `building`, `room`, `notes` (опц.) |
| `retake_cancelled` | `discipline`, `reason` (опц.) |

Если ключ отсутствует, шаблон выведет пустую строку — не упадёт.

---

## Конфигурация (переменные окружения)

| Переменная | Default | Описание |
|-----------|---------|----------|
| `SMTP_HOST` | `""` | Пустой = email отключён. В dev: `mailpit` |
| `SMTP_PORT` | `1025` | Порт SMTP. Mailpit слушает 1025 |
| `SMTP_USER` | `""` | Логин SMTP. Пустой = без авторизации |
| `SMTP_PASSWORD` | `""` | Пароль SMTP |
| `SMTP_FROM` | `noreply@localhost` | Адрес отправителя |

В dev `docker-compose.yml` уже содержит Mailpit. Достаточно добавить в `.env`:
```env
SMTP_HOST=mailpit
SMTP_PORT=1025
SMTP_FROM=noreply@academic.local
```
Письма будут видны на `http://localhost:8025`.

---

## Как отправить уведомление из другого сервиса

```go
// Инжектируй *notify.Service в свой сервис через конструктор.
// Пример: сервис пересдач получает notifySvc при создании.

err := notifySvc.Notify(ctx, notify.Event{
    UserID: studentID,
    Kind:   notify.KindRetakeScheduled,
    Payload: map[string]any{
        "discipline":       "Математика",
        "scheduled_at":     "15 июня 2026, 10:00",
        "building":         "Корп. 3",
        "room":             "301",
        "duration_minutes": 90,
    },
})
// err != nil только если запись в БД не удалась.
// WS и email — best-effort, их ошибки логируются внутри.
```

---

## Тесты

Файл: `backend/internal/service/notify/service_test.go`

Все тесты — интеграционные, требуют `TEST_DATABASE_URL`. Без него — `t.Skip`.

| Тест | Что проверяет |
|------|--------------|
| `TestNotify_Notify_PersistsToDatabase` | После `Notify` запись видна в `List`, kind и payload корректны |
| `TestNotify_CountUnread_ReflectsNewNotifications` | Счётчик 0 → 2 после двух `Notify` |
| `TestNotify_MarkRead_DecrementsUnreadCount` | После `MarkRead` счётчик падает, `read_at` проставляется |
| `TestNotify_MarkRead_IsIdempotent` | Двойной `MarkRead` не ломает данные |
| `TestNotify_MarkRead_CannotReadOthersNotification` | Другой пользователь не может прочитать чужое уведомление |
| `TestNotify_ListUnread_ReturnsOnlyUnread` | `ListUnread` не возвращает прочитанные |
| `TestNotify_List_Pagination` | 5 записей → страница 1 = 3, страница 2 = 2, без дубликатов |

В CI (`backend-ci.yml`) тест-джоб поднимает `postgres:17-alpine`, прогоняет миграции и запускает `go test -race -cover ./...` — notify-тесты включены автоматически, отдельного шага не требуют.

---

## Ограничения и будущие улучшения

- **Горизонтальное масштабирование**: Hub хранит соединения in-memory. При нескольких инстансах push дойдёт только до пользователей, подключённых к тому же инстансу. Решение при необходимости — Redis Pub/Sub как транспорт между хабами.
- **Email retry**: при SMTP-ошибке уведомление не повторяется. Если нужна гарантия доставки — добавить outbox-таблицу с воркером.
- **WS ping/pong**: текущая реализация не отправляет ping-фреймы. Браузеры обычно обрывают «мёртвые» соединения сами, но при необходимости можно добавить `conn.Ping` в горутине.
