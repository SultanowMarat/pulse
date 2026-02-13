import { useState, useCallback, useRef, useMemo, useEffect } from 'react';
import { useAuthStore, useChatStore } from '../store';
import { Avatar, Modal, IconSearch, IconUsers, IconEdit, IconTrash, IconX, formatTime, TypingDots } from './ui';
import type { UserPublic, ChatWithLastMessage, Message } from '../types';
import * as api from '../api';

interface SidebarProps { onChatSelect: () => void; onOpenProfile?: () => void; }

type ChatListTab = 'all' | 'personal' | 'favorites';

export default function Sidebar({ onChatSelect, onOpenProfile }: SidebarProps) {
  const { user } = useAuthStore();
  const { chats, activeChatId, setActiveChat, typingUsers, onlineUsers, favoriteChatIds, fetchChatsIfStale, fetchFavoritesIfStale, createPersonalChat, createGroupChat, searchUsers, searchMessages, leaveChat, setChatMuted, setNotification } = useChatStore();
  const [search, setSearch] = useState('');
  const [globalUsers, setGlobalUsers] = useState<UserPublic[]>([]);
  const [globalMessages, setGlobalMessages] = useState<Message[]>([]);
  const [globalLoading, setGlobalLoading] = useState(false);
  const [tab, setTab] = useState<ChatListTab>('all');
  const [showNewGroup, setShowNewGroup] = useState(false);
  const [allUsers, setAllUsers] = useState<UserPublic[]>([]);
  const [allUsersLoading, setAllUsersLoading] = useState(false);
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; chat: ChatWithLastMessage } | null>(null);
  const [renameChat, setRenameChat] = useState<ChatWithLastMessage | null>(null);
  const myId = user?.id || '';

  // Сразу подгружаем чаты и избранное при появлении сайдбара (есть user), чтобы при переключении на ЛИЧНЫЕ/ИЗБРАННЫЕ списки уже были
  useEffect(() => {
    if (user?.id) {
      fetchChatsIfStale();
      fetchFavoritesIfStale();
    }
  }, [user?.id, fetchChatsIfStale, fetchFavoritesIfStale]);

  useEffect(() => {
    if (tab === 'all') {
      setAllUsersLoading(true);
      api.listUsers()
        .then(setAllUsers)
        .catch(() => setAllUsers([]))
        .finally(() => setAllUsersLoading(false));
    }
  }, [tab]);

  // При переключении на «ВСЕ» — подгрузить чаты, если кеш пустой/устарел
  useEffect(() => {
    if (tab === 'all') fetchChatsIfStale();
  }, [tab, fetchChatsIfStale]);

  // При переключении на «ЛИЧНЫЕ» — запрос только если кеш устарел или инвалидирован по WS
  useEffect(() => {
    if (tab === 'personal') fetchChatsIfStale();
  }, [tab, fetchChatsIfStale]);

  // При переключении на «ИЗБРАННЫЕ» — запрос только если кеш устарел или инвалидирован
  useEffect(() => {
    if (tab === 'favorites') fetchFavoritesIfStale();
  }, [tab, fetchFavoritesIfStale]);

  const handleChatClick = useCallback((chatId: string) => {
    setActiveChat(chatId);
    onChatSelect();
  }, [setActiveChat, onChatSelect]);

  // Вкладка "ЛИЧНЫЕ" должна содержать:
  // - личные диалоги (personal) только если есть переписка (last_message != null)
  // - групповые чаты (group) всегда, чтобы новая группа сразу появлялась в списке
  const personalTabChats = useMemo(
    () =>
      chats.filter((c) => {
        if (c.chat.chat_type === 'group') return true;
        if (c.chat.chat_type === 'personal') return c.last_message != null;
        return false;
      }),
    [chats]
  );

  const groupChats = useMemo(() => chats.filter((c) => c.chat.chat_type === 'group'), [chats]);

  const filteredPersonal = useMemo(() => {
    let result = personalTabChats;
    if (search.trim()) {
      const q = search.toLowerCase();
      result = result.filter((c) => getChatName(c, myId).toLowerCase().includes(q));
    }
    return result;
  }, [personalTabChats, search, myId]);

  const favoriteChats = useMemo(() =>
    chats.filter((c) => favoriteChatIds.includes(c.chat.id)),
    [chats, favoriteChatIds]
  );
  const filteredFavorites = useMemo(() => {
    if (!search.trim()) return favoriteChats;
    const q = search.toLowerCase();
    return favoriteChats.filter((c) => getChatName(c, myId).toLowerCase().includes(q));
  }, [favoriteChats, search, myId]);
  const personalUnread = useMemo(
    () => personalTabChats.reduce((sum, c) => sum + (c.unread_count || 0), 0),
    [personalTabChats]
  );
  const favoritesUnread = useMemo(
    () => favoriteChats.reduce((sum, c) => sum + (c.unread_count || 0), 0),
    [favoriteChats]
  );

  const normalizeForSearch = useCallback((s: string): string => {
    const cyrToLat: Record<string, string> = {
      а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'e', ж: 'zh', з: 'z',
      и: 'i', й: 'j', к: 'k', л: 'l', м: 'm', н: 'n', о: 'o', п: 'p', р: 'r',
      с: 's', т: 't', у: 'u', ф: 'f', х: 'h', ц: 'ts', ч: 'ch', ш: 'sh', щ: 'sch',
      ъ: '', ы: 'y', ь: '', э: 'e', ю: 'yu', я: 'ya',
    };
    return s.toLowerCase().split('').map((c) => cyrToLat[c] ?? c).join('');
  }, []);

  const filteredAllUsers = useMemo(() => {
    if (!search.trim()) return allUsers;
    const q = search.trim().toLowerCase();
    const qLat = normalizeForSearch(search.trim());
    return allUsers.filter(
      (u) =>
        u.username.toLowerCase().includes(q) ||
        normalizeForSearch(u.username).includes(qLat) ||
        (u.email && (u.email.toLowerCase().includes(q) || normalizeForSearch(u.email).includes(qLat))) ||
        (u.phone && u.phone.includes(q))
    );
  }, [allUsers, search, normalizeForSearch]);

  const personalChatByUserId = useMemo(() => {
    const map: Record<string, ChatWithLastMessage> = {};
    for (const c of chats) {
      if (c.chat.chat_type !== 'personal') continue;
      const other = c.members.find((m) => m.id !== myId);
      if (other) map[other.id] = c;
    }
    return map;
  }, [chats, myId]);

  const notesChat = useMemo(() => chats.find((c) => c.chat.chat_type === 'notes'), [chats]);

  useEffect(() => {
    const q = search.trim();
    if (!q) {
      setGlobalUsers([]);
      setGlobalMessages([]);
      setGlobalLoading(false);
      return;
    }
    setGlobalLoading(true);
    const t = setTimeout(() => {
      Promise.all([
        searchUsers(q).catch(() => [] as UserPublic[]),
        searchMessages(q).catch(() => [] as Message[]),
      ]).then(([usersRes, messagesRes]) => {
        setGlobalUsers(usersRes || []);
        setGlobalMessages(messagesRes || []);
      }).finally(() => setGlobalLoading(false));
    }, 250);
    return () => clearTimeout(t);
  }, [search, searchUsers, searchMessages]);

  const fileResults = useMemo(() => globalMessages.filter((m) =>
    m.content_type === 'file' || m.content_type === 'image' || m.content_type === 'voice' || (m.file_name && String(m.file_name).trim() !== '')
  ), [globalMessages]);
  const textMessageResults = useMemo(() => globalMessages.filter((m) =>
    !(m.content_type === 'file' || m.content_type === 'image' || m.content_type === 'voice' || (m.file_name && String(m.file_name).trim() !== ''))
  ), [globalMessages]);

  return (
    <div className="h-full flex flex-col bg-sidebar min-w-0 overflow-x-hidden safe-top safe-bottom">
      {/* Mobile/PWA header (isolated) */}
      <div className="md:hidden px-4 pb-2 shrink-0 pt-2">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-sidebar-title font-semibold text-white flex-1 min-w-0 truncate text-left">Чаты</h2>
          <div className="flex items-center gap-1 shrink-0">
            <SidebarBtn tip="Новая группа" onClick={() => setShowNewGroup(true)}><IconUsers size={18} /></SidebarBtn>
            {onOpenProfile && (
              <button
                type="button"
                onClick={onOpenProfile}
                className="p-0.5 rounded-full hover:bg-sidebar-hover transition-colors shrink-0"
                title="Профиль"
                aria-label="Профиль"
              >
                <Avatar name={user?.username ?? ''} url={user?.avatar_url} size={32} />
              </button>
            )}
          </div>
        </div>
        <div className="flex gap-0.5 mb-1.5 p-0.5 bg-sidebar-hover rounded-compass">
          <button
            type="button"
            onClick={() => setTab('all')}
            className={`flex-1 min-h-[40px] py-1.5 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide text-[12px] transition-all duration-200 ease-out ${tab === 'all' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}>
            ВСЕ
          </button>
          <button
            type="button"
            onClick={() => setTab('personal')}
            className={`flex-1 min-h-[40px] py-1.5 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide text-[12px] transition-all duration-200 ease-out ${tab === 'personal' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}>
            <span className="inline-flex items-center justify-center gap-1.5">
              ЛИЧНЫЕ
              {personalUnread > 0 && <span className="min-w-[18px] h-[18px] px-1 rounded-full bg-primary text-white text-[10px] leading-[18px]">{personalUnread}</span>}
            </span>
          </button>
          <button
            type="button"
            onClick={() => setTab('favorites')}
            className={`flex-1 min-h-[40px] py-1.5 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide text-[12px] transition-all duration-200 ease-out ${tab === 'favorites' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}>
            <span className="inline-flex items-center justify-center gap-1.5">
              ИЗБРАННЫЕ
              {favoritesUnread > 0 && <span className="min-w-[18px] h-[18px] px-1 rounded-full bg-primary text-white text-[10px] leading-[18px]">{favoritesUnread}</span>}
            </span>
          </button>
        </div>
        <div className="relative">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sidebar-text pointer-events-none"><IconSearch size={18} /></span>
          <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Поиск"
            className="w-full pl-10 pr-3 py-2 bg-sidebar-hover rounded-compass text-[14px] text-white placeholder:text-sidebar-text border border-transparent focus:border-primary/50 focus:ring-2 focus:ring-primary/20 outline-none transition-colors" />
        </div>
      </div>

      {/* Desktop header (isolated) */}
      <div className="hidden md:block px-4 pb-2 shrink-0 pt-3">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-sidebar-title font-semibold text-white flex-1 min-w-0 truncate text-left">Чаты</h2>
          <div className="flex items-center gap-1 shrink-0">
            <SidebarBtn tip="Новая группа" onClick={() => setShowNewGroup(true)}><IconUsers size={18} /></SidebarBtn>
            {onOpenProfile && (
              <button
                type="button"
                onClick={onOpenProfile}
                className="p-0.5 rounded-full hover:bg-sidebar-hover transition-colors shrink-0"
                title="Профиль"
                aria-label="Профиль"
              >
                <Avatar name={user?.username ?? ''} url={user?.avatar_url} size={32} />
              </button>
            )}
          </div>
        </div>
        <div className="flex gap-0.5 mb-1.5 p-0.5 bg-sidebar-hover rounded-compass">
          <button
            type="button"
            onClick={() => setTab('all')}
            className={`flex-1 min-h-[44px] py-2 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide text-[13px] transition-all duration-200 ease-out ${tab === 'all' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}>
            ВСЕ
          </button>
          <button
            type="button"
            onClick={() => setTab('personal')}
            className={`flex-1 min-h-[44px] py-2 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide text-[13px] transition-all duration-200 ease-out ${tab === 'personal' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}>
            <span className="inline-flex items-center justify-center gap-1.5">
              ЛИЧНЫЕ
              {personalUnread > 0 && <span className="min-w-[18px] h-[18px] px-1 rounded-full bg-primary text-white text-[10px] leading-[18px]">{personalUnread}</span>}
            </span>
          </button>
          <button
            type="button"
            onClick={() => setTab('favorites')}
            className={`flex-1 min-h-[44px] py-2 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide text-[13px] transition-all duration-200 ease-out ${tab === 'favorites' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}>
            <span className="inline-flex items-center justify-center gap-1.5">
              ИЗБРАННЫЕ
              {favoritesUnread > 0 && <span className="min-w-[18px] h-[18px] px-1 rounded-full bg-primary text-white text-[10px] leading-[18px]">{favoritesUnread}</span>}
            </span>
          </button>
        </div>
        <div className="relative">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sidebar-text pointer-events-none"><IconSearch size={18} /></span>
          <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Поиск"
            className="w-full pl-10 pr-3 py-2.5 bg-sidebar-hover rounded-compass text-[14px] text-white placeholder:text-sidebar-text border border-transparent focus:border-primary/50 focus:ring-2 focus:ring-primary/20 outline-none transition-colors" />
        </div>
      </div>

      {/* List: ВСЕ | ЛИЧНЫЕ | ИЗБРАННЫЕ */}
      <div className="flex-1 overflow-y-auto dark-scroll mt-0.5 md:mt-1">
        {search.trim() ? (
          <>
            {globalLoading && <p className="text-center text-sidebar-text text-[13px] py-8">Поиск...</p>}
            {!globalLoading && globalUsers.length === 0 && textMessageResults.length === 0 && fileResults.length === 0 && (
              <p className="text-center text-sidebar-text text-[13px] py-8">Ничего не найдено</p>
            )}

            {!globalLoading && globalUsers.length > 0 && (
              <div className="px-4 pt-2 pb-1 text-[11px] uppercase tracking-wide text-sidebar-text">Пользователи</div>
            )}
            {!globalLoading && globalUsers.map((u) => {
              const existingChat = personalChatByUserId[u.id];
              if (existingChat) {
                return (
                  <ChatItem key={`u-chat-${existingChat.chat.id}`} chat={existingChat} active={existingChat.chat.id === activeChatId}
                    myId={myId} typing={typingUsers[existingChat.chat.id]} onlineUsers={onlineUsers}
                    onClick={() => handleChatClick(existingChat.chat.id)}
                    onContextMenu={undefined} />
                );
              }
              return (
                <UserRow key={`u-row-${u.id}`} user={u} online={onlineUsers[u.id] ?? u.is_online}
                  onClick={async () => {
                    try {
                      const chat = await createPersonalChat(u.id);
                      setActiveChat(chat.chat.id);
                      onChatSelect();
                    } catch { /* */ }
                  }} />
              );
            })}

            {!globalLoading && textMessageResults.length > 0 && (
              <div className="px-4 pt-3 pb-1 text-[11px] uppercase tracking-wide text-sidebar-text">Сообщения</div>
            )}
            {!globalLoading && textMessageResults.map((m) => (
              <SearchMessageRow
                key={`m-${m.id}`}
                message={m}
                onClick={() => {
                  setActiveChat(m.chat_id);
                  onChatSelect();
                }}
              />
            ))}

            {!globalLoading && fileResults.length > 0 && (
              <div className="px-4 pt-3 pb-1 text-[11px] uppercase tracking-wide text-sidebar-text">Файлы</div>
            )}
            {!globalLoading && fileResults.map((m) => (
              <SearchFileRow
                key={`f-${m.id}`}
                message={m}
                onClick={() => {
                  setActiveChat(m.chat_id);
                  onChatSelect();
                }}
              />
            ))}
          </>
        ) : tab === 'favorites' ? (
          filteredFavorites.length === 0 ? (
            <p className="text-center text-sidebar-text text-[13px] py-8">Нет избранных чатов</p>
          ) : (
            filteredFavorites.map((chat) => (
              <ChatItem key={chat.chat.id} chat={chat} active={chat.chat.id === activeChatId}
                myId={myId} typing={typingUsers[chat.chat.id]} onlineUsers={onlineUsers}
                onClick={() => handleChatClick(chat.chat.id)}
                onContextMenu={undefined} />
            ))
          )
        ) : tab === 'personal' ? (
          filteredPersonal.length === 0 ? (
            <p className="text-center text-sidebar-text text-[13px] py-8">Нет личных чатов и групп</p>
          ) : (
            filteredPersonal.map((chat) => (
              <ChatItem key={chat.chat.id} chat={chat} active={chat.chat.id === activeChatId}
                myId={myId} typing={typingUsers[chat.chat.id]} onlineUsers={onlineUsers}
                onClick={() => handleChatClick(chat.chat.id)}
                onContextMenu={(e) => {
                  if (chat.chat.chat_type !== 'group') return;
                  e.preventDefault();
                  setCtxMenu({ x: e.clientX, y: e.clientY, chat });
                }} />
            ))
          )
        ) : allUsersLoading ? (
          <p className="text-center text-sidebar-text text-[13px] py-8">Загрузка...</p>
        ) : (
          <>
            {groupChats.length > 0 && (
              <div className="pb-2">
                {groupChats.map((chat) => (
                  <ChatItem
                    key={chat.chat.id}
                    chat={chat}
                    active={chat.chat.id === activeChatId}
                    myId={myId}
                    typing={typingUsers[chat.chat.id]}
                    onlineUsers={onlineUsers}
                    onClick={() => handleChatClick(chat.chat.id)}
                    onContextMenu={(e) => {
                      e.preventDefault();
                      setCtxMenu({ x: e.clientX, y: e.clientY, chat });
                    }}
                  />
                ))}
                <div className="mx-4 h-px bg-sidebar-border/30" />
              </div>
            )}
            {filteredAllUsers.length === 0 && !notesChat ? (
              <p className="text-center text-sidebar-text text-[13px] py-8">Нет пользователей</p>
            ) : (
              <>
                {filteredAllUsers.map((u) => {
                  const existingChat = personalChatByUserId[u.id];
                  if (existingChat) {
                    return (
                      <ChatItem key={existingChat.chat.id} chat={existingChat} active={existingChat.chat.id === activeChatId}
                        myId={myId} typing={typingUsers[existingChat.chat.id]} onlineUsers={onlineUsers}
                        onClick={() => handleChatClick(existingChat.chat.id)}
                        onContextMenu={undefined} />
                    );
                  }
                  return (
                    <UserRow key={u.id} user={u} online={onlineUsers[u.id] ?? u.is_online}
                      onClick={async () => {
                        try {
                          const chat = await createPersonalChat(u.id);
                          setActiveChat(chat.chat.id);
                          onChatSelect();
                        } catch { /* */ }
                      }} />
                  );
                })}
                {notesChat && (
                  <ChatItem key={notesChat.chat.id} chat={notesChat} active={notesChat.chat.id === activeChatId}
                    myId={myId} typing={typingUsers[notesChat.chat.id]} onlineUsers={onlineUsers}
                    onClick={() => handleChatClick(notesChat.chat.id)}
                    onContextMenu={undefined} />
                )}
              </>
            )}
          </>
        )}
      </div>

      {/* Context menu for groups */}
      {ctxMenu && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setCtxMenu(null)} onContextMenu={(e) => { e.preventDefault(); setCtxMenu(null); }} />
          <div className="fixed z-50 bg-white dark:bg-dark-elevated rounded-compass shadow-compass-dialog border border-surface-border dark:border-dark-border py-1 min-w-[200px] animate-fade"
            style={{ left: Math.min(ctxMenu.x, innerWidth - 220), top: Math.min(ctxMenu.y, innerHeight - 120), maxWidth: 'calc(100vw - 2rem)' }}>
            <SidebarCtxItem
              icon={<IconBellOff size={14} />}
              label={ctxMenu.chat.muted ? 'Включить уведомления' : 'Отключить уведомления'}
              onClick={async () => {
                try {
                  await setChatMuted(ctxMenu.chat.chat.id, !ctxMenu.chat.muted);
                  setNotification(!ctxMenu.chat.muted ? 'Уведомления отключены' : 'Уведомления включены');
                } catch {
                  setNotification('Не удалось изменить уведомления');
                }
                setCtxMenu(null);
              }}
            />
            <SidebarCtxItem icon={<IconEdit />} label="Изменить имя группы"
              onClick={() => { setRenameChat(ctxMenu.chat); setCtxMenu(null); }} />
            {ctxMenu.chat.chat.system_key !== 'general' && (
              <SidebarCtxItem icon={<IconTrash />} label="Покинуть группу" danger
                onClick={async () => {
                  if (confirm('Покинуть группу?')) {
                    try { await leaveChat(ctxMenu.chat.chat.id); } catch { /* */ }
                  }
                  setCtxMenu(null);
                }} />
            )}
          </div>
        </>
      )}

      {/* Rename modal */}
      {renameChat && (
        <RenameGroupModal chat={renameChat} onClose={() => setRenameChat(null)} />
      )}

      <NewGroupModal open={showNewGroup} onClose={() => setShowNewGroup(false)} searchUsers={searchUsers} createGroup={createGroupChat} setActiveChat={handleChatClick} />
    </div>
  );
}

