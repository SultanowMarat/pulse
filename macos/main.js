const { app, BrowserWindow, ipcMain, Menu, Notification, session } = require('electron');
const path = require('path');
const fs = require('fs');

// Интерфейс всегда грузится с сервера, чтобы при обновлении кода все клиенты получали новый UI.
// API вызывается на том же origin (см. serverUrl.ts). Офлайн/fallback: вшитое приложение или offline.html.
const APP_URL = (process.env.BUHCHAT_APP_URL || 'https://buhchat.com').trim();
const BUNDLED_APP_PATH = path.join(__dirname, 'app', 'index.html');
const OFFLINE_PATH = path.join(__dirname, 'gate', 'offline.html');

let isQuitting = false;
/** Главное окно приложения (для показа при фокусе после клика по пушу). */
let mainWindow = null;

/** Сбрасывает кеш сессии, чтобы приложение подтянуло свежий интерфейс с сервера. */
function clearCache() {
  return session.defaultSession.clearCache();
}

/** Загружает окно: сначала с сервера (со сбросом кеша), при ошибке — вшитое приложение или offline. */
async function loadWindow(win) {
  await clearCache();
  try {
    await win.loadURL(APP_URL);
  } catch (e) {
    if (fs.existsSync(BUNDLED_APP_PATH)) {
      await win.loadFile(BUNDLED_APP_PATH).catch(() => win.loadFile(OFFLINE_PATH));
    } else {
      await win.loadFile(OFFLINE_PATH);
    }
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

  loadWindow(mainWindow);

  // При фокусе на главное окно (например после клика по пушу) — показываем его, если было скрыто
  mainWindow.on('focus', () => {
    if (mainWindow && !mainWindow.isDestroyed() && !mainWindow.isVisible()) mainWindow.show();
  });

  // Cmd+R / Cmd+Shift+R / Ctrl+R: сброс кеша и загрузка с сервера, чтобы интерфейс обновился
  mainWindow.webContents.on('before-input-event', (event, input) => {
    const isReload = (input.control || input.meta) && input.key && input.key.toLowerCase() === 'r';
    if (!isReload) return;
    event.preventDefault();
    clearCache().then(() => {
      // Всегда грузим с сервера при Reload, иначе при file:// просто перезагрузится старый bundle
      mainWindow.loadURL(APP_URL).catch(() => {
        if (fs.existsSync(BUNDLED_APP_PATH)) mainWindow.loadFile(BUNDLED_APP_PATH);
        else mainWindow.loadFile(OFFLINE_PATH);
      });
    });
  });

  const onFail = () => {
    mainWindow.webContents.removeListener('did-fail-load', onFail);
    mainWindow.loadFile(OFFLINE_PATH);
  };
  mainWindow.webContents.on('did-fail-load', (event, code) => {
    if (code !== -3) onFail();
  });

  mainWindow.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));

  // Красная кнопка (крестик) — скрыть окно; Cmd+Q или «Завершить» — полный выход.
  if (process.platform === 'darwin') {
    mainWindow.on('close', (e) => {
      if (isQuitting || mainWindow.isFullScreen()) return;
      e.preventDefault();
      mainWindow.hide();
    });
  }
}

/** Создаёт скрытое окно, которое держит контекст для Service Worker, чтобы пуш приходил при свёрнутом/закрытом главном окне. */
function createPushWindow() {
  const pushWin = new BrowserWindow({
    width: 400,
    height: 300,
    show: false,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });
  loadWindow(pushWin);
  pushWin.webContents.on('did-fail-load', (event, code) => {
    if (code !== -3) pushWin.loadFile(OFFLINE_PATH);
  });
  pushWin.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
  // Не закрывать при закрытии главного окна — это окно держит SW для push
  return pushWin;
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
  createPushWindow();
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
    return;
  }
  const wins = BrowserWindow.getAllWindows();
  if (wins.length === 0) createWindow();
  else wins[0].show();
});

ipcMain.handle('reload-app', async () => {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  await loadWindow(mainWindow);
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
