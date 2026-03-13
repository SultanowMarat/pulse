# Auth: Ð¾Ð´Ð¸Ð½ Ð±Ð¸Ð½Ð°Ñ€Ð½Ð¸Ðº, Ñ‚Ð¾Ð»ÑŒÐºÐ¾ ca-certificates Ð¸ wget (healthcheck)
FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV GOPROXY=https://proxy.golang.org,direct GOSUMDB=off
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod \
  go mod download
COPY internal/ internal/
COPY services/auth/ services/auth/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /authserver ./services/auth

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget su-exec && adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /authserver .
COPY services/infra/entrypoint-log.sh /entrypoint-log.sh
RUN chown appuser:appuser /app/authserver && chmod +x /entrypoint-log.sh
EXPOSE 8081
ENTRYPOINT ["/entrypoint-log.sh"]
CMD ["sh", "-c", "./authserver 2>&1 | tee -a /var/log/pulse/app.log"]