function SidebarBtn({ tip, onClick, children }: { tip: string; onClick: () => void; children: React.ReactNode }) {
  return (
    <button title={tip} onClick={onClick}
            className="min-w-[44px] min-h-[44px] w-10 h-10 flex items-center justify-center rounded-full hover:bg-sidebar-hover text-sidebar-text hover:text-white transition-colors">
      {children}
    </button>
  );
}

/* ─── Helpers ─── */
function getChatName(c: ChatWithLastMessage, myId: string): string {
  if (c.chat.chat_type === 'notes') return c.chat.name;
  if (c.chat.chat_type === 'group') return c.chat.name;
  return c.members.find((m) => m.id !== myId)?.username || 'Чат';
}

function getChatOnline(c: ChatWithLastMessage, myId: string, o: Record<string, boolean>): boolean | undefined {
  if (c.chat.chat_type === 'group' || c.chat.chat_type === 'notes') return undefined;
  const other = c.members.find((m) => m.id !== myId);
  return other ? (o[other.id] ?? other.is_online) : undefined;
}

function IconBellOff({ size = 12 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 8a6 6 0 00-9.33-4.9" />
      <path d="M6 8c0 7-3 9-3 9h12" />
      <path d="M13.73 21a2 2 0 01-3.46 0" />
      <line x1="2" y1="2" x2="22" y2="22" />
    </svg>
  );
}

