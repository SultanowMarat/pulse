#!/bin/bash
# Сборка образов на вашем компьютере, перенос на сервер и развёртывание.
# Используйте, когда на сервере нет доступа к Docker Registry (не все сайты работают).
#
# Запуск из корня репозитория:
#   1) Синхронизация кода (опционально): SYNC_CODE=1 ./tools/deploy/build-save-push-to-server.sh
#   2) Или только образы: ./tools/deploy/build-save-push-to-server.sh
#
# По ключу (рекомендуется): положите ключ в .deploy/messenger_deploy — scp/ssh подхватят его.
# По паролю: SSHPASS=... SUDO_PW=... ./tools/deploy/build-save-push-to-server.sh

set -e
cd "$(dirname "$0")/../.."
SERVER="${DEPLOY_SERVER:-administrator@119.235.125.154}"
SSHPASS="${SSHPASS:-}"
SUDO_PW="${SUDO_PW:-2C3deaklm195}"
PLATFORM="${DOCKER_DEFAULT_PLATFORM:-linux/amd64}"
TAR="/tmp/messenger-images.tar"
SSH_KEY="${SSH_KEY:-.deploy/messenger_deploy}"
SYNC_CODE="${SYNC_CODE:-0}"

SCP_OPTS="-o StrictHostKeyChecking=accept-new"
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=15"
[ -f "$SSH_KEY" ] && SCP_OPTS="$SCP_OPTS -i $SSH_KEY" && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"

if [ "$SYNC_CODE" = "1" ]; then
  echo "==> Синхронизация кода на сервер (rsync, без .git/data/.env)..."
  rsync -avz --exclude='.git' --exclude='web/node_modules' --exclude='data' --exclude='.env' \
    -e "ssh $SSH_OPTS" ./ "$SERVER:/opt/messenger/"
fi

echo "==> Базовые образы (pull переиспользует уже скачанные слои)..."
export DOCKER_DEFAULT_PLATFORM="$PLATFORM"
docker pull --platform "$PLATFORM" postgres:16-alpine
docker pull --platform "$PLATFORM" redis:7-alpine

echo "==> Сборка сервисов (платформа $PLATFORM)..."
export COMPOSE_PROJECT_NAME=messenger
export DOCKER_BUILDKIT=1
# Без provenance, иначе docker save может выдать "content digest not found"
export BUILDKIT_PROVENANCE=0
docker compose build

echo "==> Сохранение образов в $TAR..."
# Сначала пробуем сохранить базовые (на части машин docker save падает из-за attestation)
BASE_TAR="/tmp/messenger-base.tar"
if docker save -o "$BASE_TAR" postgres:16-alpine redis:7-alpine 2>/dev/null; then
  docker save -o "$TAR" \
    postgres:16-alpine redis:7-alpine \
    messenger-auth:latest messenger-api:latest messenger-files:latest messenger-audio:latest \
    messenger-push:latest messenger-frontend:latest messenger-nginx:latest
  rm -f "$BASE_TAR"
else
  rm -f "$BASE_TAR"
  echo "   (базовые postgres/redis не сохраняются — на сервере должны быть уже или подтянутся при первом up)"
  docker save -o "$TAR" \
    messenger-auth:latest messenger-api:latest messenger-files:latest messenger-audio:latest \
    messenger-push:latest messenger-frontend:latest messenger-nginx:latest
fi

echo "==> Копирование образов на сервер..."
if [ -n "$SSHPASS" ]; then
  sshpass -e scp $SCP_OPTS "$TAR" "$SERVER:/tmp/messenger-images.tar"
else
  scp $SCP_OPTS "$TAR" "$SERVER:/tmp/messenger-images.tar"
fi

echo "==> Загрузка образов на сервере и запуск контейнеров..."
REMOTE_CMD="docker load -i /tmp/messenger-images.tar && rm -f /tmp/messenger-images.tar && cd /opt/messenger && docker compose up -d && docker restart messenger-nginx 2>/dev/null; docker compose ps"
if [ -n "$SSHPASS" ]; then
  sshpass -e ssh $SSH_OPTS "$SERVER" "echo '$SUDO_PW' | sudo -S bash -c '$REMOTE_CMD'"
else
  if [ -n "$SUDO_PW" ]; then
    ssh $SSH_OPTS "$SERVER" "echo '$SUDO_PW' | sudo -S bash -c '$REMOTE_CMD'"
  else
    ssh $SSH_OPTS "$SERVER" "sudo bash -c '$REMOTE_CMD'"
  fi
fi

rm -f "$TAR"
echo "==> Готово."
