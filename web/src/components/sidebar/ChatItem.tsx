import { useEffect, useState } from 'react';
import { Avatar, TypingDots, formatTime } from '../ui';
import type { ChatWithLastMessage } from '../../types';
import IconBellOff from './IconBellOff';
import { getChatName, getChatOnline } from './chatMeta';

interface ChatItemProps {
  chat: ChatWithLastMessage;
  active: boolean;
  myId: string;
  typing?: string[];
  onlineUsers: Record<string, boolean>;
  onClick: () => void;
  onContextMenu?: (e: React.MouseEvent) => void;
}

export default function ChatItem({
  chat,
  active,
  myId,
  typing,
  onlineUsers,
  onClick,
  onContextMenu,
}: ChatItemProps) {
  const name = getChatName(chat, myId);
  const online = getChatOnline(chat, myId, onlineUsers);
  const hasTyping = Boolean(typing && typing.length > 0);
  const [showTyping, setShowTyping] = useState(hasTyping);

  useEffect(() => {
    if (hasTyping) {
      setShowTyping(true);
      return;
    }
    const id = setTimeout(() => setShowTyping(false), 1200);
    return () => clearTimeout(id);
  }, [hasTyping]);

  const lastMsg = chat.last_message;

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onContextMenu={onContextMenu}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onClick();
        }
      }}
      className={`w-full min-h-[48px] flex items-center gap-3 px-4 py-2.5 transition-all duration-200 ease-out text-left cursor-pointer ${
        active ? 'bg-sidebar-active' : 'hover:bg-sidebar-hover'
      }`}
    >
      {chat.chat.chat_type === 'notes' ? (
        <div className="w-11 h-11 rounded-full bg-primary flex items-center justify-center shrink-0 text-white">
          <svg width={22} height={22} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
            <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" />
            <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" />
          </svg>
        </div>
      ) : (
        <Avatar name={name} url={chat.chat.avatar_url || undefined} size={44} online={online} />
      )}

      <div className="flex-1 min-w-0 min-h-0">
        <div className="flex items-center justify-between">
          <span className={`text-sidebar-name truncate ${active ? 'font-semibold text-white' : chat.unread_count > 0 ? 'font-semibold text-white' : 'font-medium text-white/90'}`}>
            {name}
          </span>
          <span className="flex items-center gap-1 shrink-0 ml-2">
            {chat.muted && (
              <span className={active ? 'text-white/70' : 'text-sidebar-text'} title={'\u0423\u0432\u0435\u0434\u043E\u043C\u043B\u0435\u043D\u0438\u044F \u043E\u0442\u043A\u043B\u044E\u0447\u0435\u043D\u044B'}>
                <IconBellOff size={12} />
              </span>
            )}
            {lastMsg && (
              <span className={`text-sidebar-sub font-normal ${active ? 'text-white/85' : 'text-sidebar-text'}`}>
                {formatTime(lastMsg.created_at)}
              </span>
            )}
          </span>
        </div>

        <div className="flex items-center justify-between mt-0.5">
          <span className={`text-sidebar-sub truncate max-w-[180px] font-normal transition-opacity duration-200 ${active ? 'text-white/85' : 'text-sidebar-text'}`}>
            {showTyping ? (
              <span className="text-primary flex items-center gap-1">
                {'\u043F\u0435\u0447\u0430\u0442\u0430\u0435\u0442'} <TypingDots />
              </span>
            ) : lastMsg ? (
              lastMsg.is_deleted ? (
                <span className="italic">{'\u0421\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0435 \u0443\u0434\u0430\u043B\u0435\u043D\u043E'}</span>
              ) : (
                <>
                  {lastMsg.sender_id === myId && <span className={active ? 'text-white/70' : 'text-sidebar-text/50'}>{'\u0412\u044B: '}</span>}
                  {lastMsg.content_type === 'image'
                    ? '\uD83D\uDCF7 \u0424\u043E\u0442\u043E'
                    : lastMsg.content_type === 'file'
                      ? '\uD83D\uDCCE \u0424\u0430\u0439\u043B'
                      : lastMsg.content_type === 'voice'
                        ? '\uD83C\uDFA4 \u0413\u043E\u043B\u043E\u0441\u043E\u0432\u043E\u0435'
                        : lastMsg.content}
                </>
              )
            ) : (
              '\u041D\u0435\u0442 \u0441\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0439'
            )}
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