/* ─── User Row (user without existing chat — tap to start chat) ─── */
function UserRow({ user, online, onClick }: { user: UserPublic; online: boolean; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick}
      className="w-full min-h-[48px] flex items-center gap-3 px-4 py-2.5 hover:bg-sidebar-hover transition-colors text-left">
      <Avatar name={user.username} url={user.avatar_url || undefined} size={44} online={online} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between">
          <span className="text-sidebar-name font-medium text-white/90 truncate">{user.username}</span>
        </div>
        <div className="mt-0.5">
          <span className="text-sidebar-sub font-normal text-sidebar-text">Написать сообщение</span>
        </div>
      </div>
    </button>
  );
}

function SearchMessageRow({ message, onClick }: { message: Message; onClick: () => void }) {
  const text = (message.content || '').trim();
  const preview = message.content_type === 'text'
    ? (text || 'Текст')
    : message.content_type === 'voice'
      ? '🎤 Голосовое сообщение'
      : message.content_type === 'image'
        ? '📷 Изображение'
        : message.content_type === 'file'
          ? `📎 ${message.file_name || 'Файл'}`
          : (text || 'Сообщение');
  const author = message.sender?.username || 'Пользователь';
  return (
    <button type="button" onClick={onClick}
      className="w-full min-h-[52px] flex items-start gap-3 px-4 py-2.5 hover:bg-sidebar-hover transition-colors text-left">
      <Avatar name={author} url={message.sender?.avatar_url || undefined} size={36} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <span className="text-[13px] font-semibold text-white truncate">{author}</span>
          <span className="text-[11px] text-sidebar-text shrink-0">{formatTime(message.created_at)}</span>
        </div>
        <p className="text-[12px] text-sidebar-text truncate">{preview}</p>
      </div>
    </button>
  );
}

