/**
 * Базовый URL приложения (API, WebSocket, Auth).
 * В desktop-сборке адрес вшит в приложение и не настраивается пользователем.
 */

/** Адрес сервера в десктоп-приложении (не показывается в UI, не хранится в localStorage). */
const DEFAULT_PUBLIC_ORIGIN = 'https://buhchat.com';

export function isDesktopApp(): boolean {
  return typeof window !== 'undefined' && !!(window as unknown as { electronAPI?: unknown }).electronAPI;
}

/** Текущий базовый URL (без завершающего слэша). В десктопе — всегда вшитый DEFAULT_PUBLIC_ORIGIN. */
export function getApiBase(): string {
  if (typeof window === 'undefined') return '';
  if (isDesktopApp()) return DEFAULT_PUBLIC_ORIGIN;
  return window.location.origin;
}

/** URL для WebSocket: wss://host или ws://host (без path). */
export function getWebSocketBase(): string {
  const base = getApiBase();
  if (!base) return '';
  try {
    const u = new URL(base);
    return u.protocol === 'https:' ? `wss://${u.host}` : `ws://${u.host}`;
  } catch {
    return '';
  }
}
