# Push: Ð¾Ð´Ð¸Ð½ Ð±Ð¸Ð½Ð°Ñ€Ð½Ð¸Ðº, Web Push â€” Ð½ÑƒÐ¶Ð½Ñ‹ ca-certificates Ð´Ð»Ñ HTTPS
FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV GOPROXY=direct GOSUMDB=off
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod \
  go mod download
COPY internal/ internal/
COPY services/push/ services/push/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /pushserver ./services/push

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget su-exec && adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /pushserver .
COPY services/infra/entrypoint-log.sh /entrypoint-log.sh
RUN chown appuser:appuser /app/pushserver && chmod +x /entrypoint-log.sh
EXPOSE 8082
ENTRYPOINT ["/entrypoint-log.sh"]
CMD ["sh", "-c", "./pushserver 2>&1 | tee -a /var/log/pulse/app.log"]
