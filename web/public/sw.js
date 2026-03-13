/* Service Worker: push для PWA pulse (Windows/macOS/Linux/mobile) */

self.addEventListener('install', () => {
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener('message', (event) => {
  if (event?.data?.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});

const ICON_VERSION = '20260312-2';

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

function normalizeString(value) {
  if (typeof value !== 'string') return '';
  return value.trim();
}

function toAbsoluteMaybe(value) {
  const raw = normalizeString(value);
  if (!raw) return '';
  try {
    return new URL(raw, self.location.origin).toString();
  } catch {
    return '';
  }
}

function pickAvatarUrl(payload, data) {
  const candidates = [
    data.sender_avatar_url,
    data.senderAvatarUrl,
    data.avatar_url,
    data.avatarUrl,
    payload.sender_avatar_url,
    payload.senderAvatarUrl,
    payload.avatar_url,
    payload.avatarUrl,
    payload.icon,
    payload.image,
  ];
  for (const candidate of candidates) {
    const abs = toAbsoluteMaybe(candidate);
    if (abs) return abs;
  }
  return '';
}

function pickSenderName(payload, data) {
  return (
    normalizeString(data.sender_name) ||
    normalizeString(data.senderName) ||
    normalizeString(data.sender) ||
    normalizeString(payload.sender_name) ||
    normalizeString(payload.senderName) ||
    normalizeString(payload.sender) ||
    ''
  );
}

self.addEventListener('push', (event) => {
  if (!event.data) return;
  event.waitUntil((async () => {
    let payload;
    try {
      payload = event.data.json();
    } catch {
      payload = { title: 'pulse', body: event.data.text() || 'Новое уведомление' };
    }

    const data = payload.data || {};
    const sender = pickSenderName(payload, data);
    const title =
      normalizeString(payload.title) ||
      normalizeString(data.chat_name) ||
      normalizeString(data.chatName) ||
      'pulse';

    const bodyRaw =
      normalizeString(payload.body) ||
      normalizeString(data.body) ||
      'Новое сообщение';
    const body = sender && !bodyRaw.startsWith(`${sender}:`) ? `${sender}: ${bodyRaw}` : bodyRaw;

    const avatarUrl = pickAvatarUrl(payload, data);
    const icon = avatarUrl || `/icons/icon-192.png?v=${ICON_VERSION}`;

    const chatId = normalizeString(data.chat_id) || normalizeString(payload.chat_id);
    const messageId = normalizeString(data.message_id) || normalizeString(payload.message_id);
    const urlFromPayload = normalizeString(data.url) || normalizeString(payload.url);
    const url = urlFromPayload || buildMessageUrl({ chat_id: chatId });
    const tag = messageId ? `msg-${messageId}` : `chat-${chatId || 'common'}`;

    const options = {
      body,
      icon,
      badge: `/icons/icon-192.png?v=${ICON_VERSION}`,
      tag,
      renotify: false,
      requireInteraction: true,
      timestamp: Date.now(),
      data: { url, chat_id: chatId, message_id: messageId, sender, ...data },
      actions: [{ action: 'open', title: 'Открыть' }],
    };

    const windowClients = await clients.matchAll({ type: 'window', includeUncontrolled: true });
    const hasActiveClient = windowClients.some((client) => {
      if (!client.url || !client.url.includes(self.location.origin)) return false;
      return client.visibilityState === 'visible' || client.focused === true;
    });
    if (hasActiveClient) return;

    await self.registration.showNotification(title, options);
  })());
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