function SearchFileRow({ message, onClick }: { message: Message; onClick: () => void }) {
  const fileName = (message.file_name || '').replace(/\+/g, ' ').trim() || 'Файл';
  const author = message.sender?.username || 'Пользователь';
  return (
    <button type="button" onClick={onClick}
      className="w-full min-h-[50px] flex items-center gap-3 px-4 py-2.5 hover:bg-sidebar-hover transition-colors text-left">
      <div className="w-9 h-9 rounded-full bg-primary/20 text-primary flex items-center justify-center shrink-0">📎</div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <span className="text-[13px] font-semibold text-white truncate">{fileName}</span>
          <span className="text-[11px] text-sidebar-text shrink-0">{formatTime(message.created_at)}</span>
        </div>
        <p className="text-[12px] text-sidebar-text truncate">{author}</p>
      </div>
    </button>
  );
}

/* ─── Chat Item ─── */
function ChatItem({ chat, active, myId, typing, onlineUsers, onClick, onContextMenu }: {
  chat: ChatWithLastMessage; active: boolean; myId: string; typing?: string[];
  onlineUsers: Record<string, boolean>;
  onClick: () => void; onContextMenu?: (e: React.MouseEvent) => void;
}) {
  const name = getChatName(chat, myId);
  const online = getChatOnline(chat, myId, onlineUsers);
  const hasTyping = typing && typing.length > 0;
  const [showTyping, setShowTyping] = useState(hasTyping);
  useEffect(() => {
    if (hasTyping) setShowTyping(true);
    else {
      const id = setTimeout(() => setShowTyping(false), 1200);
      return () => clearTimeout(id);
    }
  }, [hasTyping]);
  const lastMsg = chat.last_message;

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onContextMenu={onContextMenu}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick(); } }}
      className={`w-full min-h-[48px] flex items-center gap-3 px-4 py-2.5 transition-all duration-200 ease-out text-left cursor-pointer ${
        active ? 'bg-sidebar-active' : 'hover:bg-sidebar-hover'
      }`}>
      {chat.chat.chat_type === 'notes' ? (
        <div className="w-11 h-11 rounded-full bg-primary flex items-center justify-center shrink-0 text-white">
          <svg width={22} height={22} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        </div>
      ) : (
        <Avatar name={name} url={chat.chat.avatar_url || undefined} size={44} online={online} />
      )}
      <div className="flex-1 min-w-0 min-h-0">
        <div className="flex items-center justify-between">
          <span className={`text-sidebar-name truncate ${active ? 'font-semibold text-white' : chat.unread_count > 0 ? 'font-semibold text-white' : 'font-medium text-white/90'}`}>{name}</span>
          <span className="flex items-center gap-1 shrink-0 ml-2">
            {chat.muted && (
              <span className={active ? 'text-white/70' : 'text-sidebar-text'} title="Уведомления отключены">
                <IconBellOff size={12} />
              </span>
            )}
            {lastMsg && (
              <span className={`text-sidebar-sub font-normal ${active ? 'text-white/85' : 'text-sidebar-text'}`}>{formatTime(lastMsg.created_at)}</span>
            )}
          </span>
        </div>
        <div className="flex items-center justify-between mt-0.5">
          <span className={`text-sidebar-sub truncate max-w-[180px] font-normal transition-opacity duration-200 ${active ? 'text-white/85' : 'text-sidebar-text'}`}>
            {showTyping ? (
              <span className="text-primary flex items-center gap-1">печатает <TypingDots /></span>
            ) : lastMsg ? (
              lastMsg.is_deleted ? <span className="italic">Сообщение удалено</span> : (
                <>
                  {lastMsg.sender_id === myId && <span className={active ? 'text-white/70' : 'text-sidebar-text/50'}>Вы: </span>}
                  {lastMsg.content_type === 'image' ? '📷 Фото' : lastMsg.content_type === 'file' ? '📎 Файл' : lastMsg.content_type === 'voice' ? '🎤 Голосовое' : lastMsg.content}
                </>
              )
            ) : 'Нет сообщений'}
          </span>
          {chat.unread_count > 0 && (
            <span className="shrink-0 ml-2 min-w-[20px] h-5 flex items-center justify-center bg-primary rounded-full px-1.5 text-sidebar-sub font-semibold text-white">
              {chat.unread_count}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

/* ─── New Group Modal ─── */
function NewGroupModal({ open, onClose, searchUsers, createGroup, setActiveChat }: {
  open: boolean; onClose: () => void; searchUsers: (q: string) => Promise<UserPublic[]>;
  createGroup: (name: string, memberIds: string[]) => Promise<ChatWithLastMessage>; setActiveChat: (chatId: string) => void;
}) {
  const [name, setName] = useState('');
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<UserPublic[]>([]);
  const [selected, setSelected] = useState<UserPublic[]>([]);
  const [loading, setLoading] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout>>();

  const handleSearch = useCallback((q: string) => {
    setQuery(q);
    if (timer.current) clearTimeout(timer.current);
    if (!q.trim()) { setResults([]); return; }
    timer.current = setTimeout(async () => {
      setLoading(true);
      try { setResults(await searchUsers(q)); } catch { /* */ }
      setLoading(false);
    }, 300);
  }, [searchUsers]);

  const toggle = (u: UserPublic) => {
    setSelected((p) => p.some((s) => s.id === u.id) ? p.filter((s) => s.id !== u.id) : [...p, u]);
  };

  const handleCreate = async () => {
    if (!name.trim()) return;
    try {
      const chat = await createGroup(name, selected.map((u) => u.id));
      setActiveChat(chat.chat.id);
      onClose();
      setName(''); setQuery(''); setResults([]); setSelected([]);
    } catch { /* */ }
  };

  return (
    <Modal open={open} onClose={onClose} title="Новая группа" size="md">
      <input type="text" value={name} onChange={(e) => setName(e.target.value)} autoFocus
        placeholder="Название группы" className="compass-input mb-3" />
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mb-3">
          {selected.map((u) => (
            <span key={u.id} className="inline-flex items-center gap-1 px-2.5 py-1 bg-primary/10 text-primary rounded-full text-[12px] font-medium">
              {u.username}
              <button onClick={() => toggle(u)} className="hover:text-danger ml-0.5 text-[14px] leading-none">&times;</button>
            </span>
          ))}
        </div>
      )}
      <input type="text" value={query} onChange={(e) => handleSearch(e.target.value)}
        placeholder="Добавить участников..." className="compass-input mb-3" />
      <div className="max-h-48 overflow-y-auto space-y-0.5 mb-4">
        {loading && <p className="text-[13px] text-txt-secondary dark:text-[#8b98a5] text-center py-2">Поиск...</p>}
        {results.map((u) => (
          <button key={u.id} onClick={() => toggle(u)}
            className={`w-full flex items-center gap-3 px-3 py-2.5 min-h-[44px] rounded-compass transition-colors text-left ${
              selected.some((s) => s.id === u.id) ? 'bg-primary/8 dark:bg-primary/20' : 'hover:bg-surface dark:hover:bg-dark-hover'
            }`}>
            <Avatar name={u.username} url={u.avatar_url || undefined} size={34} />
            <span className="text-[14px] font-medium text-txt dark:text-[#e7e9ea]">{u.username}</span>
            {selected.some((s) => s.id === u.id) && <span className="ml-auto text-primary text-[14px]">✓</span>}
          </button>
        ))}
      </div>
      <button onClick={handleCreate} disabled={!name.trim()} className="compass-btn-primary w-full py-2.5">
        Создать группу{selected.length > 0 && ` (${selected.length})`}
      </button>
    </Modal>
  );
}

