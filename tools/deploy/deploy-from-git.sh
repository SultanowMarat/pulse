#!/usr/bin/env bash
# Деплой из Git через Docker Compose: clone/pull + docker compose up -d --build.
# Первый раз: REPO=/opt/git/messenger.git APP_DIR=/opt/messenger ./tools/deploy/deploy-from-git.sh
# Обновление:  cd /opt/messenger && git pull && docker compose up -d --build
set -e

APP_DIR="${APP_DIR:-/opt/messenger}"
REPO="${REPO:-}"

if [ -n "$REPO" ] && [ ! -d "$APP_DIR/.git" ]; then
  echo "Клонирование $REPO в $APP_DIR..."
  mkdir -p "$(dirname "$APP_DIR")"
  git clone "$REPO" "$APP_DIR"
  cd "$APP_DIR"
else
  cd "$APP_DIR"
  [ "$1" = "pull" ] && git pull
fi

[ ! -f .env ] && echo "Создайте .env: cp services/infra/.env.example .env" && exit 1
docker compose up -d --build
