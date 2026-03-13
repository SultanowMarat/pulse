import type { ReactNode } from 'react';

interface SidebarCtxItemProps {
  icon: ReactNode;
  label: string;
  onClick: () => void;
  danger?: boolean;
}

export default function SidebarCtxItem({ icon, label, onClick, danger }: SidebarCtxItemProps) {
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        onClick();
      }}
      className={`w-full flex items-center gap-2.5 px-4 py-2.5 min-h-[44px] text-[13px] font-medium hover:bg-surface dark:hover:bg-dark-hover transition-colors text-left ${danger ? 'text-danger' : 'text-txt dark:text-[#e7e9ea]'}`}
    >
      <span className={danger ? 'text-danger' : 'text-txt-secondary dark:text-[#8b98a5]'}>{icon}</span>
      {label}
    </button>
  );
}
