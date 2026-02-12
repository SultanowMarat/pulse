#!/usr/bin/env bash
# На сервере: удалить все Docker-образы, взять код из git, пересобрать с нуля.
# Запуск на сервере: ./tools/deploy/rebuild-clean.sh
# Или с вашей машины: ssh root@SERVER ./tools/deploy/rebuild-clean.sh
# Переменные: APP_DIR (по умолчанию /opt/messenger), GIT_REMOTE (origin), GIT_BRANCH (main).

set -e
APP_DIR="${APP_DIR:-/opt/messenger}"
GIT_REMOTE="${GIT_REMOTE:-origin}"
GIT_BRANCH="${GIT_BRANCH:-main}"

cd "$APP_DIR"

echo "=== Код из git ($GIT_REMOTE $GIT_BRANCH) ==="
git fetch "$GIT_REMOTE"
git reset --hard "$GIT_REMOTE/$GIT_BRANCH"

echo ""
echo "=== Остановка контейнеров ==="
docker compose down 2>/dev/null || true

echo ""
echo "=== Удаление всех Docker-образов и неиспользуемых данных ==="
docker system prune -af --volumes

echo ""
echo "=== Сборка и запуск (код из текущего каталога) ==="
docker compose up -d --build

echo ""
echo "Готово. Логи: docker compose logs -f"
