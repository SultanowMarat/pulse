import { useEffect, useState, useMemo, useCallback, useRef } from 'react';
import { useAuthStore, useChatStore, useThemeStore, type ThemePreference } from '../store';
import Sidebar from '../components/Sidebar';
import Chat from '../components/Chat';
import AdminPanel from '../components/AdminPanel';
import { Avatar, Modal, IconChat, IconMenu, IconMessageCircle, IconX, IconLogout, IconMail, IconPhone, IconUser, IconSettings, IconBack, IconDownload, IconForward } from '../components/ui';
import * as api from '../api';
import ChatInfo from '../components/ChatInfo';

const LEFT_COMMANDS_HIDDEN_KEY = 'left-commands-hidden';
const ADMIN_ACCESS_CACHE_KEY = 'admin-access-cache';

export default function Pulse() {
  const { user, logout, updateProfile, loadUser, profileLoadError } = useAuthStore();
  const { activeChatId, chats, connectWS, disconnectWS, fetchChats, fetchFavorites, hydrateFavoritesFromStorage, setActiveChat, notification, setNotification, appStatus } = useChatStore();
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [commandsHidden, setCommandsHidden] = useState(false);

  useEffect(() => {
    if (!notification) return;
    const t = setTimeout(() => setNotification(null), 4000);
    return () => clearTimeout(t);
  }, [notification, setNotification]);
  const [showInfo, setShowInfo] = useState(false);
  const [showProfile, setShowProfile] = useState(false);
  const [navTab, setNavTab] = useState<'chats' | 'admin'>('chats');
  const [myAdministrator, setMyAdministrator] = useState(() => {
    try {
      return localStorage.getItem(ADMIN_ACCESS_CACHE_KEY) === '1';
    } catch {
      return false;
    }
  });

  useEffect(() => {
    try {
      setCommandsHidden(localStorage.getItem(LEFT_COMMANDS_HIDDEN_KEY) === '1');
    } catch {
      setCommandsHidden(false);
    }
  }, []);

  const toggleCommands = useCallback(() => {
    setCommandsHidden((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(LEFT_COMMANDS_HIDDEN_KEY, next ? '1' : '0');
      } catch {
        // ignore
      }
      return next;
    });
  }, []);

  const loadUserRequestedRef = useRef(false);
  useEffect(() => {
    if (user != null) loadUserRequestedRef.current = false;
    if (user === null && !profileLoadError && !loadUserRequestedRef.current) {
      loadUserRequestedRef.current = true;
      loadUser();
    }
  }, [user, profileLoadError, loadUser]);

  useEffect(() => {
    const meId = user?.id;
    if (!meId) return;
    let cancelled = false;

    const setAdmin = (next: boolean) => {
      if (cancelled) return;
      setMyAdministrator(next);
      try {
        if (next) localStorage.setItem(ADMIN_ACCESS_CACHE_KEY, '1');
        else localStorage.removeItem(ADMIN_ACCESS_CACHE_KEY);
      } catch {
        // ignore cache errors
      }
    };

    (async () => {
      try {
        const res = await api.getUserPermissions(meId);
        if (res.administrator) {
          setAdmin(true);
          return;
        }
      } catch {
        // continue with capability probe fallback
      }

      // Fallback capability probe: if this endpoint is accessible, user is admin.
      try {
        await api.listEmployeesPage({ limit: 1, offset: 0 });
        setAdmin(true);
      } catch {
        setAdmin(false);
      }
    })();

    return () => { cancelled = true; };
  }, [user?.id]);

  useEffect(() => {
    if (!myAdministrator && navTab === 'admin') {
      setNavTab('chats');
    }
  }, [myAdministrator, navTab]);

  useEffect(() => {
    connectWS();
    if (!user?.id) return () => disconnectWS();
    hydrateFavoritesFromStorage();
    fetchChats().then(() => fetchFavorites());
    return () => disconnectWS();
  }, [user?.id, hydrateFavoritesFromStorage, fetchChats, fetchFavorites, connectWS, disconnectWS]);

  const activeChat = useMemo(() => chats.find((c) => c.chat.id === activeChatId), [chats, activeChatId]);
  const showInfoModal = !!(showInfo && activeChat && activeChat.chat.chat_type === 'group');

  const handleChatSelect = useCallback(() => {
    setSidebarOpen(false);
    setShowInfo(false);
  }, []);

  if (user === null) {
    return (
      <div className="h-full flex items-center justify-center bg-surface dark:bg-dark-bg">
        <div className="text-center max-w-[280px]">
          {profileLoadError ? (
            <>
              <p className="text-txt-secondary dark:text-[#8b98a5] text-[14px] mb-1">Не удалось загрузить профиль</p>
              <p className="text-txt-muted dark:text-[#6e7a86] text-[13px] mb-4 break-words">{profileLoadError}</p>
            </>
          ) : (
            <>
              <div className="inline-block w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin mb-3" />
              <p className="text-txt-secondary dark:text-[#8b98a5] text-[14px]">Загрузка профиля...</p>
            </>
          )}
          <div className="flex flex-col sm:flex-row gap-2 justify-center items-center mt-3">
            <button type="button" onClick={() => loadUser()} className="text-[13px] text-primary hover:underline">
              Повторить
            </button>
            {profileLoadError && (
              <button type="button" onClick={() => logout()} className="text-[13px] text-txt-muted dark:text-[#6e7a86] hover:underline flex items-center gap-1">
                <IconLogout /> Выйти
              </button>
            )}
          </div>
        </div>
      </div>
    );
  }

  if (navTab === 'admin' && myAdministrator) {
    return (
    <div className="h-[var(--app-height)] w-full bg-surface dark:bg-dark-bg safe-x overflow-x-hidden">
        {notification && (
          <div
            className="fixed left-1/2 -translate-x-1/2 z-[100] px-4 py-3 bg-txt text-white text-[13px] font-medium rounded-xl shadow-lg animate-fade max-w-[90vw]"
            style={{ top: 'max(3rem, calc(env(safe-area-inset-top) + 1rem))' }}
          >
            {notification}
          </div>
        )}
        <div className="h-full flex flex-col">
          <header className="shrink-0 border-b border-surface-border dark:border-dark-border px-4 py-3 flex items-center justify-between">
            <button
              type="button"
              onClick={() => setNavTab('chats')}
              className="inline-flex items-center gap-2 min-h-[40px] px-3 rounded-compass bg-sidebar-hover text-white text-[13px] font-semibold hover:brightness-110 transition"
            >
              <IconBack />
              Назад
            </button>
            <div className="flex items-center gap-2">
              <span className="text-sidebar-text"><IconSettings /></span>
              <h1 className="text-[18px] font-bold text-txt dark:text-white">Администрирование</h1>
            </div>
          </header>
          <div className="flex-1 min-h-0">
            <AdminPanel standalone onNotify={(text) => setNotification(text)} />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="h-[var(--app-height)] w-full max-w-[100vw] flex flex-col md:flex-row bg-surface dark:bg-dark-bg safe-x overflow-x-hidden">
      {/* ── Toast: уведомление (участник добавлен/удалён и т.д.) — ниже статус-бара, не перекрывает контент ── */}
      {notification && (
        <div
          className="fixed left-1/2 -translate-x-1/2 z-[100] px-4 py-3 bg-txt text-white text-[13px] font-medium rounded-xl shadow-lg animate-fade max-w-[90vw]"
          style={{ top: 'max(3rem, calc(env(safe-area-inset-top) + 1rem))' }}
        >
          {notification}
        </div>
      )}
      {(appStatus.maintenance || appStatus.degradation || appStatus.read_only) && (
        <div
          className={`fixed left-1/2 -translate-x-1/2 z-[95] px-4 py-2 text-[13px] font-medium rounded-xl shadow-compass-dialog max-w-[92vw] ${
            appStatus.read_only || appStatus.maintenance ? 'bg-amber-600 text-white' : 'bg-blue-600 text-white'
          }`}
          style={{ top: notification ? 'max(6rem, calc(env(safe-area-inset-top) + 4rem))' : 'max(3rem, calc(env(safe-area-inset-top) + 1rem))' }}
        >
          {appStatus.message || (appStatus.read_only ? 'Идёт обслуживание. Отправка сообщений временно отключена.' : 'Возможны задержки в работе сервиса.')}
        </div>
      )}

      {/* ── Nav Rail (кнопки слева): при «Скрыть» скрываем именно их, а не список чатов) ── */}
      <nav className={`hidden md:flex flex-col items-center w-[60px] min-w-[60px] bg-nav shrink-0 py-3 gap-0.5 safe-left ${commandsHidden ? 'md:hidden' : ''}`} style={{ paddingTop: 'max(0.75rem, env(safe-area-inset-top))', paddingBottom: 'env(safe-area-inset-bottom, 0px)' }}>
        <button
          type="button"
          className="w-11 h-11 mb-2 rounded-full flex items-center justify-center border border-white/5 transition-colors bg-[#1f2228] text-[#7f8792] hover:text-white hover:bg-[#262a31]"
          title="Скрыть панель кнопок"
          aria-label="Скрыть панель кнопок"
          onClick={toggleCommands}
        >
          <IconMenu size={20} />
        </button>
        <NavBtn icon={<IconChat />} active={navTab === 'chats'} tip="Чаты"
          onClick={() => { setNavTab('chats'); }} />
        {myAdministrator && (
          <NavBtn icon={<IconSettings />} active={navTab === 'admin'} tip="Администрирование"
            onClick={() => { setNavTab('admin'); }} />
        )}

        <div className="flex-1" />
      </nav>

      {/* ── Sidebar (список чатов): не скрываем при commandsHidden — скрываются только кнопки нав-рейла ── */}
      <div className={`${activeChatId && !sidebarOpen ? 'hidden md:flex' : 'flex'} flex-col w-full max-w-full md:w-[300px] lg:w-[320px] shrink-0 min-w-0 border-r border-surface-border dark:border-dark-border overflow-x-hidden`}>
        <Sidebar onChatSelect={handleChatSelect} onOpenProfile={() => setShowProfile(true)} navHidden={commandsHidden} onShowNav={toggleCommands} />
      </div>

      {/* ── Chat Area ── */}
      <div className={`${!activeChatId || sidebarOpen ? 'hidden md:flex' : 'flex'} flex-1 min-w-0 min-h-0 h-full md:h-auto overflow-hidden overflow-x-hidden transition-[opacity] duration-200 ease-out`}>
        {activeChatId ? (
          <div className="flex flex-1 h-full min-h-0 min-w-0 overflow-hidden">
            <div className="flex-1 flex flex-col min-w-0 overflow-hidden transition-[opacity] duration-150 ease-out">
              <Chat onBack={() => setSidebarOpen(true)} onOpenInfo={() => setShowInfo(true)} onOpenProfile={() => setShowProfile(true)} />
            </div>
          </div>
        ) : (
          <EmptyState />
        )}
      </div>

      {/* Group info should open as centered modal */}
      {showInfoModal && activeChat && (
        <div className="fixed inset-0 z-50 flex items-start sm:items-center justify-center p-2 sm:p-4 safe-area-padding min-h-[var(--app-height)] overflow-y-auto">
          <div className="absolute inset-0 bg-[rgba(4,4,10,0.5)] dark:bg-black/60" onClick={() => setShowInfo(false)} />
          <div className="relative w-full max-w-[420px] h-[calc(var(--app-height)-1rem)] sm:h-[90dvh] max-h-[calc(var(--app-height)-1rem)] sm:max-h-[90dvh] rounded-compass shadow-compass-dialog border border-transparent dark:border-dark-border overflow-hidden bg-white dark:bg-dark-elevated my-2 sm:my-0">
            <ChatInfo chat={activeChat} onClose={() => setShowInfo(false)} />
          </div>
        </div>
      )}

      {showProfile && <ProfileModal onClose={() => setShowProfile(false)} />}
    </div>
  );
}

