#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "${SCRIPT_DIR}/.." && pwd)"

cd "${ROOT_DIR}"

echo "==> Backend tests"
go test ./...

echo "==> Frontend install"
cd "${ROOT_DIR}/web"
npm ci
echo "==> Frontend unit tests"
npm run test
echo "==> Frontend build"
npm run build

echo "All tests passed."
