# Сборка под Linux-сервер (amd64)

Короткая инструкция, как собирать образы для **linux/amd64**, чтобы они запускались на сервере.

---

## Почему так

Если собирать на Mac (arm64) без указания платформы, на сервере будет ошибка `exec format error`.

---

## Требования

- Docker + Docker Compose (Compose V2)
- Запуск из корня репозитория

---

## Быстрая сборка (docker compose)

```bash
export DOCKER_DEFAULT_PLATFORM=linux/amd64
export COMPOSE_PROJECT_NAME=messenger
DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker compose build
```

Сборка отдельных сервисов:

```bash
DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 \
  docker compose build api frontend nginx
```

---

## Сборка с кешем Buildx (максимальный реюз слоёв)

Подходит, когда нужно повторно использовать слои между сборками и ускорять CI/локальные сборки.

1) Создать билдера (один раз):

```bash
docker buildx create --use --name messenger-builder >/dev/null 2>&1 || \
  docker buildx use messenger-builder
```

2) Сборка сервисов с локальным кешем:

```bash
export DOCKER_DEFAULT_PLATFORM=linux/amd64
export COMPOSE_PROJECT_NAME=messenger
export BUILDX_CACHE=.buildx-cache
mkdir -p "$BUILDX_CACHE"

for svc in auth api push files audio frontend nginx; do
  docker buildx build --load \
    --cache-from type=local,src="$BUILDX_CACHE" \
    --cache-to type=local,dest="$BUILDX_CACHE",mode=max \
    -f "services/${svc}/Dockerfile" \
    -t "${COMPOSE_PROJECT_NAME}-${svc}:latest" \
    .
done
```

> Итог: образы будут перезаписаны локально (`messenger-<service>:latest`), а кеш сохранится в `.buildx-cache`.

---

## Сохранение образов в tar (для переноса)

```bash
mkdir -p .deploy/images

docker save -o .deploy/images/messenger-auth.tar messenger-auth:latest
docker save -o .deploy/images/messenger-api.tar messenger-api:latest
docker save -o .deploy/images/messenger-push.tar messenger-push:latest
docker save -o .deploy/images/messenger-files.tar messenger-files:latest
docker save -o .deploy/images/messenger-audio.tar messenger-audio:latest
docker save -o .deploy/images/messenger-frontend.tar messenger-frontend:latest
docker save -o .deploy/images/messenger-nginx.tar messenger-nginx:latest
```

Или одним архивом:

```bash
docker save -o /tmp/messenger-images.tar \
  messenger-auth:latest messenger-api:latest messenger-push:latest \
  messenger-files:latest messenger-audio:latest \
  messenger-frontend:latest messenger-nginx:latest
```

---

## Проверка платформы образа

```bash
docker image inspect messenger-api:latest --format '{{.Os}}/{{.Architecture}}'
# Ожидается: linux/amd64
```
