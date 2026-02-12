/* Service Worker: пуш-уведомления для PWA BuhChat */
self.addEventListener('push', (event) => {
  if (!event.data) return;
  let payload;
  try {
    payload = event.data.json();
  } catch {
    payload = { title: 'BuhChat', body: event.data.text() || 'Новое уведомление' };
  }
  const title = payload.title || 'BuhChat';
  const body = payload.body || '';
  const data = payload.data || {};
  const tag = (data.message_id ? 'msg-' + data.message_id : 'chat-' + (data.chat_id || '') + '-' + Date.now());
  const options = {
    body,
    icon: '/icons/icon-192.png',
    badge: '/icons/icon-192.png',
    tag,
    data: { url: data.chat_id ? `/#/chat/${data.chat_id}` : '/', ...data },
    renotify: true,
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const url = event.notification.data?.url || '/';
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          client.navigate(url);
          return client.focus();
        }
      }
      if (clients.openWindow) return clients.openWindow(url);
    })
  );
});
