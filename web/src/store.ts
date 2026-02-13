import { create } from 'zustand';
import * as api from './api';
import { CACHE_TTL_MS } from './config';
import { updateFaviconBadge } from './faviconBadge';
import { getWebSocketBase } from './serverUrl';
import { unsubscribePushIfEnabled } from './push';
import type { ChatWithLastMessage, Message, UserPublic, PinnedMessage } from './types';

/** "+" в имени файла часто приходит вместо пробела — нормализуем при получении с сервера. */
function normalizeMessageFileName(m: Message): Message {
  if (!m?.file_name) return m;
  const name = m.file_name.replace(/\+/g, ' ').trim();
  return name === m.file_name ? m : { ...m, file_name: name };
}

/* ─── Auth Store ─── */
const SESSION_ID_KEY = 'session_id';
const SESSION_SECRET_KEY = 'session_secret';

const PROFILE_LOAD_TIMEOUT_MS = 15000;

function profileLoadErrorMessage(err: unknown): string {
  if (err instanceof Error) {
    if (err.message === 'Failed to fetch' || err.message.includes('NetworkError')) return 'Нет связи с сервером. Проверьте интернет и повторите.';
    return err.message;
  }
  return 'Не удалось загрузить профиль';
}

/** getMe с таймаутом, чтобы не зависать на «Загрузка профиля...» при недоступном сервере. */
async function getMeWithTimeout(): Promise<Awaited<ReturnType<typeof api.getMe>>> {
  return Promise.race([
    api.getMe(),
    new Promise<never>((_, reject) =>
      setTimeout(() => reject(new Error('Таймаут. Сервер не отвечает.')), PROFILE_LOAD_TIMEOUT_MS)
    ),
  ]);
}

interface AuthState {
  user: UserPublic | null;
  isAuthenticated: boolean;
  /** Сообщение об ошибке загрузки профиля (сеть / сервер); сбрасывается при повторной попытке или успехе. */
  profileLoadError: string | null;
  requestCode: (email: string) => Promise<boolean>;
  verifyCode: (email: string, code: string, deviceName?: string) => Promise<void>;
  logout: () => void;
  loadUser: () => Promise<void>;
  loadUserOrRetry: () => Promise<void>;
  updateProfile: (data: { username?: string; avatar_url?: string; email?: string; phone?: string; company?: string; position?: string }) => Promise<void>;
  init: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isAuthenticated: false,
  profileLoadError: null,

  init: () => {
    const sessionId = localStorage.getItem(SESSION_ID_KEY);
    const sessionSecret = localStorage.getItem(SESSION_SECRET_KEY);
    if (sessionId && sessionSecret) {
      set({ isAuthenticated: true });
      get().loadUser();
    }
  },

  requestCode: async (email) => {
    const res = await api.requestCode(email);
    if (res.session_id && res.session_secret) {
      localStorage.setItem(SESSION_ID_KEY, res.session_id);
      localStorage.setItem(SESSION_SECRET_KEY, res.session_secret);
      set({ isAuthenticated: true });
      await get().loadUserOrRetry();
      return true;
    }
    return false;
  },

  verifyCode: async (email, code, deviceName) => {
    const res = await api.verifyCode(email, code, deviceName);
    localStorage.setItem(SESSION_ID_KEY, res.session_id);
    localStorage.setItem(SESSION_SECRET_KEY, res.session_secret);
    set({ isAuthenticated: true });
    await get().loadUserOrRetry();
  },

  logout: () => {
    unsubscribePushIfEnabled().catch(() => {});
    localStorage.removeItem(SESSION_ID_KEY);
    localStorage.removeItem(SESSION_SECRET_KEY);
    set({ user: null, isAuthenticated: false, profileLoadError: null });
    useChatStore.getState().reset();
  },

  loadUser: async () => {
    set({ profileLoadError: null });
    try {
      const user = await getMeWithTimeout();
      set({ user, profileLoadError: null });
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 401) {
        get().logout();
      } else {
        set({ user: null, profileLoadError: profileLoadErrorMessage(err) });
      }
    }
  },

  /** Загрузка профиля с однократным повтором при 401 (сессия могла ещё не попасть в Redis). */
  loadUserOrRetry: async () => {
    set({ profileLoadError: null });
    try {
      const user = await getMeWithTimeout();
      set({ user, profileLoadError: null });
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 401) {
        await new Promise((r) => setTimeout(r, 400));
        try {
          const user = await getMeWithTimeout();
          set({ user, profileLoadError: null });
        } catch (retryErr) {
          if (retryErr instanceof api.ApiError && (retryErr as api.ApiError).status === 401) {
            get().logout();
          } else {
            set({ user: null, profileLoadError: profileLoadErrorMessage(retryErr) });
          }
        }
      } else {
        set({ user: null, profileLoadError: profileLoadErrorMessage(err) });
      }
    }
  },

  updateProfile: async (data) => {
    const user = await api.updateProfile(data);
    set({ user });
  },
}));

/* ─── Theme Store (light / dark / system) ─── */
export type ThemePreference = 'light' | 'dark' | 'system';

const THEME_KEY = 'compass-theme';

function getSystemDark(): boolean {
  if (typeof window === 'undefined') return false;
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function setThemeColorMeta(dark: boolean) {
  if (typeof document === 'undefined') return;
  const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
  if (meta) meta.setAttribute('content', dark ? '#1C1C1C' : '#F8F8F8');
}

function applyTheme(dark: boolean) {
  if (typeof document === 'undefined') return;
  if (dark) document.documentElement.classList.add('dark');
  else document.documentElement.classList.remove('dark');
  setThemeColorMeta(dark);
}

interface ThemeState {
  preference: ThemePreference;
  isDark: boolean;
  setTheme: (preference: ThemePreference) => void;
  init: () => void;
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  preference: 'system',
  isDark: false,

  setTheme: (preference) => {
    localStorage.setItem(THEME_KEY, preference);
    const isDark = preference === 'dark' || (preference === 'system' && getSystemDark());
    applyTheme(isDark);
    set({ preference, isDark });
  },

  init: () => {
    const stored = localStorage.getItem(THEME_KEY) as ThemePreference | null;
    const preference = stored === 'light' || stored === 'dark' || stored === 'system' ? stored : 'system';
    const isDark = preference === 'dark' || (preference === 'system' && getSystemDark());
    applyTheme(isDark);
    set({ preference, isDark });
    if (preference === 'system' && typeof window !== 'undefined') {
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
        const next = getSystemDark();
        applyTheme(next);
        set({ isDark: next });
      });
    }
  },
}));

