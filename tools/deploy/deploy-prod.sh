#!/usr/bin/env bash
# Деплой только через Docker Compose. Выполняет docker compose up -d --build.
# Запуск из корня проекта: ./tools/deploy/deploy-prod.sh
# Требует: Docker и docker compose установлены, .env создан (см. services/infra/.env.example).

set -e
cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
  echo "Создайте .env: cp services/infra/.env.example .env && отредактируйте SMTP и т.д."
  exit 1
fi

echo "Запуск стека через Docker Compose..."
docker compose up -d --build

echo ""
echo "Готово. Приложение: http://$(hostname -I 2>/dev/null | awk '{print $1}'):8080  Auth: :8081"
echo "Логи: docker compose logs -f"
