#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <direct_download_url> <output_file>"
  exit 1
fi

DOWNLOAD_URL="$1"
OUT_FILE="$2"

if [[ ! "${DOWNLOAD_URL}" =~ ^https?:// ]]; then
  echo "Expected direct HTTP(S) URL. Got: ${DOWNLOAD_URL}"
  exit 1
fi

echo "Downloading file to ${OUT_FILE}..."
curl -fL --connect-timeout 60 --max-time 0 --retry 10 --retry-all-errors -C - "${DOWNLOAD_URL}" -o "${OUT_FILE}"
echo "Done"
