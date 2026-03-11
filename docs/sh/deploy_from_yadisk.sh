#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <release_url> <service> [project_dir] [image_url]"
  echo "Examples:"
  echo "  $0 'https://downloader.disk.yandex.ru/.../pulse-release.tar.gz?...' frontend /srv/pulse 'https://downloader.disk.yandex.ru/.../frontend-image.tar?...'"
  echo "  $0 'https://downloader.disk.yandex.ru/.../pulse-release.tar.gz?...' frontend"
  exit 1
fi

INPUT_RELEASE_SOURCE="$1"
SERVICE="$2"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_PROJECT_DIR="${PULSE_PROJECT_DIR:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"
PROJECT_DIR="${3:-${DEFAULT_PROJECT_DIR}}"
INPUT_IMAGE_SOURCE="${4:-}"
MIGRATE_CMD="${MIGRATE_CMD:-}"

ARCHIVE="/tmp/pulse-release-$(date +%Y%m%d-%H%M%S).tar.gz"
IMAGE_ARCHIVE=""
if [[ -n "${INPUT_IMAGE_SOURCE}" ]]; then
  IMAGE_ARCHIVE="/tmp/pulse-image-$(date +%Y%m%d-%H%M%S).tar"
fi

"${SCRIPT_DIR}/yadisk_download.sh" "${INPUT_RELEASE_SOURCE}" "${ARCHIVE}"
if [[ -n "${INPUT_IMAGE_SOURCE}" ]]; then
  "${SCRIPT_DIR}/yadisk_download.sh" "${INPUT_IMAGE_SOURCE}" "${IMAGE_ARCHIVE}"
fi
"${SCRIPT_DIR}/apply_release.sh" "${ARCHIVE}" "${PROJECT_DIR}"

if [[ -n "${MIGRATE_CMD}" ]]; then
  case "${MIGRATE_CMD}" in
    *"drop database"*|*"DROP DATABASE"*|*"docker compose down -v"*|*"docker volume prune"*|*"docker system prune -a --volumes"*)
      echo "Unsafe migration command blocked: ${MIGRATE_CMD}"
      exit 1
      ;;
  esac
  echo "Running migrations command..."
  cd "${PROJECT_DIR}"
  bash -lc "${MIGRATE_CMD}"
fi

"${SCRIPT_DIR}/deploy_service.sh" "${SERVICE}" "${PROJECT_DIR}" "${IMAGE_ARCHIVE}"

echo "Deploy completed"
