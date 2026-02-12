import { useState, useCallback, useRef, useEffect } from 'react';
import { useAuthStore, useChatStore } from '../store';
import { Avatar, Modal, IconX, IconPlus, IconPin, IconEdit, IconCheck } from './ui';
import UserCard from './UserCard';
import type { UserPublic, ChatWithLastMessage } from '../types';
import * as api from '../api';

interface Props { chat: ChatWithLastMessage; onClose: () => void; }

/* ── Icons ── */
function IconUserPlus({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M16 21v-2a4 4 0 00-4-4H5a4 4 0 00-4-4v2" /><circle cx="8.5" cy="7" r="4" /><line x1="20" y1="8" x2="20" y2="14" /><line x1="23" y1="11" x2="17" y2="11" />
    </svg>
  );
}
function IconBell({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 01-3.46 0" />
    </svg>
  );
}
function IconSettings({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <line x1="4" y1="21" x2="4" y2="14" /><line x1="4" y1="10" x2="4" y2="3" /><line x1="12" y1="21" x2="12" y2="12" /><line x1="12" y1="8" x2="12" y2="3" /><line x1="20" y1="21" x2="20" y2="16" /><line x1="20" y1="12" x2="20" y2="3" />
      <line x1="1" y1="14" x2="7" y2="14" /><line x1="9" y1="8" x2="15" y2="8" /><line x1="17" y1="16" x2="23" y2="16" />
    </svg>
  );
}
function IconDotsV({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor">
      <circle cx="12" cy="5" r="1.5" /><circle cx="12" cy="12" r="1.5" /><circle cx="12" cy="19" r="1.5" />
    </svg>
  );
}
function IconLogout({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4" /><polyline points="16 17 21 12 16 7" /><line x1="21" y1="12" x2="9" y2="12" />
    </svg>
  );
}
function IconBan({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" /><line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
    </svg>
  );
}

