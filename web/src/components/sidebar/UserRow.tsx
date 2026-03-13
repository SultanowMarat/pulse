import { Avatar } from '../ui';
import type { UserPublic } from '../../types';

interface UserRowProps {
  user: UserPublic;
  online: boolean;
  onClick: () => void;
}

export default function UserRow({ user, online, onClick }: UserRowProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full min-h-[48px] flex items-center gap-3 px-4 py-2.5 hover:bg-sidebar-hover transition-colors text-left"
    >
      <Avatar name={user.username} url={user.avatar_url || undefined} size={44} online={online} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between">
          <span className="text-sidebar-name font-medium text-white/90 truncate">{user.username}</span>
        </div>
        <div className="mt-0.5">
          <span className="text-sidebar-sub font-normal text-sidebar-text">
            {'\u041D\u0430\u043F\u0438\u0441\u0430\u0442\u044C \u0441\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0435'}
          </span>
        </div>
      </div>
    </button>
  );
}
