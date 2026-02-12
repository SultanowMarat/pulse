# Обратный прокси (только HTTP)
FROM nginx:alpine
COPY services/nginx/config/nginx.conf /etc/nginx/conf.d/default.conf
COPY services/nginx/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
