# Vue 3 + Vite

This template should help get you started developing with Vue 3 in Vite. The template uses Vue 3 `<script setup>` SFCs, check out the [script setup docs](https://v3.vuejs.org/api/sfc-script-setup.html#sfc-script-setup) to learn more.

Learn more about IDE Support for Vue in the [Vue Docs Scaling up Guide](https://vuejs.org/guide/scaling-up/tooling.html#ide-support).

# Frontend: запуск в Docker (Dev)

Этот документ для фронтенд-разработки через `frontend/Dockerfile` с `target=dev`.
`docker-compose.yml` остаётся backend-only; фронт в dev запускается отдельными командами.

## Быстрый старт через Makefile

Из корня проекта:

```bash
make frontend-up
```

После запуска:
- Frontend: `http://localhost:5173`

Полезные команды:

```bash
make frontend-build    # только пересобрать dev-образ фронта
make frontend-logs     # смотреть логи фронта
make frontend-shell    # зайти в контейнер фронта
make frontend-restart  # перезапуск контейнера фронта
make frontend-down     # остановить и удалить контейнер фронта
```

## Переменные (опционально)

Можно переопределить порт и адрес backend API:

```bash
make frontend-up FRONTEND_PORT=5174 VITE_API_BASE_URL=http://localhost:8080
```

## Эквивалент без Makefile

```bash
docker build --target dev -t academic-frontend-dev ./frontend

docker rm -f academic-frontend-dev 2>/dev/null || true

docker run -d \
  --name academic-frontend-dev \
  -p 5173:5173 \
  -e VITE_API_BASE_URL=http://localhost:8080 \
  -v "$(pwd)/frontend:/src" \
  -v academic-frontend-dev-node_modules:/src/node_modules \
  academic-frontend-dev
```
