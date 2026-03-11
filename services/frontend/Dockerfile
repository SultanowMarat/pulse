# Runtime: только nginx + уже собранная статика web/dist
FROM nginx:alpine
RUN rm -f /etc/nginx/conf.d/default.conf
COPY web/dist /usr/share/nginx/html
COPY services/frontend/config/nginx-frontend.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
