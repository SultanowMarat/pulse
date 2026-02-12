#!/bin/sh
# Создаёт каталог для логов и переключается на appuser перед запуском команды.
# Если задана переменная CHOWN_DIRS (пути через пробел), создаёт их и отдаёт appuser (для volume uploads).
# Использование: ENTRYPOINT ["/entrypoint-log.sh"] CMD ["sh", "-c", "./server 2>&1 | tee -a /var/log/messenger/app.log"]
set -e
mkdir -p /var/log/messenger
chown appuser:appuser /var/log/messenger
if [ -n "$CHOWN_DIRS" ]; then
	for d in $CHOWN_DIRS; do
		mkdir -p "$d"
		chown -R appuser:appuser "$d"
	done
fi
exec su-exec appuser "$@"
