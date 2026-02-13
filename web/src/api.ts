import type { ChatWithLastMessage, Message, UserPublic, UserStats, FileUploadResponse, PinnedMessage, Reaction } from './types';
import { getApiBase } from './serverUrl';

/** Префикс API-маршрутов; должен совпадать с маршрутами на бэкенде (path = r.URL.Path). */
const API = '/api';

function getApiRoot(): string {
  return getApiBase() + API;
}

/** Ошибка API с кодом ответа (401 = не авторизован, сессия недействительна). */
export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

const SESSION_ID_KEY = 'session_id';
const SESSION_SECRET_KEY = 'session_secret';

function getSessionId(): string | null {
  return localStorage.getItem(SESSION_ID_KEY);
}

function getSessionSecret(): string | null {
  return localStorage.getItem(SESSION_SECRET_KEY);
}

/** HMAC-SHA256(secret, method+path+body+timestamp), результат в hex (как на бэкенде). */
async function signSessionPayload(
  secretBase64: string,
  method: string,
  path: string,
  body: string,
  timestamp: string
): Promise<string> {
  const keyBytes = Uint8Array.from(atob(secretBase64), (c) => c.charCodeAt(0));
  const key = await crypto.subtle.importKey(
    'raw',
    keyBytes,
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign']
  );
  const payload = method + path + body + timestamp;
  const sig = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(payload));
  return Array.from(new Uint8Array(sig))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

async function getSessionAuthHeaders(method: string, path: string, body: string): Promise<Record<string, string> | null> {
  const sessionId = getSessionId();
  const sessionSecret = getSessionSecret();
  if (!sessionId || !sessionSecret) return null;
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const signature = await signSessionPayload(sessionSecret, method, path, body, timestamp);
  return {
    'X-Session-Id': sessionId,
    'X-Timestamp': timestamp,
    'X-Signature': signature,
  };
}

async function request<T>(path: string, opts?: RequestInit): Promise<T> {
  const method = opts?.method ?? 'GET';
  const bodyStr = opts?.body != null && !(opts.body instanceof FormData) ? String(opts.body) : '';
  const headers: Record<string, string> = {};
  if (opts?.body && !(opts.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }
  // Путь для подписи = r.URL.Path на сервере: только pathname с префиксом /api, без query.
  const pathname = path.includes('?') ? path.slice(0, path.indexOf('?')) : path;
  const pathForSignature = `${API}${pathname}`;
  const pathForFetch = `${getApiRoot()}${path}`;
  const sessionHeaders = await getSessionAuthHeaders(method, pathForSignature, bodyStr);
  if (sessionHeaders) Object.assign(headers, sessionHeaders);

  const res = await fetch(pathForFetch, { ...opts, headers: { ...headers, ...(opts?.headers as Record<string, string>) } });

  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    const msg = data.error || `HTTP ${res.status}`;
    const friendly = res.status === 500 ? 'Ошибка сервера. Попробуйте позже.' : msg;
    if (res.status === 500 && msg !== friendly) console.error('API 500:', msg);
    throw new ApiError(friendly, res.status);
  }
  return res.json();
}

/** Публичный запрос без токена (для конфига кеша и т.п.) */
async function requestPublic<T>(path: string): Promise<T> {
  const res = await fetch(`${getApiRoot()}${path}`);
  const text = await res.text();
  if (!res.ok) return Promise.reject(new Error(`HTTP ${res.status}`));
  try {
    return text ? (JSON.parse(text) as T) : ({} as T);
  } catch {
    return Promise.reject(new Error('Invalid response'));
  }
}

// Config (public)
export const getCacheConfig = () =>
  requestPublic<{ ttl_minutes: number }>('/config/cache');

export interface PushConfig {
  enabled: boolean;
  vapid_public_key?: string;
}
export const getPushConfig = () =>
  requestPublic<PushConfig>('/config/push');

export interface AppConfig {
  maintenance: boolean;
  read_only: boolean;
  degradation: boolean;
  message: string;
}
export const getAppConfig = () =>
  requestPublic<AppConfig>('/config/app');

// Install links (public)
export interface InstallLinksConfig {
  install_windows_url: string;
  install_android_url: string;
  install_macos_url: string;
  install_ios_url: string;
}
export const getInstallLinks = () =>
  requestPublic<InstallLinksConfig>('/config/links');

