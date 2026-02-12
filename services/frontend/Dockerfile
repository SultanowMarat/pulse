# Сборка SPA (только статика)
FROM node:20-alpine AS builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm,id=npm-cache \
  npm ci --silent
COPY web/ ./
RUN npm run build

# Runtime: только nginx + собранная статика, без лишних конфигов
FROM nginx:alpine
RUN rm -f /etc/nginx/conf.d/default.conf
COPY --from=builder /app/web/dist /usr/share/nginx/html
COPY services/frontend/config/nginx-frontend.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
