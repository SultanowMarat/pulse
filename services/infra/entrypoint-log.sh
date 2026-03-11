#!/bin/sh
# !>740Ñ‘Ñ‚ :0Ñ‚0;>3 4;O ;>3>2 8 ?5Ñ€5:;ÑŽÑ‡05Ñ‚AO =0 appuser ?5Ñ€54 70?ÑƒA:>< :><0=4Ñ‹.
# Ð•A;8 7040=0 ?5Ñ€5<5==0O CHOWN_DIRS (?ÑƒÑ‚8 Ñ‡5Ñ€57 ?Ñ€>15;), A>740Ñ‘Ñ‚ 8Ñ… 8 >Ñ‚40Ñ‘Ñ‚ appuser (4;O volume uploads).
# Ð˜A?>;ÑŒ7>20=85: ENTRYPOINT ["/entrypoint-log.sh"] CMD ["sh", "-c", "./server 2>&1 | tee -a /var/log/pulse/app.log"]
set -e
mkdir -p /var/log/pulse
chown appuser:appuser /var/log/pulse
if [ -n "$CHOWN_DIRS" ]; then
	for d in $CHOWN_DIRS; do
		mkdir -p "$d"
		chown -R appuser:appuser "$d"
	done
fi
exec su-exec appuser "$@"
