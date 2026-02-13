import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';

function isIOSDevice(): boolean {
  if (typeof navigator === 'undefined') return false;
  const ua = navigator.userAgent || '';
  const platform = navigator.platform || '';
  return /iP(hone|od|ad)/.test(ua) || (platform === 'MacIntel' && navigator.maxTouchPoints > 1);
}

function syncAppHeightVar() {
  if (typeof window === 'undefined' || typeof document === 'undefined') return;
  const vv = window.visualViewport;
  const innerH = window.innerHeight;
  const vvH = vv?.height ?? innerH;
  const vvTop = vv?.offsetTop ?? 0;
  const standalone = window.matchMedia?.('(display-mode: standalone)').matches || (navigator as Navigator & { standalone?: boolean }).standalone === true;
  const isIOSStandalone = standalone && isIOSDevice();

  // iOS/PWA keyboard can report a very small visualViewport height, which creates
  // an extra gap under sticky composer. When keyboard is open, keep layout height
  // bound to innerHeight to avoid "double shrink" and big empty space.
  const keyboardLikelyOpen = innerH - vvH > 120;
  // Для iOS standalone при открытой клавиатуре используем именно visual viewport:
  // так шапка не уезжает вверх и последнее сообщение не отрывается от низа.
  const useVisualViewportHeight = isIOSStandalone && keyboardLikelyOpen;
  const visibleViewportHeight = Math.max(0, Math.round(vvH + vvTop));
  const nextHeight = useVisualViewportHeight
    ? Math.min(innerH, visibleViewportHeight || innerH)
    : (keyboardLikelyOpen ? innerH : Math.min(innerH, vvH));

  document.documentElement.style.setProperty('--app-height', `${Math.round(nextHeight)}px`);

  // Поле ввода максимально внизу: при открытой клавиатуре — по низу visualViewport (как в Telegram).
  const rawComposerBottom = Math.max(0, Math.round(innerH - vvTop - vvH));
  const composerBottom = useVisualViewportHeight ? 0 : rawComposerBottom;
  document.documentElement.style.setProperty('--composer-bottom', `${composerBottom}px`);

  // В standalone/PWA и при открытой клавиатуре убираем нижний padding у композера:
  // это устраняет лишнюю пустую полосу снизу на iOS.
  const keyboardOpen = rawComposerBottom > 60 || useVisualViewportHeight;
  if (keyboardOpen || standalone) {
    document.documentElement.style.setProperty('--composer-padding-bottom', '0');
  } else {
    document.documentElement.style.removeProperty('--composer-padding-bottom');
  }
}

if (typeof window !== 'undefined') {
  syncAppHeightVar();
  window.addEventListener('resize', syncAppHeightVar, { passive: true });
  window.addEventListener('orientationchange', syncAppHeightVar, { passive: true });
  if (window.visualViewport) {
    window.visualViewport.addEventListener('resize', syncAppHeightVar, { passive: true });
    window.visualViewport.addEventListener('scroll', syncAppHeightVar, { passive: true });
  }
}

// PWA: регистрация SW для установки на Android, iOS, Windows, macOS (push подключается отдельно в push.ts)
if (typeof navigator !== 'undefined' && 'serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js', { scope: '/' }).catch(() => {});
  });
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: unknown;
  showDetails: boolean;
}

class ErrorBoundary extends React.Component<{ children: React.ReactNode }, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false, error: null, showDetails: false };

  static getDerivedStateFromError(error: unknown): Partial<ErrorBoundaryState> {
    return { hasError: true, error };
  }

  componentDidCatch(error: unknown) {
    console.error('App error:', error);
  }

  render() {
    if (this.state.hasError) {
      const err = this.state.error;
      const message = err instanceof Error ? err.message : String(err);
      const isDev = (typeof import.meta !== 'undefined' && (import.meta as { env?: { DEV?: boolean } }).env?.DEV) === true;

      return (
        <div style={{
          minHeight: '100vh',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          padding: 24,
          background: '#1C1C1C',
          color: '#e7e9ea',
          fontFamily: 'system-ui, sans-serif',
          textAlign: 'center',
        }}>
          <h1 style={{ fontSize: 18, marginBottom: 12 }}>Произошла ошибка</h1>
          <p style={{ fontSize: 14, color: '#8b98a5', marginBottom: 20 }}>
            Сначала нажмите «Выйти и обновить». Если ошибка повторится — обновите страницу (Ctrl+F5).
          </p>
          {(isDev || this.state.showDetails) && message && (
            <pre style={{
              fontSize: 12,
              color: '#8b98a5',
              background: 'rgba(0,0,0,0.3)',
              padding: 12,
              borderRadius: 8,
              maxWidth: '90vw',
              overflow: 'auto',
              textAlign: 'left',
              marginBottom: 20,
            }}>
              {message}
            </pre>
          )}
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', justifyContent: 'center' }}>
            <button
              type="button"
              onClick={() => {
                localStorage.removeItem('session_id');
                localStorage.removeItem('session_secret');
                window.location.reload();
              }}
              style={{
                padding: '10px 20px',
                fontSize: 15,
                fontWeight: 600,
                color: '#fff',
                background: '#007AFF',
                border: 'none',
                borderRadius: 8,
                cursor: 'pointer',
              }}
            >
              Выйти и обновить
            </button>
            <button
              type="button"
              onClick={() => window.location.reload()}
              style={{
                padding: '10px 20px',
                fontSize: 15,
                fontWeight: 600,
                color: '#e7e9ea',
                background: 'transparent',
                border: '1px solid #8b98a5',
                borderRadius: 8,
                cursor: 'pointer',
              }}
            >
              Обновить страницу
            </button>
            {!isDev && (
              <button
                type="button"
                onClick={() => this.setState((s) => ({ showDetails: !s.showDetails }))}
                style={{
                  padding: '10px 20px',
                  fontSize: 15,
                  fontWeight: 600,
                  color: '#8b98a5',
                  background: 'transparent',
                  border: '1px solid #8b98a5',
                  borderRadius: 8,
                  cursor: 'pointer',
                }}
              >
                {this.state.showDetails ? 'Скрыть подробности' : 'Подробности'}
              </button>
            )}
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </React.StrictMode>,
);