export interface PushSubscriptionKeys {
  p256dh: string;
  auth: string;
}
export interface PushSubscriptionJson {
  endpoint: string;
  keys: PushSubscriptionKeys;
}
export const pushSubscribe = (subscription: PushSubscriptionJson) =>
  request<void>('/push/subscribe', { method: 'POST', body: JSON.stringify({ subscription }) });
export const pushUnsubscribe = (endpoint: string) =>
  request<void>('/push/subscribe', { method: 'DELETE', body: JSON.stringify({ endpoint }) });

// Auth (passwordless: email → OTP → session)
export interface VerifyCodeResponse {
  session_id: string;
  session_secret: string;
  is_new_user: boolean;
}
export interface RequestCodeResponse {
  status?: string;
  session_id?: string;
  session_secret?: string;
  is_new_user?: boolean;
}

/** UUID v4: использует randomUUID или fallback через getRandomValues (для старых браузеров / без HTTPS). */
function randomUUID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function getDeviceId(): string {
  let id = localStorage.getItem('device_id');
  if (!id) {
    id = randomUUID();
    localStorage.setItem('device_id', id);
  }
  return id;
}

export const requestCode = (email: string): Promise<RequestCodeResponse> =>
  fetch(`${getApiRoot()}/auth/request-code`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: email.trim().toLowerCase(), device_id: getDeviceId(), device_name: 'Web' }),
  }).then(async (res) => {
    const data = await res.json().catch(() => ({} as RequestCodeResponse & { error?: string }));
    if (!res.ok) {
      throw new Error(data.error || `HTTP ${res.status}`);
    }
    return data;
  });

export const verifyCode = (email: string, code: string, deviceName?: string): Promise<VerifyCodeResponse> =>
  fetch(`${getApiRoot()}/auth/verify-code`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: email.trim().toLowerCase(),
      code: code.trim(),
      device_id: getDeviceId(),
      device_name: deviceName ?? 'Web',
    }),
  }).then(async (res) => {
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    return data as VerifyCodeResponse;
  });

/** Query string для WebSocket /ws с подписью сессии (session_id, timestamp, signature). */
export async function getSessionWsQuery(): Promise<string | null> {
  const sessionId = getSessionId();
  const sessionSecret = getSessionSecret();
  if (!sessionId || !sessionSecret) return null;
  const path = '/ws';
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const signature = await signSessionPayload(sessionSecret, 'GET', path, '', timestamp);
  return `session_id=${encodeURIComponent(sessionId)}&timestamp=${encodeURIComponent(timestamp)}&signature=${encodeURIComponent(signature)}`;
}

// Users
export const getMe = () => request<UserPublic>('/users/me');
export const getUser = (id: string) => request<UserPublic>(`/users/${id}`);
export const getUserStats = (id: string) => request<UserStats>(`/users/${id}/stats`);

export interface UserPermissions {
  user_id: string;
  administrator: boolean;
  member: boolean;
  assistant_administrator: boolean;
  updated_at?: string;
}

export interface MailSettings {
  host: string;
  port: number;
  username: string;
  password: string;
  from_email: string;
  from_name: string;
  updated_at?: string;
}

export interface MailTestError {
  error?: string;
  error_code?: string;
  detail?: string;
}
export interface FileSettings {
  max_file_size_mb: number;
  updated_at?: string;
}

export const getUserPermissions = (userId: string) =>
  request<UserPermissions>(`/users/${userId}/permissions`);
export const updateUserPermissions = (userId: string, data: Partial<Record<keyof Omit<UserPermissions, 'user_id' | 'updated_at'>, boolean>>) =>
  request<UserPermissions>(`/users/${userId}/permissions`, { method: 'PUT', body: JSON.stringify(data) });
export const getMailSettings = () =>
  request<MailSettings>('/admin/mail-settings');
export const updateMailSettings = (data: MailSettings) =>
  request<MailSettings>('/admin/mail-settings', { method: 'PUT', body: JSON.stringify(data) });
