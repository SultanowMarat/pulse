const { app, BrowserWindow, ipcMain, Menu, Notification, session } = require('electron');
const path = require('path');

if (process.platform === 'darwin') {
  // Avoid stale GPU shader cache issues in packaged macOS builds.
  app.commandLine.appendSwitch('disable-gpu-shader-disk-cache');
}

// Интерфейс всегда грузится с сервера, чтобы при обновлении кода все клиенты получали новый UI.
// API вызывается на том же origin (см. serverUrl.ts). Офлайн/fallback: вшитое приложение или offline.html.
const APP_URLS = (process.env.BUHCHAT_APP_URLS || process.env.BUHCHAT_APP_URL || 'https://buhchat.com')
  .split(',')
  .map((s) => s.trim())
  .filter(Boolean);
const SERVER_RETRY_MS = Math.max(5000, Number(process.env.BUHCHAT_SERVER_RETRY_MS || 15000) || 15000);
const OFFLINE_PATH = path.join(__dirname, 'gate', 'offline.html');

let isQuitting = false;
/** Главное окно приложения (для показа при фокусе после клика по пушу). */
let mainWindow = null;
let pushWindow = null;
const serverRetryTimers = new Map();
const windowLoadQueue = new Map();
let clearCacheTask = Promise.resolve();
let lastCacheClearAt = 0;

/** Сбрасывает кеш сессии, чтобы приложение подтянуло свежий интерфейс с сервера. */
function clearCache(force = false) {
  const now = Date.now();
  if (!force && now - lastCacheClearAt < 15_000) {
    return clearCacheTask;
  }
  clearCacheTask = clearCacheTask
    .catch(() => {})
    .then(async () => {
      const ts = Date.now();
      if (!force && ts - lastCacheClearAt < 15_000) {
        return;
      }
      await session.defaultSession.clearCache();
      lastCacheClearAt = Date.now();
    });
  return clearCacheTask;
}

function withNoCacheParam(url) {
  const t = Date.now();
  const sep = url.includes('?') ? '&' : '?';
  return `${url}${sep}desktop_nocache=${t}`;
}

function isServerURL(url) {
  if (!url) return false;
  return APP_URLS.some((base) => url.startsWith(base));
}

async function loadFromServer(win, options = {}) {
  const { clearSessionCache = false } = options;
  let lastErr = null;
  if (clearSessionCache) await clearCache(true);
  for (const base of APP_URLS) {
    try {
      await win.loadURL(withNoCacheParam(base));
      return;
    } catch (e) {
      lastErr = e;
    }
  }
  throw lastErr || new Error('all app urls failed');
}

function queueWindowLoad(win, options = {}) {
  if (!win || win.isDestroyed()) return Promise.resolve();
  const previous = windowLoadQueue.get(win.id) || Promise.resolve();
  const next = previous
    .catch(() => {})
    .then(() => loadWindow(win, options));
  windowLoadQueue.set(win.id, next);
  next.finally(() => {
    if (windowLoadQueue.get(win.id) === next) {
      windowLoadQueue.delete(win.id);
    }
  });
  return next;
}

function stopServerRetry(win) {
  if (!win) return;
  const t = serverRetryTimers.get(win.id);
  if (t) {
    clearInterval(t);
    serverRetryTimers.delete(win.id);
  }
}

function startServerRetry(win) {
  if (!win || win.isDestroyed()) return;
  stopServerRetry(win);
  const timer = setInterval(async () => {
    if (!win || win.isDestroyed()) {
      stopServerRetry(win);
      return;
    }
    const current = win.webContents.getURL();
    if (isServerURL(current)) {
      stopServerRetry(win);
      return;
    }
    try {
      await loadFromServer(win, { clearSessionCache: false });
      stopServerRetry(win);
    } catch {
      // Keep retrying silently while app stays on bundled/offline UI.
    }
  }, SERVER_RETRY_MS);
  serverRetryTimers.set(win.id, timer);
}

