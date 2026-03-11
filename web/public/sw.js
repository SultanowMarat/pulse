/* Service Worker: push для PWA pulse (Windows/macOS/Linux/mobile) */

self.addEventListener('install', () => {
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim());
});

function buildAbsoluteUrl(path) {
  try {
    return new URL(path || '/', self.location.origin).toString();
  } catch {
    return self.location.origin + '/';
  }
}

function buildMessageUrl(data) {
  if (data && data.chat_id) return `/#/chat/${data.chat_id}`;
  return '/';
}

self.addEventListener('push', (event) => {
  if (!event.data) return;
  let payload;
  try {
    payload = event.data.json();
  } catch {
    payload = { title: 'pulse', body: event.data.text() || 'Новое уведомление' };
  }

  const data = payload.data || {};
  const title = payload.title || 'pulse';
  const body = payload.body || '';
  const tag = data.message_id ? `msg-${data.message_id}` : `chat-${data.chat_id || 'common'}`;

  const options = {
    body,
    icon: '/icons/icon-192.png?v=20260311-6',
    badge: '/icons/icon-192.png?v=20260311-6',
    tag,
    renotify: true,
    requireInteraction: true,
    timestamp: Date.now(),
    data: { url: buildMessageUrl(data), ...data },
    actions: [{ action: 'open', title: 'Открыть' }],
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const rawUrl = event.notification.data && event.notification.data.url ? event.notification.data.url : '/';
  const targetUrl = buildAbsoluteUrl(rawUrl);

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        if (!client.url || !client.url.includes(self.location.origin)) continue;
        if ('navigate' in client) client.navigate(targetUrl);
        if ('focus' in client) return client.focus();
      }
      if (clients.openWindow) return clients.openWindow(targetUrl);
      return undefined;
    })
  );
});

// On subscription rotation/expiration ask opened client to re-register push.
self.addEventListener('pushsubscriptionchange', (event) => {
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        client.postMessage({ type: 'PUSH_SUBSCRIPTION_CHANGED' });
      }
    })
  );
});

