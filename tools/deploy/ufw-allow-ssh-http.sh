#!/usr/bin/env bash
# Настройка UFW: открыты только SSH и HTTP (порт 80).
# Запуск на сервере с правами root (или sudo):
#   sudo bash ./tools/deploy/ufw-allow-ssh-http.sh
#
# Внимание: сначала добавляются правила, затем включается фаервол.
# Убедитесь, что SSH_PORT совпадает с портом, с которого вы подключаетесь.

set -e

SSH_PORT="${SSH_PORT:-22}"
HTTP_PORT="${HTTP_PORT:-80}"

echo "=== Установка UFW (если ещё не установлен) ==="
apt-get update -y
apt-get install -y ufw

echo ""
echo "=== Сброс правил UFW (оставляем только SSH и HTTP) ==="
ufw --force reset

echo ""
echo "=== Политики по умолчанию ==="
ufw default deny incoming
ufw default allow outgoing

echo ""
echo "=== Разрешаем только SSH (порт $SSH_PORT) и HTTP (порт $HTTP_PORT) ==="
ufw allow "$SSH_PORT/tcp" comment 'SSH'
ufw allow "$HTTP_PORT/tcp" comment 'HTTP'

echo ""
echo "=== Включение UFW ==="
ufw --force enable

echo ""
echo "=== Статус UFW ==="
ufw status verbose | head -50

echo ""
echo "Готово. Открыты только порты: TCP $SSH_PORT (SSH), TCP $HTTP_PORT (HTTP)."
