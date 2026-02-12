#!/usr/bin/env bash
# Диагностика сервисов на сервере. Запуск из каталога проекта (например /opt/messenger):
#   ./tools/deploy/check-services.sh

set -e
cd "$(dirname "$0")/../.."

echo "=== docker compose ps ==="
docker compose ps -a

echo ""
echo "=== API logs (last 80 lines) ==="
docker compose logs api --tail 80 2>&1 || true

echo ""
echo "=== Nginx logs (last 30 lines) ==="
docker compose logs nginx --tail 30 2>&1 || true

echo ""
echo "=== Frontend logs (last 20 lines) ==="
docker compose logs frontend --tail 20 2>&1 || true

echo ""
echo "=== Quick HTTP check (from host) ==="
curl -s -o /dev/null -w "GET / -> %{http_code}\n" http://127.0.0.1:80/ 2>/dev/null || echo "curl / failed"
curl -s -o /dev/null -w "GET /health -> %{http_code}\n" http://127.0.0.1:80/health 2>/dev/null || echo "curl /health failed"