/* ── Nav Button ── */
function NavBtn({ icon, active, tip, onClick }: { icon: React.ReactNode; active?: boolean; tip?: string; onClick?: () => void }) {
  return (
    <button title={tip} onClick={onClick}
      className={`min-w-[44px] min-h-[44px] w-10 h-10 flex items-center justify-center rounded-[10px] transition-all duration-200 ease-out ${
        active ? 'bg-nav-active text-white' : 'text-sidebar-text hover:text-white hover:bg-nav-hover'
      }`}>
      {icon}
    </button>
  );
}

/* ── Empty State ── */
function EmptyState() {
  return (
    <div className="flex-1 flex flex-col items-center justify-center bg-white dark:bg-dark-bg">
      <div className="w-20 h-20 rounded-full bg-surface dark:bg-dark-elevated flex items-center justify-center mb-4">
        <IconMessageCircle />
      </div>
      <p className="text-txt-secondary dark:text-[#8b98a5] text-[14px]">Выберите чат для начала общения</p>
    </div>
  );
}

/* ── Validation helpers ── */
const PHONE_RE = /^\+\d{8,15}$/;
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function validatePhone(v: string): string {
  if (!v) return '';
  if (!v.startsWith('+')) return 'Номер должен начинаться с +';
  if (!PHONE_RE.test(v)) return 'Неверный формат номера (после + только цифры, 8–15 знаков)';
  return '';
}

