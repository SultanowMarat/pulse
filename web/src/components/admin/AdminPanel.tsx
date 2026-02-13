import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import * as api from '../../api';
import type { UserPublic } from '../../types';
import { Avatar, IconDownload, IconFile, IconMail, IconUser, IconX } from '../ui';
import { useChatStore } from '../../store';

type MailForm = {
  host: string;
  port: string;
  username: string;
  password: string;
  from_email: string;
  from_name: string;
};

const emptyMailForm: MailForm = {
  host: '',
  port: '',
  username: '',
  password: '',
  from_email: '',
  from_name: '',
};

type SectionKey = 'users' | 'links' | 'mail' | 'files' | 'service' | 'backup';

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

type UserSortKey = 'username' | 'email' | 'phone' | 'status' | 'last_seen_at' | 'role';
type SortDir = 'asc' | 'desc';

function fmtLastActivity(u: UserPublic): string {
  if (u.is_online) return 'Сейчас онлайн';
  const s = (u.last_seen_at || '').trim();
  if (!s) return '—';
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return '—';
  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(d);
}

function cmpText(a: string, b: string): number {
  return a.localeCompare(b, 'ru', { sensitivity: 'base' });
}

function roleLabel(role: string): string {
  if (role === 'administrator') return 'Администратор';
  if (role === 'assistant_administrator') return 'Помощник администратора';
  return 'Пользователь';
}

