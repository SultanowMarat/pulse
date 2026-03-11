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
  if (u.is_online) return 'Ð¡ÐµÐ¹Ñ‡Ð°Ñ Ð¾Ð½Ð»Ð°Ð¹Ð½';
  const s = (u.last_seen_at || '').trim();
  if (!s) return 'â€”';
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return 'â€”';
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
  if (role === 'administrator') return 'ÐÐ´Ð¼Ð¸Ð½Ð¸ÑÑ‚Ñ€Ð°Ñ‚Ð¾Ñ€';
  return 'ÐŸÐ¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÑŒ';
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
      setServiceError(e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ Ð·Ð°Ð³Ñ€ÑƒÐ·Ð¸Ñ‚ÑŒ ÑÐ»ÑƒÐ¶ÐµÐ±Ð½Ñ‹Ðµ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸');
    } finally {
      setLoadingService(false);
    }
  }, []);

  const onSaveServiceSettings = useCallback(async () => {
    setSavingService(true);
    setServiceError('');
    try {
      const v = serviceForm;
      if (!Number.isInteger(v.max_ws_connections) || v.max_ws_connections < 2000) throw new Error('max_ws_connections: Ð¼Ð¸Ð½Ð¸Ð¼ÑƒÐ¼ 2000');
      if (!Number.isInteger(v.ws_send_buffer_size) || v.ws_send_buffer_size < 64) throw new Error('ws_send_buffer_size: Ð¼Ð¸Ð½Ð¸Ð¼ÑƒÐ¼ 64');
      if (!Number.isInteger(v.ws_write_timeout) || v.ws_write_timeout < 1) throw new Error('ws_write_timeout: Ð¼Ð¸Ð½Ð¸Ð¼ÑƒÐ¼ 1');
      if (!Number.isInteger(v.ws_pong_timeout) || v.ws_pong_timeout < 5) throw new Error('ws_pong_timeout: Ð¼Ð¸Ð½Ð¸Ð¼ÑƒÐ¼ 5');
      if (!Number.isInteger(v.ws_max_message_size) || v.ws_max_message_size < 1024) throw new Error('ws_max_message_size: Ð¼Ð¸Ð½Ð¸Ð¼ÑƒÐ¼ 1024');
      await api.updateServiceSettings(v);
      // Apply maintenance/read-only banner immediately in current UI.
      reloadAppStatus();
      onNotify('Ð¡Ð»ÑƒÐ¶ÐµÐ±Ð½Ñ‹Ðµ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ ÑÐ¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ñ‹');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ ÑÐ¾Ñ…Ñ€Ð°Ð½Ð¸Ñ‚ÑŒ ÑÐ»ÑƒÐ¶ÐµÐ±Ð½Ñ‹Ðµ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸';
      setServiceError(msg);
      onNotify(`ÐžÑˆÐ¸Ð±ÐºÐ°: ${msg}`);
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
      setMailError(e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ Ð·Ð°Ð³Ñ€ÑƒÐ·Ð¸Ñ‚ÑŒ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ð¿Ð¾Ñ‡Ñ‚Ñ‹');
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
      setFileError(e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ Ð·Ð°Ð³Ñ€ÑƒÐ·Ð¸Ñ‚ÑŒ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ñ„Ð°Ð¹Ð»Ð¾Ð²');
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
      setUsersError(e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ Ð·Ð°Ð³Ñ€ÑƒÐ·Ð¸Ñ‚ÑŒ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÐµÐ¹');
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
      onNotify('ÐÐ°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ð¿Ð¾Ñ‡Ñ‚Ñ‹ ÑÐ¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ñ‹');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'ÐžÑˆÐ¸Ð±ÐºÐ° ÑÐ¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¸Ñ Ð¿Ð¾Ñ‡Ñ‚Ð¾Ð²Ñ‹Ñ… Ð½Ð°ÑÑ‚Ñ€Ð¾ÐµÐº';
      setMailError(msg);
      onNotify(`ÐžÑˆÐ¸Ð±ÐºÐ°: ${msg}`);
    } finally {
      setSavingMail(false);
    }
  }, [mailForm, onNotify]);

  const onTestMail = useCallback(async () => {
    if (!testEmail.trim()) {
      setMailError('Ð£ÐºÐ°Ð¶Ð¸Ñ‚Ðµ email Ð´Ð»Ñ Ñ‚ÐµÑÑ‚Ð¾Ð²Ð¾Ð³Ð¾ Ð¿Ð¸ÑÑŒÐ¼Ð°');
      return;
    }
    setTestingMail(true);
    setMailError('');
    try {
      await api.sendTestMail(testEmail);
      onNotify('Ð¢ÐµÑÑ‚Ð¾Ð²Ð¾Ðµ Ð¿Ð¸ÑÑŒÐ¼Ð¾ Ð¾Ñ‚Ð¿Ñ€Ð°Ð²Ð»ÐµÐ½Ð¾ ÑƒÑÐ¿ÐµÑˆÐ½Ð¾');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'ÐžÑˆÐ¸Ð±ÐºÐ° Ð¾Ñ‚Ð¿Ñ€Ð°Ð²ÐºÐ¸ Ñ‚ÐµÑÑ‚Ð¾Ð²Ð¾Ð³Ð¾ Ð¿Ð¸ÑÑŒÐ¼Ð°';
      setMailError(msg);
      onNotify(`ÐžÑˆÐ¸Ð±ÐºÐ°: ${msg}`);
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
        throw new Error('Ð’Ð²ÐµÐ´Ð¸Ñ‚Ðµ Ñ†ÐµÐ»Ð¾Ðµ Ñ‡Ð¸ÑÐ»Ð¾ Ð¾Ñ‚ 1 Ð´Ð¾ 200 ÐœÐ‘');
      }
      await api.updateAdminFileSettings(v);
      useChatStore.setState({ maxFileSizeMB: v });
      await syncMaxFileSize();
      onNotify('ÐžÐ³Ñ€Ð°Ð½Ð¸Ñ‡ÐµÐ½Ð¸Ðµ Ñ€Ð°Ð·Ð¼ÐµÑ€Ð° Ñ„Ð°Ð¹Ð»Ð° ÑÐ¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¾');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'ÐžÑˆÐ¸Ð±ÐºÐ° ÑÐ¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¸Ñ Ð½Ð°ÑÑ‚Ñ€Ð¾ÐµÐº Ñ„Ð°Ð¹Ð»Ð¾Ð²';
      setFileError(msg);
      onNotify(`ÐžÑˆÐ¸Ð±ÐºÐ°: ${msg}`);
    } finally {
      setSavingFiles(false);
    }
  }, [maxFileSizeMB, onNotify, syncMaxFileSize]);

  return (
    <div className={`h-full w-full min-h-0 flex flex-col md:flex-row ${standalone ? 'bg-surface dark:bg-dark-bg' : 'bg-sidebar'}`}>
      <aside className="w-full md:w-[250px] md:shrink-0 border-b md:border-b-0 md:border-r border-surface-border dark:border-dark-border p-2 sm:p-3">
        <div className="flex md:flex-col gap-2 overflow-x-auto md:overflow-visible pb-1 md:pb-0">
          <NavItem active={section === 'users'} icon={<IconUser size={16} />} title="ÐŸÐ¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ð¸" subtitle="Ð¡Ð¿Ð¸ÑÐ¾Ðº Ð¸ ÑƒÐ¿Ñ€Ð°Ð²Ð»ÐµÐ½Ð¸Ðµ" onClick={() => setSection('users')} />
          <NavItem active={section === 'links'} icon={<IconDownload size={16} />} title="Ð¡ÑÑ‹Ð»ÐºÐ¸" subtitle="Ð£ÑÑ‚Ð°Ð½Ð¾Ð²ÐºÐ° ÐºÐ»Ð¸ÐµÐ½Ñ‚Ð¾Ð²" onClick={() => setSection('links')} />
          <NavItem active={section === 'mail'} icon={<IconMail size={16} />} title="ÐŸÐ¾Ñ‡Ñ‚Ð°" subtitle="SMTP Ð¸ Ñ‚ÐµÑÑ‚Ð¾Ð²Ð¾Ðµ Ð¿Ð¸ÑÑŒÐ¼Ð¾" onClick={() => setSection('mail')} />
          <NavItem active={section === 'files'} icon={<IconFile size={16} />} title="Ð¤Ð°Ð¹Ð»Ñ‹" subtitle="Ð›Ð¸Ð¼Ð¸Ñ‚ Ñ€Ð°Ð·Ð¼ÐµÑ€Ð°" onClick={() => setSection('files')} />
          <NavItem active={section === 'service'} icon={<IconFile size={16} />} title="Ð¡Ð»ÑƒÐ¶ÐµÐ±Ð½Ñ‹Ðµ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸" subtitle="ENV/yaml -> UI" onClick={() => setSection('service')} />
          <NavItem active={section === 'backup'} icon={<IconFile size={16} />} title="Ð ÐµÐ·ÐµÑ€Ð²Ð½Ð¾Ðµ ÐºÐ¾Ð¿Ð¸Ñ€Ð¾Ð²Ð°Ð½Ð¸Ðµ" subtitle="Backup Ð¸ restore" onClick={() => setSection('backup')} />
        </div>
      </aside>

      <main className="flex-1 min-w-0 min-h-0 overflow-y-auto p-3 sm:p-4">
        {section === 'links' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {loadingService ? (
              <p className="text-[13px] text-sidebar-text">Ð—Ð°Ð³Ñ€ÑƒÐ·ÐºÐ°...</p>
            ) : (
              <>
                {serviceError && <ErrorBox text={serviceError} />}
                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">Ð¡ÑÑ‹Ð»ÐºÐ¸ ÑƒÑÑ‚Ð°Ð½Ð¾Ð²ÐºÐ¸</h3>
                  <p className="text-[12px] text-sidebar-text">
                    Ð­Ñ‚Ð¸ ÑÑÑ‹Ð»ÐºÐ¸ Ð¾Ñ‚ÐºÑ€Ñ‹Ð²Ð°ÑŽÑ‚ÑÑ Ð² Ð¿Ñ€Ð¾Ñ„Ð¸Ð»Ðµ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ Ð¿Ñ€Ð¸ Ð²Ñ‹Ð±Ð¾Ñ€Ðµ Ð¿Ð»Ð°Ñ‚Ñ„Ð¾Ñ€Ð¼Ñ‹ (Windows/Android/macOS/iPhone).
                  </p>
                  <Field
                    label="Windows"
                    value={serviceForm.install_windows_url}
                    onChange={(v) => setServiceForm((p) => ({ ...p, install_windows_url: v }))}
                    placeholder="https://..."
                    hint="Ð¡ÑÑ‹Ð»ÐºÐ° Ð½Ð° ÑƒÑÑ‚Ð°Ð½Ð¾Ð²Ñ‰Ð¸Ðº Ð¸Ð»Ð¸ ÑÑ‚Ñ€Ð°Ð½Ð¸Ñ†Ñƒ Ð·Ð°Ð³Ñ€ÑƒÐ·ÐºÐ¸ Ð´Ð»Ñ Windows. ÐžÑ‚ÐºÑ€Ð¾ÐµÑ‚ÑÑ Ð¿Ð¾ ÐºÐ½Ð¾Ð¿ÐºÐµ Ð² Ð¿Ñ€Ð¾Ñ„Ð¸Ð»Ðµ."
                  />
                  <Field
                    label="Android"
                    value={serviceForm.install_android_url}
                    onChange={(v) => setServiceForm((p) => ({ ...p, install_android_url: v }))}
                    placeholder="https://..."
                    hint="Ð¡ÑÑ‹Ð»ÐºÐ° Ð½Ð° Google Play Ð¸Ð»Ð¸ APK. ÐžÑ‚ÐºÑ€Ð¾ÐµÑ‚ÑÑ Ð¿Ð¾ ÐºÐ½Ð¾Ð¿ÐºÐµ Ð² Ð¿Ñ€Ð¾Ñ„Ð¸Ð»Ðµ."
                  />
                  <Field
                    label="macOS"
                    value={serviceForm.install_macos_url}
                    onChange={(v) => setServiceForm((p) => ({ ...p, install_macos_url: v }))}
                    placeholder="https://..."
                    hint="Ð¡ÑÑ‹Ð»ÐºÐ° Ð½Ð° DMG/PKG Ð¸Ð»Ð¸ ÑÑ‚Ñ€Ð°Ð½Ð¸Ñ†Ñƒ Ð·Ð°Ð³Ñ€ÑƒÐ·ÐºÐ¸ Ð´Ð»Ñ macOS. ÐžÑ‚ÐºÑ€Ð¾ÐµÑ‚ÑÑ Ð¿Ð¾ ÐºÐ½Ð¾Ð¿ÐºÐµ Ð² Ð¿Ñ€Ð¾Ñ„Ð¸Ð»Ðµ."
                  />
                  <Field
                    label="iPhone (iOS)"
                    value={serviceForm.install_ios_url}
                    onChange={(v) => setServiceForm((p) => ({ ...p, install_ios_url: v }))}
                    placeholder="https://..."
                    hint="Ð¡ÑÑ‹Ð»ÐºÐ° Ð½Ð° App Store Ð¸Ð»Ð¸ ÑÑ‚Ñ€Ð°Ð½Ð¸Ñ†Ñƒ ÑƒÑÑ‚Ð°Ð½Ð¾Ð²ÐºÐ¸. ÐžÑ‚ÐºÑ€Ð¾ÐµÑ‚ÑÑ Ð¿Ð¾ ÐºÐ½Ð¾Ð¿ÐºÐµ Ð² Ð¿Ñ€Ð¾Ñ„Ð¸Ð»Ðµ."
                  />
                  <button
                    type="button"
                    onClick={onSaveServiceSettings}
                    disabled={savingService}
                    className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60"
                  >
                    {savingService ? 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¸Ðµ...' : 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½Ð¸Ñ‚ÑŒ ÑÑÑ‹Ð»ÐºÐ¸'}
                  </button>
                </section>
              </>
            )}
          </div>
        )}

        {section === 'mail' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {loadingMail ? (
              <p className="text-[13px] text-sidebar-text">Ð—Ð°Ð³Ñ€ÑƒÐ·ÐºÐ°...</p>
            ) : (
              <>
                {mailError && <ErrorBox text={mailError} />}
                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">ÐÐ°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ° Ð¿Ð¾Ñ‡Ñ‚Ñ‹</h3>
                  <Field label="SMTP Host" value={mailForm.host} onChange={(v) => setMailField('host', v)} placeholder="smtp.yandex.ru" />
                  <Field label="SMTP Port" value={mailForm.port} onChange={(v) => setMailField('port', v.replace(/\D/g, ''))} placeholder="587" />
                  <Field label="Ð›Ð¾Ð³Ð¸Ð½ (SMTP Username)" value={mailForm.username} onChange={(v) => setMailField('username', v)} placeholder="user@example.com" />
                  <Field label="ÐŸÐ°Ñ€Ð¾Ð»ÑŒ (SMTP Password)" value={mailForm.password} onChange={(v) => setMailField('password', v)} placeholder="password" type="password" />
                  <Field label="From Email" value={mailForm.from_email} onChange={(v) => setMailField('from_email', v)} placeholder="noreply@example.com" />
                  <Field label="From Name" value={mailForm.from_name} onChange={(v) => setMailField('from_name', v)} placeholder="pulse Auth" />
                  <button type="button" onClick={onSaveMail} disabled={savingMail} className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60">
                    {savingMail ? 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¸Ðµ...' : 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½Ð¸Ñ‚ÑŒ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸'}
                  </button>
                  <p className="text-[12px] text-sidebar-text">Ð•ÑÐ»Ð¸ Ð¿Ð¾Ð»Ñ SMTP Ð¿ÑƒÑÑ‚Ñ‹Ðµ, Ð²Ñ…Ð¾Ð´ Ð±ÑƒÐ´ÐµÑ‚ Ð±ÐµÐ· Ð¾Ñ‚Ð¿Ñ€Ð°Ð²ÐºÐ¸ ÐºÐ¾Ð´Ð°.</p>
                </section>

                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[13px] font-semibold text-white">Ð¢ÐµÑÑ‚Ð¾Ð²Ð¾Ðµ Ð¿Ð¸ÑÑŒÐ¼Ð¾</h3>
                  <Field label="Email Ð¿Ð¾Ð»ÑƒÑ‡Ð°Ñ‚ÐµÐ»Ñ" value={testEmail} onChange={setTestEmail} placeholder="test@example.com" />
                  <button type="button" onClick={onTestMail} disabled={testingMail} className="w-full min-h-[42px] rounded-compass bg-[#16a34a] text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60">
                    {testingMail ? 'ÐžÑ‚Ð¿Ñ€Ð°Ð²ÐºÐ°...' : 'ÐžÑ‚Ð¿Ñ€Ð°Ð²Ð¸Ñ‚ÑŒ Ñ‚ÐµÑÑ‚Ð¾Ð²Ð¾Ðµ Ð¿Ð¸ÑÑŒÐ¼Ð¾'}
                  </button>
                </section>
              </>
            )}
          </div>
        )}

        {section === 'files' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {loadingFiles ? (
              <p className="text-[13px] text-sidebar-text">Ð—Ð°Ð³Ñ€ÑƒÐ·ÐºÐ°...</p>
            ) : (
              <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                <h3 className="text-[14px] font-semibold text-white">ÐÐ°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ñ„Ð°Ð¹Ð»Ð¾Ð²</h3>
                {fileError && <ErrorBox text={fileError} />}
                <Field
                  label="ÐœÐ°ÐºÑÐ¸Ð¼Ð°Ð»ÑŒÐ½Ñ‹Ð¹ Ñ€Ð°Ð·Ð¼ÐµÑ€ Ñ„Ð°Ð¹Ð»Ð° Ð´Ð»Ñ Ð¾Ñ‚Ð¿Ñ€Ð°Ð²ÐºÐ¸ ÑÐ¾Ð¾Ð±Ñ‰ÐµÐ½Ð¸Ð¹ (ÐœÐ‘)"
                  value={maxFileSizeMB}
                  onChange={(v) => setMaxFileSizeMB(v.replace(/\D/g, ''))}
                  placeholder="20"
                />
                <button type="button" onClick={onSaveFiles} disabled={savingFiles} className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60">
                  {savingFiles ? 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¸Ðµ...' : 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½Ð¸Ñ‚ÑŒ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸'}
                </button>
                <p className="text-[12px] text-sidebar-text">Ð”Ð¸Ð°Ð¿Ð°Ð·Ð¾Ð½: Ð¾Ñ‚ 1 Ð´Ð¾ 200 ÐœÐ‘.</p>
              </section>
            )}
          </div>
        )}

        {section === 'users' && (
          <div className="max-w-[1200px] w-full mx-auto space-y-3">
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="ÐŸÐ¾Ð¸ÑÐº Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ"
                className="w-full h-10 rounded-compass bg-sidebar border border-sidebar-border/40 px-3 text-[13px] text-white placeholder:text-sidebar-text/80 outline-none focus:border-primary/60"
              />
              <button
                type="button"
                onClick={() => setAddOpen(true)}
                className="h-10 px-4 rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition"
              >
                Ð”Ð¾Ð±Ð°Ð²Ð¸Ñ‚ÑŒ
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
                <table className="w-full min-w-[840px] text-left text-[13px]">
                  <thead className="bg-sidebar-hover/60 text-sidebar-text sticky top-0 z-10">
                  <tr>
                    <th className="px-3 py-2">Ð¤Ð¾Ñ‚Ð¾</th>
                    <SortableTH label="Ð˜Ð¼Ñ" active={sortKey === 'username'} dir={sortDir} onClick={() => toggleSort('username')} />
                    <SortableTH label="Email" active={sortKey === 'email'} dir={sortDir} onClick={() => toggleSort('email')} />
                    <SortableTH label="Ð¢ÐµÐ»ÐµÑ„Ð¾Ð½" active={sortKey === 'phone'} dir={sortDir} onClick={() => toggleSort('phone')} />
                    <SortableTH label="Ð Ð¾Ð»ÑŒ" active={sortKey === 'role'} dir={sortDir} onClick={() => toggleSort('role')} />
                    <SortableTH label="Ð¡Ñ‚Ð°Ñ‚ÑƒÑ" active={sortKey === 'status'} dir={sortDir} onClick={() => toggleSort('status')} />
                    <SortableTH label="ÐŸÐ¾ÑÐ»ÐµÐ´Ð½ÑÑ Ð°ÐºÑ‚Ð¸Ð²Ð½Ð¾ÑÑ‚ÑŒ" active={sortKey === 'last_seen_at'} dir={sortDir} onClick={() => toggleSort('last_seen_at')} />
                    <th className="px-3 py-2 text-right">Ð”ÐµÐ¹ÑÑ‚Ð²Ð¸Ñ</th>
                  </tr>
                </thead>
                <tbody>
                  {loadingUsers && (
                    <tr>
                      <td colSpan={8} className="px-3 py-6 text-center text-sidebar-text">Ð—Ð°Ð³Ñ€ÑƒÐ·ÐºÐ°...</td>
                    </tr>
                  )}
                  {!loadingUsers && users.length === 0 && (
                    <tr>
                      <td colSpan={8} className="px-3 py-6 text-center text-sidebar-text">ÐŸÐ¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ð¸ Ð½Ðµ Ð½Ð°Ð¹Ð´ÐµÐ½Ñ‹</td>
                    </tr>
                  )}
                  {!loadingUsers && users.map((u) => (
                    <tr key={u.id} className="border-t border-sidebar-border/30">
                      <td className="px-3 py-2">
                        <Avatar name={u.username} url={u.avatar_url || undefined} size={32} />
                      </td>
                      <td className="px-3 py-2 text-white break-words max-w-[180px]">{u.username}</td>
                      <td className="px-3 py-2 text-white/90 break-all max-w-[230px]">{u.email || 'â€”'}</td>
                      <td className="px-3 py-2 text-white/90 break-all max-w-[140px]">{u.phone || 'â€”'}</td>
                      <td className="px-3 py-2 text-white/90 max-w-[150px]">{roleLabel(u.role)}</td>
                      <td className="px-3 py-2">
                        <span className={`inline-flex px-2 py-1 rounded-full text-[11px] font-semibold ${u.disabled_at ? 'bg-danger/20 text-danger' : 'bg-green/20 text-green-300'}`}>
                          {u.disabled_at ? 'ÐžÑ‚ÐºÐ»ÑŽÑ‡ÐµÐ½' : 'ÐÐºÑ‚Ð¸Ð²ÐµÐ½'}
                        </span>
                      </td>
                      <td className="px-3 py-2 text-white/80 break-words max-w-[170px]">{fmtLastActivity(u)}</td>
                      <td className="px-3 py-2 text-right">
                        <div className="flex flex-col items-stretch gap-1 min-w-[112px]">
                          <button type="button" onClick={() => setEditUser(u)} className="h-8 px-3 rounded-compass bg-sidebar-hover text-white text-[12px] font-medium hover:brightness-110 transition">
                            Ð˜Ð·Ð¼ÐµÐ½Ð¸Ñ‚ÑŒ
                          </button>
                          <button
                            type="button"
                            onClick={async () => {
                              if (!confirm('Ð¡Ð±Ñ€Ð¾ÑÐ¸Ñ‚ÑŒ ÑÐµÑÑÐ¸Ð¸ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ Ð½Ð° Ð²ÑÐµÑ… ÑƒÑÑ‚Ñ€Ð¾Ð¹ÑÑ‚Ð²Ð°Ñ…?')) return;
                              try {
                                const res = await api.logoutAllUserSessions(u.id);
                                onNotify(`Ð¡ÐµÑÑÐ¸Ð¸ ÑÐ±Ñ€Ð¾ÑˆÐµÐ½Ñ‹ (Ð¾Ñ‚Ð¾Ð·Ð²Ð°Ð½Ð¾: ${res.revoked || 0})`);
                              } catch (e: unknown) {
                                const msg = e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ ÑÐ±Ñ€Ð¾ÑÐ¸Ñ‚ÑŒ ÑÐµÑÑÐ¸Ð¸';
                                onNotify(`ÐžÑˆÐ¸Ð±ÐºÐ°: ${msg}`);
                              }
                            }}
                            className="h-8 px-3 rounded-compass bg-danger/20 text-danger text-[12px] font-medium hover:brightness-110 transition"
                            title="Ð’Ñ‹ÐºÐ¸Ð½ÑƒÑ‚ÑŒ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ ÑÐ¾ Ð²ÑÐµÑ… ÑƒÑÑ‚Ñ€Ð¾Ð¹ÑÑ‚Ð²"
                          >
                            Ð¡Ð±Ñ€Ð¾ÑÐ¸Ñ‚ÑŒ
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                  {!loadingUsers && loadingMoreUsers && (
                    <tr>
                      <td colSpan={8} className="px-3 py-4 text-center text-sidebar-text">Ð—Ð°Ð³Ñ€ÑƒÐ·ÐºÐ°...</td>
                    </tr>
                  )}
                </tbody>
              </table>
              </div>
            </div>
            {!loadingUsers && usersTotal > 0 && (
              <p className="text-[12px] text-sidebar-text">
                ÐŸÐ¾ÐºÐ°Ð·Ð°Ð½Ð¾: {users.length} Ð¸Ð· {usersTotal}
              </p>
            )}
          </div>
        )}

        {section === 'service' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {loadingService ? (
              <p className="text-[13px] text-sidebar-text">Ð—Ð°Ð³Ñ€ÑƒÐ·ÐºÐ°...</p>
            ) : (
              <>
                {serviceError && <ErrorBox text={serviceError} />}
                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">Ð¡Ð¾ÑÑ‚Ð¾ÑÐ½Ð¸Ðµ Ð¿Ñ€Ð¸Ð»Ð¾Ð¶ÐµÐ½Ð¸Ñ</h3>
                  <p className="text-[12px] text-sidebar-text">
                    Ð­Ñ‚Ð¸ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸ Ð·Ð°Ð¼ÐµÐ½ÑÑŽÑ‚ Ð¿ÐµÑ€ÐµÐ¼ÐµÐ½Ð½Ñ‹Ðµ Ð¾ÐºÑ€ÑƒÐ¶ÐµÐ½Ð¸Ñ `APP_MAINTENANCE`, `APP_READ_ONLY`, `APP_DEGRADATION`, `APP_STATUS_MESSAGE`.
                    Ð’Ð»Ð¸ÑÑŽÑ‚ Ð½Ð° Ð±Ð°Ð½Ð½ÐµÑ€ Ð¸ Ð¾Ð³Ñ€Ð°Ð½Ð¸Ñ‡ÐµÐ½Ð¸Ñ Ð² ÐºÐ»Ð¸ÐµÐ½Ñ‚Ðµ. Ð¡Ð¾Ñ…Ñ€Ð°Ð½ÑÑŽÑ‚ÑÑ Ð² Ð±Ð°Ð·Ðµ Ð¸ Ð¿Ð¾Ð¿Ð°Ð´Ð°ÑŽÑ‚ Ð² Ñ€ÐµÐ·ÐµÑ€Ð²Ð½ÑƒÑŽ ÐºÐ¾Ð¿Ð¸ÑŽ.
                  </p>
                  <div className="space-y-1">
                    <label className="flex items-center gap-2 text-[13px] text-white">
                      <input type="checkbox" checked={serviceForm.maintenance} onChange={(e) => setServiceForm((p) => ({ ...p, maintenance: e.target.checked }))} />
                      Ð ÐµÐ¶Ð¸Ð¼ Ð¾Ð±ÑÐ»ÑƒÐ¶Ð¸Ð²Ð°Ð½Ð¸Ñ (maintenance)
                    </label>
                    <p className="ml-6 text-[11px] leading-snug text-sidebar-text">
                      ÐŸÐ¾ÐºÐ°Ð·Ñ‹Ð²Ð°ÐµÑ‚ Ð±Ð°Ð½Ð½ÐµÑ€ Ð¸ Ð¼Ð¾Ð¶ÐµÑ‚ Ð¸ÑÐ¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÑŒÑÑ ÐºÐ°Ðº ÑÐ¸Ð³Ð½Ð°Ð» Ð´Ð»Ñ Ñ€Ð°Ð±Ð¾Ñ‚ Ð¿Ð¾ ÑÐµÑ€Ð²ÐµÑ€Ñƒ Ð¸ Ð¼Ð¸Ð³Ñ€Ð°Ñ†Ð¸Ð¹.
                    </p>
                  </div>
                  <div className="space-y-1">
                    <label className="flex items-center gap-2 text-[13px] text-white">
                      <input type="checkbox" checked={serviceForm.read_only} onChange={(e) => setServiceForm((p) => ({ ...p, read_only: e.target.checked }))} />
                      Ð¢Ð¾Ð»ÑŒÐºÐ¾ Ñ‡Ñ‚ÐµÐ½Ð¸Ðµ (read-only): Ð·Ð°Ð¿Ñ€ÐµÑ‚ Ð¾Ñ‚Ð¿Ñ€Ð°Ð²ÐºÐ¸
                    </label>
                    <p className="ml-6 text-[11px] leading-snug text-sidebar-text">
                      ÐšÐ»Ð¸ÐµÐ½Ñ‚ Ð±Ð»Ð¾ÐºÐ¸Ñ€ÑƒÐµÑ‚ Ð¾Ñ‚Ð¿Ñ€Ð°Ð²ÐºÑƒ ÑÐ¾Ð¾Ð±Ñ‰ÐµÐ½Ð¸Ð¹ Ð¸ Ð·Ð°Ð³Ñ€ÑƒÐ·ÐºÑƒ Ñ„Ð°Ð¹Ð»Ð¾Ð². Ð£Ð´Ð¾Ð±Ð½Ð¾ Ð¿ÐµÑ€ÐµÐ´ Ð²Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð»ÐµÐ½Ð¸ÐµÐ¼ Ð¸Ð· Ð±ÑÐºÐ°Ð¿Ð°.
                    </p>
                  </div>
                  <div className="space-y-1">
                    <label className="flex items-center gap-2 text-[13px] text-white">
                      <input type="checkbox" checked={serviceForm.degradation} onChange={(e) => setServiceForm((p) => ({ ...p, degradation: e.target.checked }))} />
                      Ð”ÐµÐ³Ñ€Ð°Ð´Ð°Ñ†Ð¸Ñ: Ð²Ð¾Ð·Ð¼Ð¾Ð¶Ð½Ñ‹ Ð·Ð°Ð´ÐµÑ€Ð¶ÐºÐ¸
                    </label>
                    <p className="ml-6 text-[11px] leading-snug text-sidebar-text">
                      Ð˜Ð½Ñ„Ð¾Ñ€Ð¼Ð¸Ñ€ÑƒÐµÑ‚ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÐµÐ¹ Ð¾ Ð²Ð¾Ð·Ð¼Ð¾Ð¶Ð½Ñ‹Ñ… Ð·Ð°Ð´ÐµÑ€Ð¶ÐºÐ°Ñ… (Ð¿Ð¸Ðº Ð½Ð°Ð³Ñ€ÑƒÐ·ÐºÐ¸, Ð¿Ñ€Ð¾Ð±Ð»ÐµÐ¼Ñ‹ ÑÐµÑ‚Ð¸, Ñ„Ð¾Ð½Ð¾Ð²Ñ‹Ðµ Ñ€Ð°Ð±Ð¾Ñ‚Ñ‹).
                    </p>
                  </div>
                  <label className="block">
                    <span className="block text-[12px] text-sidebar-text mb-1">Ð¡Ð¾Ð¾Ð±Ñ‰ÐµÐ½Ð¸Ðµ ÑÑ‚Ð°Ñ‚ÑƒÑÐ° (Ð¾Ð¿Ñ†Ð¸Ð¾Ð½Ð°Ð»ÑŒÐ½Ð¾)</span>
                    <textarea
                      value={serviceForm.status_message}
                      onChange={(e) => setServiceForm((p) => ({ ...p, status_message: e.target.value }))}
                      className="w-full min-h-[72px] rounded-compass bg-sidebar border border-sidebar-border/40 px-3 py-2 text-[13px] text-white placeholder:text-sidebar-text/80 outline-none focus:border-primary/60"
                      placeholder="Ð•ÑÐ»Ð¸ Ð¿ÑƒÑÑ‚Ð¾, Ñ‚ÐµÐºÑÑ‚ Ð±ÑƒÐ´ÐµÑ‚ Ð²Ñ‹Ð±Ñ€Ð°Ð½ Ð°Ð²Ñ‚Ð¾Ð¼Ð°Ñ‚Ð¸Ñ‡ÐµÑÐºÐ¸."
                    />
                    <p className="mt-1 text-[11px] leading-snug text-sidebar-text">
                      Ð¢ÐµÐºÑÑ‚ Ð±Ð°Ð½Ð½ÐµÑ€Ð° Ð²Ð²ÐµÑ€Ñ…Ñƒ Ð¿Ñ€Ð¸Ð»Ð¾Ð¶ÐµÐ½Ð¸Ñ. Ð•ÑÐ»Ð¸ Ð¾ÑÑ‚Ð°Ð²Ð¸Ñ‚ÑŒ Ð¿ÑƒÑÑ‚Ñ‹Ð¼, Ñ‚ÐµÐºÑÑ‚ Ð¿Ð¾Ð´ÑÑ‚Ð°Ð²Ð¸Ñ‚ÑÑ Ð°Ð²Ñ‚Ð¾Ð¼Ð°Ñ‚Ð¸Ñ‡ÐµÑÐºÐ¸ Ð¿Ð¾ Ð²Ñ‹Ð±Ñ€Ð°Ð½Ð½Ñ‹Ð¼ Ñ„Ð»Ð°Ð³Ð°Ð¼.
                    </p>
                  </label>
                </section>

                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">CORS</h3>
                  <p className="text-[12px] text-sidebar-text">
                    Ð—Ð°Ð¼ÐµÐ½ÑÐµÑ‚ Ð¿ÐµÑ€ÐµÐ¼ÐµÐ½Ð½ÑƒÑŽ `CORS_ALLOWED_ORIGINS`. ÐŸÑ€Ð¸Ð¼ÐµÑ€: `https://buhchat.example.com`.
                    Ð•ÑÐ»Ð¸ Ð¿Ñ€Ð¸Ð»Ð¾Ð¶ÐµÐ½Ð¸Ðµ Ð¾Ñ‚ÐºÑ€Ñ‹Ð²Ð°ÐµÑ‚ÑÑ Ñ‡ÐµÑ€ÐµÐ· Ñ‚Ð¾Ñ‚ Ð¶Ðµ Ð´Ð¾Ð¼ÐµÐ½ (nginx), Ð¾Ð±Ñ‹Ñ‡Ð½Ð¾ Ð´Ð¾ÑÑ‚Ð°Ñ‚Ð¾Ñ‡Ð½Ð¾ `*`.
                  </p>
                  <Field
                    label="Allowed origins"
                    value={serviceForm.cors_allowed_origins}
                    onChange={(v) => setServiceForm((p) => ({ ...p, cors_allowed_origins: v }))}
                    placeholder="* Ð¸Ð»Ð¸ ÑÐ¿Ð¸ÑÐ¾Ðº Ñ‡ÐµÑ€ÐµÐ· Ð·Ð°Ð¿ÑÑ‚ÑƒÑŽ"
                    hint="Ð¡Ð¿Ð¸ÑÐ¾Ðº Ð´Ð¾Ð¼ÐµÐ½Ð¾Ð², ÐºÐ¾Ñ‚Ð¾Ñ€Ñ‹Ð¼ Ñ€Ð°Ð·Ñ€ÐµÑˆÐµÐ½Ñ‹ Ð·Ð°Ð¿Ñ€Ð¾ÑÑ‹ Ðº API Ð¸Ð· Ð±Ñ€Ð°ÑƒÐ·ÐµÑ€Ð°. `*` Ñ€Ð°Ð·Ñ€ÐµÑˆÐ°ÐµÑ‚ Ð²ÑÐµÐ¼. ÐŸÑ€Ð¸Ð¼ÐµÐ½ÑÐµÑ‚ÑÑ ÑÑ€Ð°Ð·Ñƒ."
                  />
                  <p className="text-[11px] text-sidebar-text">ÐŸÑ€Ð¸Ð¼ÐµÑ‡Ð°Ð½Ð¸Ðµ: Ð¿Ð¾ÑÐ»Ðµ Ð¸Ð·Ð¼ÐµÐ½ÐµÐ½Ð¸Ñ CORS Ð¼Ð¾Ð¶ÐµÑ‚ Ð¿Ð¾Ð½Ð°Ð´Ð¾Ð±Ð¸Ñ‚ÑŒÑÑ Ð¾Ð±Ð½Ð¾Ð²Ð¸Ñ‚ÑŒ ÑÑ‚Ñ€Ð°Ð½Ð¸Ñ†Ñƒ Ð² Ð±Ñ€Ð°ÑƒÐ·ÐµÑ€Ðµ.</p>
                </section>

                <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
                  <h3 className="text-[14px] font-semibold text-white">WebSocket (Ð´Ð»Ñ 2000+ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÐµÐ¹)</h3>
                  <p className="text-[12px] text-sidebar-text">
                    Ð­Ñ‚Ð¸ Ð¿Ð¾Ð»Ñ ÑÐ¾Ð¾Ñ‚Ð²ÐµÑ‚ÑÑ‚Ð²ÑƒÑŽÑ‚ Ð¿Ð°Ñ€Ð°Ð¼ÐµÑ‚Ñ€Ð°Ð¼ `max_ws_connections`, `ws_send_buffer_size`, `ws_write_timeout`, `ws_pong_timeout`, `ws_max_message_size`.
                    Ð—Ð½Ð°Ñ‡ÐµÐ½Ð¸Ñ Ð¿Ð¾ ÑƒÐ¼Ð¾Ð»Ñ‡Ð°Ð½Ð¸ÑŽ Ð¿Ð¾Ð´Ð¾Ð±Ñ€Ð°Ð½Ñ‹ Ð´Ð»Ñ 2000+ Ð¾Ð´Ð½Ð¾Ð²Ñ€ÐµÐ¼ÐµÐ½Ð½Ñ‹Ñ… Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÐµÐ¹.
                  </p>
                  <Field
                    label="max_ws_connections"
                    value={String(serviceForm.max_ws_connections)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, max_ws_connections: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="10000"
                    hint="ÐœÐ°ÐºÑÐ¸Ð¼ÑƒÐ¼ Ð¾Ð´Ð½Ð¾Ð²Ñ€ÐµÐ¼ÐµÐ½Ð½Ñ‹Ñ… WebSocket-Ð¿Ð¾Ð´ÐºÐ»ÑŽÑ‡ÐµÐ½Ð¸Ð¹. Ð•ÑÐ»Ð¸ Ð»Ð¸Ð¼Ð¸Ñ‚ Ð´Ð¾ÑÑ‚Ð¸Ð³Ð½ÑƒÑ‚, Ð½Ð¾Ð²Ñ‹Ðµ Ð¿Ð¾Ð´ÐºÐ»ÑŽÑ‡ÐµÐ½Ð¸Ñ Ð±ÑƒÐ´ÑƒÑ‚ Ð¾Ñ‚ÐºÐ»Ð¾Ð½ÑÑ‚ÑŒÑÑ."
                  />
                  <Field
                    label="ws_send_buffer_size"
                    value={String(serviceForm.ws_send_buffer_size)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, ws_send_buffer_size: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="256"
                    hint="Ð Ð°Ð·Ð¼ÐµÑ€ Ð±ÑƒÑ„ÐµÑ€Ð° Ð¸ÑÑ…Ð¾Ð´ÑÑ‰Ð¸Ñ… ÑÐ¾Ð¾Ð±Ñ‰ÐµÐ½Ð¸Ð¹ (Ð² ÑÐ¾Ð¾Ð±Ñ‰ÐµÐ½Ð¸ÑÑ…). Ð‘Ð¾Ð»ÑŒÑˆÐµ: ÑÑ‚Ð°Ð±Ð¸Ð»ÑŒÐ½ÐµÐµ Ð¿Ñ€Ð¸ Ð¿Ð¸ÐºÐ°Ñ…, Ð½Ð¾ Ð±Ð¾Ð»ÑŒÑˆÐµ Ñ€Ð°ÑÑ…Ð¾Ð´ Ð¿Ð°Ð¼ÑÑ‚Ð¸."
                  />
                  <Field
                    label="ws_write_timeout (ÑÐµÐº)"
                    value={String(serviceForm.ws_write_timeout)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, ws_write_timeout: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="10"
                    hint="Ð¡ÐºÐ¾Ð»ÑŒÐºÐ¾ Ð¶Ð´Ð°Ñ‚ÑŒ Ð·Ð°Ð¿Ð¸ÑÑŒ Ð² ÑÐ¾ÐºÐµÑ‚. ÐœÐµÐ½ÑŒÑˆÐµ: Ð±Ñ‹ÑÑ‚Ñ€ÐµÐµ Ð·Ð°ÐºÑ€Ñ‹Ð²Ð°ÑŽÑ‚ÑÑ Â«Ð¼ÐµÐ´Ð»ÐµÐ½Ð½Ñ‹ÐµÂ» ÐºÐ»Ð¸ÐµÐ½Ñ‚Ñ‹. Ð‘Ð¾Ð»ÑŒÑˆÐµ: Ñ‚ÐµÑ€Ð¿Ð¸Ð¼ÐµÐµ Ðº Ð¿Ð»Ð¾Ñ…Ð¾Ð¹ ÑÐµÑ‚Ð¸."
                  />
                  <Field
                    label="ws_pong_timeout (ÑÐµÐº)"
                    value={String(serviceForm.ws_pong_timeout)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, ws_pong_timeout: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="60"
                    hint="Ð¡ÐºÐ¾Ð»ÑŒÐºÐ¾ Ð¶Ð´Ð°Ñ‚ÑŒ pong Ð¾Ñ‚ ÐºÐ»Ð¸ÐµÐ½Ñ‚Ð°. Ð•ÑÐ»Ð¸ pong Ð½Ðµ Ð¿Ñ€Ð¸ÑˆÑ‘Ð», ÑÐ¾ÐµÐ´Ð¸Ð½ÐµÐ½Ð¸Ðµ ÑÑ‡Ð¸Ñ‚Ð°ÐµÑ‚ÑÑ Ð¿Ð¾Ñ‚ÐµÑ€ÑÐ½Ð½Ñ‹Ð¼."
                  />
                  <Field
                    label="ws_max_message_size (Ð±Ð°Ð¹Ñ‚)"
                    value={String(serviceForm.ws_max_message_size)}
                    onChange={(v) => setServiceForm((p) => ({ ...p, ws_max_message_size: Number(v.replace(/\\D/g, '')) || 0 }))}
                    placeholder="4096"
                    hint="ÐžÐ³Ñ€Ð°Ð½Ð¸Ñ‡ÐµÐ½Ð¸Ðµ Ñ€Ð°Ð·Ð¼ÐµÑ€Ð° Ð²Ñ…Ð¾Ð´ÑÑ‰Ð¸Ñ… ÑÐ¾Ð¾Ð±Ñ‰ÐµÐ½Ð¸Ð¹ Ð¿Ð¾ WS. Ð—Ð°Ñ‰Ð¸Ñ‚Ð° Ð¾Ñ‚ ÑÐ»Ð¸ÑˆÐºÐ¾Ð¼ Ð±Ð¾Ð»ÑŒÑˆÐ¸Ñ… payload."
                  />
                  <p className="text-[11px] text-sidebar-text">ÐŸÑ€Ð¸Ð¼ÐµÑ‡Ð°Ð½Ð¸Ðµ: Ð¿Ñ€Ð¸ Ð¸Ð·Ð¼ÐµÐ½ÐµÐ½Ð¸Ð¸ WS-Ð¿Ð°Ñ€Ð°Ð¼ÐµÑ‚Ñ€Ð¾Ð² ÐºÐ»Ð¸ÐµÐ½Ñ‚Ñ‹ Ð¼Ð¾Ð³ÑƒÑ‚ Ð±Ñ‹Ñ‚ÑŒ Ð¿Ñ€Ð¸Ð½ÑƒÐ´Ð¸Ñ‚ÐµÐ»ÑŒÐ½Ð¾ Ð¿ÐµÑ€ÐµÐ¿Ð¾Ð´ÐºÐ»ÑŽÑ‡ÐµÐ½Ñ‹.</p>
                </section>

                <button
                  type="button"
                  onClick={onSaveServiceSettings}
                  disabled={savingService}
                  className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60"
                >
                  {savingService ? 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¸Ðµ...' : 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½Ð¸Ñ‚ÑŒ ÑÐ»ÑƒÐ¶ÐµÐ±Ð½Ñ‹Ðµ Ð½Ð°ÑÑ‚Ñ€Ð¾Ð¹ÐºÐ¸'}
                </button>
              </>
            )}
          </div>
        )}

        {section === 'backup' && (
          <div className="max-w-[760px] mx-auto space-y-4">
            {backupError && <ErrorBox text={backupError} />}

            <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
              <h3 className="text-[14px] font-semibold text-white">Ð¡Ð¾Ð·Ð´Ð°Ñ‚ÑŒ Ñ€ÐµÐ·ÐµÑ€Ð²Ð½ÑƒÑŽ ÐºÐ¾Ð¿Ð¸ÑŽ</h3>
              <p className="text-[12px] text-sidebar-text">
                Ð‘ÑƒÐ´ÐµÑ‚ ÑÐºÐ°Ñ‡Ð°Ð½ ZIP Ñ Ð´Ð°Ð¼Ð¿Ð¾Ð¼ Ð±Ð°Ð·Ñ‹ Ð´Ð°Ð½Ð½Ñ‹Ñ… Ð¸ Ñ„Ð°Ð¹Ð»Ð°Ð¼Ð¸ (uploads/audio/ÐºÐ»ÑŽÑ‡Ð¸ push). ÐÐ° Ð±Ð¾Ð»ÑŒÑˆÐ¸Ñ… Ð´Ð°Ð½Ð½Ñ‹Ñ… ÑÑ‚Ð¾ Ð¼Ð¾Ð¶ÐµÑ‚ Ð·Ð°Ð½ÑÑ‚ÑŒ Ð²Ñ€ÐµÐ¼Ñ.
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
                    onNotify('Ð ÐµÐ·ÐµÑ€Ð²Ð½Ð°Ñ ÐºÐ¾Ð¿Ð¸Ñ ÑÐ¾Ð·Ð´Ð°Ð½Ð°');
                  } catch (e: unknown) {
                    const msg = e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ ÑÐ¾Ð·Ð´Ð°Ñ‚ÑŒ Ñ€ÐµÐ·ÐµÑ€Ð²Ð½ÑƒÑŽ ÐºÐ¾Ð¿Ð¸ÑŽ';
                    setBackupError(msg);
                    onNotify(`ÐžÑˆÐ¸Ð±ÐºÐ°: ${msg}`);
                  } finally {
                    setCreatingBackup(false);
                  }
                }}
                className="w-full min-h-[42px] rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60"
              >
                {creatingBackup ? 'Ð¡Ð¾Ð·Ð´Ð°Ð½Ð¸Ðµ...' : 'Ð¡Ð¾Ð·Ð´Ð°Ñ‚ÑŒ Ñ€ÐµÐ·ÐµÑ€Ð²Ð½ÑƒÑŽ ÐºÐ¾Ð¿Ð¸ÑŽ (ZIP)'}
              </button>
            </section>

            <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/40 space-y-3">
              <h3 className="text-[14px] font-semibold text-white">Ð’Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð¸Ñ‚ÑŒ Ð¸Ð· Ñ€ÐµÐ·ÐµÑ€Ð²Ð½Ð¾Ð¹ ÐºÐ¾Ð¿Ð¸Ð¸</h3>
              <p className="text-[12px] text-sidebar-text">
                Ð’Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð»ÐµÐ½Ð¸Ðµ Ð·Ð°Ð¼ÐµÐ½Ð¸Ñ‚ Ð±Ð°Ð·Ñƒ Ð´Ð°Ð½Ð½Ñ‹Ñ… Ð¸ Ñ„Ð°Ð¹Ð»Ñ‹ (uploads, audio, ÐºÐ»ÑŽÑ‡Ð¸ push). ÐŸÐ¾ÑÐ»Ðµ Ð²Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð»ÐµÐ½Ð¸Ñ Ñ€ÐµÐºÐ¾Ð¼ÐµÐ½Ð´ÑƒÐµÑ‚ÑÑ Ð¿ÐµÑ€ÐµÐ·Ð°Ð¿ÑƒÑÑ‚Ð¸Ñ‚ÑŒ ÐºÐ¾Ð½Ñ‚ÐµÐ¹Ð½ÐµÑ€ API: <code className="bg-sidebar-border/50 px-1 rounded">docker compose restart api</code>.
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
                  if (!confirm('Ð’Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð¸Ñ‚ÑŒ Ð¸Ð· Ñ€ÐµÐ·ÐµÑ€Ð²Ð½Ð¾Ð¹ ÐºÐ¾Ð¿Ð¸Ð¸? Ð­Ñ‚Ð¾ Ð·Ð°Ð¼ÐµÐ½Ð¸Ñ‚ Ñ‚ÐµÐºÑƒÑ‰Ð¸Ðµ Ð´Ð°Ð½Ð½Ñ‹Ðµ.')) return;
                  setBackupError('');
                  setRestoringBackup(true);
                  try {
                    await api.restoreAdminBackup(backupFile);
                    onNotify('Ð’Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð»ÐµÐ½Ð¸Ðµ Ð·Ð°Ð²ÐµÑ€ÑˆÐµÐ½Ð¾. ÐŸÐµÑ€ÐµÐ·Ð°Ð³Ñ€ÑƒÐ·ÐºÐ°...');
                    location.reload();
                  } catch (e: unknown) {
                    const msg = e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ Ð²Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð¸Ñ‚ÑŒ Ñ€ÐµÐ·ÐµÑ€Ð²Ð½ÑƒÑŽ ÐºÐ¾Ð¿Ð¸ÑŽ';
                    setBackupError(msg);
                    onNotify(`ÐžÑˆÐ¸Ð±ÐºÐ°: ${msg}`);
                  } finally {
                    setRestoringBackup(false);
                  }
                }}
                className="w-full min-h-[42px] rounded-compass bg-[#16a34a] text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60"
              >
                {restoringBackup ? 'Ð’Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð»ÐµÐ½Ð¸Ðµ...' : 'Ð’Ð¾ÑÑÑ‚Ð°Ð½Ð¾Ð²Ð¸Ñ‚ÑŒ Ð¸Ð· ZIP'}
              </button>
            </section>
          </div>
        )}
      </main>

      {addOpen && (
        <UserEditorModal
          title="Ð”Ð¾Ð±Ð°Ð²Ð¸Ñ‚ÑŒ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ"
          submitLabel="Ð”Ð¾Ð±Ð°Ð²Ð¸Ñ‚ÑŒ"
          onClose={() => setAddOpen(false)}
          onSubmit={async (payload) => {
            await api.createUser({
              email: payload.email,
              username: payload.username,
              phone: payload.phone || undefined,
              position: payload.position || undefined,
              permissions: payload.permissions,
            });
            reloadUsers();
            onNotify('ÐŸÐ¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÑŒ Ð´Ð¾Ð±Ð°Ð²Ð»ÐµÐ½');
            setAddOpen(false);
          }}
        />
      )}

      {editUser && (
        <UserEditorModal
          title="Ð˜Ð·Ð¼ÐµÐ½Ð¸Ñ‚ÑŒ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ"
          submitLabel="Ð¡Ð¾Ñ…Ñ€Ð°Ð½Ð¸Ñ‚ÑŒ"
          initial={editUser}
          onClose={() => setEditUser(null)}
          onSubmit={async (payload) => {
            await api.updateUserProfile(editUser.id, {
              username: payload.username,
              email: payload.email,
              phone: payload.phone,
              position: payload.position,
            });
            if (payload.permissions) {
              await api.updateUserPermissions(editUser.id, payload.permissions);
            }
            if (typeof payload.disabled === 'boolean') {
              await api.setUserDisabled(editUser.id, payload.disabled);
            }
            reloadUsers();
            onNotify('Ð”Ð°Ð½Ð½Ñ‹Ðµ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ Ð¾Ð±Ð½Ð¾Ð²Ð»ÐµÐ½Ñ‹');
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
        title="Ð¡Ð¾Ñ€Ñ‚Ð¸Ñ€Ð¾Ð²Ð°Ñ‚ÑŒ"
      >
        <span>{label}</span>
        <span className={`text-[11px] ${active ? 'opacity-100' : 'opacity-40'}`}>{active ? (dir === 'asc' ? 'â–²' : 'â–¼') : 'â†•'}</span>
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
  const [position, setPosition] = useState(initial?.position || '');
  const [disabled, setDisabled] = useState(!!initial?.disabled_at);
  const [role, setRole] = useState<'member' | 'administrator'>('member');
  const [loadingPerm, setLoadingPerm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [loginKey, setLoginKey] = useState('');
  const [generatingKey, setGeneratingKey] = useState(false);
  const [copyMessage, setCopyMessage] = useState('');

  // For edit: load current permissions and map to a single "role" selector.
  useEffect(() => {
    if (!initial?.id) return;
    let cancelled = false;
    setLoadingPerm(true);
    api.getUserPermissions(initial.id)
      .then((p) => {
        if (cancelled) return;
        if (p.administrator) setRole('administrator');
        else setRole('member');
      })
      .catch(() => {
        if (!cancelled) setError('ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ Ð·Ð°Ð³Ñ€ÑƒÐ·Ð¸Ñ‚ÑŒ Ñ€Ð¾Ð»Ð¸ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ');
      })
      .finally(() => {
        if (!cancelled) setLoadingPerm(false);
      });
    return () => { cancelled = true; };
  }, [initial?.id]);

  const permissionsFromRole = useCallback((): Partial<Record<keyof Omit<api.UserPermissions, 'user_id' | 'updated_at'>, boolean>> => {
    return {
      member: true,
      administrator: role === 'administrator',
    };
  }, [role]);

  const handleGenerateLoginKey = async () => {
    if (!initial?.id) return;
    setGeneratingKey(true);
    setError('');
    setCopyMessage('');
    try {
      const res = await api.generateUserLoginKey(initial.id);
      setLoginKey(res.login_key);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ ÑÐ³ÐµÐ½ÐµÑ€Ð¸Ñ€Ð¾Ð²Ð°Ñ‚ÑŒ ÐºÐ»ÑŽÑ‡ Ð²Ñ…Ð¾Ð´Ð°');
    } finally {
      setGeneratingKey(false);
    }
  };

  const handleCopyLoginKey = async () => {
    if (!loginKey) return;
    try {
      await navigator.clipboard.writeText(loginKey);
      setCopyMessage('ÐšÐ»ÑŽÑ‡ ÑÐºÐ¾Ð¿Ð¸Ñ€Ð¾Ð²Ð°Ð½');
    } catch {
      const ta = document.createElement('textarea');
      ta.value = loginKey;
      ta.setAttribute('readonly', 'true');
      ta.style.position = 'fixed';
      ta.style.left = '-9999px';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      setCopyMessage('ÐšÐ»ÑŽÑ‡ ÑÐºÐ¾Ð¿Ð¸Ñ€Ð¾Ð²Ð°Ð½');
    }
  };

  const validate = () => {
    if (!username.trim()) return 'Ð˜Ð¼Ñ Ð¾Ð±ÑÐ·Ð°Ñ‚ÐµÐ»ÑŒÐ½Ð¾';
    if (!email.trim()) return 'Email Ð¾Ð±ÑÐ·Ð°Ñ‚ÐµÐ»ÐµÐ½';
    if (!EMAIL_RE.test(email.trim().toLowerCase())) return 'ÐÐµÐ²ÐµÑ€Ð½Ñ‹Ð¹ Ñ„Ð¾Ñ€Ð¼Ð°Ñ‚ email';
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
        position: position.trim(),
        disabled,
        permissions: permissionsFromRole(),
      });
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'ÐÐµ ÑƒÐ´Ð°Ð»Ð¾ÑÑŒ ÑÐ¾Ñ…Ñ€Ð°Ð½Ð¸Ñ‚ÑŒ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»Ñ');
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

        <Field label="Ð˜Ð¼Ñ" value={username} onChange={setUsername} placeholder="Ð˜Ð¼Ñ" />
        <Field label="Email" value={email} onChange={setEmail} placeholder="email@example.com" />
        <Field label="Ð¢ÐµÐ»ÐµÑ„Ð¾Ð½" value={phone} onChange={setPhone} placeholder="+7..." />
        <Field label="Ð”Ð¾Ð»Ð¶Ð½Ð¾ÑÑ‚ÑŒ (Ð¾Ð¿Ñ†Ð¸Ð¾Ð½Ð°Ð»ÑŒÐ½Ð¾)" value={position} onChange={setPosition} placeholder="Ð”Ð¾Ð»Ð¶Ð½Ð¾ÑÑ‚ÑŒ" />

        <label className="block">
          <span className="block text-[12px] text-sidebar-text mb-1">Ð Ð¾Ð»ÑŒ</span>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value as typeof role)}
            disabled={loadingPerm}
            className="w-full h-10 rounded-compass bg-sidebar border border-sidebar-border/40 px-3 text-[13px] text-white outline-none focus:border-primary/60 disabled:opacity-60"
          >
            <option value="member">ÐŸÐ¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÑŒ</option>
            <option value="administrator">ÐÐ´Ð¼Ð¸Ð½Ð¸ÑÑ‚Ñ€Ð°Ñ‚Ð¾Ñ€</option>
          </select>
          <p className="mt-1 text-[11px] text-sidebar-text">Ð Ð¾Ð»Ð¸ Ð¿Ñ€Ð¸Ð¼ÐµÐ½ÑÑŽÑ‚ÑÑ ÑÑ€Ð°Ð·Ñƒ Ð¿Ð¾ÑÐ»Ðµ ÑÐ¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¸Ñ.</p>
        </label>

        {initial && (
          <section className="rounded-compass border border-sidebar-border/40 p-3 bg-sidebar-hover/20 space-y-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-[12px] text-sidebar-text">ÐšÐ»ÑŽÑ‡ Ð²Ñ…Ð¾Ð´Ð°</span>
              <button
                type="button"
                onClick={handleGenerateLoginKey}
                disabled={generatingKey}
                className="h-8 px-3 rounded-compass bg-primary text-white text-[12px] font-semibold hover:brightness-110 transition disabled:opacity-60"
              >
                {generatingKey ? 'Ð“ÐµÐ½ÐµÑ€Ð°Ñ†Ð¸Ñ...' : 'Ð¡Ð³ÐµÐ½ÐµÑ€Ð¸Ñ€Ð¾Ð²Ð°Ñ‚ÑŒ ÐºÐ»ÑŽÑ‡'}
              </button>
            </div>

            <div className="flex items-center gap-2">
              <input
                readOnly
                value={loginKey}
                placeholder="ÐšÐ»ÑŽÑ‡ Ð¿Ð¾ÑÐ²Ð¸Ñ‚ÑÑ Ð¿Ð¾ÑÐ»Ðµ Ð³ÐµÐ½ÐµÑ€Ð°Ñ†Ð¸Ð¸"
                className="flex-1 h-10 rounded-compass bg-sidebar border border-sidebar-border/40 px-3 text-[12px] text-white placeholder:text-sidebar-text/80 outline-none"
              />
              {loginKey && (
                <button
                  type="button"
                  onClick={handleCopyLoginKey}
                  className="h-10 px-3 rounded-compass border border-sidebar-border/60 text-white text-[12px] hover:bg-sidebar-hover"
                >
                  Ð¡ÐºÐ¾Ð¿Ð¸Ñ€Ð¾Ð²Ð°Ñ‚ÑŒ
                </button>
              )}
            </div>

            <p className="text-[11px] text-amber-300/90">
              Ð¡Ð¾Ñ…Ñ€Ð°Ð½Ð¸Ñ‚Ðµ ÐºÐ»ÑŽÑ‡ ÑÑ€Ð°Ð·Ñƒ Ð¿Ð¾ÑÐ»Ðµ Ð³ÐµÐ½ÐµÑ€Ð°Ñ†Ð¸Ð¸. ÐŸÐ¾ÑÐ»Ðµ Ð¿Ð¾Ð²Ñ‚Ð¾Ñ€Ð½Ð¾Ð¹ Ð³ÐµÐ½ÐµÑ€Ð°Ñ†Ð¸Ð¸ ÑÑ‚Ð°Ñ€Ñ‹Ð¹ ÐºÐ»ÑŽÑ‡ ÑÑ‚Ð°Ð½ÐµÑ‚ Ð½ÐµÐ´ÐµÐ¹ÑÑ‚Ð²Ð¸Ñ‚ÐµÐ»ÑŒÐ½Ñ‹Ð¼.
            </p>
            <p className="text-[11px] text-sidebar-text">
              ÐšÐ»ÑŽÑ‡ Ð´Ð»Ñ Ð²Ñ…Ð¾Ð´Ð° Ð±ÐµÐ· email. Ð”Ð¾ÑÑ‚ÑƒÐ¿ÐµÐ½ Ñ‚Ð¾Ð»ÑŒÐºÐ¾ Ð´Ð»Ñ 3 Ð¿Ð¾Ð¿Ñ‹Ñ‚Ð¾Ðº Ð²Ñ…Ð¾Ð´Ð°, Ð¿Ð¾ÑÐ»Ðµ Ñ‡ÐµÐ³Ð¾ Ð°Ð²Ñ‚Ð¾Ð¼Ð°Ñ‚Ð¸Ñ‡ÐµÑÐºÐ¸ ÑÐ±Ñ€Ð°ÑÑ‹Ð²Ð°ÐµÑ‚ÑÑ.
            </p>
            {copyMessage && <p className="text-[11px] text-[#22c55e]">{copyMessage}</p>}
          </section>
        )}

        {initial && (
          <label className="flex items-center gap-2 text-[13px] text-white">
            <input type="checkbox" checked={disabled} onChange={(e) => setDisabled(e.target.checked)} />
            ÐžÑ‚ÐºÐ»ÑŽÑ‡Ð¸Ñ‚ÑŒ Ð²Ñ…Ð¾Ð´ Ð¿Ð¾Ð»ÑŒÐ·Ð¾Ð²Ð°Ñ‚ÐµÐ»ÑŽ
          </label>
        )}

        <button type="button" onClick={handleSubmit} disabled={saving} className="w-full h-10 rounded-compass bg-primary text-white text-[13px] font-semibold hover:brightness-110 transition disabled:opacity-60">
          {saving ? 'Ð¡Ð¾Ñ…Ñ€Ð°Ð½ÐµÐ½Ð¸Ðµ...' : submitLabel}
        </button>
      </div>
    </div>
  );
}
