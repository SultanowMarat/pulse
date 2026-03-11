# API: Ð¾Ð´Ð¸Ð½ Ð±Ð¸Ð½Ð°Ñ€Ð½Ð¸Ðº + Ð¼Ð¸Ð³Ñ€Ð°Ñ†Ð¸Ð¸, Ð¼Ð¸Ð½Ð¸Ð¼Ð°Ð»ÑŒÐ½Ñ‹Ð¹ runtime
FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV GOPROXY=direct GOSUMDB=off
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod \
  go mod download
COPY internal/ internal/
COPY migrations/ migrations/
COPY services/api/ services/api/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./services/api

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget su-exec postgresql-client && adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /server .
COPY --from=builder /app/migrations ./migrations
COPY services/infra/entrypoint-log.sh /entrypoint-log.sh
RUN chown -R appuser:appuser /app && chmod +x /entrypoint-log.sh
EXPOSE 8080
ENTRYPOINT ["/entrypoint-log.sh"]
CMD ["sh", "-c", "./server 2>&1 | tee -a /var/log/pulse/app.log"]
