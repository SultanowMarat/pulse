#!/usr/bin/env bash
# Перезапуск и пересборка контейнеров на удалённом сервере.
# Запускать только на сервере (например после ssh administrator@119.235.125.154):
#   cd /opt/messenger && ./tools/deploy/server-restart.sh
#
# Опционально: перед перезапуском подтянуть код с git (если репозиторий есть).
#   UPDATE_FROM_GIT=1 ./tools/deploy/server-restart.sh

set -e

cd "$(dirname "$0")/.."
REPO_ROOT="$(pwd)"

if [ ! -f docker-compose.yml ]; then
  echo "Ошибка: docker-compose.yml не найден. Запускайте из корня проекта."
  exit 1
fi

if [ -n "${UPDATE_FROM_GIT:-}" ] && [ -d .git ]; then
  echo "Обновление кода из git..."
  git pull
fi

echo "Остановка контейнеров..."
docker compose down

echo "Сборка и запуск..."
docker compose up -d --build

echo ""
echo "Готово. Логи: docker compose logs -f"
