#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <ftp_or_http_url> <output_file> [ftp_user] [ftp_password]"
  exit 1
fi

DOWNLOAD_URL="$1"
OUT_FILE="$2"
FTP_USER="${3:-${FTP_USER:-}}"
FTP_PASSWORD="${4:-${FTP_PASSWORD:-}}"

if [[ ! "${DOWNLOAD_URL}" =~ ^(ftp|http|https):// ]]; then
  echo "Expected FTP/HTTP(S) URL. Got: ${DOWNLOAD_URL}"
  exit 1
fi

mkdir -p "$(dirname "${OUT_FILE}")"

echo "Downloading file to ${OUT_FILE}..."
if [[ "${DOWNLOAD_URL}" =~ ^ftp:// ]]; then
  if [[ -z "${FTP_USER}" || -z "${FTP_PASSWORD}" ]]; then
    echo "FTP credentials required for FTP URL (set FTP_USER/FTP_PASSWORD or pass as args 3/4)."
    exit 1
  fi
  curl -f --ftp-pasv --connect-timeout 60 --max-time 0 --retry 10 --retry-all-errors -u "${FTP_USER}:${FTP_PASSWORD}" -C - "${DOWNLOAD_URL}" -o "${OUT_FILE}"
else
  curl -fL --connect-timeout 60 --max-time 0 --retry 10 --retry-all-errors -C - "${DOWNLOAD_URL}" -o "${OUT_FILE}"
fi
echo "Done"