export default function ChatInfo({ chat, onClose }: Props) {
  const { user } = useAuthStore();
  const { leaveChat, onlineUsers, pinnedMessages, activeChatId, fetchChats, uploadFile, setChatMuted, setNotification } = useChatStore();
  const [showAdd, setShowAdd] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [userCardId, setUserCardId] = useState<string | null>(null);
  const [memberMenu, setMemberMenu] = useState<string | null>(null);
  const [muteBusy, setMuteBusy] = useState(false);
  const [myAdministrator, setMyAdministrator] = useState(false);
  const isGroup = chat.chat.chat_type === 'group';
  const isNotes = chat.chat.chat_type === 'notes';
  const isGeneral = chat.chat.system_key === 'general';
  const canEditGeneralChat = !isGeneral || myAdministrator;
  const pinned = activeChatId ? pinnedMessages[activeChatId] || [] : [];
  const chatName = isGroup || isNotes ? chat.chat.name : (chat.members.find((m) => m.id !== user?.id)?.username || 'Чат');
  const isCreator = chat.chat.created_by === user?.id;
  const isMuted = !!chat.muted;

  useEffect(() => {
    const meId = user?.id;
    if (!meId) return;
    let cancelled = false;
    api.getUserPermissions(meId)
      .then((res) => { if (!cancelled) setMyAdministrator(!!res.administrator); })
      .catch(() => { if (!cancelled) setMyAdministrator(false); });
    return () => { cancelled = true; };
  }, [user?.id]);

  const handleRemove = useCallback(async (id: string) => {
    if (!confirm('Исключить участника из группы?')) return;
    try {
      await api.removeMember(chat.chat.id, id);
      await fetchChats();
    } catch { /* */ }
    setMemberMenu(null);
  }, [chat.chat.id, fetchChats]);

  const handleLeave = useCallback(async () => {
    if (!confirm('Покинуть группу?')) return;
    await leaveChat(chat.chat.id);
    onClose();
  }, [chat.chat.id, leaveChat, onClose]);

  const handleToggleMute = useCallback(async () => {
    if (muteBusy) return;
    setMuteBusy(true);
    try {
      await setChatMuted(chat.chat.id, !isMuted);
      setNotification(!isMuted ? 'Уведомления отключены' : 'Уведомления включены');
    } catch {
      setNotification('Не удалось изменить уведомления');
    } finally {
      setMuteBusy(false);
    }
  }, [muteBusy, setChatMuted, chat.chat.id, isMuted, setNotification]);

  return (
    <div className="h-full flex flex-col bg-white dark:bg-dark-bg min-w-0" onClick={() => setMemberMenu(null)}>
      {/* Header bar */}
      <div className="shrink-0 flex items-center justify-between px-4 py-3 pt-[max(0.75rem,env(safe-area-inset-top))] border-b border-surface-border dark:border-dark-border">
        <h3 className="text-[14px] font-semibold text-txt dark:text-[#e7e9ea]">Информация</h3>
        <button onClick={onClose} className="min-w-[44px] min-h-[44px] flex items-center justify-center rounded-full hover:bg-surface dark:hover:bg-dark-hover transition-colors text-txt-secondary hover:text-txt dark:text-[#8b98a5] dark:hover:text-[#e7e9ea] -mr-2">
          <IconX size={12} />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto overflow-x-hidden">
        {/* ── Avatar, Name, Count ── */}
        <div className="flex flex-col items-center pt-8 pb-5 px-5 relative">
          {isGroup && canEditGeneralChat && (
            <button onClick={() => setShowEditModal(true)}
              className="absolute top-3 right-4 w-8 h-8 flex items-center justify-center rounded-full bg-surface dark:bg-dark-hover hover:bg-surface-light dark:hover:bg-dark-border text-txt-secondary hover:text-txt dark:text-dark-muted dark:hover:text-[#e7e9ea] transition-colors"
              title="Редактировать группу">
              <IconEdit />
            </button>
          )}
          {isNotes ? (
            <div className="w-24 h-24 rounded-full bg-primary flex items-center justify-center text-white shrink-0">
              <svg width={48} height={48} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
            </div>
          ) : (
            <Avatar name={chatName} url={chat.chat.avatar_url || undefined} size={96} />
          )}
          <>
            <h2 className="mt-3 text-[18px] font-bold text-txt dark:text-[#e7e9ea] text-center">{chatName}</h2>
            {isNotes && <p className="mt-1 text-[13px] text-txt-secondary dark:text-[#8b98a5]">Персональный чат</p>}
            {isGroup && (
              <p className="mt-1 text-[13px] text-txt-secondary dark:text-[#8b98a5]">{chat.members.length} участник{chat.members.length > 4 ? 'ов' : chat.members.length > 1 ? 'а' : ''}</p>
            )}
            {(isGroup || isNotes) && chat.chat.description && (
              <p className="mt-1.5 text-[12px] text-txt-secondary dark:text-[#8b98a5] text-center max-w-[240px] whitespace-pre-line">{chat.chat.description}</p>
            )}
            {!isGroup && !isNotes && (() => {
              const other = chat.members.find((m) => m.id !== user?.id);
              const online = other ? (onlineUsers[other.id] ?? other.is_online) : false;
              return <p className={`mt-1 text-[13px] font-medium ${online ? 'text-green' : 'text-txt-secondary dark:text-[#8b98a5]'}`}>{online ? 'Сейчас онлайн' : 'Не в сети'}</p>;
            })()}
          </>
        </div>

        {/* ── Edit Group Modal ── */}
        {isGroup && showEditModal && (
          <EditGroupModal
            chat={chat}
            onClose={() => setShowEditModal(false)}
            onSaved={() => { fetchChats(); setShowEditModal(false); }}
            uploadFile={uploadFile}
          />
        )}

        {/* ── Action Buttons (group) ── */}
        {isGroup && (
          <div className="flex justify-center gap-2.5 px-5 pb-5">
            {!isGeneral && <ActionBtn icon={<IconUserPlus />} label="добавить" onClick={() => setShowAdd(true)} />}
            <ActionBtn
              icon={<IconBell />}
              label={isMuted ? 'уведомл. выкл' : 'уведомл.'}
              onClick={handleToggleMute}
              active={isMuted}
              disabled={muteBusy}
            />
            {canEditGeneralChat && <ActionBtn icon={<IconSettings />} label="настройки" onClick={() => setShowEditModal(true)} />}
          </div>
        )}

        {/* ── Pinned ── */}
        {pinned.length > 0 && (
          <div className="px-5 pb-4">
            <h4 className="text-[11px] font-bold text-txt-secondary dark:text-[#8b98a5] uppercase tracking-wide mb-2">Закреплённые ({pinned.length})</h4>
            <div className="space-y-1">
              {pinned.map((p) => (
                <div key={p.message_id} className="flex items-start gap-2 px-3 py-2 bg-surface dark:bg-dark-elevated rounded-compass">
                  <span className="text-txt-secondary dark:text-[#8b98a5] shrink-0"><IconPin /></span>
                  <div className="min-w-0">
                    <p className="text-[11px] font-semibold text-primary">{p.message?.sender?.username}</p>
                    <p className="text-[12px] text-txt dark:text-[#e7e9ea] truncate">{p.message?.content}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* ── Members Section ── */}
        <div className="px-5 pb-4">
          <div className="flex items-center justify-between mb-3 border-b border-surface-border dark:border-dark-border pb-2">
            <h4 className="text-[13px] font-semibold text-primary">Участники</h4>
            {isGroup && !isGeneral && (
              <button onClick={() => setShowAdd(true)} className="text-[12px] text-primary font-medium hover:underline flex items-center gap-1">
                <IconPlus size={12} /> Добавить
              </button>
            )}
          </div>

          {/* Creator/Admin */}
          {isGroup && (() => {
            const creator = chat.members.find((m) => m.id === chat.chat.created_by);
            if (!creator) return null;
            return (
              <>
                <p className="text-[10px] font-bold text-txt-secondary dark:text-[#8b98a5] uppercase tracking-wider mb-1.5">Администраторы</p>
                <MemberRow
                  member={creator} isMe={creator.id === user?.id}
                  online={onlineUsers[creator.id] ?? creator.is_online}
                  role="Admin" roleColor="bg-primary"
                  onUserClick={() => creator.id !== user?.id && setUserCardId(creator.id)}
                />
              </>
            );
          })()}

          {/* Regular members */}
          {isGroup && chat.members.filter((m) => m.id !== chat.chat.created_by).length > 0 && (
            <>
              <p className="text-[10px] font-bold text-txt-secondary dark:text-[#8b98a5] uppercase tracking-wider mt-4 mb-1.5">Участники</p>
              {chat.members.filter((m) => m.id !== chat.chat.created_by).map((m) => (
                <div key={m.id} className="relative">
                  <MemberRow
                    member={m} isMe={m.id === user?.id}
                    online={onlineUsers[m.id] ?? m.is_online}
                    role="Member" roleColor="bg-green"
                    onUserClick={() => m.id !== user?.id && setUserCardId(m.id)}
                    action={isCreator && !isGeneral && m.id !== user?.id ? (
                      <button onClick={(e) => { e.stopPropagation(); setMemberMenu(memberMenu === m.id ? null : m.id); }}
                        className="min-w-[44px] min-h-[44px] flex items-center justify-center rounded-full hover:bg-surface-light dark:hover:bg-dark-border text-txt-secondary dark:text-[#8b98a5] hover:text-txt dark:hover:text-[#e7e9ea] transition-colors -mr-1">
                        <IconDotsV size={16} />
                      </button>
                    ) : undefined}
                  />
                  {/* Member dropdown */}
                  {memberMenu === m.id && (
                    <>
                      <div className="fixed inset-0 z-30" onClick={(e) => { e.stopPropagation(); setMemberMenu(null); }} />
                      <div className="absolute right-2 top-full -mt-1 z-40 bg-white dark:bg-dark-elevated rounded-compass shadow-compass-dialog border border-surface-border dark:border-dark-border py-1 min-w-[180px] animate-fade">
                        <button onClick={(e) => { e.stopPropagation(); handleRemove(m.id); }}
                          className="w-full flex items-center gap-2.5 px-4 py-2.5 text-[13px] font-medium text-danger hover:bg-danger/5 transition-colors">
                          <IconBan size={15} />
                          Исключить из группы
                        </button>
                      </div>
                    </>
                  )}
                </div>
              ))}
            </>
          )}

          {/* Personal chat - just list the other member */}
          {!isGroup && chat.members.filter((m) => m.id !== user?.id).map((m) => (
            <MemberRow key={m.id}
              member={m} isMe={false}
              online={onlineUsers[m.id] ?? m.is_online}
              onUserClick={() => setUserCardId(m.id)}
            />
          ))}
        </div>

        {/* ── Leave button ── */}
        {isGroup && !isGeneral && (
          <div className="px-5 pb-6 safe-bottom">
            <button onClick={handleLeave}
              className="w-full flex items-center justify-center gap-2 min-h-[44px] py-2.5 text-danger text-[13px] font-semibold rounded-compass hover:bg-danger/5 dark:hover:bg-danger/10 transition-colors border border-danger/20">
              <IconLogout size={15} />
              Покинуть группу
            </button>
          </div>
        )}
      </div>

      {!isGeneral && <AddMembersModal open={showAdd} onClose={() => setShowAdd(false)} chatId={chat.chat.id} existing={chat.members} onAdded={fetchChats} />}
      {userCardId && <UserCard userId={userCardId} onClose={() => setUserCardId(null)} />}
    </div>
  );
}

/* ── Action Button (Compass style, touch-friendly) ── */
function ActionBtn({
  icon,
  label,
  onClick,
  active,
  disabled,
}: {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  active?: boolean;
  disabled?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`flex flex-col items-center justify-center gap-1.5 min-h-[44px] w-[72px] py-2.5 rounded-[12px] transition-colors group ${
        active
          ? 'bg-danger/10 dark:bg-danger/20 hover:bg-danger/15 dark:hover:bg-danger/25'
          : 'bg-surface dark:bg-dark-elevated hover:bg-surface-light dark:hover:bg-dark-hover'
      } ${disabled ? 'opacity-60 cursor-not-allowed' : ''}`}
      title={label}
    >
      <span
        className={`transition-colors ${
          active ? 'text-danger' : 'text-txt-secondary dark:text-[#8b98a5] group-hover:text-primary'
        }`}
      >
        {icon}
      </span>
      <span
        className={`text-[10px] font-medium transition-colors ${
          active
            ? 'text-danger'
            : 'text-txt-secondary dark:text-[#8b98a5] group-hover:text-txt dark:group-hover:text-[#e7e9ea]'
        }`}
      >
        {label}
      </span>
    </button>
  );
}

/* ── Member Row (touch-friendly min height) ── */
function MemberRow({ member, isMe, online, role, roleColor, onUserClick, action }: {
  member: UserPublic; isMe: boolean; online: boolean;
  role?: string; roleColor?: string;
  onUserClick?: () => void; action?: React.ReactNode;
}) {
  return (
    <div className="flex items-center gap-3 px-2 py-2.5 min-h-[44px] rounded-compass hover:bg-surface dark:hover:bg-dark-hover transition-colors cursor-pointer"
      onClick={onUserClick}>
      <Avatar name={member.username} url={member.avatar_url || undefined} size={40} online={online} />
      <div className="flex-1 min-w-0">
        <p className="text-[13px] font-semibold text-txt dark:text-[#e7e9ea] truncate">{member.username}{isMe && ' (вы)'}</p>
        <p className="text-[11px] text-txt-secondary dark:text-[#8b98a5]">Пользователь BuhChat</p>
      </div>
      {role && (
        <span className={`px-2.5 py-0.5 rounded-full text-[10px] font-bold text-white ${roleColor || 'bg-primary'}`}>
          {role}
        </span>
      )}
      {action}
    </div>
  );
}

/* ── Edit Group Modal (имя, описание, фото) ── */
function EditGroupModal({
  chat,
  onClose,
  onSaved,
  uploadFile,
}: {
  chat: ChatWithLastMessage;
  onClose: () => void;
  onSaved: () => void;
  uploadFile: (file: File) => Promise<{ url: string }>;
}) {
  const [editName, setEditName] = useState(chat.chat.name);
  const [editDesc, setEditDesc] = useState(chat.chat.description || '');
  const [newAvatarUrl, setNewAvatarUrl] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const displayAvatarUrl = newAvatarUrl ?? (chat.chat.avatar_url || undefined);

  const handleAvatarClick = useCallback(() => {
    fileRef.current?.click();
  }, []);

  const handleFileChange = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file || !file.type.startsWith('image/')) return;
      setUploading(true);
      try {
        const res = await uploadFile(file);
        setNewAvatarUrl(res.url);
      } catch { /* */ }
      setUploading(false);
      if (fileRef.current) fileRef.current.value = '';
    },
    [uploadFile]
  );

  const handleDone = useCallback(async () => {
    if (!editName.trim()) return;
    setSaving(true);
    try {
      await api.updateChat(chat.chat.id, {
        name: editName.trim(),
        description: editDesc.trim() || undefined,
        ...(newAvatarUrl !== null && { avatar_url: newAvatarUrl }),
      });
      onSaved();
    } catch { /* */ }
    setSaving(false);
  }, [chat.chat.id, editName, editDesc, newAvatarUrl, onSaved]);

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center p-4 safe-area-padding">
      <div className="absolute inset-0 bg-black/50 dark:bg-black/60" onClick={onClose} />
      <div className="relative bg-white dark:bg-dark-elevated rounded-compass shadow-compass-dialog w-full max-w-[400px] border border-transparent dark:border-dark-border animate-dialog">
        <div className="flex items-center justify-between px-5 pt-4 pb-2">
          <h3 className="text-[17px] font-bold text-txt dark:text-[#e7e9ea]">Редактировать группу</h3>
          <button onClick={onClose} className="w-7 h-7 flex items-center justify-center rounded-full hover:bg-surface dark:hover:bg-dark-hover text-txt-secondary dark:text-dark-muted">
            <IconX size={12} />
          </button>
        </div>
        <div className="px-5 pb-5 space-y-4">
          <div className="flex gap-4 items-start">
            <button
              type="button"
              onClick={handleAvatarClick}
              disabled={uploading}
              className="relative shrink-0 rounded-full overflow-hidden bg-surface dark:bg-dark-hover ring-2 ring-transparent hover:ring-primary/40 transition-all disabled:opacity-60"
            >
              <Avatar name={chat.chat.name} url={displayAvatarUrl} size={72} />
              {uploading && (
                <div className="absolute inset-0 flex items-center justify-center bg-black/50 rounded-full">
                  <span className="text-white text-[11px] font-medium">...</span>
                </div>
              )}
            </button>
            <input type="file" ref={fileRef} accept="image/*" className="hidden" onChange={handleFileChange} />
            <div className="flex-1 min-w-0 space-y-2">
              <label className="block text-[12px] font-medium text-txt-secondary dark:text-[#8b98a5]">Название группы</label>
              <div className="relative">
                <input
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  className="compass-input pr-9"
                  placeholder="Название группы"
                  onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleDone(); } }}
                />
                {editName && (
                  <button
                    type="button"
                    onClick={() => setEditName('')}
                    className="absolute right-2 top-1/2 -translate-y-1/2 w-6 h-6 flex items-center justify-center rounded-full hover:bg-surface dark:hover:bg-dark-hover text-txt-placeholder dark:text-[#8b98a5]"
                  >
                    <IconX size={10} />
                  </button>
                )}
              </div>
            </div>
          </div>
          <div>
            <label className="block text-[12px] font-medium text-txt-secondary dark:text-[#8b98a5] mb-1">Описание группы</label>
            <textarea
              value={editDesc}
              onChange={(e) => setEditDesc(e.target.value)}
              className="compass-input min-h-[80px] resize-y"
              placeholder="Описание группы (необязательно)"
              rows={3}
            />
          </div>
          <div className="flex gap-3 pt-1">
            <button type="button" onClick={onClose} className="compass-btn-secondary flex-1 py-2.5">
              Отмена
            </button>
            <button
              type="button"
              onClick={handleDone}
              disabled={saving || !editName.trim()}
              className="compass-btn-primary flex-1 py-2.5 flex items-center justify-center gap-2"
            >
              <IconCheck />
              {saving ? 'Сохранение...' : 'Готово'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ── Add Members Modal ── */
function AddMembersModal({ open, onClose, chatId, existing, onAdded }: {
  open: boolean; onClose: () => void; chatId: string; existing: UserPublic[]; onAdded?: () => void | Promise<void>;
}) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<UserPublic[]>([]);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<UserPublic | null>(null);
  const [adding, setAdding] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    if (open) {
      setQuery('');
      setResults([]);
      setSelected(null);
    }
  }, [open]);

  const handleSearch = useCallback((q: string) => {
    setQuery(q);
    setSelected(null);
    if (timer.current) clearTimeout(timer.current);
    if (!q.trim()) { setResults([]); return; }
    timer.current = setTimeout(async () => {
      setLoading(true);
      try { setResults((await api.searchUsers(q)).filter((u) => !existing.some((m) => m.id === u.id))); } catch { /* */ }
      setLoading(false);
    }, 300);
  }, [existing]);

  const handleOk = useCallback(async () => {
    if (!selected) return;
    setAdding(true);
    try {
      await api.addMembers(chatId, [selected.id]);
      await onAdded?.();
      setSelected(null);
      setQuery('');
      setResults([]);
      onClose();
    } catch { /* */ }
    setAdding(false);
  }, [chatId, selected, onAdded, onClose]);

  const handleClose = useCallback(() => {
    setSelected(null);
    setQuery('');
    setResults([]);
    onClose();
  }, [onClose]);

  return (
    <Modal open={open} onClose={handleClose} title="Добавить участника">
      <input type="text" value={query} onChange={(e) => handleSearch(e.target.value)} autoFocus
        placeholder="Поиск участника..." className="compass-input mb-3" />
      <div className="max-h-56 overflow-y-auto space-y-0.5 mb-4">
        {loading && <p className="text-[13px] text-txt-secondary dark:text-[#8b98a5] text-center py-4">Поиск...</p>}
        {!loading && query.trim() && results.length === 0 && (
          <p className="text-[13px] text-txt-secondary dark:text-[#8b98a5] text-center py-4">Никого не найдено</p>
        )}
        {results.map((u) => (
          <button key={u.id} type="button" onClick={() => setSelected(u)}
            className={`w-full flex items-center gap-3 px-3 py-2.5 min-h-[44px] rounded-compass transition-colors text-left ${
              selected?.id === u.id ? 'bg-primary/15 dark:bg-primary/20 ring-2 ring-primary' : 'hover:bg-surface dark:hover:bg-dark-hover'
            }`}>
            <Avatar name={u.username} url={u.avatar_url || undefined} size={40} />
            <span className="text-[14px] font-medium text-txt dark:text-[#e7e9ea] flex-1 truncate">{u.username}</span>
            {selected?.id === u.id && <span className="text-primary text-[12px] font-semibold">Выбран</span>}
          </button>
        ))}
      </div>
      {selected && (
        <div className="flex gap-3">
          <button type="button" onClick={handleOk} disabled={adding}
            className="compass-btn-primary flex-1 py-2.5">
            {adding ? 'Добавление...' : 'ОК'}
          </button>
          <button type="button" onClick={handleClose} className="compass-btn-secondary flex-1 py-2.5">
            Отмена
          </button>
        </div>
      )}
    </Modal>
  );
}
