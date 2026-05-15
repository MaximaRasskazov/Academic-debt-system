# Корневой Makefile проекта. Команды уровня "поднять весь стек".
# Backend-специфичные таргеты — в backend/Makefile.

.PHONY: help setup up down restart logs ps psql clean reset test backend-shell

ENV_FILE := .env
ENV_EXAMPLE := .env.example

help: ## Список доступных команд
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

setup: ## Полная подготовка: создаёт .env + JWT_SECRET + поднимает контейнеры с миграциями
	@if [ ! -f $(ENV_FILE) ]; then \
		echo ">>> Creating $(ENV_FILE) from $(ENV_EXAMPLE)"; \
		cp $(ENV_EXAMPLE) $(ENV_FILE); \
		SECRET=$$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 64); \
		sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=$$SECRET|" $(ENV_FILE) && rm -f $(ENV_FILE).bak; \
		echo ">>> Generated fresh JWT_SECRET"; \
	else \
		echo ">>> $(ENV_FILE) already exists, leaving it as is"; \
	fi
	@echo ">>> Building and starting docker compose..."
	docker compose up -d --build
	@echo ""
	@echo "  ✓ Backend:  http://localhost:8080/health"
	@echo "  ✓ Mailpit:  http://localhost:8025"
	@echo "  ✓ Postgres: localhost:15432 (academic/academic@academic_debts)"
	@echo ""
	@echo "  Если SEED_DEV_ACCOUNTS=true в .env — доступны демо-аккаунты:"
	@echo "    admin@academic.local / password    (роль admin)"
	@echo "    dean@academic.local / password     (роль dean)"
	@echo "    teacher1@academic.local / password (роль teacher)"
	@echo "    student1@academic.local / password (роль student)"

up: ## Запустить docker compose (без пересборки)
	docker compose up -d

down: ## Остановить docker compose (volumes остаются)
	docker compose down

restart: ## Перезапустить весь стек
	docker compose restart

logs: ## Tail-логи всех сервисов
	docker compose logs -f --tail=50

ps: ## Статус контейнеров
	docker compose ps

psql: ## Открыть psql внутри контейнера postgres
	docker compose exec postgres psql -U academic -d academic_debts

backend-shell: ## sh внутри backend-контейнера
	docker compose exec backend sh

test: ## Прогнать все тесты бэкенда (интеграционные требуют запущенный postgres)
	$(MAKE) -C backend test-integration

reset: ## ПОЛНЫЙ сброс: контейнеры + volumes + .env (требует подтверждения)
	@read -p "Удалить контейнеры, volumes и .env? Введите YES: " ans && [ "$$ans" = "YES" ] || (echo "Отменено"; exit 1)
	docker compose down -v
	rm -f $(ENV_FILE)
	@echo ">>> Reset done. Запустите 'make setup' для нового окружения."

clean: ## Удалить кеш и build-артефакты backend
	$(MAKE) -C backend clean