export const sendTestMail = async (toEmail: string): Promise<{ status: string }> => {
  const res = await fetch(`${getApiRoot()}/admin/mail-settings/test`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(await getSessionAuthHeaders('POST', '/api/admin/mail-settings/test', JSON.stringify({ to_email: toEmail.trim().toLowerCase() })) || {}),
    },
    body: JSON.stringify({ to_email: toEmail.trim().toLowerCase() }),
  });
  const data = await res.json().catch(() => ({} as MailTestError));
  if (!res.ok) {
    const code = (data as MailTestError).error_code || `HTTP_${res.status}`;
    const detail = (data as MailTestError).detail || (data as MailTestError).error || `HTTP ${res.status}`;
    throw new Error(`${code}: ${detail}`);
  }
  return data as { status: string };
};
export const getAdminFileSettings = () =>
  request<FileSettings>('/admin/file-settings');
export const updateAdminFileSettings = (maxFileSizeMB: number) =>
  request<FileSettings>('/admin/file-settings', { method: 'PUT', body: JSON.stringify({ max_file_size_mb: maxFileSizeMB }) });

// Service settings (admin)
export interface ServiceSettings {
  maintenance: boolean;
  read_only: boolean;
  degradation: boolean;
  status_message: string;
  cors_allowed_origins: string;
  install_windows_url: string;
  install_android_url: string;
  install_macos_url: string;
  install_ios_url: string;
  max_ws_connections: number;
  ws_send_buffer_size: number;
  ws_write_timeout: number;
  ws_pong_timeout: number;
  ws_max_message_size: number;
  updated_at?: string;
}
export const getServiceSettings = () =>
  request<ServiceSettings>('/admin/service-settings');
export const updateServiceSettings = (data: ServiceSettings) =>
  request<ServiceSettings>('/admin/service-settings', { method: 'PUT', body: JSON.stringify(data) });

// Backup / restore (admin)
export const downloadAdminBackup = async (): Promise<Blob> => {
  const path = '/admin/backup';
  const headers = await getSessionAuthHeaders('GET', '/api/admin/backup', '');
  const res = await fetch(`${getApiRoot()}${path}`, {
    method: 'GET',
    headers: {
      ...(headers || {}),
    },
  });
  if (!res.ok) {
    const msg = await res.text().catch(() => '');
    throw new ApiError(msg || `HTTP ${res.status}`, res.status);
  }
  return await res.blob();
};

export const restoreAdminBackup = (file: File) => {
  const fd = new FormData();
  fd.append('file', file);
  return request<{ status: string }>('/admin/restore', { method: 'POST', body: fd });
};
export const getPublicFileSettings = () =>
  requestPublic<FileSettings>('/config/file-settings');
export const listUsers = () => request<UserPublic[]>('/users');
/** Список всех сотрудников (только для администратора). */
export const listEmployees = () => request<UserPublic[]>('/users/employees');
export interface EmployeePublic extends UserPublic {
  role: 'member' | 'assistant_administrator' | 'administrator';
}
export interface EmployeesPage {
  users: EmployeePublic[];
  total: number;
  limit: number;
  offset: number;
}
export const listEmployeesPage = (params: { q?: string; limit?: number; offset?: number; sort_key?: string; sort_dir?: string }) => {
  const sp = new URLSearchParams();
  if (params.q) sp.set('q', params.q);
  if (typeof params.limit === 'number') sp.set('limit', String(params.limit));
  if (typeof params.offset === 'number') sp.set('offset', String(params.offset));
  if (params.sort_key) sp.set('sort_key', params.sort_key);
  if (params.sort_dir) sp.set('sort_dir', params.sort_dir);
  const qs = sp.toString();
  return request<EmployeesPage>(`/users/employees/page${qs ? `?${qs}` : ''}`);
};
/** Создать пользователя (админ). При первом входе по этой почте это будет его профиль. */
export const createUser = (data: {
  email: string;
  username: string;
  phone?: string;
  company?: string;
  position?: string;
  avatar_url?: string;
  permissions?: Partial<Record<keyof Omit<UserPermissions, 'user_id' | 'updated_at'>, boolean>>;
}) => request<UserPublic>('/users', { method: 'POST', body: JSON.stringify(data) });
export const searchUsers = (q: string) => request<UserPublic[]>(`/users/search?q=${encodeURIComponent(q)}`);
export const updateProfile = (data: { username?: string; avatar_url?: string; email?: string; phone?: string; company?: string; position?: string }) =>
  request<UserPublic>('/users/me', { method: 'PUT', body: JSON.stringify(data) });
