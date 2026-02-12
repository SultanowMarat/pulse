#!/bin/sh
# Пересборка фронта (после изменений в web/src). На экране входа остаётся только email → код.
cd "$(dirname "$0")/../web" && npm run build
