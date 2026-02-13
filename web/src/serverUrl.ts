/**
 * Базовый URL приложения (API, WebSocket, Auth).
 * В desktop-сборке адрес вшит в приложение и не настраивается пользователем.
 */

/** Адрес сервера в десктоп-приложении (не показывается в UI, не хранится в localStorage). */
const DEFAULT_PUBLIC_ORIGIN = 'https://buhchat.com';

export function isDesktopApp(): boolean {
  return typeof window !== 'undefined' && !!(window as unknown as { electronAPI?: unknown }).electronAPI;
}

function trimTrailingSlash(url: string): string {
  return url.endsWith('/') ? url.slice(0, -1) : url;
}

/** Текущий базовый URL (без завершающего слэша). */
export function getApiBase(): string {
  if (typeof window === 'undefined') return '';
  // Если UI загружен по http(s), всегда используем текущий origin.
  // Это устраняет рассинхрон API/WS в desktop при смене домена сервера.
  if (window.location.protocol === 'http:' || window.location.protocol === 'https:') {
    return trimTrailingSlash(window.location.origin);
  }
  // Для file:// (offline fallback в десктопе) используем встроенный origin сервера.
  if (isDesktopApp()) return DEFAULT_PUBLIC_ORIGIN;
  return trimTrailingSlash(window.location.origin);
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
