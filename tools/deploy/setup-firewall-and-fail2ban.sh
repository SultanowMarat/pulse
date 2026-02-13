#!/usr/bin/env bash
# Настройка UFW (порты 22, 80) и Fail2Ban (SSH порт 22, 3 попытки → бан 15 мин).
# Запуск на сервере (потребуется пароль sudo):
#   ssh -i ssh/id_ed25519 -p 6005 marat@95.85.108.139
#   sudo bash /tmp/setup-firewall-and-fail2ban.sh
#
# Внимание: после выполнения UFW будет открыт только SSH на порту 22 и HTTP на 80.
# Подключайтесь дальше по: ssh -i ssh/id_ed25519 -p 22 marat@95.85.108.139

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
UFW_SCRIPT="${SCRIPT_DIR}/ufw-allow-ssh-http.sh"
F2B_SCRIPT="${SCRIPT_DIR}/fail2ban-install-ssh.sh"

if [ ! -f "$UFW_SCRIPT" ] || [ ! -f "$F2B_SCRIPT" ]; then
  echo "Скрипты не найдены в $SCRIPT_DIR. Запускайте из /tmp после копирования."
  UFW_SCRIPT="/tmp/ufw-allow-ssh-http.sh"
  F2B_SCRIPT="/tmp/fail2ban-install-ssh.sh"
  [ -f "$UFW_SCRIPT" ] || { echo "Нет $UFW_SCRIPT"; exit 1; }
  [ -f "$F2B_SCRIPT" ] || { echo "Нет $F2B_SCRIPT"; exit 1; }
fi

echo "=== 1/2 UFW: порты 22 (SSH) и 80 (HTTP) ==="
bash "$UFW_SCRIPT"

echo ""
echo "=== 2/2 Fail2Ban: порт 22, 3 попытки → бан 15 мин ==="
bash "$F2B_SCRIPT"

echo ""
echo "Готово. UFW и Fail2Ban настроены."
