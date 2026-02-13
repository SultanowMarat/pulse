# Обратный прокси (только HTTP). Главный конфиг — под 10k+ соединений.
FROM nginx:alpine
COPY services/nginx/config/nginx_main.conf /etc/nginx/nginx.conf
COPY services/nginx/config/nginx.conf /etc/nginx/conf.d/default.conf
COPY services/nginx/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
