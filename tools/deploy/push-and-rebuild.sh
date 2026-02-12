#!/usr/bin/env bash
# Пуш в origin (GitHub) и пересборка на удалённом сервере.
# Запуск из корня проекта: ./tools/deploy/push-and-rebuild.sh
# Только изменённые сервисы: BUILD_SERVICES="api frontend files" ./tools/deploy/push-and-rebuild.sh
# Требует: git origin = GitHub, SSH (ключ .deploy/messenger_deploy или ssh-agent).

set -e
cd "$(dirname "$0")/.."

REMOTE="${REMOTE:-origin}"
BRANCH="${BRANCH:-main}"
SSH_HOST="${SSH_HOST:-119.235.125.154}"
SSH_USER="${SSH_USER:-administrator}"
APP_DIR="${APP_DIR:-/opt/messenger}"
SSH_KEY="${SSH_KEY:-.deploy/messenger_deploy}"
# Только эти сервисы пересобрать и перезапустить (пусто = все)
BUILD_SERVICES="${BUILD_SERVICES:-}"

SSH_OPTS="-o ConnectTimeout=15 -o BatchMode=yes"
[ -f "$SSH_KEY" ] && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"

echo "=== Push to $REMOTE ==="
git push "$REMOTE" "$BRANCH"

echo ""
echo "=== Rebuild on server $SSH_USER@$SSH_HOST ==="
if [ -n "$BUILD_SERVICES" ]; then
  ssh $SSH_OPTS "$SSH_USER@$SSH_HOST" "cd $APP_DIR && git pull && docker compose build $BUILD_SERVICES && docker compose up -d $BUILD_SERVICES"
else
  ssh $SSH_OPTS "$SSH_USER@$SSH_HOST" "cd $APP_DIR && git pull && docker compose up -d --build"
fi

echo ""
echo "Done. Logs: ssh $SSH_OPTS $SSH_USER@$SSH_HOST 'cd $APP_DIR && docker compose logs -f'"
