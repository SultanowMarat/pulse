.PHONY: up down restart check clear-db reset

# Развёртывание через Docker Compose (docker-compose.yml в корне).
export DOCKER_BUILDKIT := 1

up:
	docker compose up -d --build

down:
	docker compose down

restart:
	docker compose down
	docker compose up -d --build
	@echo "Готово. Логи: docker compose logs -f"

# DEV ONLY: очистить БД. Требует CONFIRM_CLEAR_DB=development, APP_ENV != production.
clear-db:
	@if [ "$${APP_ENV}" = "production" ]; then echo "clear-db: запрещено в production"; exit 1; fi
	CONFIRM_CLEAR_DB=development go run ./tools/dev/cleardb

# Полный сброс: удалить данные в data/, поднять стек заново.
reset:
	docker compose down
	rm -rf data/
	docker compose up -d --build
	@echo "Стек перезапущен с пустыми данными. Логи: docker compose logs -f api"

check:
	@chmod +x tools/dev/check-env.sh 2>/dev/null || true
	@./tools/dev/check-env.sh
