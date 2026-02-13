#!/usr/bin/env bash
# Установка и настройка Fail2Ban для защиты SSH.
# Запуск на сервере с правами root (или sudo):
#   sudo bash ./tools/deploy/fail2ban-install-ssh.sh
# Параметры: 3 неудачные попытки входа (на любом из портов) → бан на 15 минут.
# SSH порт задаётся переменной SSH_PORT (по умолчанию 22).

set -e

SSH_PORT="${SSH_PORT:-22}"
BANTIME="${BANTIME:-15m}"
FINDTIME="${FINDTIME:-10m}"
MAXRETRY="${MAXRETRY:-3}"

echo "=== Установка Fail2Ban ==="
apt-get update -y
apt-get install -y fail2ban

echo ""
echo "=== Создание jail (порт SSH=$SSH_PORT, maxretry=$MAXRETRY, bantime=$BANTIME) ==="
if [ -f /etc/fail2ban/jail.conf ] && [ ! -f /etc/fail2ban/jail.local ]; then
  cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local
fi

# Секция [DEFAULT]: общие настройки
cat > /etc/fail2ban/jail.d/sshd-buhchat.local << EOF
[DEFAULT]
ignoreip = 127.0.0.1/8 ::1
bantime  = $BANTIME
findtime = $FINDTIME
maxretry  = $MAXRETRY

[sshd]
enabled = true
port    = $SSH_PORT
filter  = sshd
logpath = %(sshd_log)s
maxretry = $MAXRETRY
bantime  = $BANTIME
findtime = $FINDTIME
EOF

echo "Создан /etc/fail2ban/jail.d/sshd-buhchat.local"

echo ""
echo "=== Включение и запуск Fail2Ban ==="
systemctl enable fail2ban
systemctl restart fail2ban

echo ""
echo "=== Статус ==="
systemctl status fail2ban --no-pager
echo ""
fail2ban-client status sshd 2>/dev/null || true

echo ""
echo "Готово. SSH (порт $SSH_PORT): при $MAXRETRY неудачных попытках — бан на $BANTIME."
echo "Разбан: fail2ban-client set sshd unbanip <IP>"
echo "Статус: fail2ban-client status sshd"