export const updateUserProfile = (userId: string, data: { username?: string; avatar_url?: string; email?: string; phone?: string; company?: string; position?: string }) =>
  request<UserPublic>(`/users/${userId}`, { method: 'PUT', body: JSON.stringify(data) });
/** Отключить или включить пользователя (только администратор). Отключённый не может войти. */
export const setUserDisabled = (userId: string, disabled: boolean) =>
  request<{ disabled: boolean }>(`/users/${userId}/disable`, { method: 'PUT', body: JSON.stringify({ disabled }) });

/** Выкинуть пользователя со всех устройств (только администратор). */
export const logoutAllUserSessions = (userId: string) =>
  request<{ status: string; revoked: number }>(`/users/${userId}/logout-all`, { method: 'POST' });
export const getFavorites = () =>
  request<{ chat_ids: string[] }>('/users/me/favorites').then((r) => r.chat_ids);
export const addFavorite = (chatId: string) =>
  request<unknown>('/users/me/favorites', { method: 'POST', body: JSON.stringify({ chat_id: chatId }) });
export const removeFavorite = (chatId: string) =>
  request<unknown>(`/users/me/favorites/${chatId}`, { method: 'DELETE' });

// Chats
export const getChats = () => request<ChatWithLastMessage[]>('/chats');
export const getChat = (id: string) => request<ChatWithLastMessage>(`/chats/${id}`);
export const createPersonalChat = (userId: string) =>
  request<ChatWithLastMessage>('/chats/personal', { method: 'POST', body: JSON.stringify({ user_id: userId }) });
export const createGroupChat = (name: string, memberIds: string[]) =>
  request<ChatWithLastMessage>('/chats/group', { method: 'POST', body: JSON.stringify({ name, member_ids: memberIds }) });
export const updateChat = (id: string, data: { name?: string; description?: string; avatar_url?: string }) =>
  request<unknown>(`/chats/${id}`, { method: 'PUT', body: JSON.stringify(data) });
export const setChatMuted = (chatId: string, muted: boolean) =>
  request<{ muted: boolean }>(`/chats/${chatId}/mute`, { method: 'POST', body: JSON.stringify({ muted }) });
export const clearChatHistory = (chatId: string) =>
  request<unknown>(`/chats/${chatId}/clear`, { method: 'POST' });
export const addMembers = (chatId: string, memberIds: string[]) =>
  request<unknown>(`/chats/${chatId}/members`, { method: 'POST', body: JSON.stringify({ member_ids: memberIds }) });
export const removeMember = (chatId: string, memberId: string) =>
  request<unknown>(`/chats/${chatId}/members/${memberId}`, { method: 'DELETE' });
export const leaveChat = (chatId: string) =>
  request<unknown>(`/chats/${chatId}/leave`, { method: 'POST' });

// Messages
export const getMessages = (chatId: string, limit = 50, offset = 0) =>
  request<Message[]>(`/chats/${chatId}/messages?limit=${limit}&offset=${offset}`);
export const markAsRead = (chatId: string) =>
  request<unknown>(`/chats/${chatId}/read`, { method: 'POST' });
export const searchMessages = (q: string, limit = 30, chatId?: string) => {
  const params = new URLSearchParams({ q, limit: String(limit) });
  if (chatId) params.set('chat_id', chatId);
  return request<Message[]>(`/messages/search?${params.toString()}`);
};
export const getPinnedMessages = (chatId: string) =>
  request<PinnedMessage[]>(`/chats/${chatId}/pinned`);
export const getReactions = (messageId: string) =>
  request<Reaction[]>(`/messages/${messageId}/reactions`);

// Files
export const uploadFile = async (file: File): Promise<FileUploadResponse> => {
  const fd = new FormData();
  fd.append('file', file);
  return request<FileUploadResponse>('/files/upload', { method: 'POST', body: fd });
};

// Voice (audio messages — отдельный микросервис)
export const uploadAudio = async (file: File): Promise<FileUploadResponse> => {
  const fd = new FormData();
  fd.append('file', file);
  return request<FileUploadResponse>('/audio/upload', { method: 'POST', body: fd });
};
