#!/usr/bin/env bash
# Создать релиз на GitHub (тег + описание). Требует GITHUB_TOKEN с правом repo.
# Использование: GITHUB_TOKEN=ghp_xxx ./tools/deploy/create-release.sh [v1.0.0]
# Или: gh release create v1.0.0 --title "v1.0.0" --notes "Первый релиз."  (если установлен gh)

set -e
cd "$(dirname "$0")/.."

TAG="${1:-v1.0.0}"
NAME="$TAG"
if [ -z "$GITHUB_TOKEN" ]; then
  echo "Задайте GITHUB_TOKEN (Settings → Developer settings → Personal access tokens)."
  echo "Или создайте релиз вручную: https://github.com/SultanowMarat/messenger/releases/new?tag=$TAG&title=$NAME"
  exit 1
fi

echo "Создание релиза $TAG..."
curl -sS -X POST "https://api.github.com/repos/SultanowMarat/messenger/releases" \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  -d '{"tag_name":"'"$TAG"'","name":"'"$NAME"'","body":"Release '"$TAG"'. macOS app (DMG) is built automatically in Actions and will appear here in a few minutes.","draft":false}'

echo ""
echo "Готово. Релиз: https://github.com/SultanowMarat/messenger/releases/tag/$TAG"
