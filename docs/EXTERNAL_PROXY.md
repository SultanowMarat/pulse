# Внешний прокси (buhchat.com:443 → сервер:80)

Чтобы при обращении к **https://buhchat.com** всё работало, внешний nginx (который слушает 443) должен:

## 1. Проксировать на бэкенд

- **Upstream**: IP и порт сервера с BuhChat, например `http://95.85.108.139:80`.
- Проксировать весь трафик (/, /api/, /ws, /health и т.д.) на этот upstream.

## 2. Передавать заголовки

Обязательно отправлять на наш nginx (порт 80):

| Заголовок | Значение | Зачем |
|-----------|----------|--------|
| **Host** | `buhchat.com` (или тот host, с которым пришёл запрос) | Чтобы приложение и куки работали с правильным доменом |
| **X-Forwarded-Proto** | `https` | Чтобы бэкенд считал запрос HTTPS (куки, редиректы, ссылки) |
| **X-Forwarded-For** | IP клиента | Для логов и лимитов (реальный IP пользователя) |

Пример для nginx (внешний прокси):

```nginx
location / {
    proxy_pass http://BACKEND_IP:80;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;   # $scheme будет https при listen 443 ssl
}
```

## 3. WebSocket (/ws)

Для работы мессенджера в реальном времени нужно пробрасывать WebSocket:

```nginx
location /ws {
    proxy_pass http://BACKEND_IP:80;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

(Либо один общий `location /` с теми же заголовками Upgrade/Connection — nginx сам поднимет WebSocket.)

## 4. Проверка

- Открыть https://buhchat.com — должна загружаться SPA.
- Открыть https://buhchat.com/health — должен ответить API (JSON).
- Войти в мессенджер — чат и уведомления в реальном времени работают только при корректном проксировании /ws.

На стороне нашего сервера (BuhChat) конфиг nginx уже настроен: он пробрасывает `X-Forwarded-Proto` от внешнего прокси в API и фронт, так что при правильной настройке внешнего прокси всё должно работать.
