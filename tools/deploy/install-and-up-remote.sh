#!/bin/bash
# Установка Docker, клонирование messenger и запуск контейнеров на удалённом сервере.
# Запуск с вашей машины (пароль SSH и sudo = 2C3deaklm195, см. docs/DEPLOY.md):
#   SSHPASS=2C3deaklm195 sshpass -e ssh administrator@119.235.125.154 'SUDO_PW=2C3deaklm195 bash -s' < tools/deploy/install-and-up-remote.sh
# Либо скопировать на сервер и выполнить там (подставив SUDO_PW):
#   scp tools/deploy/install-and-up-remote.sh administrator@119.235.125.154:/tmp/
#   ssh administrator@119.235.125.154 'SUDO_PW=2C3deaklm195 bash /tmp/install-and-up-remote.sh'

set -e
SUDO_PW="${SUDO_PW:-}"
export DEBIAN_FRONTEND=noninteractive

sudo_cmd() {
  if [ -n "$SUDO_PW" ]; then
    echo "$SUDO_PW" | sudo -S "$@"
  else
    sudo "$@"
  fi
}

echo "==> Установка Docker..."
if ! command -v docker &>/dev/null; then
  sudo_cmd apt-get update -qq
  sudo_cmd apt-get install -y docker.io
  sudo_cmd systemctl enable --now docker
fi
if ! docker compose version &>/dev/null; then
  sudo_cmd apt-get install -y docker-compose-plugin 2>/dev/null || \
  sudo_cmd apt-get install -y docker-compose-v2 2>/dev/null || true
fi
# Запуск docker без sudo (текущая сессия)
sudo_cmd usermod -aG docker "$USER" 2>/dev/null || true
# Запуск docker (с sudo если нет прав в группе docker)
run_docker() {
  if docker info &>/dev/null 2>&1; then docker "$@"; else sudo_cmd docker "$@"; fi
}

echo "==> Клонирование репозитория в /opt/messenger..."
sudo_cmd mkdir -p /opt
if [ -d /opt/messenger/.git ]; then
  cd /opt/messenger
  sudo_cmd git fetch origin
  sudo_cmd git reset --hard origin/main
  sudo_cmd chown -R "$USER:$USER" /opt/messenger
  cd /opt/messenger && git pull --ff-only
else
  sudo_cmd rm -rf /opt/messenger
  sudo_cmd git clone --depth 1 https://github.com/SultanowMarat/messenger.git /opt/messenger
  sudo_cmd chown -R "$USER:$USER" /opt/messenger
fi
cd /opt/messenger

echo "==> Создание .env (ваши настройки)..."
cat > .env << 'ENVFILE'
# SMTP для отправки OTP (Яндекс.Почта)
SMTP_HOST=smtp.yandex.ru
SMTP_PORT=587
SMTP_USERNAME=m.sultan0w@yandex.ru
SMTP_PASSWORD=tzvoggatcedcmrhw
SMTP_FROM_EMAIL=m.sultan0w@yandex.ru
SMTP_FROM_NAME=Auth Service
#DEBUG=1
ENVFILE

echo "==> Запуск контейнеров (docker compose up -d --build)..."
run_docker compose up -d --build

echo "==> Готово. Проверка: run_docker compose ps"
run_docker compose ps