/* ─── Sidebar Context Menu Item ─── */
function SidebarCtxItem({ icon, label, onClick, danger }: { icon: React.ReactNode; label: string; onClick: () => void; danger?: boolean }) {
  return (
    <button onClick={onClick}
      className={`w-full flex items-center gap-2.5 px-4 py-2.5 min-h-[44px] text-[13px] font-medium hover:bg-surface dark:hover:bg-dark-hover transition-colors text-left ${danger ? 'text-danger' : 'text-txt dark:text-[#e7e9ea]'}`}>
      <span className={danger ? 'text-danger' : 'text-txt-secondary dark:text-[#8b98a5]'}>{icon}</span>
      {label}
    </button>
  );
}

/* ─── Rename Group Modal ─── */
function RenameGroupModal({ chat, onClose }: { chat: ChatWithLastMessage; onClose: () => void }) {
  const [name, setName] = useState(chat.chat.name);
  const [saving, setSaving] = useState(false);
  const { fetchChats } = useChatStore();

  const handleSave = useCallback(async () => {
    if (!name.trim() || name === chat.chat.name) { onClose(); return; }
    setSaving(true);
    try {
      await api.updateChat(chat.chat.id, { name });
      await fetchChats();
    } catch { /* */ }
    setSaving(false);
    onClose();
  }, [name, chat.chat.id, chat.chat.name, onClose, fetchChats]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 safe-area-padding">
      <div className="absolute inset-0 bg-[rgba(4,4,10,0.55)] dark:bg-black/60" onClick={onClose} />
      <div className="relative bg-white dark:bg-dark-elevated rounded-compass shadow-compass-dialog w-full max-w-[380px] animate-dialog border border-transparent dark:border-dark-border">
        <div className="flex items-center justify-between px-5 pt-4 pb-2">
          <h3 className="text-[17px] font-bold text-txt dark:text-[#e7e9ea]">Изменить имя группы</h3>
          <button onClick={onClose} className="min-w-[44px] min-h-[44px] flex items-center justify-center rounded-full hover:bg-surface-light dark:hover:bg-dark-hover transition-colors text-txt-secondary hover:text-txt dark:text-[#8b98a5] dark:hover:text-[#e7e9ea] -mr-2">
            <IconX size={12} />
          </button>
        </div>
        <div className="px-5 pb-5 space-y-4">
          <div className="flex items-center gap-3 pb-2">
            <Avatar name={name || chat.chat.name} url={chat.chat.avatar_url || undefined} size={48} />
            <div className="min-w-0 flex-1">
              <p className="text-[12px] text-txt-secondary dark:text-[#8b98a5]">Текущее название</p>
              <p className="text-[14px] font-semibold text-txt dark:text-[#e7e9ea] truncate">{chat.chat.name}</p>
            </div>
          </div>
          <div>
            <label className="block text-[13px] font-medium text-txt-secondary dark:text-[#8b98a5] mb-1.5">Новое название</label>
            <input type="text" value={name} onChange={(e) => setName(e.target.value)} autoFocus
              className="compass-input" placeholder="Введите новое название..."
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleSave(); } }} />
          </div>
          <div className="flex gap-3">
            <button onClick={handleSave} disabled={saving || !name.trim()}
              className="compass-btn-primary flex-1 py-2.5 min-h-[44px]">
              {saving ? 'Сохранение...' : 'Сохранить'}
            </button>
            <button onClick={onClose} className="compass-btn-secondary flex-1 py-2.5 min-h-[44px]">Отмена</button>
          </div>
        </div>
      </div>
    </div>
  );
}
