#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <archive.tar.gz> <project_dir>"
  exit 1
fi

ARCHIVE="$1"
PROJECT_DIR="$2"

if [[ ! -f "${ARCHIVE}" ]]; then
  echo "Archive not found: ${ARCHIVE}"
  exit 1
fi

if [[ ! -d "${PROJECT_DIR}" ]]; then
  echo "Project directory not found: ${PROJECT_DIR}"
  exit 1
fi

TS="$(date +%Y%m%d-%H%M%S)"
TMP_DIR="$(mktemp -d)"
BACKUP_DIR="/tmp/pulse-release-backup-${TS}"
mkdir -p "${BACKUP_DIR}"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo "Extracting ${ARCHIVE}..."
tar -xzf "${ARCHIVE}" -C "${TMP_DIR}"

SRC_ROOT="$(find "${TMP_DIR}" -mindepth 1 -maxdepth 1 -type d | head -n 1 || true)"
if [[ -z "${SRC_ROOT}" ]]; then
  SRC_ROOT="${TMP_DIR}"
fi

echo "Creating backup of files that will be overwritten..."
while IFS= read -r -d '' file; do
  rel="${file#${SRC_ROOT}/}"
  dst="${PROJECT_DIR}/${rel}"
  if [[ -f "${dst}" ]]; then
    mkdir -p "${BACKUP_DIR}/$(dirname "${rel}")"
    cp -a "${dst}" "${BACKUP_DIR}/${rel}"
  fi
done < <(find "${SRC_ROOT}" -type f -print0)

echo "Applying release over ${PROJECT_DIR}..."
cp -a "${SRC_ROOT}/." "${PROJECT_DIR}/"

echo "Done"
echo "Backup: ${BACKUP_DIR}"
