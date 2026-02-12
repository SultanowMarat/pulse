#!/bin/sh
# Проверка окружения для развёртывания: Docker и docker compose config.
# .env больше не обязателен (SMTP настраивается через админ-панель и хранится в БД).
set -e
cd "$(dirname "$0")/.."
echo "=== Docker ==="
if ! command -v docker >/dev/null 2>&1; then
  echo "Ошибка: Docker не установлен. Развёртывание только через Docker Compose."
  echo "  См. docs/install/INSTALL_DOCKER.md"
  exit 1
fi
docker --version
if ! docker compose version >/dev/null 2>&1; then
  echo "Ошибка: docker compose не найден."
  exit 1
fi
echo "docker compose: OK"
echo ""
echo "=== docker compose config ==="
docker compose config -q && echo "OK" || { echo "Ошибка в docker-compose.yml"; exit 1; }
echo ""
echo "Готово. Запуск: docker compose up -d --build"