/* ─── Chat Store ─── */
interface ChatState {
  chats: ChatWithLastMessage[];
  activeChatId: string | null;
  messages: Record<string, Message[]>;
  typingUsers: Record<string, string[]>;
  onlineUsers: Record<string, boolean>;
  pinnedMessages: Record<string, PinnedMessage[]>;
  favoriteChatIds: string[];
  lastChatsFetchAt: number;
  lastFavoritesFetchAt: number;
  cacheTTLMs: number;
  maxFileSizeMB: number;
  replyTo: Message | null;
  editingMessage: Message | null;
  notification: string | null;
  appStatus: { maintenance: boolean; read_only: boolean; degradation: boolean; message: string };
  setNotification: (text: string | null) => void;
  loadAppStatus: () => Promise<void>;
  ws: WebSocket | null;
  wsReconnectAttempt: number;
  wsReconnectTimer: ReturnType<typeof setTimeout> | null;
  pendingMessages: PendingMessageEntry[];

  fetchChats: () => Promise<void>;
  setActiveChat: (chatId: string | null) => void;
  fetchMessages: (chatId: string) => Promise<void>;
  sendMessage: (chatId: string, content: string, opts?: SendMessageOpts) => void;
  sendTyping: (chatId: string) => void;
  markAsRead: (chatId: string) => void;
  editMessage: (messageId: string, content: string) => void;
  deleteMessage: (messageId: string) => void;
  addReaction: (messageId: string, emoji: string) => void;
  removeReaction: (messageId: string, emoji: string) => void;
  pinMessage: (chatId: string, messageId: string) => void;
  unpinMessage: (chatId: string, messageId: string) => void;
  fetchPinnedMessages: (chatId: string) => Promise<void>;
  fetchFavorites: () => Promise<void>;
  fetchChatsIfStale: () => Promise<void>;
  fetchFavoritesIfStale: () => Promise<void>;
  loadCacheConfig: () => Promise<void>;
  loadFileSettings: () => Promise<void>;
  invalidateChatsCache: () => void;
  invalidateFavoritesCache: () => void;
  hydrateFavoritesFromStorage: () => void;
  addFavorite: (chatId: string) => Promise<void>;
  removeFavorite: (chatId: string) => Promise<void>;
  toggleFavorite: (chatId: string) => Promise<void>;
  setChatMuted: (chatId: string, muted: boolean) => Promise<void>;
  clearChatHistory: (chatId: string) => Promise<void>;
  setReplyTo: (msg: Message | null) => void;
  setEditingMessage: (msg: Message | null) => void;
  connectWS: () => void;
  disconnectWS: () => void;
  flushPendingMessages: () => void;
  createPersonalChat: (userId: string) => Promise<ChatWithLastMessage>;
  createGroupChat: (name: string, memberIds: string[]) => Promise<ChatWithLastMessage>;
  searchUsers: (query: string) => Promise<UserPublic[]>;
  searchMessages: (query: string, chatId?: string) => Promise<Message[]>;
  uploadFile: (file: File) => Promise<{ url: string; file_name: string; file_size: number; content_type: string }>;
  uploadVoice: (file: File) => Promise<{ url: string; file_name: string; file_size: number; content_type: string }>;
  addOptimisticVoiceMessage: (chatId: string) => { optId: string; clientMsgId: string };
  removeOptimisticMessage: (chatId: string, optId: string) => void;
  updateOptimisticVoiceMessage: (chatId: string, optId: string, opts: { fileUrl: string; fileName?: string; fileSize?: number }) => void;
  sendMessageWsOnly: (chatId: string, content: string, opts?: SendMessageOpts) => void;
  leaveChat: (chatId: string) => Promise<void>;
  handleWSMessage: (data: { type: string; payload: any }) => void;
  updateElectronBadge: () => void;
  reset: () => void;
}

function favoritesStorageKey(userId: string): string {
  return `compass-favorites-${userId || 'anon'}`;
}

// Таймеры снятия «печатает» по (chat_id:user_id); сбрасываются при новом typing-событии
const typingClearTimeouts: Record<string, ReturnType<typeof setTimeout>> = {};

// Время отключения WS; при переподключении после долгого простоя — полное обновление страницы (перезапуск докера)
let wsDisconnectedAt: number | null = null;
const WS_LONG_DISCONNECT_MS = 5000;

type SendMessageOpts = {
  contentType?: string;
  fileUrl?: string;
  fileName?: string;
  fileSize?: number;
  replyToId?: string;
  clientMsgId?: string;
};

type PendingMessageEntry = {
  chatId: string;
  content: string;
  clientMsgId: string;
  opts?: SendMessageOpts;
};

function loadFavoritesFromStorage(userId: string): string[] {
  try {
    const s = localStorage.getItem(favoritesStorageKey(userId));
    if (s) {
      const a = JSON.parse(s);
      if (Array.isArray(a)) return a;
    }
  } catch { /* ignore */ }
  return [];
}

function saveFavoritesToStorage(userId: string, ids: string[]) {
  try {
    localStorage.setItem(favoritesStorageKey(userId), JSON.stringify(ids));
  } catch { /* ignore */ }
}

let chatsFetchInFlight: Promise<void> | null = null;

const initialChatState = {
  chats: [] as ChatWithLastMessage[],
  activeChatId: null as string | null,
  messages: {} as Record<string, Message[]>,
  typingUsers: {} as Record<string, string[]>,
  onlineUsers: {} as Record<string, boolean>,
  pinnedMessages: {} as Record<string, PinnedMessage[]>,
  favoriteChatIds: [] as string[],
  lastChatsFetchAt: 0 as number,
  lastFavoritesFetchAt: 0 as number,
  cacheTTLMs: CACHE_TTL_MS,
  maxFileSizeMB: 20,
  replyTo: null as Message | null,
  editingMessage: null as Message | null,
  notification: null as string | null,
  appStatus: { maintenance: false, read_only: false, degradation: false, message: '' } as { maintenance: boolean; read_only: boolean; degradation: boolean; message: string },
  setNotification: (() => {}) as (text: string | null) => void,
  ws: null as WebSocket | null,
  wsReconnectAttempt: 0,
  wsReconnectTimer: null as ReturnType<typeof setTimeout> | null,
  pendingMessages: [] as PendingMessageEntry[],
};

function fileLimitMessage(maxFileSizeMB: number): string {
  return `Превышен лимит размера файла: ${maxFileSizeMB} МБ.`;
}

function mentionTokenFromUsername(username: string): string {
  return username.trim().replace(/\s+/g, '_').toLowerCase();
}

