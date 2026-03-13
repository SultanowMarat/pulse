/**
 * Регистрация пуш-уведомлений: запрос разрешения, подписка и отправка на сервер.
 * Для PWA без push: запрос разрешения на уведомления для in-app (WebSocket) уведомлений.
 */
import { getPushConfig, pushSubscribe, pushUnsubscribe, type PushSubscriptionJson } from '../api';
import { APP_VERSION } from '../appVersion';

const SW_PATH = `/sw.js?v=${encodeURIComponent(APP_VERSION)}`;
const RECHECK_DELAY_MS = 1200;
let registerInFlight: Promise<void> | null = null;

function canUseWebPush(): boolean {
  return (
    typeof window !== 'undefined' &&
    'Notification' in window &&
    'serviceWorker' in navigator &&
    'PushManager' in window
  );
}

/** Запросить разрешение на уведомления. В Electron — чтобы показывать native уведомления при новых сообщениях. В PWA — то же. */
export async function requestNotificationPermissionForPWA(): Promise<void> {
  if (!canUseWebPush()) return;
  if (Notification.permission !== 'default') return;
  try {
    await Notification.requestPermission();
  } catch {
    // ignore
  }
}

export async function registerPushIfEnabled(): Promise<void> {
  if (!canUseWebPush()) return;
  if (registerInFlight) {
    await registerInFlight;
    return;
  }
  registerInFlight = (async () => {
    try {
      const config = await getPushConfig();
      if (!config.enabled || !config.vapid_public_key) return;

      const permission = await Notification.requestPermission();
      if (permission !== 'granted') return;

      const reg = await navigator.serviceWorker.register(SW_PATH, { scope: '/' });
      await reg.update();
      let subscription = await reg.pushManager.getSubscription();
      if (!subscription) {
        subscription = await reg.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(config.vapid_public_key),
        });
      }
      if (subscription) {
        const subJson = subscription.toJSON() as unknown as PushSubscriptionJson;
        await pushSubscribe(subJson);
      }
    } catch (e) {
      console.warn('Push registration failed:', e);
    } finally {
      registerInFlight = null;
    }
  })();
  await registerInFlight;
}

// Поддержка фоновых уведомлений в PWA: периодически обновляет подписку при возврате в приложение.
export function startPushBackgroundMaintenance(): () => void {
  if (!canUseWebPush()) return () => {};

  let timer: ReturnType<typeof setTimeout> | null = null;
  const scheduleRegister = () => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      void registerPushIfEnabled();
    }, RECHECK_DELAY_MS);
  };

  const onVisibility = () => {
    if (document.visibilityState === 'visible') scheduleRegister();
  };
  const onServiceWorkerMessage = (ev: MessageEvent) => {
    if (ev?.data?.type === 'PUSH_SUBSCRIPTION_CHANGED') scheduleRegister();
  };

  window.addEventListener('focus', scheduleRegister, { passive: true });
  window.addEventListener('online', scheduleRegister, { passive: true });
  document.addEventListener('visibilitychange', onVisibility);
  navigator.serviceWorker.addEventListener('message', onServiceWorkerMessage);

  scheduleRegister();
  return () => {
    if (timer) clearTimeout(timer);
    window.removeEventListener('focus', scheduleRegister);
    window.removeEventListener('online', scheduleRegister);
    document.removeEventListener('visibilitychange', onVisibility);
    navigator.serviceWorker.removeEventListener('message', onServiceWorkerMessage);
  };
}

// Безопасная отписка на logout: не оставлять уведомления старому пользователю на общем устройстве.
export async function unsubscribePushIfEnabled(): Promise<void> {
  if (!canUseWebPush()) return;
  try {
    const reg = await navigator.serviceWorker.ready;
    const sub = await reg.pushManager.getSubscription();
    if (!sub) return;

    try {
      const json = sub.toJSON() as unknown as PushSubscriptionJson;
      if (json?.endpoint) {
        await pushUnsubscribe(json.endpoint);
      }
    } finally {
      await sub.unsubscribe().catch(() => {});
    }
  } catch {
    // ignore
  }
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = atob(base64);
  const output = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; ++i) {
    output[i] = rawData.charCodeAt(i);
  }
  return output;
}
