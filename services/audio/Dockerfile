FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod \
  go mod download
COPY internal/ internal/
COPY services/audio/ services/audio/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /audioserver ./services/audio

FROM alpine:3.19
RUN apk add --no-cache wget su-exec && adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /audioserver .
COPY services/infra/entrypoint-log.sh /entrypoint-log.sh
RUN chown appuser:appuser /app/audioserver && chmod +x /entrypoint-log.sh
EXPOSE 8084
ENTRYPOINT ["/entrypoint-log.sh"]
CMD ["sh", "-c", "./audioserver 2>&1 | tee -a /var/log/messenger/app.log"]
