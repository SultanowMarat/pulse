#!/usr/bin/env bash
# Собрать образы локально, скопировать проект и образы на сервер, развернуть без сборки на сервере.
# Запуск из корня проекта: ./tools/deploy/build-copy-deploy.sh
# Требует: локально запущен Docker, SSH-ключ .deploy/messenger_deploy (или ssh-agent).

set -e
cd "$(dirname "$0")/.."

SSH_HOST="${SSH_HOST:-119.235.125.154}"
SSH_USER="${SSH_USER:-administrator}"
APP_DIR="${APP_DIR:-/opt/messenger}"
SSH_KEY="${SSH_KEY:-.deploy/messenger_deploy}"
TAR_DIR="${TAR_DIR:-./.deploy/images}"

SSH_OPTS="-o ConnectTimeout=15 -o BatchMode=yes"
[ -f "$SSH_KEY" ] && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"

echo "=== 1. Сборка образов локально ==="
docker compose build

echo ""
echo "=== 2. Сохранение образов в tar ==="
mkdir -p "$TAR_DIR"
docker save -o "$TAR_DIR/postgres.tar" postgres:16-alpine
docker save -o "$TAR_DIR/redis.tar" redis:7-alpine
docker save -o "$TAR_DIR/messenger-apps.tar" \
  messenger-auth:latest \
  messenger-push:latest \
  messenger-files:latest \
  messenger-audio:latest \
  messenger-frontend:latest \
  messenger-api:latest \
  messenger-nginx:latest

echo ""
echo "=== 3. Копирование проекта и образов на сервер ==="
rsync -avz --delete \
  -e "ssh $SSH_OPTS" \
  --exclude 'macos' \
  --exclude 'web/node_modules' \
  --exclude 'data' \
  --exclude '.env' \
  --exclude 'services/*/logs/*.log' \
  --exclude '.git/objects' \
  ./ "${SSH_USER}@${SSH_HOST}:${APP_DIR}/"

rsync -avz -e "ssh $SSH_OPTS" "$TAR_DIR/" "${SSH_USER}@${SSH_HOST}:${APP_DIR}/.deploy/images/"

echo ""
echo "=== 4. Загрузка образов и запуск контейнеров на сервере ==="
ssh $SSH_OPTS "$SSH_USER@$SSH_HOST" "cd $APP_DIR && \
  docker load -i .deploy/images/postgres.tar && \
  docker load -i .deploy/images/redis.tar && \
  docker load -i .deploy/images/messenger-apps.tar && \
  docker compose up -d"

echo ""
echo "Готово. Логи: ssh $SSH_OPTS $SSH_USER@$SSH_HOST 'cd $APP_DIR && docker compose logs -f'"
