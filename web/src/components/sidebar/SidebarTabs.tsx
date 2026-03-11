export type SidebarTab = 'all' | 'personal' | 'favorites';

interface SidebarTabsProps {
  tab: SidebarTab;
  onTabChange: (tab: SidebarTab) => void;
  personalUnread: number;
  favoritesUnread: number;
  mobile?: boolean;
}

export default function SidebarTabs({
  tab,
  onTabChange,
  personalUnread,
  favoritesUnread,
  mobile = false,
}: SidebarTabsProps) {
  const minHeightClass = mobile ? 'min-h-[40px] py-1.5 text-[12px]' : 'min-h-[44px] py-2 text-[13px]';

  return (
    <div className="flex gap-0.5 mb-1.5 p-0.5 bg-sidebar-hover rounded-compass">
      <button
        type="button"
        onClick={() => onTabChange('all')}
        className={`flex-1 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide transition-all duration-200 ease-out ${minHeightClass} ${tab === 'all' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}
      >
        ВСЕ
      </button>
      <button
        type="button"
        onClick={() => onTabChange('personal')}
        className={`flex-1 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide transition-all duration-200 ease-out ${minHeightClass} ${tab === 'personal' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}
      >
        <span className="inline-flex items-center justify-center gap-1.5">
          ЛИЧНЫЕ
          {personalUnread > 0 && (
            <span className="min-w-[18px] h-[18px] px-1 rounded-full bg-primary text-white text-[10px] leading-[18px]">
              {personalUnread}
            </span>
          )}
        </span>
      </button>
      <button
        type="button"
        onClick={() => onTabChange('favorites')}
        className={`flex-1 rounded-[6px] text-sidebar-tab font-medium uppercase tracking-wide transition-all duration-200 ease-out ${minHeightClass} ${tab === 'favorites' ? 'text-primary' : 'text-sidebar-text hover:text-white'}`}
      >
        <span className="inline-flex items-center justify-center gap-1.5">
          ИЗБРАННЫЕ
          {favoritesUnread > 0 && (
            <span className="min-w-[18px] h-[18px] px-1 rounded-full bg-primary text-white text-[10px] leading-[18px]">
              {favoritesUnread}
            </span>
          )}
        </span>
      </button>
    </div>
  );
}
