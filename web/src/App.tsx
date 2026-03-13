import { useEffect } from 'react';
import { useAuthStore, useChatStore, useThemeStore } from './store';
import Auth from './pages/Auth';
import Pulse from './pages/Pulse';
import { registerPushIfEnabled, requestNotificationPermissionForPWA, startPushBackgroundMaintenance } from './push';
import { setupMobileViewportManager } from './mobileViewport';

export default function App() {
  const { isAuthenticated, init } = useAuthStore();
  const themeInit = useThemeStore((s) => s.init);

  useEffect(() => {
    themeInit();
  }, [themeInit]);
  useEffect(() => {
    return setupMobileViewportManager();
  }, []);
  useEffect(() => {
    init();
  }, [init]);
  useEffect(() => {
    const t = setTimeout(() => {
      useChatStore.getState().loadCacheConfig().catch(() => {});
      useChatStore.getState().loadFileSettings().catch(() => {});
    }, 500);
    return () => clearTimeout(t);
  }, []);
  useEffect(() => {
    if (!isAuthenticated) return;
    const load = () => useChatStore.getState().loadAppStatus().catch(() => {});
    load();
    const id = setInterval(load, 30000);
    return () => clearInterval(id);
  }, [isAuthenticated]);
  useEffect(() => {
    if (!isAuthenticated) return;
    const t = setTimeout(() => {
      requestNotificationPermissionForPWA().catch(() => {});
      registerPushIfEnabled().catch(() => {});
    }, 1000);
    return () => clearTimeout(t);
  }, [isAuthenticated]);
  useEffect(() => {
    if (!isAuthenticated) return;
    return startPushBackgroundMaintenance();
  }, [isAuthenticated]);

  if (!isAuthenticated) return <Auth />;
  return <Pulse />;
}
