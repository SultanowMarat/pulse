/**
 * Регистрация пуш-уведомлений: запрос разрешения, подписка и отправка на сервер.
 * Для PWA без push: запрос разрешения на уведомления для in-app (WebSocket) уведомлений.
 */
import { getPushConfig, pushSubscribe, type PushSubscriptionJson } from '../api';

const SW_PATH = '/sw.js';

/** Запросить разрешение на уведомления для PWA (in-app уведомления при новых сообщениях). Вызывать при загрузке. */
export async function requestNotificationPermissionForPWA(): Promise<void> {
  if (typeof window === 'undefined' || !('Notification' in window)) return;
  if ((window as unknown as { electronAPI?: unknown }).electronAPI) return;
  const isStandalone = window.matchMedia('(display-mode: standalone)').matches || (window.navigator as Navigator & { standalone?: boolean }).standalone === true;
  if (!isStandalone) return;
  if (Notification.permission !== 'default') return;
  try {
    await Notification.requestPermission();
  } catch {
    // ignore
  }
}

export async function registerPushIfEnabled(): Promise<void> {
  if (!('Notification' in window) || !('serviceWorker' in navigator) || !('PushManager' in window)) {
    return;
  }
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
