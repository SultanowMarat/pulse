#!/bin/sh
# Проверка микросервиса авторизации после docker compose up -d

set -e
AUTH_URL="${AUTH_URL:-http://localhost:8081}"

echo "=== 1. Health auth ==="
curl -s "$AUTH_URL/health" && echo ""

echo ""
echo "=== 2. Запрос кода на email (подставьте свой email) ==="
curl -s -X POST "$AUTH_URL/api/auth/request-code" \
  -H "Content-Type: application/json" \
  -d '{"email":"m.sultan0w@yandex.ru","device_id":"550e8400-e29b-41d4-a716-446655440000","device_name":"Test"}' | head -c 500
echo ""

echo ""
echo "Если в ответе {\"status\":\"ok\"} — код отправлен на почту. Проверьте письмо и выполните verify-code с полученным кодом."
