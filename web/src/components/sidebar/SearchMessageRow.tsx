import { Avatar, formatTime } from '../ui';
import type { Message } from '../../types';

interface SearchMessageRowProps {
  message: Message;
  onClick: () => void;
}

export default function SearchMessageRow({ message, onClick }: SearchMessageRowProps) {
  const text = (message.content || '').trim();
  const preview = message.content_type === 'text'
    ? (text || '\u0422\u0435\u043A\u0441\u0442')
    : message.content_type === 'voice'
      ? '\uD83C\uDFA4 \u0413\u043E\u043B\u043E\u0441\u043E\u0432\u043E\u0435 \u0441\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0435'
      : message.content_type === 'image'
        ? '\uD83D\uDCF7 \u0418\u0437\u043E\u0431\u0440\u0430\u0436\u0435\u043D\u0438\u0435'
        : message.content_type === 'file'
          ? `\uD83D\uDCCE ${message.file_name || '\u0424\u0430\u0439\u043B'}`
          : (text || '\u0421\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0435');
  const author = message.sender?.username || '\u041F\u043E\u043B\u044C\u0437\u043E\u0432\u0430\u0442\u0435\u043B\u044C';

  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full min-h-[52px] flex items-start gap-3 px-4 py-2.5 hover:bg-sidebar-hover transition-colors text-left"
    >
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
