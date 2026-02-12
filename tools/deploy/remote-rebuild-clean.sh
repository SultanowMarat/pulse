#!/usr/bin/env bash
# С вашей машины: подключиться к серверу и выполнить полную пересборку
# (удалить образы, подтянуть код из git, собрать заново).
# Запуск из корня проекта: ./tools/deploy/remote-rebuild-clean.sh
# Переменные: SSH_HOST (119.235.125.154), SSH_USER (administrator), APP_DIR (/opt/messenger).

set -e
SSH_HOST="${SSH_HOST:-119.235.125.154}"
SSH_USER="${SSH_USER:-administrator}"
APP_DIR="${APP_DIR:-/opt/messenger}"

echo "Подключение к $SSH_USER@$SSH_HOST..."
ssh -o ConnectTimeout=15 "$SSH_USER@$SSH_HOST" "cd $APP_DIR && APP_DIR=$APP_DIR ./tools/deploy/rebuild-clean.sh"
