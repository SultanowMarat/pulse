#!/bin/sh
# Скачивает Node.js LTS в .node/ (darwin-arm64), если ещё не установлен. Нужен для make frontend-build.
set -e
NODE_VERSION="v22.22.0"
NODE_DIR="$(cd "$(dirname "$0")/.." && pwd)/.node"
NODE_BIN="${NODE_DIR}/node-${NODE_VERSION}-darwin-arm64"

if [ -x "${NODE_BIN}/bin/npm" ]; then
  echo "Node ${NODE_VERSION} уже в ${NODE_BIN}"
  exit 0
fi

ARCH=$(uname -m)
OS=$(uname -s)
if [ "$OS" != "Darwin" ] || [ "$ARCH" != "arm64" ]; then
  echo "Скрипт поддерживает только darwin/arm64. Установите Node вручную: https://nodejs.org"
  exit 1
fi

mkdir -p "$NODE_DIR"
cd "$NODE_DIR"
echo "Скачивание Node ${NODE_VERSION}..."
curl -fsSL "https://nodejs.org/dist/${NODE_VERSION}/node-${NODE_VERSION}-darwin-arm64.tar.gz" -o node.tar.gz
tar -xzf node.tar.gz
rm node.tar.gz
echo "Готово. Добавьте в PATH: export PATH=\"${NODE_BIN}/bin:\$PATH\""
echo "Или запускайте: make frontend-build"
