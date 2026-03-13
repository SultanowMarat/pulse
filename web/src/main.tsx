import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';
import { APP_VERSION } from './appVersion';
import { useChatStore } from './store';

if (typeof navigator !== 'undefined' && 'serviceWorker' in navigator) {
  const VERSION_KEY = 'pulse_frontend_version';
  const SW_PATH = `/sw.js?v=${encodeURIComponent(APP_VERSION)}`;

  const forceReloadWithVersion = () => {
    try {
      const url = new URL(window.location.href);
      url.searchParams.set('_fv', APP_VERSION);
      window.location.replace(url.toString());
    } catch {
      window.location.reload();
    }
  };

  const clearClientCaches = async () => {
    if (typeof window !== 'undefined' && 'caches' in window) {
      try {
        const keys = await caches.keys();
        await Promise.all(keys.map((k) => caches.delete(k)));
      } catch {
        // ignore cache cleanup errors
      }
    }
  };

  const storedVersion = (() => {
    try {
      return localStorage.getItem(VERSION_KEY) || '';
    } catch {
      return '';
    }
  })();

  if (storedVersion && storedVersion !== APP_VERSION) {
    try {
      localStorage.setItem(VERSION_KEY, APP_VERSION);
    } catch {
      // ignore
    }
    clearClientCaches().finally(forceReloadWithVersion);
  } else {
    try {
      localStorage.setItem(VERSION_KEY, APP_VERSION);
    } catch {
      // ignore
    }
  }

  window.addEventListener('load', () => {
    let refreshing = false;
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      if (refreshing) return;
      refreshing = true;
      forceReloadWithVersion();
    });

    navigator.serviceWorker
      .register(SW_PATH, { scope: '/' })
      .then(async (reg) => {
        await reg.update().catch(() => {});
        if (reg.waiting) reg.waiting.postMessage({ type: 'SKIP_WAITING' });
      })
      .catch(() => {});
  });
}

if (typeof window !== 'undefined') {
  window.addEventListener('pulse-desktop-open-chat', (event: Event) => {
    const chatId = (event as CustomEvent<{ chatId?: string }>).detail?.chatId;
    if (!chatId || !chatId.trim()) return;
    const id = chatId.trim();
    const state = useChatStore.getState();
    state.setActiveChat(id);
    state.markAsRead(id);
    window.focus();
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
          minHeight: 'var(--app-height)',
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