function validateEmail(v: string): string {
  if (!v) return '';
  if (!EMAIL_RE.test(v)) return 'Некорректный формат email';
  return '';
}

/* Поле редактирования в стиле первого слайда: иконка слева, подпись сверху, инпут снизу */
function ProfileEditField(
  props: React.InputHTMLAttributes<HTMLInputElement> & {
    icon: React.ReactNode;
    label: string;
    error?: string;
    hint?: string;
  }
) {
  const { icon, label, error, hint, className, ...rest } = props;
  return (
    <div className="flex flex-col gap-1">
      <div className={`flex items-start gap-2.5 sm:gap-3 px-3 sm:px-4 py-2.5 sm:py-3 rounded-[12px] bg-[#f0f0f0] dark:bg-[#2f3336] border border-transparent dark:border-white/10 ${error ? 'ring-2 ring-danger/40' : ''}`}>
        <span className="text-txt-secondary dark:text-[#8b98a5] flex-shrink-0 mt-0.5">{icon}</span>
        <div className="min-w-0 flex-1">
          <p className="text-[10px] sm:text-[11px] text-txt-placeholder dark:text-[#8b98a5] mb-0.5 sm:mb-1">{label}</p>
          <input
            className={`w-full bg-transparent text-[13px] sm:text-[14px] font-medium text-txt dark:text-[#e7e9ea] placeholder:text-txt-placeholder dark:placeholder:text-[#8b98a5] focus:outline-none ${className ?? ''}`}
            {...rest}
          />
        </div>
      </div>
      {error && <p className="text-[12px] text-danger">{error}</p>}
      {!error && hint && <p className="text-[11px] text-txt-placeholder dark:text-[#8b98a5]">{hint}</p>}
    </div>
  );
}