export default function AdminPanel({ onNotify, standalone = false }: { onNotify: (text: string) => void; standalone?: boolean }) {
  const syncMaxFileSize = useChatStore((s) => s.loadFileSettings);
  const reloadAppStatus = useChatStore((s) => s.loadAppStatus);
  const [section, setSection] = useState<SectionKey>('mail');

  const [loadingMail, setLoadingMail] = useState(true);
  const [savingMail, setSavingMail] = useState(false);
  const [testingMail, setTestingMail] = useState(false);
  const [mailForm, setMailForm] = useState<MailForm>(emptyMailForm);
  const [mailError, setMailError] = useState('');
  const [testEmail, setTestEmail] = useState('');

  const [loadingFiles, setLoadingFiles] = useState(true);
  const [savingFiles, setSavingFiles] = useState(false);
  const [fileError, setFileError] = useState('');
  const [maxFileSizeMB, setMaxFileSizeMB] = useState('20');

  const [loadingUsers, setLoadingUsers] = useState(true);
  const [usersError, setUsersError] = useState('');
  const [users, setUsers] = useState<api.EmployeePublic[]>([]);
  const [search, setSearch] = useState('');
  const [sortKey, setSortKey] = useState<UserSortKey>('username');
  const [sortDir, setSortDir] = useState<SortDir>('asc');
  const [usersTotal, setUsersTotal] = useState(0);
  const [usersOffset, setUsersOffset] = useState(0);
  const [hasMoreUsers, setHasMoreUsers] = useState(true);
  const [loadingMoreUsers, setLoadingMoreUsers] = useState(false);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const usersInitRef = useRef(false);
  const loadMoreLockRef = useRef(false);

  const [addOpen, setAddOpen] = useState(false);
  const [editUser, setEditUser] = useState<UserPublic | null>(null);

  const [creatingBackup, setCreatingBackup] = useState(false);
  const [restoringBackup, setRestoringBackup] = useState(false);
  const [backupFile, setBackupFile] = useState<File | null>(null);
  const [backupError, setBackupError] = useState('');

  const [loadingService, setLoadingService] = useState(true);
  const [savingService, setSavingService] = useState(false);
  const [serviceError, setServiceError] = useState('');
  const [serviceForm, setServiceForm] = useState<api.ServiceSettings>({
    maintenance: false,
    read_only: false,
    degradation: false,
    status_message: '',
    cors_allowed_origins: '*',
    install_windows_url: '',
    install_android_url: '',
    install_macos_url: '',
    install_ios_url: '',
    max_ws_connections: 10000,
    ws_send_buffer_size: 256,
    ws_write_timeout: 10,
    ws_pong_timeout: 60,
    ws_max_message_size: 4096,
  });

  const loadServiceSettings = useCallback(async () => {
    setLoadingService(true);
    setServiceError('');
    try {
      const s = await api.getServiceSettings();
      setServiceForm({
        maintenance: !!s.maintenance,
        read_only: !!s.read_only,
        degradation: !!s.degradation,
        status_message: s.status_message || '',
        cors_allowed_origins: s.cors_allowed_origins || '*',
        install_windows_url: s.install_windows_url || '',
        install_android_url: s.install_android_url || '',
        install_macos_url: s.install_macos_url || '',
        install_ios_url: s.install_ios_url || '',
        max_ws_connections: Number(s.max_ws_connections) || 10000,
        ws_send_buffer_size: Number(s.ws_send_buffer_size) || 256,
        ws_write_timeout: Number(s.ws_write_timeout) || 10,
        ws_pong_timeout: Number(s.ws_pong_timeout) || 60,
        ws_max_message_size: Number(s.ws_max_message_size) || 4096,
      });
    } catch (e: unknown) {
      setServiceError(e instanceof Error ? e.message : 'Не удалось загрузить служебные настройки');
    } finally {
      setLoadingService(false);
    }
  }, []);

  const onSaveServiceSettings = useCallback(async () => {
    setSavingService(true);
    setServiceError('');
    try {
      const v = serviceForm;
      if (!Number.isInteger(v.max_ws_connections) || v.max_ws_connections < 2000) throw new Error('max_ws_connections: минимум 2000');
      if (!Number.isInteger(v.ws_send_buffer_size) || v.ws_send_buffer_size < 64) throw new Error('ws_send_buffer_size: минимум 64');
      if (!Number.isInteger(v.ws_write_timeout) || v.ws_write_timeout < 1) throw new Error('ws_write_timeout: минимум 1');
      if (!Number.isInteger(v.ws_pong_timeout) || v.ws_pong_timeout < 5) throw new Error('ws_pong_timeout: минимум 5');
      if (!Number.isInteger(v.ws_max_message_size) || v.ws_max_message_size < 1024) throw new Error('ws_max_message_size: минимум 1024');
      await api.updateServiceSettings(v);
      // Apply maintenance/read-only banner immediately in current UI.
      reloadAppStatus();
      onNotify('Служебные настройки сохранены');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Не удалось сохранить служебные настройки';
      setServiceError(msg);
      onNotify(`Ошибка: ${msg}`);
    } finally {
      setSavingService(false);
    }
  }, [serviceForm, onNotify, reloadAppStatus]);

  const loadMail = useCallback(async () => {
    setLoadingMail(true);
    setMailError('');
    try {
      const ms = await api.getMailSettings();
      setMailForm({
        host: ms.host || '',
        port: ms.port > 0 ? String(ms.port) : '',
        username: ms.username || '',
        password: ms.password || '',
        from_email: ms.from_email || '',
        from_name: ms.from_name || '',
      });
    } catch (e: unknown) {
      setMailError(e instanceof Error ? e.message : 'Не удалось загрузить настройки почты');
    } finally {
      setLoadingMail(false);
    }
  }, []);

  const loadFiles = useCallback(async () => {
    setLoadingFiles(true);
    setFileError('');
    try {
      const fs = await api.getAdminFileSettings();
      setMaxFileSizeMB(String(fs.max_file_size_mb || 20));
    } catch (e: unknown) {
      setFileError(e instanceof Error ? e.message : 'Не удалось загрузить настройки файлов');
    } finally {
      setLoadingFiles(false);
    }
  }, []);

  const fetchUsers = useCallback(async (offset: number, reset: boolean) => {
    const limit = 50;
    if (reset) {
      setLoadingUsers(true);
      setUsersError('');
    } else {
      setLoadingMoreUsers(true);
    }
    try {
      const res = await api.listEmployeesPage({
        q: search.trim() || undefined,
        limit,
        offset,
        sort_key: sortKey,
        sort_dir: sortDir,
      });
      setUsersTotal(res.total || 0);
      const nextOffset = offset + (res.users?.length || 0);
      setUsersOffset(nextOffset);
      setHasMoreUsers(nextOffset < (res.total || 0));
      setUsers((prev) => (reset ? (res.users || []) : [...prev, ...(res.users || [])]));
    } catch (e: unknown) {
      setUsersError(e instanceof Error ? e.message : 'Не удалось загрузить пользователей');
      setHasMoreUsers(false);
    } finally {
      setLoadingUsers(false);
      setLoadingMoreUsers(false);
    }
  }, [search, sortKey, sortDir]);

  const reloadUsers = useCallback(() => {
    loadMoreLockRef.current = false;
    setUsers([]);
    setUsersTotal(0);
    setUsersOffset(0);
    setHasMoreUsers(true);
    // Reset scroll to top so "infinite" starts from beginning after new query/sort.
    if (scrollRef.current) scrollRef.current.scrollTop = 0;
    fetchUsers(0, true);
  }, [fetchUsers]);

  const loadMoreUsers = useCallback(() => {
    if (loadMoreLockRef.current || loadingUsers || loadingMoreUsers || !hasMoreUsers) return;
    loadMoreLockRef.current = true;
    void fetchUsers(usersOffset, false).finally(() => {
      loadMoreLockRef.current = false;
    });
  }, [fetchUsers, hasMoreUsers, loadingMoreUsers, loadingUsers, usersOffset]);

  useEffect(() => {
    loadMail();
    loadFiles();
  }, [loadMail, loadFiles]);

  useEffect(() => {
    if (section !== 'service' && section !== 'links') return;
    loadServiceSettings();
  }, [section, loadServiceSettings]);

  useEffect(() => {
    if (section !== 'users') {
      usersInitRef.current = false;
      return;
    }
    usersInitRef.current = true;
    reloadUsers();
  }, [section, reloadUsers]);

  // Debounced search for server-side filtering.
  useEffect(() => {
    if (section !== 'users' || !usersInitRef.current) return;
    const t = setTimeout(() => reloadUsers(), 250);
    return () => clearTimeout(t);
  }, [search, section, reloadUsers]);

  const toggleSort = useCallback((key: UserSortKey) => {
    setSortKey((prevKey) => {
      if (prevKey !== key) {
        setSortDir('asc');
        return key;
      }
      setSortDir((prevDir) => (prevDir === 'asc' ? 'desc' : 'asc'));
      return prevKey;
    });
  }, []);

  // When sort changes, reload server-side list immediately (no debounce).
  useEffect(() => {
    if (section !== 'users' || !usersInitRef.current) return;
    reloadUsers();
  }, [sortKey, sortDir, section, reloadUsers]);

  const setMailField = (k: keyof MailForm, v: string) => setMailForm((prev) => ({ ...prev, [k]: v }));

  const onSaveMail = useCallback(async () => {
    setSavingMail(true);
    setMailError('');
    try {
      await api.updateMailSettings({
        host: mailForm.host.trim(),
        port: Number(mailForm.port) || 0,
        username: mailForm.username.trim(),
        password: mailForm.password,
        from_email: mailForm.from_email.trim().toLowerCase(),
        from_name: mailForm.from_name.trim(),
      });
      onNotify('Настройки почты сохранены');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Ошибка сохранения почтовых настроек';
      setMailError(msg);
      onNotify(`Ошибка: ${msg}`);
    } finally {
      setSavingMail(false);
    }
  }, [mailForm, onNotify]);

  const onTestMail = useCallback(async () => {
    if (!testEmail.trim()) {
      setMailError('Укажите email для тестового письма');
      return;
    }
    setTestingMail(true);
    setMailError('');
    try {
      await api.sendTestMail(testEmail);
      onNotify('Тестовое письмо отправлено успешно');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Ошибка отправки тестового письма';
      setMailError(msg);
      onNotify(`Ошибка: ${msg}`);
    } finally {
      setTestingMail(false);
    }
  }, [testEmail, onNotify]);

  const onSaveFiles = useCallback(async () => {
    setSavingFiles(true);
    setFileError('');
    try {
      const v = Number(maxFileSizeMB);
      if (!Number.isInteger(v) || v < 1 || v > 200) {
        throw new Error('Введите целое число от 1 до 200 МБ');
      }
      await api.updateAdminFileSettings(v);
      useChatStore.setState({ maxFileSizeMB: v });
      await syncMaxFileSize();
      onNotify('Ограничение размера файла сохранено');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Ошибка сохранения настроек файлов';
      setFileError(msg);
      onNotify(`Ошибка: ${msg}`);
    } finally {
      setSavingFiles(false);
    }
  }, [maxFileSizeMB, onNotify, syncMaxFileSize]);

  return (
    <div className={`h-full w-full min-h-0 flex flex-col md:flex-row ${standalone ? 'bg-surface dark:bg-dark-bg' : 'bg-sidebar'}`}>
      <aside className="w-full md:w-[250px] md:shrink-0 border-b md:border-b-0 md:border-r border-surface-border dark:border-dark-border p-2 sm:p-3">
        <div className="flex md:flex-col gap-2 overflow-x-auto md:overflow-visible pb-1 md:pb-0">
          <NavItem active={section === 'users'} icon={<IconUser size={16} />} title="Пользователи" subtitle="Список и управление" onClick={() => setSection('users')} />
          <NavItem active={section === 'links'} icon={<IconDownload size={16} />} title="Ссылки" subtitle="Установка клиентов" onClick={() => setSection('links')} />
          <NavItem active={section === 'mail'} icon={<IconMail size={16} />} title="Почта" subtitle="SMTP и тестовое письмо" onClick={() => setSection('mail')} />
          <NavItem active={section === 'files'} icon={<IconFile size={16} />} title="Файлы" subtitle="Лимит размера" onClick={() => setSection('files')} />
          <NavItem active={section === 'service'} icon={<IconFile size={16} />} title="Служебные настройки" subtitle="ENV/yaml -> UI" onClick={() => setSection('service')} />
          <NavItem active={section === 'backup'} icon={<IconFile size={16} />} title="Резервное копирование" subtitle="Backup и restore" onClick={() => setSection('backup')} />
        </div>
      </aside>

      <main className="flex-1 min-w-0 min-h-0 overflow-y-auto p-3 sm:p-4">
        {section === 'links' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {loadingService ? (
              <p className="text-[13px] text-sidebar-text">Загрузка...</p>
            ) : (
              <>
                {serviceError && <ErrorBox text={serviceError} />}
                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">Ссылки установки</h3>
                  <p className="text-[12px] text-sidebar-text">
                    Эти ссылки открываются в профиле пользователя при выборе платформы (Windows/Android/macOS/iPhone).
                  </p>
                  <Field
                    label="Windows"
                    value={serviceForm.install_windows_url}
                    onChange={(v) => setServiceForm((p) => ({ ...p, install_windows_url: v }))}
                    placeholder="https://..."
                    hint="Ссылка на установщик или страницу загрузки для Windows. Откроется по кнопке в профиле."
                  />
                  <Field
                    label="Android"
                    value={serviceForm.install_android_url}
                    onChange={(v) => setServiceForm((p) => ({ ...p, install_android_url: v }))}
                    placeholder="https://..."
                    hint="Ссылка на Google Play или APK. Откроется по кнопке в профиле."
                  />
                  <Field
                    label="macOS"
                    value={serviceForm.install_macos_url}
                    onChange={(v) => setServiceForm((p) => ({ ...p, install_macos_url: v }))}
                    placeholder="https://..."
                    hint="Ссылка на DMG/PKG или страницу загрузки для macOS. Откроется по кнопке в профиле."
                  />
                  <Field
                    label="iPhone (iOS)"
                    value={serviceForm.install_ios_url}
                    onChange={(v) => setServiceForm((p) => ({ ...p, install_ios_url: v }))}
                    placeholder="https://..."
                    hint="Ссылка на App Store или страницу установки. Откроется по кнопке в профиле."
                  />
                  <button
                    type="button"
                    onClick={onSaveServiceSettings}
                    disabled={savingService}
                    className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60"
                  >
                    {savingService ? 'Сохранение...' : 'Сохранить ссылки'}
                  </button>
                </section>
              </>
            )}
          </div>
        )}

        {section === 'mail' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {loadingMail ? (
              <p className="text-[13px] text-sidebar-text">Загрузка...</p>
            ) : (
              <>
                {mailError && <ErrorBox text={mailError} />}
                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">Настройка почты</h3>
                  <Field label="SMTP Host" value={mailForm.host} onChange={(v) => setMailField('host', v)} placeholder="smtp.yandex.ru" />
                  <Field label="SMTP Port" value={mailForm.port} onChange={(v) => setMailField('port', v.replace(/\D/g, ''))} placeholder="587" />
                  <Field label="Логин (SMTP Username)" value={mailForm.username} onChange={(v) => setMailField('username', v)} placeholder="user@example.com" />
                  <Field label="Пароль (SMTP Password)" value={mailForm.password} onChange={(v) => setMailField('password', v)} placeholder="password" type="password" />
                  <Field label="From Email" value={mailForm.from_email} onChange={(v) => setMailField('from_email', v)} placeholder="noreply@example.com" />
                  <Field label="From Name" value={mailForm.from_name} onChange={(v) => setMailField('from_name', v)} placeholder="BuhChat Auth" />
                  <button type="button" onClick={onSaveMail} disabled={savingMail} className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60">
                    {savingMail ? 'Сохранение...' : 'Сохранить настройки'}
                  </button>
                  <p className="text-[12px] text-sidebar-text">Если поля SMTP пустые, вход будет без отправки кода.</p>
                </section>

                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[13px] font-semibold text-white">Тестовое письмо</h3>
                  <Field label="Email получателя" value={testEmail} onChange={setTestEmail} placeholder="test@example.com" />
                  <button type="button" onClick={onTestMail} disabled={testingMail} className="w-full min-h-[42px] rounded-compass bg-[#16a34a] text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60">
                    {testingMail ? 'Отправка...' : 'Отправить тестовое письмо'}
                  </button>
                </section>
              </>
            )}
          </div>
        )}

        {section === 'files' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {loadingFiles ? (
              <p className="text-[13px] text-sidebar-text">Загрузка...</p>
            ) : (
              <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                <h3 className="text-[14px] font-semibold text-white">Настройки файлов</h3>
                {fileError && <ErrorBox text={fileError} />}
                <Field
                  label="Максимальный размер файла для отправки сообщений (МБ)"
                  value={maxFileSizeMB}
                  onChange={(v) => setMaxFileSizeMB(v.replace(/\D/g, ''))}
                  placeholder="20"
                />
                <button type="button" onClick={onSaveFiles} disabled={savingFiles} className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60">
                  {savingFiles ? 'Сохранение...' : 'Сохранить настройки'}
                </button>
                <p className="text-[12px] text-sidebar-text">Диапазон: от 1 до 200 МБ.</p>
              </section>
            )}
          </div>
        )}

        {section === 'users' && (
          <div className="max-w-[980px] mx-auto space-y-3">
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Поиск пользователя"
                className="w-full h-10 rounded-compass bg-sidebar border border-sidebar-border/40 px-3 text-[13px] text-white placeholder:text-sidebar-text/80 outline-none focus:border-primary/60"
              />
              <button
                type="button"
                onClick={() => setAddOpen(true)}
                className="h-10 px-4 rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition"
              >
                Добавить
              </button>
            </div>

            {usersError && <ErrorBox text={usersError} />}

            <div className="rounded-compass border border-sidebar-border/40 overflow-hidden">
              <div
                ref={scrollRef}
                className="overflow-y-auto overflow-x-auto dark-scroll"
                style={{ maxHeight: 'calc(100dvh - 260px)' }}
                onScroll={(e) => {
                  const el = e.currentTarget;
                  // Do not trigger infinite loading when list does not actually overflow vertically.
                  if (el.scrollHeight <= el.clientHeight + 2) return;
                  const remain = el.scrollHeight - el.scrollTop - el.clientHeight;
                  if (remain < 220) loadMoreUsers();
                }}
              >
                <table className="w-full min-w-[920px] text-left text-[13px]">
                  <thead className="bg-sidebar-hover/60 text-sidebar-text sticky top-0 z-10">
                  <tr>
                    <th className="px-3 py-2">Фото</th>
                    <SortableTH label="Имя" active={sortKey === 'username'} dir={sortDir} onClick={() => toggleSort('username')} />
                    <SortableTH label="Email" active={sortKey === 'email'} dir={sortDir} onClick={() => toggleSort('email')} />
                    <SortableTH label="Телефон" active={sortKey === 'phone'} dir={sortDir} onClick={() => toggleSort('phone')} />
                    <SortableTH label="Роль" active={sortKey === 'role'} dir={sortDir} onClick={() => toggleSort('role')} />
                    <SortableTH label="Статус" active={sortKey === 'status'} dir={sortDir} onClick={() => toggleSort('status')} />
                    <SortableTH label="Последняя активность" active={sortKey === 'last_seen_at'} dir={sortDir} onClick={() => toggleSort('last_seen_at')} />
                    <th className="px-3 py-2 text-right">Действия</th>
                  </tr>
                </thead>
                <tbody>
                  {loadingUsers && (
                    <tr>
                      <td colSpan={8} className="px-3 py-6 text-center text-sidebar-text">Загрузка...</td>
                    </tr>
                  )}
                  {!loadingUsers && users.length === 0 && (
                    <tr>
                      <td colSpan={8} className="px-3 py-6 text-center text-sidebar-text">Пользователи не найдены</td>
                    </tr>
                  )}
                  {!loadingUsers && users.map((u) => (
                    <tr key={u.id} className="border-t border-sidebar-border/30">
                      <td className="px-3 py-2">
                        <Avatar name={u.username} url={u.avatar_url || undefined} size={32} />
                      </td>
                      <td className="px-3 py-2 text-white">{u.username}</td>
                      <td className="px-3 py-2 text-white/90">{u.email || '—'}</td>
                      <td className="px-3 py-2 text-white/90">{u.phone || '—'}</td>
                      <td className="px-3 py-2 text-white/90">{roleLabel(u.role)}</td>
                      <td className="px-3 py-2">
                        <span className={`inline-flex px-2 py-1 rounded-full text-[11px] font-semibold ${u.disabled_at ? 'bg-danger/20 text-danger' : 'bg-green/20 text-green-300'}`}>
                          {u.disabled_at ? 'Отключен' : 'Активен'}
                        </span>
                      </td>
                      <td className="px-3 py-2 text-white/80">{fmtLastActivity(u)}</td>
                      <td className="px-3 py-2 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <button type="button" onClick={() => setEditUser(u)} className="h-8 px-3 rounded-compass bg-sidebar-hover text-white text-[12px] font-medium hover:brightness-110 transition">
                            Изменить
                          </button>
                          <button
                            type="button"
                            onClick={async () => {
                              if (!confirm('Сбросить сессии пользователя на всех устройствах?')) return;
                              try {
                                const res = await api.logoutAllUserSessions(u.id);
                                onNotify(`Сессии сброшены (отозвано: ${res.revoked || 0})`);
                              } catch (e: unknown) {
                                const msg = e instanceof Error ? e.message : 'Не удалось сбросить сессии';
                                onNotify(`Ошибка: ${msg}`);
                              }
                            }}
                            className="h-8 px-3 rounded-compass bg-danger/20 text-danger text-[12px] font-medium hover:brightness-110 transition"
                            title="Выкинуть пользователя со всех устройств"
                          >
                            Сбросить
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                  {!loadingUsers && loadingMoreUsers && (
                    <tr>
                      <td colSpan={8} className="px-3 py-4 text-center text-sidebar-text">Загрузка...</td>
                    </tr>
                  )}
                </tbody>
              </table>
              </div>
            </div>
            {!loadingUsers && usersTotal > 0 && (
              <p className="text-[12px] text-sidebar-text">
                Показано: {users.length} из {usersTotal}
              </p>
            )}
          </div>
        )}

        {section === 'service' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {loadingService ? (
              <p className="text-[13px] text-sidebar-text">Загрузка...</p>
            ) : (
              <>
                {serviceError && <ErrorBox text={serviceError} />}
                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">Состояние приложения</h3>
                  <p className="text-[12px] text-sidebar-text">
                    Эти настройки заменяют переменные окружения `APP_MAINTENANCE`, `APP_READ_ONLY`, `APP_DEGRADATION`, `APP_STATUS_MESSAGE`.
                    Влияют на баннер и ограничения в клиенте. Сохраняются в базе и попадают в резервную копию.
                  </p>
                  <div className="space-y-1">
                    <label className="flex items-center gap-2 text-[13px] text-white">
                      <input type="checkbox" checked={serviceForm.maintenance} onChange={(e) => setServiceForm((p) => ({ ...p, maintenance: e.target.checked }))} />
                      Режим обслуживания (maintenance)
                    </label>
                    <p className="ml-6 text-[11px] leading-snug text-sidebar-text">
                      Показывает баннер и может использоваться как сигнал для работ по серверу и миграций.
                    </p>
                  </div>
                  <div className="space-y-1">
                    <label className="flex items-center gap-2 text-[13px] text-white">
                      <input type="checkbox" checked={serviceForm.read_only} onChange={(e) => setServiceForm((p) => ({ ...p, read_only: e.target.checked }))} />
                      Только чтение (read-only): запрет отправки
                    </label>
                    <p className="ml-6 text-[11px] leading-snug text-sidebar-text">
                      Клиент блокирует отправку сообщений и загрузку файлов. Удобно перед восстановлением из бэкапа.
                    </p>
                  </div>
                  <div className="space-y-1">
                    <label className="flex items-center gap-2 text-[13px] text-white">
                      <input type="checkbox" checked={serviceForm.degradation} onChange={(e) => setServiceForm((p) => ({ ...p, degradation: e.target.checked }))} />
                      Деградация: возможны задержки
                    </label>
                    <p className="ml-6 text-[11px] leading-snug text-sidebar-text">
                      Информирует пользователей о возможных задержках (пик нагрузки, проблемы сети, фоновые работы).
                    </p>
                  </div>
                  <label className="block">
                    <span className="block text-[12px] text-sidebar-text mb-1">Сообщение статуса (опционально)</span>
                    <textarea
                      value={serviceForm.status_message}
                      onChange={(e) => setServiceForm((p) => ({ ...p, status_message: e.target.value }))}
                      className="w-full min-h-[72px] rounded-compass bg-sidebar border border-sidebar-border/40 px-3 py-2 text-[13px] text-white placeholder:text-sidebar-text/80 outline-none focus:border-primary/60"
                      placeholder="Если пусто, текст будет выбран автоматически."
                    />
                    <p className="mt-1 text-[11px] leading-snug text-sidebar-text">
                      Текст баннера вверху приложения. Если оставить пустым, текст подставится автоматически по выбранным флагам.
                    </p>
                  </label>
                </section>

                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">CORS</h3>
                  <p className="text-[12px] text-sidebar-text">
                    Заменяет переменную `CORS_ALLOWED_ORIGINS`. Пример: `https://buhchat.example.com`.
                    Если приложение открывается через тот же домен (nginx), обычно достаточно `*`.
                  </p>
                  <Field
                    label="Allowed origins"
                    value={serviceForm.cors_allowed_origins}
                    onChange={(v) => setServiceForm((p) => ({ ...p, cors_allowed_origins: v }))}
                    placeholder="* или список через запятую"
                    hint="Список доменов, которым разрешены запросы к API из браузера. `*` разрешает всем. Применяется сразу."
                  />
                  <p className="text-[11px] text-sidebar-text">Примечание: после изменения CORS может понадобиться обновить страницу в браузере.</p>
                </section>

                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">WebSocket (для 2000+ пользователей)</h3>
                  <p className="text-[12px] text-sidebar-text">
                    Эти поля соответствуют параметрам `max_ws_connections`, `ws_send_buffer_size`, `ws_write_timeout`, `ws_pong_timeout`, `ws_max_message_size`.
                    Значения по умолчанию подобраны для 2000+ одновременных пользователей.
                  </p>
                  <Field
                    label="max_ws_connections"
                    value={String(serviceForm.max_ws_connections)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, max_ws_connections: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="10000"
                    hint="Максимум одновременных WebSocket-подключений. Если лимит достигнут, новые подключения будут отклоняться."
                  />
                  <Field
                    label="ws_send_buffer_size"
                    value={String(serviceForm.ws_send_buffer_size)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, ws_send_buffer_size: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="256"
                    hint="Размер буфера исходящих сообщений (в сообщениях). Больше: стабильнее при пиках, но больше расход памяти."
                  />
                  <Field
                    label="ws_write_timeout (сек)"
                    value={String(serviceForm.ws_write_timeout)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, ws_write_timeout: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="10"
                    hint="Сколько ждать запись в сокет. Меньше: быстрее закрываются «медленные» клиенты. Больше: терпимее к плохой сети."
                  />
                  <Field
                    label="ws_pong_timeout (сек)"
                    value={String(serviceForm.ws_pong_timeout)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, ws_pong_timeout: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="60"
                    hint="Сколько ждать pong от клиента. Если pong не пришёл, соединение считается потерянным."
                  />
                  <Field
                    label="ws_max_message_size (байт)"
                    value={String(serviceForm.ws_max_message_size)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, ws_max_message_size: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="4096"
                    hint="Ограничение размера входящих сообщений по WS. Защита от слишком больших payload."
                  />
                  <p className="text-[11px] text-sidebar-text">Примечание: при изменении WS-параметров клиенты могут быть принудительно переподключены.</p>
                </section>

                <button
                  type="button"
                  onClick={onSaveServiceSettings}
                  disabled={savingService}
                  className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60"
                >
                  {savingService ? 'Сохранение...' : 'Сохранить служебные настройки'}
                </button>
              </>
            )}
          </div>
        )}

        {section === 'backup' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {backupError && <ErrorBox text={backupError} />}

            <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
              <h3 className="text-[14px] font-semibold text-white">Создать резервную копию</h3>
              <p className="text-[12px] text-sidebar-text">
                Будет скачан ZIP с дампом базы данных и файлами (uploads/audio/ключи push). На больших данных это может занять время.
              </p>
              <button
                type="button"
                disabled={creatingBackup}
                onClick={async () => {
                  setBackupError('');
                  setCreatingBackup(true);
                  try {
                    const blob = await api.downloadAdminBackup();
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    const ts = new Date().toISOString().replace(/[:.]/g, '-');
                    a.href = url;
                    a.download = `buhchat-backup-${ts}.zip`;
                    document.body.appendChild(a);
                    a.click();
                    a.remove();
                    URL.revokeObjectURL(url);
                    onNotify('Резервная копия создана');
                  } catch (e: unknown) {
                    const msg = e instanceof Error ? e.message : 'Не удалось создать резервную копию';
                    setBackupError(msg);
                    onNotify(`Ошибка: ${msg}`);
                  } finally {
                    setCreatingBackup(false);
                  }
                }}
                className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60"
              >
                {creatingBackup ? 'Создание...' : 'Создать резервную копию (ZIP)'}
              </button>
            </section>

            <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
              <h3 className="text-[14px] font-semibold text-white">Восстановить из резервной копии</h3>
              <p className="text-[12px] text-sidebar-text">
                Восстановление заменит базу данных и файлы. Рекомендуется выполнять на новом сервере или в режиме обслуживания.
              </p>
              <input
                type="file"
                accept=".zip,application/zip"
                onChange={(e) => setBackupFile(e.target.files?.[0] || null)}
                className="block w-full text-[12px] text-sidebar-text file:mr-3 file:rounded-compass file:border-0 file:bg-sidebar file:px-3 file:py-2 file:text-[12px] file:font-semibold file:text-white hover:file:brightness-110"
              />
              <button
                type="button"
                disabled={restoringBackup || !backupFile}
                onClick={async () => {
                  if (!backupFile) return;
                  if (!confirm('Восстановить из резервной копии? Это заменит текущие данные.')) return;
                  setBackupError('');
                  setRestoringBackup(true);
                  try {
                    await api.restoreAdminBackup(backupFile);
                    onNotify('Восстановление завершено. Перезагрузка...');
                    location.reload();
                  } catch (e: unknown) {
                    const msg = e instanceof Error ? e.message : 'Не удалось восстановить резервную копию';
                    setBackupError(msg);
                    onNotify(`Ошибка: ${msg}`);
                  } finally {
                    setRestoringBackup(false);
                  }
                }}
                className="w-full min-h-[42px] rounded-compass bg-[#16a34a] text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60"
              >
                {restoringBackup ? 'Восстановление...' : 'Восстановить из ZIP'}
              </button>
            </section>
          </div>
        )}
      </main>

      {addOpen && (
        <UserEditorModal
          title="Добавить пользователя"
          submitLabel="Добавить"
          onClose={() => setAddOpen(false)}
          onSubmit={async (payload) => {
            await api.createUser({
              email: payload.email,
              username: payload.username,
              phone: payload.phone || undefined,
              company: payload.company || undefined,
              position: payload.position || undefined,
              permissions: payload.permissions,
            });
            reloadUsers();
            onNotify('Пользователь добавлен');
            setAddOpen(false);
          }}
        />
      )}

      {editUser && (
        <UserEditorModal
          title="Изменить пользователя"
          submitLabel="Сохранить"
          initial={editUser}
          onClose={() => setEditUser(null)}
          onSubmit={async (payload) => {
            await api.updateUserProfile(editUser.id, {
              username: payload.username,
              email: payload.email,
              phone: payload.phone,
              company: payload.company,
              position: payload.position,
            });
            if (payload.permissions) {
              await api.updateUserPermissions(editUser.id, payload.permissions);
            }
            if (typeof payload.disabled === 'boolean') {
              await api.setUserDisabled(editUser.id, payload.disabled);
            }
            reloadUsers();
            onNotify('Данные пользователя обновлены');
            setEditUser(null);
          }}
        />
      )}
    </div>
  );
}

function SortableTH({ label, active, dir, onClick }: { label: string; active: boolean; dir: SortDir; onClick: () => void }) {
  return (
    <th className="px-3 py-2 select-none">
      <button
        type="button"
        onClick={onClick}
        className={`inline-flex items-center gap-1.5 hover:text-white transition-colors ${active ? 'text-white' : ''}`}
        title="Сортировать"
      >
        <span>{label}</span>
        <span className={`text-[11px] ${active ? 'opacity-100' : 'opacity-40'}`}>{active ? (dir === 'asc' ? '▲' : '▼') : '↕'}</span>
      </button>
    </th>
  );
}

function NavItem({ active, icon, title, subtitle, onClick }: { active: boolean; icon: React.ReactNode; title: string; subtitle: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`w-full min-w-[170px] md:min-w-0 text-left rounded-compass border px-3 py-2 transition ${active ? 'border-primary/60 bg-primary/10' : 'border-sidebar-border/40 hover:border-sidebar-border/80 bg-sidebar-hover/20'}`}
    >
      <div className="flex items-center gap-2 text-white">
        <span className="text-sidebar-text">{icon}</span>
        <span className="text-[13px] font-semibold">{title}</span>
      </div>
      <p className="mt-1 text-[11px] text-sidebar-text hidden md:block">{subtitle}</p>
    </button>
  );
}

function ErrorBox({ text }: { text: string }) {
  return <div className="bg-danger/10 text-danger text-[13px] rounded-compass px-3 py-2">{text}</div>;
}

function Field({ label, value, onChange, placeholder, type = 'text', hint }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  type?: string;
  hint?: string;
}) {
  return (
    <label className="block">
      <span className="block text-[12px] text-sidebar-text mb-1">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full h-10 rounded-compass bg-sidebar border border-sidebar-border/40 px-3 text-[13px] text-white placeholder:text-sidebar-text/80 outline-none focus:border-primary/60"
      />
      {hint && <p className="mt-1 text-[11px] text-sidebar-text">{hint}</p>}
    </label>
  );
}

type UserEditorPayload = {
  username: string;
  email: string;
  phone: string;
  company: string;
  position: string;
  disabled?: boolean;
  permissions?: Partial<Record<keyof Omit<api.UserPermissions, 'user_id' | 'updated_at'>, boolean>>;
};

function UserEditorModal({
  title,
  submitLabel,
  initial,
  onClose,
  onSubmit,
}: {
  title: string;
  submitLabel: string;
  initial?: UserPublic;
  onClose: () => void;
  onSubmit: (payload: UserEditorPayload) => Promise<void>;
}) {
  const [username, setUsername] = useState(initial?.username || '');
  const [email, setEmail] = useState(initial?.email || '');
  const [phone, setPhone] = useState(initial?.phone || '');
  const [company, setCompany] = useState(initial?.company || '');
  const [position, setPosition] = useState(initial?.position || '');
  const [disabled, setDisabled] = useState(!!initial?.disabled_at);
  const [role, setRole] = useState<'member' | 'assistant_administrator' | 'administrator'>('member');
  const [loadingPerm, setLoadingPerm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  // For edit: load current permissions and map to a single "role" selector.
  useEffect(() => {
    if (!initial?.id) return;
    let cancelled = false;
    setLoadingPerm(true);
    api.getUserPermissions(initial.id)
      .then((p) => {
        if (cancelled) return;
        if (p.administrator) setRole('administrator');
        else if (p.assistant_administrator) setRole('assistant_administrator');
        else setRole('member');
      })
      .catch(() => {
        if (!cancelled) setError('Не удалось загрузить роли пользователя');
      })
      .finally(() => {
        if (!cancelled) setLoadingPerm(false);
      });
    return () => { cancelled = true; };
  }, [initial?.id]);

  const permissionsFromRole = useCallback((): Partial<Record<keyof Omit<api.UserPermissions, 'user_id' | 'updated_at'>, boolean>> => {
    return {
      member: true,
      assistant_administrator: role === 'assistant_administrator',
      administrator: role === 'administrator',
    };
  }, [role]);

  const validate = () => {
    if (!username.trim()) return 'Имя обязательно';
    if (!email.trim()) return 'Email обязателен';
    if (!EMAIL_RE.test(email.trim().toLowerCase())) return 'Неверный формат email';
    return '';
  };

  const handleSubmit = async () => {
    const err = validate();
    if (err) {
      setError(err);
      return;
    }
    setSaving(true);
    setError('');
    try {
      await onSubmit({
        username: username.trim(),
        email: email.trim().toLowerCase(),
        phone: phone.trim(),
        company: company.trim(),
        position: position.trim(),
        disabled,
        permissions: permissionsFromRole(),
      });
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Не удалось сохранить пользователя');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[120] flex items-center justify-center p-4 safe-area-padding">
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />
      <div className="relative w-full max-w-[420px] rounded-compass bg-sidebar border border-sidebar-border/50 p-4 space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-[16px] font-semibold text-white">{title}</h3>
          <button type="button" onClick={onClose} className="w-8 h-8 rounded-full flex items-center justify-center text-sidebar-text hover:text-white hover:bg-sidebar-hover">
            <IconX size={12} />
          </button>
        </div>

        {error && <ErrorBox text={error} />}

        <Field label="Имя" value={username} onChange={setUsername} placeholder="Имя" />
        <Field label="Email" value={email} onChange={setEmail} placeholder="email@example.com" />
        <Field label="Телефон" value={phone} onChange={setPhone} placeholder="+7..." />
        <Field label="Компания (опционально)" value={company} onChange={setCompany} placeholder="Компания" />
        <Field label="Должность (опционально)" value={position} onChange={setPosition} placeholder="Должность" />

        <label className="block">
          <span className="block text-[12px] text-sidebar-text mb-1">Роль</span>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value as typeof role)}
            disabled={loadingPerm}
            className="w-full h-10 rounded-compass bg-sidebar border border-sidebar-border/40 px-3 text-[13px] text-white outline-none focus:border-primary/60 disabled:opacity-60"
          >
            <option value="member">Пользователь</option>
            <option value="assistant_administrator">Помощник администратора</option>
            <option value="administrator">Администратор</option>
          </select>
          <p className="mt-1 text-[11px] text-sidebar-text">Роли применяются сразу после сохранения.</p>
        </label>

        {initial && (
          <label className="flex items-center gap-2 text-[13px] text-white">
            <input type="checkbox" checked={disabled} onChange={(e) => setDisabled(e.target.checked)} />
            Отключить вход пользователю
          </label>
        )}

        <button type="button" onClick={handleSubmit} disabled={saving} className="w-full h-10 rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60">
          {saving ? 'Сохранение...' : submitLabel}
        </button>
      </div>
    </div>
  );
}
