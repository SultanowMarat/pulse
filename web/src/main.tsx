import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';

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
