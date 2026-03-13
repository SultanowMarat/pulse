import { formatTime } from '../ui';
import type { Message } from '../../types';

interface SearchFileRowProps {
  message: Message;
  onClick: () => void;
}

export default function SearchFileRow({ message, onClick }: SearchFileRowProps) {
  const fileName = (message.file_name || '').replace(/\+/g, ' ').trim() || '\u0424\u0430\u0439\u043B';
  const author = message.sender?.username || '\u041F\u043E\u043B\u044C\u0437\u043E\u0432\u0430\u0442\u0435\u043B\u044C';

  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full min-h-[50px] flex items-center gap-3 px-4 py-2.5 hover:bg-sidebar-hover transition-colors text-left"
    >
      <div className="w-9 h-9 rounded-full bg-primary/20 text-primary flex items-center justify-center shrink-0">\uD83D\uDCCE</div>
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
