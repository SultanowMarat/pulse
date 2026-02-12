# Files: только загрузка/раздача файлов, без исходящего HTTPS — без ca-certificates
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod \
  go mod download
COPY internal/ internal/
COPY services/files/ services/files/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /fileserver ./services/files

FROM alpine:3.19
RUN apk add --no-cache wget su-exec && adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /fileserver .
COPY services/infra/entrypoint-log.sh /entrypoint-log.sh
RUN chown appuser:appuser /app/fileserver && chmod +x /entrypoint-log.sh
EXPOSE 8083
ENTRYPOINT ["/entrypoint-log.sh"]
CMD ["sh", "-c", "./fileserver 2>&1 | tee -a /var/log/messenger/app.log"]
