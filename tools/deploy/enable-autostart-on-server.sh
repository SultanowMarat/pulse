#!/bin/bash
# Включает автозапуск Docker и контейнеров Messenger на сервере при загрузке (после отключения света и т.п.).
# Запуск из корня репозитория: ./tools/deploy/enable-autostart-on-server.sh
# Требуется sudo на сервере (SUDO_PW или пароль при запросе).

set -e
cd "$(dirname "$0")/../.."
SERVER="${DEPLOY_SERVER:-administrator@119.235.125.154}"
SSH_KEY="${SSH_KEY:-.deploy/messenger_deploy}"
SUDO_PW="${SUDO_PW:-}"

SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=15"
[ -f "$SSH_KEY" ] && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"

echo "==> Копирование systemd-юнита на сервер..."
scp $SSH_OPTS tools/deploy/messenger-docker.service "$SERVER:/tmp/messenger-docker.service"

echo "==> Включение автозапуска Docker и контейнеров Messenger..."
run_remote() {
  ssh $SSH_OPTS "$SERVER" "$@"
}
REMOTE_SCRIPT='set -e
mv -f /tmp/messenger-docker.service /etc/systemd/system/messenger-docker.service
systemctl daemon-reload
systemctl enable docker
systemctl enable messenger-docker
echo "Docker: $(systemctl is-enabled docker 2>/dev/null || true)"
echo "messenger-docker: $(systemctl is-enabled messenger-docker 2>/dev/null || true)"
echo "Готово. После перезагрузки сервера контейнеры запустятся автоматически."
'
if [ -n "$SUDO_PW" ]; then
  { printf '%s\n' "$SUDO_PW"; echo "$REMOTE_SCRIPT"; } | run_remote "sudo -S bash -s"
else
  echo "$REMOTE_SCRIPT" | run_remote "sudo bash -s"
fi

echo "==> Готово."
