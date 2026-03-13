import { IconSearch, IconX } from '../ui';

interface SidebarSearchInputProps {
  value: string;
  onChange: (value: string) => void;
  mobile?: boolean;
}

export default function SidebarSearchInput({ value, onChange, mobile = false }: SidebarSearchInputProps) {
  const paddingClass = mobile ? 'py-2' : 'py-2.5';
  const hasValue = value.trim().length > 0;

  return (
    <div className="relative">
      <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sidebar-text pointer-events-none">
        <IconSearch size={18} />
      </span>
      <button
        type="button"
        onClick={() => onChange('')}
        disabled={!hasValue}
        className={`absolute right-2 top-1/2 -translate-y-1/2 w-7 h-7 flex items-center justify-center rounded-full transition-colors ${
          hasValue
            ? 'text-sidebar-text hover:text-white hover:bg-sidebar/40'
            : 'text-sidebar-text/35 cursor-default pointer-events-none'
        }`}
        aria-label="Очистить поиск"
        title="Очистить поиск"
      >
        <IconX size={12} />
      </button>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Поиск"
        className={`w-full pl-10 pr-10 ${paddingClass} bg-sidebar-hover rounded-compass text-[14px] text-white placeholder:text-sidebar-text border border-transparent focus:border-primary/50 focus:ring-2 focus:ring-primary/20 outline-none transition-colors`}
      />
    </div>
  );
}
