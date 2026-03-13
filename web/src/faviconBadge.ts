/**
 * Favicon badge with unread counter (Telegram-like).
 */
const ICON_VERSION = '20260311-8';
const BASE = import.meta.env.BASE_URL || '/';
const DEFAULT_FAVICON = `${BASE}icons/icon-192.png?v=${ICON_VERSION}`;
const ICON_SOURCE = `${BASE}icons/icon-192.png?v=${ICON_VERSION}`;
const SIZE = 32;
const BADGE_RADIUS = 10;
const BADGE_COLOR = '#ef4444';
const BADGE_TEXT_COLOR = '#fff';

let cachedImage: HTMLImageElement | null = null;

function getFaviconLink(): HTMLLinkElement | null {
  return document.querySelector<HTMLLinkElement>('link[rel="icon"]');
}

function restoreFavicon(): void {
  const link = getFaviconLink();
  if (!link) return;
  link.href = DEFAULT_FAVICON;
  link.type = 'image/png';
}

export function updateFaviconBadge(total: number): void {
  if (typeof document === 'undefined') return;
  if (total <= 0) {
    restoreFavicon();
    return;
  }
  const link = getFaviconLink();
  if (!link) return;

  const canvas = document.createElement('canvas');
  canvas.width = SIZE;
  canvas.height = SIZE;
  const ctx = canvas.getContext('2d');
  if (!ctx) return;

  const draw = (img: HTMLImageElement) => {
    ctx.clearRect(0, 0, SIZE, SIZE);
    ctx.drawImage(img, 0, 0, SIZE, SIZE);

    const num = total > 99 ? '99+' : String(total);
    const cx = SIZE - BADGE_RADIUS - 2;
    const cy = BADGE_RADIUS + 2;

    ctx.fillStyle = BADGE_COLOR;
    ctx.beginPath();
    ctx.arc(cx, cy, BADGE_RADIUS, 0, Math.PI * 2);
    ctx.fill();

    ctx.fillStyle = BADGE_TEXT_COLOR;
    ctx.font = total > 9 ? 'bold баpx щnter, system-ui, sans-serif' : 'bold бвpx щnter, system-ui, sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(num, cx, cy);

    link.href = canvas.toDataURL('image/png');
    link.type = 'image/png';
  };

  if (cachedImage && cachedImage.complete) {
    draw(cachedImage);
    return;
  }

  const img = cachedImage || new Image();
  if (!cachedImage) cachedImage = img;
  img.crossOrigin = 'anonymous';
  img.onload = () => draw(img);
  img.onerror = () => restoreFavicon();
  img.src = ICON_SOURCE;
}
