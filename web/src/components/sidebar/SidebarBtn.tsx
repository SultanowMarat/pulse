import type { ReactNode } from 'react';

interface SidebarBtnProps {
  tip: string;
  onClick: () => void;
  children: ReactNode;
}

export default function SidebarBtn({ tip, onClick, children }: SidebarBtnProps) {
  return (
    <button
      title={tip}
      onClick={onClick}
      className="min-w-[44px] min-h-[44px] w-10 h-10 flex items-center justify-center rounded-full hover:bg-sidebar-hover text-sidebar-text hover:text-white transition-colors"
    >
      {children}
    </button>
  );
}