/* ── Profile Modal (стиль первого слайда, с учётом выбранной темы) ── */
function ProfileModal({ onClose }: { onClose: () => void }) {
  const { user, updateProfile, logout } = useAuthStore();
  const { preference: themePreference, setTheme } = useThemeStore();
  const [username, setUsername] = useState(user?.username || '');
  const [email, setEmail] = useState(user?.email || '');
  const [phone, setPhone] = useState(user?.phone || '');
  const [position, setPosition] = useState(user?.position || '');
  const [installLinks, setInstallLinks] = useState<api.InstallLinksConfig | null>(null);
  const [showInstall, setShowInstall] = useState(false);
  const [emailErr, setEmailErr] = useState('');
  const [phoneErr, setPhoneErr] = useState('');
  const [saving, setSaving] = useState(false);
  const { uploadFile } = useChatStore();
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    let cancelled = false;
    api.getInstallLinks()
      .then((res) => { if (!cancelled) setInstallLinks(res); })
      .catch(() => { if (!cancelled) setInstallLinks(null); });
    return () => { cancelled = true; };
  }, []);

  const handleEmailChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value;
    setEmail(v);
    setEmailErr(validateEmail(v));
  }, []);

  const handlePhoneChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    let v = e.target.value;
    v = v.replace(/[^\d+]/g, '');
    if (v.length > 0 && !v.startsWith('+')) v = '+' + v;
    if (v.length > 16) v = v.slice(0, 16);
    setPhone(v);
    setPhoneErr(validatePhone(v));
  }, []);

  const hasErrors = !!emailErr || !!phoneErr;

  const handleSave = useCallback(async () => {
    if (!username.trim()) return;
    const eErr = validateEmail(email);
    const pErr = validatePhone(phone);
    setEmailErr(eErr);
    setPhoneErr(pErr);
    if (eErr || pErr) return;
    setSaving(true);
    try {
      await updateProfile({ username, email, phone, position });
    } catch { /* */ }
    setSaving(false);
    onClose();
  }, [username, email, phone, position, updateProfile, onClose]);

  const handleAvatar = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setSaving(true);
    try {
      const res = await uploadFile(file);
      await updateProfile({ avatar_url: res.url });
    } catch { /* */ }
    setSaving(false);
  }, [uploadFile, updateProfile]);

  const openInstall = useCallback((url: string | undefined | null) => {
    const u = (url || '').trim();
    if (!u) return;
    window.open(u, '_blank', 'noopener,noreferrer');
  }, []);

  const InstallTiles = useCallback(() => {
    const items = [
      { key: 'win', title: 'Windows', subtitle: 'Установить', url: installLinks?.install_windows_url },
      { key: 'and', title: 'Android', subtitle: 'Установить', url: installLinks?.install_android_url },
      { key: 'mac', title: 'macOS', subtitle: 'Установить', url: installLinks?.install_macos_url },
      { key: 'ios', title: 'iPhone', subtitle: 'Установить', url: installLinks?.install_ios_url },
    ];
    return (
      <div className="space-y-2">
        <div className="grid grid-cols-2 gap-2">
          {items.map((it) => {
            const enabled = !!(it.url || '').trim();
            return (
              <button
                key={it.key}
                type="button"
                onClick={() => openInstall(it.url)}
                disabled={!enabled}
                className={`w-full aspect-[4/3] flex flex-col items-start justify-between gap-3 px-4 py-3 rounded-[16px] transition border ${
                  enabled
                    ? 'bg-[#f0f0f0] dark:bg-[#2f3336] hover:bg-[#e8e8e8] dark:hover:bg-white/10 border-transparent dark:border-white/10'
                    : 'bg-[#f0f0f0]/60 dark:bg-[#2f3336]/60 border-transparent dark:border-white/10 opacity-60 cursor-not-allowed'
                }`}
              >
                <div className="w-full flex items-start justify-between gap-3">
                  <span className={`shrink-0 ${enabled ? 'text-primary' : 'text-txt-secondary dark:text-[#8b98a5]'}`}><IconDownload size={22} /></span>
                  <span className={`shrink-0 ${enabled ? 'text-txt-secondary dark:text-[#8b98a5]' : 'text-txt-secondary/70 dark:text-[#8b98a5]/70'}`}><IconForward /></span>
                </div>
                <div className="min-w-0">
                  <div className="text-[14px] font-semibold text-txt dark:text-[#e7e9ea] truncate">{it.title}</div>
                  <div className="text-[12px] text-txt-secondary dark:text-[#8b98a5] truncate">{it.subtitle}</div>
                  {!enabled && <div className="mt-1 text-[11px] text-txt-placeholder dark:text-[#8b98a5]">Ссылка не задана</div>}
                </div>
              </button>
            );
          })}
        </div>
      </div>
    );
  }, [installLinks?.install_android_url, installLinks?.install_ios_url, installLinks?.install_macos_url, installLinks?.install_windows_url, openInstall]);

  return (
    <div className="fixed inset-0 z-50 flex items-start sm:items-center justify-center p-2 sm:p-4 safe-area-padding overflow-y-auto">
      <div className="absolute inset-0 bg-[rgba(4,4,10,0.55)] dark:bg-black/60" onClick={onClose} />
      <div className="relative bg-white dark:bg-dark-elevated rounded-[16px] shadow-compass-dialog w-full max-w-[400px] animate-dialog border border-transparent dark:border-dark-border max-h-[calc(var(--app-height)-1rem)] sm:max-h-[calc(var(--app-height)-2rem)] overflow-hidden flex flex-col my-2 sm:my-0">
        {/* Кнопка закрытия — как на первом слайде, с учётом темы */}
        <button
          onClick={onClose}
          className="absolute top-3 right-3 z-10 w-8 h-8 flex items-center justify-center rounded-full bg-black/10 hover:bg-black/20 dark:bg-white/10 dark:hover:bg-white/20 text-txt dark:text-white transition-colors"
          aria-label="Закрыть"
        >
          <IconX size={14} />
        </button>

        <div className="overflow-y-auto overscroll-contain">
          {/* Шапка: аватар и имя — как первый слайд, градиент в светлой теме */}
          <div className="flex flex-col items-center pt-6 sm:pt-8 pb-4 sm:pb-5 bg-gradient-to-b from-primary/5 to-transparent dark:from-transparent dark:to-transparent">
            <div className="relative cursor-pointer group" onClick={() => fileRef.current?.click()}>
              <Avatar name={user?.username || ''} url={user?.avatar_url} size={80} online className="sm:!w-24 sm:!h-24" />
              <div className="absolute inset-0 rounded-full bg-black/0 group-hover:bg-black/30 dark:group-hover:bg-black/40 transition-colors flex items-center justify-center">
                <span className="text-white text-[11px] font-medium opacity-0 group-hover:opacity-100 transition-opacity">Изменить</span>
              </div>
            </div>
            <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={handleAvatar} />
            <h2 className="mt-3 text-[18px] sm:text-[20px] font-bold text-txt dark:text-[#e7e9ea] text-center px-10">{username || user?.username || 'Профиль'}</h2>
            <p className="text-[11px] sm:text-[12px] text-txt-placeholder dark:text-[#8b98a5] mt-1">Нажмите для смены аватара</p>
          </div>

          <div className="px-4 sm:px-5 pb-4 sm:pb-5 space-y-3 sm:space-y-4">
            {/* Тема — кнопки в стиле первого слайда (скруглённые блоки), выбранная = primary */}
            <div>
              <label className="block text-[12px] sm:text-[13px] font-medium text-txt-secondary dark:text-[#8b98a5] mb-2">Тема</label>
              <div className="flex gap-2 flex-wrap">
                {(['light', 'dark', 'system'] as ThemePreference[]).map((t) => (
                  <button
                    key={t}
                    type="button"
                    onClick={() => setTheme(t)}
                    className={`flex-1 min-w-0 px-2.5 sm:px-3 py-2 sm:py-2.5 rounded-[12px] text-[12px] sm:text-[13px] font-medium transition-colors ${
                      themePreference === t
                        ? 'bg-primary text-white'
                        : 'bg-[#f0f0f0] dark:bg-[#2f3336] text-txt dark:text-[#e7e9ea] hover:bg-[#e8e8e8] dark:hover:bg-white/10 border border-transparent dark:border-white/10'
                    }`}
                  >
                    {t === 'light' ? 'Светлая' : t === 'dark' ? 'Тёмная' : 'Как в системе'}
                  </button>
                ))}
              </div>
            </div>

            {/* Поля в стиле первого слайда: иконка слева, подпись сверху, инпут */}
            <ProfileEditField
              icon={<IconUser size={20} />}
              label="Имя пользователя"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="Имя"
            />
            <ProfileEditField
              icon={<IconMail size={20} />}
              label="Адрес электронной почты"
              value={email}
              onChange={handleEmailChange}
              placeholder="user@example.com"
              type="email"
              error={emailErr}
            />
            <ProfileEditField
              icon={<IconPhone size={20} />}
              label="Номер телефона"
              value={phone}
              onChange={handlePhoneChange}
              placeholder="+7 999 123-45-67"
              type="tel"
              error={phoneErr}
              hint="Международный формат: + и цифры"
            />
            <ProfileEditField
              icon={<IconUser size={20} />}
              label="Должность (опционально)"
              value={position}
              onChange={(e) => setPosition(e.target.value)}
              placeholder="Должность"
            />

            {/* Установка клиента (отдельное окно) */}
            <button
              type="button"
              onClick={() => setShowInstall(true)}
              className="w-full flex items-center justify-between gap-3 px-3 sm:px-4 py-2.5 sm:py-3 rounded-[12px] bg-[#f0f0f0] dark:bg-[#2f3336] hover:bg-[#e8e8e8] dark:hover:bg-white/10 transition-colors border border-transparent dark:border-white/10"
            >
              <div className="flex items-center gap-3 min-w-0">
                <span className="text-primary shrink-0"><IconDownload size={20} /></span>
                <div className="min-w-0">
                  <div className="text-[12px] sm:text-[13px] font-semibold text-txt dark:text-[#e7e9ea] truncate">Установка</div>
                  <div className="text-[10px] sm:text-[11px] text-txt-secondary dark:text-[#8b98a5] truncate">Открыть варианты установки</div>
                </div>
              </div>
              <span className="text-txt-secondary dark:text-[#8b98a5] shrink-0"><IconForward /></span>
            </button>

            {/* Футер: Сохранить (primary), Выйти (красный текст, как на втором слайде) */}
            <div className="flex gap-2.5 sm:gap-3 pt-1 sm:pt-2">
              <button
                onClick={handleSave}
                disabled={saving || !username.trim() || hasErrors}
                className="flex-1 py-2 sm:py-2.5 rounded-[12px] font-semibold text-[14px] sm:text-[15px] bg-primary text-white hover:bg-primary-hover disabled:opacity-50 disabled:pointer-events-none transition-colors"
              >
                Сохранить
              </button>
              <button
                onClick={() => { logout(); onClose(); }}
                className="flex items-center gap-1.5 px-3 sm:px-4 py-2 sm:py-2.5 text-danger hover:bg-danger/5 dark:hover:bg-danger/10 rounded-[12px] transition-colors"
              >
                <IconLogout />
                <span className="text-[13px] sm:text-[14px] font-semibold">Выйти</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Install modal */}
      {showInstall && (
        <div className="fixed inset-0 z-[70] flex items-start sm:items-center justify-center p-2 sm:p-4 safe-area-padding min-h-[var(--app-height)] overflow-y-auto">
          <div className="absolute inset-0 bg-[rgba(4,4,10,0.55)] dark:bg-black/60" onClick={() => setShowInstall(false)} />
          <div className="relative w-full max-w-[420px] rounded-[16px] shadow-compass-dialog border border-transparent dark:border-dark-border bg-white dark:bg-dark-elevated animate-dialog overflow-hidden max-h-[calc(var(--app-height)-1rem)] sm:max-h-[90dvh] my-2 sm:my-0">
            <div className="flex items-center justify-between px-5 pt-4 pb-2">
              <h3 className="text-[17px] font-bold text-txt dark:text-[#e7e9ea] leading-[26px]">Установка</h3>
              <button
                type="button"
                onClick={() => setShowInstall(false)}
                className="min-w-[44px] min-h-[44px] flex items-center justify-center rounded-full hover:bg-surface-light dark:hover:bg-dark-hover transition-colors text-txt-secondary hover:text-txt dark:text-dark-muted dark:hover:text-[#e7e9ea] -mr-2"
                aria-label="Закрыть"
              >
                <IconX size={14} />
              </button>
            </div>
            <div className="px-5 pb-5">
              <InstallTiles />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

