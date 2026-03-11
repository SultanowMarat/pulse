#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <service> [project_dir] [image_tar]"
  exit 1
fi

SERVICE="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_PROJECT_DIR="${PULSE_PROJECT_DIR:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"
PROJECT_DIR="${2:-${DEFAULT_PROJECT_DIR}}"
IMAGE_TAR="${3:-}"

if [[ ! -d "${PROJECT_DIR}" ]]; then
  echo "Project directory not found: ${PROJECT_DIR}"
  exit 1
fi

COMPOSE_FILE=""
for f in docker-compose.yml docker-compose.yaml compose.yml compose.yaml; do
  if [[ -f "${PROJECT_DIR}/${f}" ]]; then
    COMPOSE_FILE="${PROJECT_DIR}/${f}"
    break
  fi
done

if [[ -z "${COMPOSE_FILE}" ]]; then
  echo "No docker compose file found in ${PROJECT_DIR}"
  exit 1
fi

echo "Using compose file: ${COMPOSE_FILE}"
cd "${PROJECT_DIR}"

if [[ -n "${IMAGE_TAR}" ]]; then
  if [[ ! -f "${IMAGE_TAR}" ]]; then
    echo "Image archive not found: ${IMAGE_TAR}"
    exit 1
  fi
  echo "Loading local image archive: ${IMAGE_TAR}"
  docker load -i "${IMAGE_TAR}"
fi

echo "Restarting service: ${SERVICE}"
docker compose -f "${COMPOSE_FILE}" up -d --no-build "${SERVICE}"

echo "Service status:"
docker compose -f "${COMPOSE_FILE}" ps "${SERVICE}"

CID="$(docker compose -f "${COMPOSE_FILE}" ps -q "${SERVICE}" || true)"
if [[ -n "${CID}" ]]; then
  echo "Last logs for ${SERVICE}:"
  docker logs --tail 200 "${CID}"
fi
