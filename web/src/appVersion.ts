declare const __APP_VERSION__: string | undefined;

function readMetaVersion(): string {
  if (typeof document === 'undefined') return '';
  const v = document.querySelector('meta[name="pulse-app-version"]')?.getAttribute('content') || '';
  return v.trim();
}

export const APP_VERSION = (typeof __APP_VERSION__ !== 'undefined' && __APP_VERSION__) || readMetaVersion() || 'dev';

