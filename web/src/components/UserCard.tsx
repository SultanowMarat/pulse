import { useState, useEffect, useCallback, useMemo } from 'react';
import { Avatar, IconX, IconPhone } from './ui';
import type { UserStats } from '../types';
import * as api from '../api';
import { useChatStore } from '../store';

interface Props {
  userId: string;
  onClose: () => void;
  onOpenChat?: (userId: string) => void;
}

/* ── Helpers ── */
function formatLastSeen(iso: string): string {
  if (!iso) return 'Неизвестно';
  const d = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - d.getTime();
  const mins = Math.floor(diffMs / 60000);
  if (mins < 1) return 'Только что';
  if (mins < 60) return `${mins} мин. назад`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} ч. назад`;
  const days = Math.floor(hours / 24);
  if (days === 1) return 'Вчера';
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' });
}

function formatResponseTime(sec: number): string {
  if (sec <= 0) return '—';
  if (sec < 60) return `~${Math.round(sec)} сек.`;
  const m = Math.round(sec / 60);
  if (m < 60) return `~${m} мин.`;
  const h = Math.round(m / 60);
  return `~${h} ч.`;
}

/* ── Icons ── */
function IconMail({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="4" width="20" height="16" rx="2" />
      <path d="M22 7l-10 7L2 7" />
    </svg>
  );
}
function IconBuilding({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 21h18" />
      <path d="M6 21V7a2 2 0 012-2h8a2 2 0 012 2v14" />
      <path d="M9 9h.01M12 9h.01M15 9h.01" />
      <path d="M9 12h.01M12 12h.01M15 12h.01" />
      <path d="M9 15h.01M12 15h.01M15 15h.01" />
    </svg>
  );
}
function IconBriefcase({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M10 6V5a2 2 0 012-2h0a2 2 0 012 2v1" />
      <rect x="3" y="6" width="18" height="14" rx="2" />
      <path d="M3 12h18" />
      <path d="M12 12v3" />
    </svg>
  );
}
function IconClock({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <polyline points="12 6 12 12 16 14" />
    </svg>
  );
}
function IconMessageCircleFilled({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" />
    </svg>
  );
}
function IconDotsH({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor">
      <circle cx="5" cy="12" r="1.5" />
      <circle cx="12" cy="12" r="1.5" />
      <circle cx="19" cy="12" r="1.5" />
    </svg>
  );
}
function IconBell({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9" />
      <path d="M13.73 21a2 2 0 01-3.46 0" />
    </svg>
  );
}
function IconTrash({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <polyline points="3 6 5 6 21 6" />
      <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2" />
      <line x1="10" y1="11" x2="10" y2="17" />
      <line x1="14" y1="11" x2="14" y2="17" />
    </svg>
  );
}

export default function UserCard({ userId, onClose, onOpenChat }: Props) {
  const setNotification = useChatStore((s) => s.setNotification);
  const chats = useChatStore((s) => s.chats);
  const setChatMuted = useChatStore((s) => s.setChatMuted);
  const clearChatHistory = useChatStore((s) => s.clearChatHistory);
  const [data, setData] = useState<UserStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [menuOpen, setMenuOpen] = useState(false);

  const personalChat = useMemo(() => (
    chats.find((c) => c.chat.chat_type === 'personal' && c.members.some((m) => m.id === userId))
  ), [chats, userId]);
  const isMuted = personalChat?.muted ?? false;

  const requirePersonalChat = useCallback(() => {
    if (personalChat) return personalChat;
    setNotification('Личный чат ещё не создан.');
    return null;
  }, [personalChat, setNotification]);

  const handleToggleMute = useCallback(async () => {
    const chat = requirePersonalChat();
    if (!chat) return;
    try {
      await setChatMuted(chat.chat.id, !isMuted);
      setNotification(!isMuted ? 'Уведомления отключены' : 'Уведомления включены');
    } catch {
      setNotification('Не удалось изменить уведомления');
    }
  }, [requirePersonalChat, setChatMuted, isMuted, setNotification]);

  const handleDeleteChat = useCallback(async () => {
    const chat = requirePersonalChat();
    if (!chat) return;
    if (!confirm('Очистить чат с этим пользователем?')) return;
    try {
      await clearChatHistory(chat.chat.id);
      setNotification('Чат очищен у вас. Если второй пользователь тоже очистит чат, сообщения и файлы удалятся полностью.');
    } catch {
      setNotification('Не удалось очистить чат');
    }
  }, [requirePersonalChat, clearChatHistory, setNotification]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api.getUserStats(userId).then((res) => {
      if (!cancelled) { setData(res); setLoading(false); }
    }).catch(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [userId]);

  const u = data?.user;

  return (
    <div className="fixed inset-0 z-50 flex items-start sm:items-center justify-center p-2 sm:p-4 safe-area-padding min-h-[var(--app-height)] overflow-y-auto">
      <div className="absolute inset-0 bg-[rgba(4,4,10,0.55)] dark:bg-black/60" onClick={onClose} />
      <div className="relative bg-white dark:bg-dark-elevated rounded-[16px] shadow-compass-dialog w-full max-w-[400px] max-h-[calc(var(--app-height)-1rem)] sm:max-h-[90dvh] overflow-y-auto animate-dialog border border-transparent dark:border-dark-border my-2 sm:my-0">
        {/* Кнопка: закрыть */}
        <div className="absolute top-3 right-3 z-10 flex items-center gap-2">
          <button onClick={onClose}
            className="w-8 h-8 flex items-center justify-center rounded-full bg-black/10 hover:bg-black/20 dark:bg-white/10 dark:hover:bg-white/20 text-white dark:text-txt transition-colors"
            aria-label="Закрыть">
            <IconX size={14} />
          </button>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-20">
            <div className="w-8 h-8 border-3 border-primary/30 border-t-primary rounded-full animate-spin" />
          </div>
        ) : u ? (
          <>
            {/* Header: Avatar + Name + Status */}
            <div className="flex flex-col items-center pt-8 pb-5 bg-gradient-to-b from-primary/5 to-transparent dark:from-transparent dark:to-transparent">
              <Avatar name={u.username} url={u.avatar_url || undefined} size={96} online={u.is_online} />
              <h2 className="mt-3 text-[20px] font-bold text-txt dark:text-[#e7e9ea]">{u.username}</h2>
              <p className={`text-[13px] font-medium mt-0.5 ${u.is_online ? 'text-green' : 'text-txt-secondary dark:text-[#8b98a5]'}`}>
                {u.is_online ? 'Сейчас онлайн' : `Был(а) ${formatLastSeen(u.last_seen_at)}`}
              </p>
              {u.disabled_at && (
                <p className="text-[12px] font-medium mt-1 text-danger">Пользователь отключён</p>
              )}
            </div>

            {/* Action buttons */}
            <div className="flex justify-center gap-2.5 sm:gap-3 px-4 sm:px-6 pb-4 -mt-1 relative">
              <ActionBtn icon={<IconMessageCircleFilled />} label="написать" onClick={() => { onOpenChat?.(userId); onClose(); }} />
              <div className="relative flex justify-end">
                <ActionBtn icon={<IconDotsH />} label="ещё" onClick={() => setMenuOpen((v) => !v)} />
                {menuOpen && (
                  <>
                    <div className="fixed inset-0 z-0" aria-hidden onClick={() => setMenuOpen(false)} />
                    <div className="absolute right-0 top-full z-10 mt-2 w-[max(220px,min(280px,calc(100vw-2rem)))] max-w-[calc(100vw-2rem)] py-1 bg-[#2f3336] dark:bg-dark-hover rounded-lg shadow-lg border border-white/10 dark:border-dark-border">
                      <MenuBtn icon={<IconBell />} label={isMuted ? 'Включить уведомления' : 'Отключить уведомления'} onClick={() => { setMenuOpen(false); handleToggleMute(); }} />
                      <MenuBtn icon={<IconTrash />} label="Удалить чат" onClick={() => { setMenuOpen(false); handleDeleteChat(); }} />
                    </div>
                  </>
                )}
              </div>
            </div>

            {/* Contact info — поля как на макете: иконка слева, подпись сверху, значение снизу (Почта и Телефон по аналогии) */}
            <div className="mx-5 mb-3 flex flex-col gap-3">
              <ContactField icon={<IconBuilding size={20} />} label="Компания" value={u.company || '—'} />
              <ContactField icon={<IconBriefcase size={20} />} label="Должность" value={u.position || '—'} />
              <ContactField icon={<IconMail size={20} />} label="Почта" value={u.email || '—'} />
              <ContactField icon={<IconPhone size={20} />} label="Телефон" value={u.phone || '—'} />
            </div>

            {/* Activity stats */}
            <div className="mx-5 mb-5 bg-surface dark:bg-dark-hover rounded-[12px] overflow-hidden divide-y divide-surface-border dark:divide-dark-border">
              <StatRow icon={<IconClock />} label="Последний раз в сети" value={u.is_online ? 'Сейчас' : formatLastSeen(u.last_seen_at)} />
              <StatRow icon={<IconClock />} label="Отвечает в течение" value={formatResponseTime(data?.avg_response_sec ?? 0)} />
            </div>
          </>
        ) : (
          <div className="py-16 text-center text-txt-secondary dark:text-[#8b98a5] text-[14px]">Пользователь не найден</div>
        )}
      </div>
    </div>
  );
}

/* ── Sub-components ── */
function ActionBtn({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button onClick={onClick}
      className="flex flex-col items-center gap-1.5 w-[74px] sm:w-[80px] py-2.5 sm:py-3 rounded-[12px] bg-[#f0f0f0] dark:bg-[#2f3336] hover:bg-[#e8e8e8] dark:hover:bg-white/10 transition-colors group border border-transparent dark:border-white/10">
      <span className="text-txt-secondary dark:text-[#8b98a5] group-hover:text-primary transition-colors">{icon}</span>
      <span className="text-[11px] font-medium text-txt-secondary dark:text-[#8b98a5] group-hover:text-txt dark:group-hover:text-[#e7e9ea] transition-colors">{label}</span>
    </button>
  );
}

function MenuBtn({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick}
      className="w-full flex items-center gap-3 px-4 py-2.5 text-left text-[14px] text-white hover:bg-white/10 transition-colors">
      <span className="text-[#8b98a5] flex-shrink-0">{icon}</span>
      <span className="min-w-0 break-words">{label}</span>
    </button>
  );
}

/* Поле контакта в стиле макета: светлый скруглённый блок, иконка слева, подпись сверху, значение снизу */
function ContactField({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-start gap-3 px-4 py-3 rounded-[12px] bg-[#f0f0f0] dark:bg-[#2f3336] border border-transparent dark:border-white/10">
      <span className="text-txt-secondary dark:text-[#8b98a5] flex-shrink-0 mt-0.5">{icon}</span>
      <div className="min-w-0 flex-1">
        <p className="text-[11px] text-txt-placeholder dark:text-[#8b98a5] mb-0.5">{label}</p>
        <p className="text-[14px] font-medium text-txt dark:text-[#e7e9ea] truncate">{value}</p>
      </div>
    </div>
  );
}

function StatRow({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-center justify-between px-4 py-3.5">
      <div className="flex items-center gap-2.5">
        <span className="text-txt-secondary dark:text-[#8b98a5]">{icon}</span>
        <span className="text-[13px] text-txt dark:text-[#e7e9ea]">{label}</span>
      </div>
      <span className="text-[14px] font-bold text-txt dark:text-[#e7e9ea]">{value}</span>
    </div>
  );
}