function hasUserMention(content: string, username: string): boolean {
  const token = mentionTokenFromUsername(username);
  if (!content || !token) return false;
  const mentionRe = /@([^\s@]+)/g;
  let m: RegExpExecArray | null = null;
  while ((m = mentionRe.exec(content)) !== null) {
    const v = m[1].toLowerCase();
    if (v === token) return true;
  }
  return false;
}

function makeClientMsgID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `cm-${crypto.randomUUID()}`;
  }
  return `cm-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export const useChatStore = create<ChatState>((set, get) => ({
  ...initialChatState,

  setNotification: (text) => set({ notification: text }),
  loadAppStatus: async () => {
    try {
      const status = await api.getAppConfig();
      set({ appStatus: status });
    } catch {
      // Keep previous status on temporary network errors.
    }
  },
  loadFileSettings: async () => {
    try {
      const s = await api.getPublicFileSettings();
      if (s.max_file_size_mb > 0) set({ maxFileSizeMB: s.max_file_size_mb });
    } catch {
      // keep default
    }
  },

  fetchChats: async () => {
    if (chatsFetchInFlight) return chatsFetchInFlight;
    const run = (async () => {
      const prev = get();
      let chats: ChatWithLastMessage[];
      let favoriteIds: string[] | null = null;
      try {
        chats = await api.getChats();
      } catch {
        return; // при ошибке не трогаем список — остаётся предыдущее состояние
      }
      try {
        const ids = await api.getFavorites();
        favoriteIds = Array.isArray(ids) ? ids : [];
      } catch {
        favoriteIds = null;
      }
      chats = chats.map((c) => ({
        ...c,
        muted: c.muted ?? false,
        ...(c.last_message ? { last_message: normalizeMessageFileName(c.last_message) } : {}),
      }));
      chats.sort((a, b) => {
        const at = a.last_message?.created_at || a.chat.created_at;
        const bt = b.last_message?.created_at || b.chat.created_at;
        return new Date(bt).getTime() - new Date(at).getTime();
      });
      const onlineUsers: Record<string, boolean> = {};
      for (const c of chats) {
        for (const m of c.members) {
          onlineUsers[m.id] = m.is_online;
        }
      }
      const next: { chats: ChatWithLastMessage[]; onlineUsers: Record<string, boolean>; favoriteChatIds?: string[] } = { chats, onlineUsers };
      if (favoriteIds !== null) {
        next.favoriteChatIds = favoriteIds;
        const uid = useAuthStore.getState().user?.id;
        if (uid) saveFavoritesToStorage(uid, favoriteIds);
      } else {
        next.favoriteChatIds = prev.favoriteChatIds;
      }
      set({ ...next, lastChatsFetchAt: Date.now() });
      get().updateElectronBadge();
    })();
    chatsFetchInFlight = run;
    try {
      await run;
    } finally {
      if (chatsFetchInFlight === run) chatsFetchInFlight = null;
    }
  },

  setActiveChat: (chatId) => {
    set({ activeChatId: chatId, replyTo: null, editingMessage: null });
    if (chatId) {
      get().fetchMessages(chatId);
      get().markAsRead(chatId);
      get().fetchPinnedMessages(chatId);
    }
  },

  fetchMessages: async (chatId) => {
    const msgs = (await api.getMessages(chatId, 100, 0)).map(normalizeMessageFileName);
    msgs.reverse();
    set((s) => {
      const existing = s.messages[chatId] || [];
      const optimistic = existing.filter((m) => m.id.startsWith('opt-'));
      const merged = [...msgs];
      for (const opt of optimistic) {
        if (!merged.some((m) => m.id === opt.id)) merged.push(opt);
      }
      merged.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
      return { messages: { ...s.messages, [chatId]: merged } };
    });
  },

  sendMessage: (chatId, content, opts) => {
    if (get().appStatus.read_only) {
      get().setNotification('Идёт обслуживание: отправка сообщений временно отключена.');
      return;
    }
    const { ws, replyTo } = get();
    const user = useAuthStore.getState().user;
    const now = new Date().toISOString();
    const clientMsgId = (opts?.clientMsgId || makeClientMsgID()).trim();
    const replyToID = opts?.replyToId || replyTo?.id;
    const optimistic: Message = {
      id: `opt-${clientMsgId}`,
      client_msg_id: clientMsgId,
      chat_id: chatId,
      sender_id: user?.id ?? '',
      content,
      content_type: (opts?.contentType as Message['content_type']) || 'text',
      file_url: opts?.fileUrl ?? '',
      file_name: opts?.fileName ?? '',
      file_size: opts?.fileSize ?? 0,
      status: 'sent',
      is_deleted: false,
      created_at: now,
      sender: user ? { ...user, is_online: true, last_seen_at: now } : undefined,
      reply_to_id: replyToID,
      reply_to: replyTo ?? undefined,
    };
    const pendingEntry: PendingMessageEntry = {
      chatId,
      content,
      clientMsgId,
      opts: { ...opts, replyToId: replyToID, clientMsgId },
    };
    const shouldQueue = !ws || ws.readyState !== WebSocket.OPEN;

    set((s) => {
      const nextMessages = {
        ...s.messages,
        [chatId]: [...(s.messages[chatId] || []).filter((m) => m.client_msg_id !== clientMsgId), optimistic],
      };
      const nextChats = s.chats.map((c) =>
        c.chat.id === chatId ? { ...c, last_message: optimistic } : c
      );
      nextChats.sort((a, b) => {
        const at = a.last_message?.created_at || a.chat.created_at;
        const bt = b.last_message?.created_at || b.chat.created_at;
        return new Date(bt).getTime() - new Date(at).getTime();
      });
      return {
        messages: nextMessages,
        chats: nextChats,
        replyTo: null,
        pendingMessages: shouldQueue ? [...s.pendingMessages, pendingEntry] : s.pendingMessages,
      };
    });

    if (shouldQueue) {
      get().setNotification('Сообщение будет отправлено при появлении связи.');
      return;
    }

    try {
      ws.send(JSON.stringify({
        type: 'new_message',
        chat_id: chatId,
        client_msg_id: clientMsgId,
        content,
        content_type: opts?.contentType || 'text',
        file_url: opts?.fileUrl || '',
        file_name: opts?.fileName || '',
        file_size: opts?.fileSize || 0,
        reply_to_id: replyToID || '',
      }));
    } catch (e) {
      console.error('ws send error:', e);
      set((s) => ({
        pendingMessages: [...s.pendingMessages.filter((p) => p.clientMsgId !== clientMsgId), pendingEntry],
      }));
      get().setNotification('Сообщение будет отправлено при появлении связи.');
    }
  },

  sendTyping: (chatId) => {
    const { ws } = get();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: 'typing', chat_id: chatId }));
  },

  markAsRead: (chatId) => {
    const { ws } = get();
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'message_read', chat_id: chatId }));
    }
    set((s) => ({
      chats: s.chats.map((c) => c.chat.id === chatId ? { ...c, unread_count: 0 } : c),
    }));
    get().updateElectronBadge();
  },

  updateElectronBadge: () => {
    if (typeof window === 'undefined') return;
    const total = get().chats.reduce((n, c) => n + (c.unread_count || 0), 0);
    document.title = total > 0 ? `(${total}) BuhChat` : 'BuhChat';
    updateFaviconBadge(total);
    const api = (window as unknown as { electronAPI?: { setBadgeCount?: (n: number) => void } }).electronAPI;
    if (api?.setBadgeCount) {
      api.setBadgeCount(total);
      return;
    }
    const nav = navigator as Navigator & { setAppBadge?(n: number): Promise<void>; clearAppBadge?(): Promise<void> };
    if (nav.setAppBadge) {
      if (total > 0) nav.setAppBadge(total).catch(() => {});
      else nav.clearAppBadge?.().catch(() => {});
    }
  },

  editMessage: (messageId, content) => {
    const { ws } = get();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const editedAt = new Date().toISOString();
    set((s) => {
      const nextMessages: Record<string, Message[]> = { ...s.messages };
      for (const [chatId, msgs] of Object.entries(s.messages)) {
        nextMessages[chatId] = msgs.map((m) =>
          m.id === messageId ? { ...m, content, edited_at: editedAt, is_deleted: false } : m
        );
      }
      return {
        messages: nextMessages,
        chats: s.chats.map((c) => {
          if (!c.last_message || c.last_message.id !== messageId) return c;
          return { ...c, last_message: { ...c.last_message, content, edited_at: editedAt, is_deleted: false } };
        }),
      };
    });
    try { ws.send(JSON.stringify({ type: 'message_edited', message_id: messageId, content })); } catch { /* */ }
    set({ editingMessage: null });
  },

  deleteMessage: (messageId) => {
    const { ws } = get();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const tombstone = (m: Message): Message => ({
      ...m,
      is_deleted: true,
      content: '',
      content_type: 'text',
      file_url: '',
      file_name: '',
      file_size: 0,
    });
    set((s) => {
      const nextMessages: Record<string, Message[]> = { ...s.messages };
      const nextPinned: Record<string, PinnedMessage[]> = { ...s.pinnedMessages };
      for (const [chatId, msgs] of Object.entries(s.messages)) {
        nextMessages[chatId] = msgs.map((m) => (m.id === messageId ? tombstone(m) : m));
        const pins = s.pinnedMessages[chatId];
        if (pins) {
          nextPinned[chatId] = pins.map((p) => {
            if (p.message_id !== messageId || !p.message) return p;
            return { ...p, message: tombstone(p.message) };
          });
        }
      }
      return {
        messages: nextMessages,
        pinnedMessages: nextPinned,
        chats: s.chats.map((c) => {
          if (!c.last_message || c.last_message.id !== messageId) return c;
          return { ...c, last_message: tombstone(c.last_message) };
        }),
      };
    });
    try { ws.send(JSON.stringify({ type: 'message_deleted', message_id: messageId })); } catch { /* */ }
  },

  addReaction: (messageId, emoji) => {
    const { ws } = get();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const myId = useAuthStore.getState().user?.id;
    if (myId) {
      set((s) => {
        const nextMessages: Record<string, Message[]> = { ...s.messages };
        for (const [chatId, msgs] of Object.entries(s.messages)) {
          nextMessages[chatId] = msgs.map((m) => {
            if (m.id !== messageId) return m;
            const reactions = [...(m.reactions || [])];
            if (!reactions.some((r) => r.user_id === myId && r.emoji === emoji)) {
              reactions.push({ message_id: messageId, user_id: myId, emoji, created_at: new Date().toISOString() });
            }
            return { ...m, reactions };
          });
        }
        return { messages: nextMessages };
      });
    }
    try { ws.send(JSON.stringify({ type: 'reaction_added', message_id: messageId, emoji })); } catch { /* */ }
  },

  removeReaction: (messageId, emoji) => {
    const { ws } = get();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const myId = useAuthStore.getState().user?.id;
    if (myId) {
      set((s) => {
        const nextMessages: Record<string, Message[]> = { ...s.messages };
        for (const [chatId, msgs] of Object.entries(s.messages)) {
          nextMessages[chatId] = msgs.map((m) => {
            if (m.id !== messageId) return m;
            return { ...m, reactions: (m.reactions || []).filter((r) => !(r.user_id === myId && r.emoji === emoji)) };
          });
        }
        return { messages: nextMessages };
      });
    }
    try { ws.send(JSON.stringify({ type: 'reaction_removed', message_id: messageId, emoji })); } catch { /* */ }
  },

  pinMessage: (chatId, messageId) => {
    const { ws } = get();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    try { ws.send(JSON.stringify({ type: 'message_pinned', chat_id: chatId, message_id: messageId })); } catch { /* */ }
  },

  unpinMessage: (chatId, messageId) => {
    set((s) => ({
      pinnedMessages: {
        ...s.pinnedMessages,
        [chatId]: (s.pinnedMessages[chatId] || []).filter((p) => p.message_id !== messageId),
      },
    }));
    const { ws } = get();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    try { ws.send(JSON.stringify({ type: 'message_unpinned', chat_id: chatId, message_id: messageId })); } catch { /* */ }
  },

  fetchPinnedMessages: async (chatId) => {
    try {
      const pinned = await api.getPinnedMessages(chatId);
      const normalized = pinned.map((p) =>
        p.message ? { ...p, message: normalizeMessageFileName(p.message) } : p
      );
      set((s) => ({ pinnedMessages: { ...s.pinnedMessages, [chatId]: normalized } }));
    } catch { /* ignore */ }
  },

  fetchFavorites: async () => {
    try {
      const chatIds = await api.getFavorites();
      const ids = Array.isArray(chatIds) ? chatIds : [];
      set({ favoriteChatIds: ids, lastFavoritesFetchAt: Date.now() });
      const uid = useAuthStore.getState().user?.id;
      if (uid) saveFavoritesToStorage(uid, ids);
    } catch { /* при ошибке не трогаем избранное — остаётся из storage */ }
  },

  fetchChatsIfStale: async () => {
    const s = get();
    const ttl = s.cacheTTLMs || CACHE_TTL_MS;
    if (s.lastChatsFetchAt > 0 && Date.now() - s.lastChatsFetchAt <= ttl) return;
    await get().fetchChats();
  },

  fetchFavoritesIfStale: async () => {
    const s = get();
    const ttl = s.cacheTTLMs || CACHE_TTL_MS;
    if (s.lastFavoritesFetchAt > 0 && Date.now() - s.lastFavoritesFetchAt <= ttl) return;
    await get().fetchFavorites();
  },

  loadCacheConfig: async () => {
    try {
      const res = await api.getCacheConfig();
      if (res?.ttl_minutes > 0) {
        set({ cacheTTLMs: res.ttl_minutes * 60 * 1000 });
      }
    } catch { /* оставляем значение по умолчанию */ }
  },

  invalidateChatsCache: () => set({ lastChatsFetchAt: 0 }),
  invalidateFavoritesCache: () => set({ lastFavoritesFetchAt: 0 }),

  hydrateFavoritesFromStorage: () => {
    const uid = useAuthStore.getState().user?.id;
    if (!uid) return;
    const ids = loadFavoritesFromStorage(uid);
    if (ids.length > 0) set({ favoriteChatIds: ids });
  },

  addFavorite: async (chatId) => {
    const prev = get().favoriteChatIds || [];
    const next = Array.isArray(prev) && prev.includes(chatId) ? prev : [...(Array.isArray(prev) ? prev : []), chatId];
    set({ favoriteChatIds: next });
    const uid = useAuthStore.getState().user?.id;
    if (uid) saveFavoritesToStorage(uid, next);
    try {
      await api.addFavorite(chatId);
      set({ lastFavoritesFetchAt: Date.now() });
    } catch {
      set({ favoriteChatIds: prev });
      if (uid) saveFavoritesToStorage(uid, prev);
      get().setNotification('Не удалось добавить в избранное');
    }
  },

  removeFavorite: async (chatId) => {
    const prev = get().favoriteChatIds || [];
    const next = (Array.isArray(prev) ? prev : []).filter((id) => id !== chatId);
    set({ favoriteChatIds: next });
    const uid = useAuthStore.getState().user?.id;
    if (uid) saveFavoritesToStorage(uid, next);
    try {
      await api.removeFavorite(chatId);
      set({ lastFavoritesFetchAt: Date.now() });
    } catch {
      set({ favoriteChatIds: prev });
      if (uid) saveFavoritesToStorage(uid, prev);
      get().setNotification('Не удалось убрать из избранного');
    }
  },

  toggleFavorite: async (chatId) => {
    const ids = get().favoriteChatIds;
    if (Array.isArray(ids) && ids.includes(chatId)) await get().removeFavorite(chatId);
    else await get().addFavorite(chatId);
  },

  setChatMuted: async (chatId, muted) => {
    await api.setChatMuted(chatId, muted);
    set((s) => ({
      chats: s.chats.map((c) => c.chat.id === chatId ? { ...c, muted } : c),
    }));
  },

  clearChatHistory: async (chatId) => {
    await api.clearChatHistory(chatId);
    set((s) => ({
      messages: { ...s.messages, [chatId]: [] },
      pinnedMessages: { ...s.pinnedMessages, [chatId]: [] },
      chats: s.chats.map((c) => c.chat.id === chatId ? { ...c, last_message: undefined, unread_count: 0 } : c),
      replyTo: s.activeChatId === chatId ? null : s.replyTo,
      editingMessage: s.activeChatId === chatId ? null : s.editingMessage,
    }));
  },

  setReplyTo: (msg) => set({ replyTo: msg, editingMessage: null }),
  setEditingMessage: (msg) => set({ editingMessage: msg, replyTo: null }),

  connectWS: () => {
    const { ws: existing, wsReconnectTimer } = get();
    if (existing && (existing.readyState === WebSocket.OPEN || existing.readyState === WebSocket.CONNECTING)) return;
    if (wsReconnectTimer) clearTimeout(wsReconnectTimer);
    if (wsReconnectTimer) set({ wsReconnectTimer: null });

    api.getSessionWsQuery().then((query) => {
      if (!query) return;
      const wsBase = getWebSocketBase();
      if (!wsBase) return;
      const socket = new WebSocket(`${wsBase}/ws?${query}`);

      socket.onopen = () => {
        const wasLongDisconnect = wsDisconnectedAt !== null && (Date.now() - wsDisconnectedAt) > WS_LONG_DISCONNECT_MS;
        wsDisconnectedAt = null;
        set({ ws: socket, wsReconnectAttempt: 0, wsReconnectTimer: null });
        if (wasLongDisconnect) {
          location.reload();
          return;
        }
        get().flushPendingMessages();
        const activeChatId = get().activeChatId;
        if (activeChatId) get().fetchMessages(activeChatId);
      };

      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          get().handleWSMessage(data);
        } catch { /* ignore parse errors */ }
      };

      socket.onclose = () => {
        wsDisconnectedAt = Date.now();
        set({ ws: null });
        const attempt = get().wsReconnectAttempt;
        const delay = Math.min(1000 * Math.pow(2, attempt), 30000);
        const timer = setTimeout(() => get().connectWS(), delay);
        set({ wsReconnectTimer: timer, wsReconnectAttempt: attempt + 1 });
      };

      socket.onerror = () => {
        socket.close();
      };

      set({ ws: socket });
    }).catch(() => { /* нет сессии или ошибка подписи — просто не подключаем WS */ });
  },

  flushPendingMessages: () => {
    const { pendingMessages, ws } = get();
    if (pendingMessages.length === 0 || !ws || ws.readyState !== WebSocket.OPEN) return;
    set({ pendingMessages: [] });
    for (const { chatId, content, clientMsgId, opts } of pendingMessages) {
      try {
        ws.send(JSON.stringify({
          type: 'new_message',
          chat_id: chatId,
          client_msg_id: clientMsgId,
          content,
          content_type: opts?.contentType || 'text',
          file_url: opts?.fileUrl || '',
          file_name: opts?.fileName || '',
          file_size: opts?.fileSize || 0,
          reply_to_id: opts?.replyToId || '',
        }));
      } catch (e) {
        console.error('flushPendingMessages send error:', e);
      }
    }
  },

  disconnectWS: () => {
    const { ws, wsReconnectTimer } = get();
    if (wsReconnectTimer) clearTimeout(wsReconnectTimer);
    if (ws) ws.close();
    set({ ws: null, wsReconnectTimer: null, wsReconnectAttempt: 0 });
  },

  createPersonalChat: async (userId) => {
    const raw = await api.createPersonalChat(userId);
    const chat = { ...raw, muted: raw.muted ?? false };
    get().invalidateChatsCache();
    set((s) => {
      const exists = s.chats.some((c) => c.chat.id === chat.chat.id);
      if (exists) return {};
      return { chats: [chat, ...s.chats] };
    });
    return chat;
  },

  createGroupChat: async (name, memberIds) => {
    const raw = await api.createGroupChat(name, memberIds);
    const chat = { ...raw, muted: raw.muted ?? false };
    get().invalidateChatsCache();
    set((s) => ({ chats: [chat, ...s.chats] }));
    return chat;
  },

  searchUsers: (query) => api.searchUsers(query),
  searchMessages: (query, chatId) => api.searchMessages(query, 30, chatId),
  uploadFile: async (file) => {
    if (get().appStatus.read_only) {
      get().setNotification('Идёт обслуживание: загрузка файлов временно отключена.');
      throw new Error('read-only mode');
    }
    const maxBytes = get().maxFileSizeMB * 1024 * 1024;
    if (file.size > maxBytes) {
      const text = fileLimitMessage(get().maxFileSizeMB);
      get().setNotification(text);
      throw new Error(text);
    }
    try {
      return await api.uploadFile(file);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      const lower = msg.toLowerCase();
      if (
        lower.includes('file too large') ||
        lower.includes('too large') ||
        lower.includes('request entity too large') ||
        lower.includes('http 413') ||
        lower.includes('413')
      ) {
        get().setNotification(fileLimitMessage(get().maxFileSizeMB));
      }
      throw e;
    }
  },
  uploadVoice: async (file) => {
    if (get().appStatus.read_only) {
      get().setNotification('Идёт обслуживание: отправка голосовых временно отключена.');
      throw new Error('read-only mode');
    }
    try {
      return await api.uploadAudio(file);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.toLowerCase().includes('file too large') || msg.toLowerCase().includes('too large')) {
        get().setNotification('Файл не получится загрузить: стоит ограничение по размеру.');
      }
      throw e;
    }
  },

  addOptimisticVoiceMessage: (chatId) => {
    const user = useAuthStore.getState().user;
    const clientMsgId = makeClientMsgID();
    const optId = `opt-${clientMsgId}`;
    const now = new Date().toISOString();
    const optimistic: Message = {
      id: optId,
      client_msg_id: clientMsgId,
      chat_id: chatId,
      sender_id: user?.id ?? '',
      content: 'Голосовое сообщение',
      content_type: 'voice',
      file_url: '',
      file_name: '',
      file_size: 0,
      status: 'sent',
      is_deleted: false,
      created_at: now,
      sender: user ? { ...user, is_online: true, last_seen_at: now } : undefined,
      reply_to_id: get().replyTo?.id,
      reply_to: get().replyTo ?? undefined,
    };
    set((s) => ({
      messages: {
        ...s.messages,
        [chatId]: [...(s.messages[chatId] || []), optimistic],
      },
    }));
    return { optId, clientMsgId };
  },

  removeOptimisticMessage: (chatId, optId) => {
    set((s) => ({
      messages: {
        ...s.messages,
        [chatId]: (s.messages[chatId] || []).filter((m) => m.id !== optId),
      },
    }));
  },

  updateOptimisticVoiceMessage: (chatId, optId, opts) => {
    set((s) => ({
      messages: {
        ...s.messages,
        [chatId]: (s.messages[chatId] || []).map((m) =>
          m.id === optId ? { ...m, file_url: opts.fileUrl, file_name: opts.fileName ?? m.file_name, file_size: opts.fileSize ?? m.file_size } : m
        ),
      },
    }));
  },

  sendMessageWsOnly: (chatId, content, opts) => {
    if (get().appStatus.read_only) {
      get().setNotification('Идёт обслуживание: отправка сообщений временно отключена.');
      return;
    }
    const { ws, replyTo } = get();
    const clientMsgId = (opts?.clientMsgId || makeClientMsgID()).trim();
    const replyToID = opts?.replyToId || replyTo?.id;
    const pendingEntry: PendingMessageEntry = {
      chatId,
      content,
      clientMsgId,
      opts: { ...opts, clientMsgId, replyToId: replyToID },
    };
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      set((s) => ({
        pendingMessages: [...s.pendingMessages.filter((p) => p.clientMsgId !== clientMsgId), pendingEntry],
      }));
      get().setNotification('Сообщение будет отправлено при появлении связи.');
      return;
    }
    try {
      ws.send(JSON.stringify({
        type: 'new_message',
        chat_id: chatId,
        client_msg_id: clientMsgId,
        content,
        content_type: opts?.contentType || 'text',
        file_url: opts?.fileUrl || '',
        file_name: opts?.fileName || '',
        file_size: opts?.fileSize || 0,
        reply_to_id: replyToID || '',
      }));
    } catch (e) {
      console.error('ws send error:', e);
      set((s) => ({
        pendingMessages: [...s.pendingMessages.filter((p) => p.clientMsgId !== clientMsgId), pendingEntry],
      }));
      get().setNotification('Сообщение будет отправлено при появлении связи.');
    }
  },

  leaveChat: async (chatId) => {
    await api.leaveChat(chatId);
    set((s) => ({
      chats: s.chats.filter((c) => c.chat.id !== chatId),
      activeChatId: s.activeChatId === chatId ? null : s.activeChatId,
    }));
  },

  handleWSMessage: (data) => {
    const { type, payload } = data;
    const myId = useAuthStore.getState().user?.id;

    switch (type) {
      case 'new_message': {
        const msg = normalizeMessageFileName(payload as Message);
        const fromMe = msg.sender_id === myId;
        const me = useAuthStore.getState().user;
        const clientMsgId = (msg.client_msg_id || '').trim();
        set((s) => {
          const chatMsgs = s.messages[msg.chat_id] || [];
          const existsByID = chatMsgs.some((m) => m.id === msg.id);
          let nextList = chatMsgs;
          if (!existsByID) {
            if (fromMe) {
              let replaceIndex = -1;
              if (clientMsgId) {
                replaceIndex = chatMsgs.findIndex((m) => m.client_msg_id === clientMsgId || m.id === `opt-${clientMsgId}`);
              }
              if (replaceIndex < 0) {
                const isVoice = msg.content_type === 'voice';
                replaceIndex = chatMsgs.findIndex((m) => {
                  if (!m.id.startsWith('opt-')) return false;
                  if (isVoice && m.content_type !== 'voice') return false;
                  if (!isVoice && m.content_type === 'voice') return false;
                  if (msg.file_url && m.file_url && m.file_url !== msg.file_url) return false;
                  return true;
                });
              }
              if (replaceIndex >= 0) {
                const out = [...chatMsgs];
                out[replaceIndex] = msg;
                nextList = out;
              } else {
                nextList = [...chatMsgs, msg];
              }
            } else {
              nextList = [...chatMsgs, msg];
            }
          }
          nextList = [...nextList].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
          const newMessages = { ...s.messages, [msg.chat_id]: nextList };

          let newChats = s.chats.map((c) => {
            if (c.chat.id !== msg.chat_id) return c;
            const unreadInc = !fromMe && s.activeChatId !== msg.chat_id ? 1 : 0;
            return {
              ...c,
              last_message: msg,
              unread_count: c.unread_count + unreadInc,
            };
          });

          if (!newChats.some((c) => c.chat.id === msg.chat_id)) {
            // If chat is unknown locally, fetch and insert it immediately so sidebar updates without tab switch.
            api.getChat(msg.chat_id).then((chat) => {
              const incoming = {
                ...chat,
                muted: chat.muted ?? false,
                ...(msg ? { last_message: msg } : {}),
              };
              set((cur) => {
                const exists = cur.chats.some((c) => c.chat.id === incoming.chat.id);
                const merged = exists
                  ? cur.chats.map((c) => (c.chat.id === incoming.chat.id ? { ...c, ...incoming } : c))
                  : [incoming, ...cur.chats];
                merged.sort((a, b) => {
                  const at = a.last_message?.created_at || a.chat.created_at;
                  const bt = b.last_message?.created_at || b.chat.created_at;
                  return new Date(bt).getTime() - new Date(at).getTime();
                });
                return { chats: merged };
              });
            }).catch(() => {});
            // Fallback full refresh for consistency.
            get().invalidateChatsCache();
            get().fetchChats();
          }

          newChats.sort((a, b) => {
            const at = a.last_message?.created_at || a.chat.created_at;
            const bt = b.last_message?.created_at || b.chat.created_at;
            return new Date(bt).getTime() - new Date(at).getTime();
          });

          const typingArr = (s.typingUsers[msg.chat_id] || []).filter((id) => id !== msg.sender_id);
          const nextPending = fromMe && clientMsgId
            ? s.pendingMessages.filter((p) => p.clientMsgId !== clientMsgId)
            : s.pendingMessages;

          return {
            messages: newMessages,
            chats: newChats,
            typingUsers: { ...s.typingUsers, [msg.chat_id]: typingArr },
            pendingMessages: nextPending,
          };
        });

        get().updateElectronBadge();
        const { activeChatId } = get();
        const chat = get().chats.find((c) => c.chat.id === msg.chat_id);
        const chatTitle = chat
          ? (chat.chat.chat_type === 'group' || chat.chat.chat_type === 'notes'
            ? chat.chat.name
            : chat.members.find((m) => m.id !== myId)?.username || 'Чат')
          : msg.sender?.username || 'Чат';
        const mentionedMe = !fromMe && !!me && msg.content_type === 'text' && hasUserMention(msg.content || '', me.username);
        if (mentionedMe) {
          get().setNotification(`Вас упомянули в чате: ${chatTitle}`);
        }
        if (activeChatId === msg.chat_id && msg.sender_id !== myId) {
          get().markAsRead(msg.chat_id);
        }

        if (!fromMe && typeof document !== 'undefined' && (document.hidden || activeChatId !== msg.chat_id)) {
          if (chat?.muted) break;
          const body =
            msg.content_type === 'text'
              ? `${mentionedMe ? 'Вас упомянули: ' : ''}${(msg.content || '').trim().slice(0, 120) || '—'}`
              : msg.content_type === 'voice'
                ? 'Голосовое сообщение'
                : (msg.file_name || 'Файл').trim().slice(0, 80);
          const electronApi = (window as unknown as { electronAPI?: { showNotification?: (opts: { title: string; body: string }) => void } }).electronAPI;
          if (electronApi?.showNotification) {
            electronApi.showNotification({ title: chatTitle, body });
          } else if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
            const n = new Notification(chatTitle, { body });
            n.onclick = () => {
              window.focus();
              n.close();
            };
          }
        }
        break;
      }

      case 'message_edited': {
        const { message_id, chat_id, content, edited_at } = payload;
        set((s) => {
          const msgs = s.messages[chat_id];
          if (!msgs) {
            return {
              chats: s.chats.map((c) => {
                if (!c.last_message || c.chat.id !== chat_id || c.last_message.id !== message_id) return c;
                return { ...c, last_message: { ...c.last_message, content, edited_at, is_deleted: false } };
              }),
            };
          }
          return {
            messages: {
              ...s.messages,
              [chat_id]: msgs.map((m) =>
                m.id === message_id ? { ...m, content, edited_at, is_deleted: false } : m
              ),
            },
            chats: s.chats.map((c) => {
              if (!c.last_message || c.chat.id !== chat_id || c.last_message.id !== message_id) return c;
              return { ...c, last_message: { ...c.last_message, content, edited_at, is_deleted: false } };
            }),
            pinnedMessages: {
              ...s.pinnedMessages,
              [chat_id]: (s.pinnedMessages[chat_id] || []).map((p) => {
                if (p.message_id !== message_id || !p.message) return p;
                return { ...p, message: { ...p.message, content, edited_at, is_deleted: false } };
              }),
            },
          };
        });
        break;
      }

      case 'message_deleted': {
        const { message_id, chat_id } = payload;
        set((s) => {
          const msgs = s.messages[chat_id];
          if (!msgs) return {};
          const tombstone = (m: Message): Message => ({
            ...m,
            is_deleted: true,
            content: '',
            content_type: 'text',
            file_url: '',
            file_name: '',
            file_size: 0,
            edited_at: m.edited_at,
          });
          const nextMsgs = msgs.map((m) => (m.id === message_id ? tombstone(m) : m));
          return {
            messages: {
              ...s.messages,
              [chat_id]: nextMsgs,
            },
            chats: s.chats.map((c) => {
              if (c.chat.id !== chat_id) return c;
              if (!c.last_message) return c;
              return c.last_message.id === message_id ? { ...c, last_message: tombstone(c.last_message) } : c;
            }),
            pinnedMessages: {
              ...s.pinnedMessages,
              [chat_id]: (s.pinnedMessages[chat_id] || []).map((p) => {
                if (p.message_id !== message_id || !p.message) return p;
                return { ...p, message: tombstone(p.message) };
              }),
            },
          };
        });
        break;
      }

      case 'reaction_added': {
        const { message_id, chat_id, user_id, emoji } = payload;
        set((s) => {
          const msgs = s.messages[chat_id];
          if (!msgs) return {};
          return {
            messages: {
              ...s.messages,
              [chat_id]: msgs.map((m) => {
                if (m.id !== message_id) return m;
                const reactions = [...(m.reactions || [])];
                if (!reactions.some((r) => r.user_id === user_id && r.emoji === emoji)) {
                  reactions.push({ message_id, user_id, emoji, created_at: new Date().toISOString() });
                }
                return { ...m, reactions };
              }),
            },
          };
        });
        break;
      }

      case 'reaction_removed': {
        const { message_id, chat_id, user_id, emoji } = payload;
        set((s) => {
          const msgs = s.messages[chat_id];
          if (!msgs) return {};
          return {
            messages: {
              ...s.messages,
              [chat_id]: msgs.map((m) => {
                if (m.id !== message_id) return m;
                return { ...m, reactions: (m.reactions || []).filter((r) => !(r.user_id === user_id && r.emoji === emoji)) };
              }),
            },
          };
        });
        break;
      }

      case 'message_pinned': {
        const { chat_id } = payload;
        get().fetchPinnedMessages(chat_id);
        break;
      }

      case 'message_unpinned': {
        const { chat_id, message_id } = payload;
        set((s) => ({
          pinnedMessages: {
            ...s.pinnedMessages,
            [chat_id]: (s.pinnedMessages[chat_id] || []).filter((p) => p.message_id !== message_id),
          },
        }));
        break;
      }

      case 'chat_created': {
        const incoming = payload as ChatWithLastMessage;
        const chatData = { ...incoming, muted: incoming.muted ?? false };
        get().invalidateChatsCache();
        set((s) => {
          if (s.chats.some((c) => c.chat.id === chatData.chat.id)) return {};
          return { chats: [chatData, ...s.chats] };
        });
        break;
      }

      case 'chat_updated': {
        const { chat_id, name, description, avatar_url } = payload;
        set((s) => ({
          chats: s.chats.map((c) =>
            c.chat.id === chat_id
              ? { ...c, chat: { ...c.chat, name, description, ...(avatar_url !== undefined && { avatar_url }) } }
              : c
          ),
        }));
        break;
      }

      case 'member_added': {
        const { chat_id, user_id, username, actor_name } = payload;
        const text = actor_name
          ? `${actor_name} добавил(а) ${username} в группу`
          : `Пользователь ${username} добавлен в группу`;
        set((s) => ({ notification: text }));
        // Always upsert chat immediately so UI updates in-place in all tabs.
        api.getChat(chat_id).then((chat) => {
          const incoming = { ...chat, muted: chat.muted ?? false };
          set((s) => {
            const exists = s.chats.some((c) => c.chat.id === chat_id);
            const merged = exists
              ? s.chats.map((c) => (c.chat.id === chat_id ? incoming : c))
              : [incoming, ...s.chats];
            merged.sort((a, b) => {
              const at = a.last_message?.created_at || a.chat.created_at;
              const bt = b.last_message?.created_at || b.chat.created_at;
              return new Date(bt).getTime() - new Date(at).getTime();
            });
            return { chats: merged };
          });
        }).catch(() => {});

        // If current user was added, refresh full list and preload messages for this chat.
        if (user_id === myId) {
          get().invalidateChatsCache();
          get().fetchChats().then(() => {
            get().fetchMessages(chat_id);
          });
        }
        break;
      }
      case 'member_removed': {
        const { chat_id, user_id, username, is_leave, actor_name } = payload;
        const text = is_leave
          ? `${username} покинул(а) группу`
          : actor_name
            ? `${actor_name} исключил(а) ${username} из группы`
            : `Пользователь ${username} исключён из группы`;
        set((s) => ({ notification: text }));
        // Если исключили текущего пользователя — обновляем список и выходим из этого чата
        if (user_id === myId) {
          set((s) => ({
            chats: s.chats.filter((c) => c.chat.id !== chat_id),
            activeChatId: s.activeChatId === chat_id ? null : s.activeChatId,
          }));
        } else {
          api.getChat(chat_id).then((chat) => {
            const incoming = { ...chat, muted: chat.muted ?? false };
            set((s) => {
              const merged = s.chats.map((c) => (c.chat.id === chat_id ? incoming : c));
              return { chats: merged };
            });
          }).catch(() => {});
        }
        break;
      }

      case 'typing': {
        const { chat_id, user_id } = payload as { chat_id: string; user_id: string };
        if (user_id === myId) break;

        const key = `${chat_id}:${user_id}`;
        if (typingClearTimeouts[key]) clearTimeout(typingClearTimeouts[key]);

        set((s) => {
          const current = s.typingUsers[chat_id] || [];
          if (current.includes(user_id)) return {};
          return { typingUsers: { ...s.typingUsers, [chat_id]: [...current, user_id] } };
        });

        typingClearTimeouts[key] = setTimeout(() => {
          delete typingClearTimeouts[key];
          set((s) => ({
            typingUsers: {
              ...s.typingUsers,
              [chat_id]: (s.typingUsers[chat_id] || []).filter((id) => id !== user_id),
            },
          }));
        }, 3000);
        break;
      }

      case 'message_read': {
        const { chat_id } = payload as { chat_id: string };
        set((s) => {
          const msgs = s.messages[chat_id];
          if (!msgs) return {};
          return {
            messages: {
              ...s.messages,
              [chat_id]: msgs.map((m) => (m.sender_id === myId ? { ...m, status: 'read' as const } : m)),
            },
          };
        });
        break;
      }

      case 'user_online':
      case 'user_offline': {
        const { user_id, online } = payload as { user_id: string; online: boolean };
        set((s) => ({
          onlineUsers: { ...s.onlineUsers, [user_id]: online },
          chats: s.chats.map((c) => ({
            ...c,
            members: c.members.map((m) => (m.id === user_id ? { ...m, is_online: online } : m)),
          })),
        }));
        break;
      }

      case 'session_revoked': {
        // Forced re-auth (admin revoked all sessions for this user).
        try {
          localStorage.removeItem(SESSION_ID_KEY);
          localStorage.removeItem(SESSION_SECRET_KEY);
        } catch { /* ignore */ }
        if (typeof location !== 'undefined') location.reload();
        break;
      }
    }
  },

  reset: () => {
    const { ws, wsReconnectTimer } = get();
    if (wsReconnectTimer) clearTimeout(wsReconnectTimer);
    if (ws) ws.close();
    for (const k of Object.keys(typingClearTimeouts)) {
      clearTimeout(typingClearTimeouts[k]);
      delete typingClearTimeouts[k];
    }
    set(initialChatState);
    get().updateElectronBadge();
  },
}));