/** Загружает окно: сначала с сервера (со сбросом кеша), при ошибке — вшитое приложение или offline. */
async function loadWindow(win, options = {}) {
  const { forceRefresh = false, enableRetry = true } = options;
  try {
    await loadFromServer(win, { clearSessionCache: forceRefresh });
    stopServerRetry(win);
  } catch (e) {
    // Fallback only to dedicated offline screen. Loading bundled web app from file://
    // may break API auth/CORS and lead to stuck "Loading profile" state.
    await win.loadFile(OFFLINE_PATH);
    // If network/server was unavailable on startup, auto-switch to fresh server UI
    // as soon as it becomes available.
    if (enableRetry) startServerRetry(win);
  }
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 960,
    height: 700,
    minWidth: 400,
    minHeight: 400,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
    title: 'BuhChat',
  });

  queueWindowLoad(mainWindow, { forceRefresh: true, enableRetry: true });

  // При фокусе на главное окно (например после клика по пушу) — показываем его, если было скрыто
  mainWindow.on('focus', () => {
    if (mainWindow && !mainWindow.isDestroyed() && !mainWindow.isVisible()) mainWindow.show();
    if (mainWindow && !mainWindow.isDestroyed()) {
      const current = mainWindow.webContents.getURL();
      if (!isServerURL(current)) {
        queueWindowLoad(mainWindow, { forceRefresh: false, enableRetry: true });
      }
    }
  });

  // Cmd+R / Cmd+Shift+R / Ctrl+R: сброс кеша и загрузка с сервера, чтобы интерфейс обновился
  mainWindow.webContents.on('before-input-event', (event, input) => {
    const isReload = (input.control || input.meta) && input.key && input.key.toLowerCase() === 'r';
    if (!isReload) return;
    event.preventDefault();
    queueWindowLoad(mainWindow, { forceRefresh: true, enableRetry: true });
  });

  const onFail = (event, code) => {
    if (code !== -3 && mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.loadFile(OFFLINE_PATH);
    }
  };
  mainWindow.webContents.on('did-fail-load', onFail);
  mainWindow.on('closed', () => {
    stopServerRetry(mainWindow);
    windowLoadQueue.delete(mainWindow.id);
    mainWindow = null;
  });

  mainWindow.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));

  // На macOS закрытие главного окна должно полностью завершать приложение.
  if (process.platform === 'darwin') {
    mainWindow.on('close', (e) => {
      if (isQuitting) return;
      e.preventDefault();
      isQuitting = true;
      app.quit();
    });
  }
}

/** Создаёт скрытое окно, которое держит контекст для Service Worker, чтобы пуш приходил при свёрнутом/закрытом главном окне. */
function createPushWindow() {
  if (pushWindow && !pushWindow.isDestroyed()) return pushWindow;
  pushWindow = new BrowserWindow({
    width: 400,
    height: 300,
    show: false,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });
  queueWindowLoad(pushWindow, { forceRefresh: false, enableRetry: true });
  pushWindow.webContents.on('did-fail-load', (event, code) => {
    if (code !== -3 && pushWindow && !pushWindow.isDestroyed()) {
      pushWindow.loadFile(OFFLINE_PATH);
    }
  });
  pushWindow.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
  pushWindow.on('closed', () => {
    stopServerRetry(pushWindow);
    pushWindow = null;
  });
  // Не закрывать при закрытии главного окна — это окно держит SW для push
  return pushWindow;
}

app.whenReady().then(() => {
  if (process.platform === 'darwin') {
    Menu.setApplicationMenu(Menu.buildFromTemplate([
      { role: 'appMenu' },
      { role: 'editMenu' },
      { role: 'viewMenu' },
      { role: 'windowMenu' },
      {
        role: 'help',
        submenu: [{ role: 'about' }],
      },
    ]));
  }
  createWindow();
  // Скрытое окно держит контекст Service Worker — пуш приходит даже когда главное окно закрыто (скрыто)
  setTimeout(() => {
    if (!isQuitting) createPushWindow();
  }, 1500);
});

app.on('before-quit', () => {
  isQuitting = true;
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin' || isQuitting) app.quit();
});

app.on('activate', () => {
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.show();
    if ((!pushWindow || pushWindow.isDestroyed()) && !isQuitting) createPushWindow();
    return;
  }
  const wins = BrowserWindow.getAllWindows();
  if (wins.length === 0) createWindow();
  else wins[0].show();
  if ((!pushWindow || pushWindow.isDestroyed()) && !isQuitting) createPushWindow();
});

ipcMain.handle('reload-app', async () => {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  await queueWindowLoad(mainWindow, { forceRefresh: true, enableRetry: true });
});

ipcMain.handle('set-badge-count', (_, count) => {
  const n = typeof count === 'number' && Number.isFinite(count) ? Math.max(0, Math.floor(count)) : 0;
  app.setBadgeCount(n);
});

ipcMain.handle('show-notification', (_, opts) => {
  if (!Notification.isSupported()) return;
  const title = (opts && typeof opts.title === 'string') ? opts.title : 'BuhChat';
  const body = (opts && typeof opts.body === 'string') ? opts.body : '';
  const n = new Notification({ title, body, silent: false });
  n.on('click', () => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.show();
      mainWindow.focus();
    }
  });
  n.show();
});
