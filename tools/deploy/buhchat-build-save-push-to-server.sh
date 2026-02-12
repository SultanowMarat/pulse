#!/usr/bin/env bash
# Build linux/amd64 images locally, copy code + images to server, load and restart via docker compose.
#
# Required env:
#   SSH_HOST=...
#   SSH_USER=...
# Optional env:
#   SSH_PORT=6005
#   SSH_KEY=ssh/id_ed25519
#   APP_DIR=/opt/messenger
#   SYNC_CODE=1        (default 0)
#   DOCKER_DEFAULT_PLATFORM=linux/amd64
#   COMPOSE_PROJECT_NAME=messenger
#
# Notes:
# - This script intentionally avoids password-based SSH/sudo.
# - Remote host must allow key auth and either:
#   - user is in docker group, OR
#   - sudo is configured for docker without password (NOPASSWD), OR
#   - you run the remote commands manually after ssh.
set -euo pipefail

cd "$(dirname "$0")/../.."

SSH_HOST="${SSH_HOST:?set SSH_HOST}"
SSH_USER="${SSH_USER:?set SSH_USER}"
SSH_PORT="${SSH_PORT:-6005}"
SSH_KEY="${SSH_KEY:-ssh/id_ed25519}"
APP_DIR="${APP_DIR:-/opt/messenger}"
SYNC_CODE="${SYNC_CODE:-0}"

SERVER="${SSH_USER}@${SSH_HOST}"
PLATFORM="${DOCKER_DEFAULT_PLATFORM:-linux/amd64}"
PROJECT="${COMPOSE_PROJECT_NAME:-messenger}"
TAR="/tmp/${PROJECT}-images.tar"

# ssh использует -p для порта, scp — -P
SSH_BASE_OPTS=(-p "$SSH_PORT" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
SCP_BASE_OPTS=(-P "$SSH_PORT" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
if [[ -f "$SSH_KEY" ]]; then
  SSH_BASE_OPTS+=(-i "$SSH_KEY")
  SCP_BASE_OPTS+=(-i "$SSH_KEY")
fi

echo "==> Local build platform: ${PLATFORM}"
echo "==> Remote: ${SERVER}:${SSH_PORT}  APP_DIR=${APP_DIR}"

if [[ "$SYNC_CODE" == "1" ]]; then
  echo "==> Sync code to server (rsync, excluding data/.env/node_modules/etc)..."
  rsync -avz \
    --exclude '.git' \
    --exclude 'macos' \
    --exclude 'web/node_modules' \
    --exclude 'data' \
    --exclude '.env' \
    --exclude 'services/*/logs/*.log' \
    --exclude 'ssh' \
    -e "ssh ${SSH_BASE_OPTS[*]}" \
    ./ "${SERVER}:${APP_DIR}/"
fi

echo "==> Build images (docker compose build)..."
export DOCKER_DEFAULT_PLATFORM="$PLATFORM"
export COMPOSE_PROJECT_NAME="$PROJECT"
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
export BUILDKIT_PROVENANCE=0
docker compose build

echo "==> Save images to ${TAR}..."
docker save -o "$TAR" \
  "${PROJECT}-auth:latest" \
  "${PROJECT}-api:latest" \
  "${PROJECT}-push:latest" \
  "${PROJECT}-files:latest" \
  "${PROJECT}-audio:latest" \
  "${PROJECT}-frontend:latest" \
  "${PROJECT}-nginx:latest"

echo "==> Copy images to server..."
scp "${SCP_BASE_OPTS[@]}" "$TAR" "${SERVER}:/tmp/${PROJECT}-images.tar"

echo "==> Load images and restart compose on server..."
REMOTE_CMD=$(cat <<'SH'
set -euo pipefail
cd "$APP_DIR"

run_docker() {
  if docker info >/dev/null 2>&1; then
    docker "$@"
  elif sudo -n docker info >/dev/null 2>&1; then
    sudo -n docker "$@"
  else
    echo "No permission to run docker (need docker group or passwordless sudo for docker)." >&2
    exit 2
  fi
}

run_docker load -i "/tmp/${PROJECT}-images.tar"
rm -f "/tmp/${PROJECT}-images.tar"

run_docker compose up -d --no-build
run_docker restart messenger-nginx 2>/dev/null || true
run_docker compose ps
SH
)

ssh "${SSH_BASE_OPTS[@]}" "$SERVER" \
  "APP_DIR='$APP_DIR' PROJECT='$PROJECT' bash -lc $(printf %q "$REMOTE_CMD")"

rm -f "$TAR"
echo "==> Done."

