const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
  reloadApp: () => ipcRenderer.invoke('reload-app'),
  setBadgeCount: (count) => ipcRenderer.invoke('set-badge-count', count),
  showNotification: (opts) => ipcRenderer.invoke('show-notification', opts),
});
