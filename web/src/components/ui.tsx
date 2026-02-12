import React from 'react';
import { getApiBase } from '../serverUrl';

/* ─── Avatar ─── */
const palette = ['#007AFF','#FF6A64','#05C46B','#FF8A00','#D05DBD','#2574A9','#009FE6','#8B5CF6','#F59E0B','#6366F1'];

function hashColor(name: string): string {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = name.charCodeAt(i) + ((h << 5) - h);
  return palette[Math.abs(h) % palette.length];
}

/** Всегда отдаём абсолютный URL аватара: в десктопе и PWA относительные пути не работают. */
function resolveAvatarUrl(url: string | undefined): string | undefined {
  if (!url || typeof url !== 'string') return url;
  const t = url.trim();
  if (t.startsWith('http://') || t.startsWith('https://')) return t;
  const base = getApiBase();
  if (!base) return url;
  const norm = base.replace(/\/$/, '');
  return t.startsWith('/') ? norm + t : norm + '/' + t;
}

interface AvatarProps {
  name: string;
  url?: string;
  size?: number;
  online?: boolean;
  className?: string;
}

export const Avatar = React.memo(function Avatar({ name, url, size = 40, online, className = '' }: AvatarProps) {
  const [loadFailed, setLoadFailed] = React.useState(false);
  const imgSrc = resolveAvatarUrl(url);
  React.useEffect(() => setLoadFailed(false), [url]);
  const showImg = imgSrc && !loadFailed;
  const initials = name.slice(0, 2).toUpperCase();
  const bg = hashColor(name);
  const fontSize = size * 0.36;
  const dotSize = Math.max(size * 0.24, 8);

  return (
    <div className={`relative shrink-0 ${className}`} style={{ width: size, height: size }}>
      {showImg ? (
        <img
          src={imgSrc}
          alt={name}
          className="w-full h-full rounded-full object-cover"
          referrerPolicy="no-referrer"
          onError={() => setLoadFailed(true)}
        />
      ) : (
        <div
          className="w-full h-full rounded-full flex items-center justify-center text-white font-bold select-none"
          style={{ backgroundColor: bg, fontSize }}
        >
          {initials}
        </div>
      )}
      {online !== undefined && (
        <span
          className={`absolute bottom-0 right-0 block rounded-full border-2 border-white ${online ? 'bg-green' : 'bg-txt-placeholder'}`}
          style={{ width: dotSize, height: dotSize }}
        />
      )}
    </div>
  );
});

/* ─── Modal (Compass-style dialog) ─── */
interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  size?: 'sm' | 'md';
}

export function Modal({ open, onClose, title, children, size = 'sm' }: ModalProps) {
  if (!open) return null;
  const maxW = size === 'md' ? 'max-w-[448px]' : 'max-w-[380px]';
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 safe-area-padding min-h-[100dvh]">
      <div className="absolute inset-0 bg-[rgba(4,4,10,0.5)] dark:bg-black/60" onClick={onClose} />
      <div className={`relative bg-white dark:bg-dark-elevated rounded-compass shadow-compass-dialog border border-transparent dark:border-dark-border ${maxW} w-full max-h-[90dvh] flex flex-col animate-dialog`}>
        <div className="flex items-center justify-between px-5 pt-4 pb-2 shrink-0">
          <h3 className="text-[17px] font-bold text-txt dark:text-[#e7e9ea] leading-[26px] truncate pr-2">{title}</h3>
          <button onClick={onClose} className="min-w-[44px] min-h-[44px] flex items-center justify-center rounded-full hover:bg-surface-light dark:hover:bg-dark-hover transition-colors text-txt-secondary hover:text-txt dark:text-dark-muted dark:hover:text-[#e7e9ea] -mr-2 shrink-0">
            <svg width={14} height={14} viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round">
              <path d="M1 1l12 12M13 1L1 13"/>
            </svg>
          </button>
        </div>
        <div className="px-5 pb-5 overflow-y-auto min-h-0">{children}</div>
      </div>
    </div>
  );
}

/* ─── Typing Dots ─── */
export function TypingDots() {
  return (
    <span className="inline-flex items-center gap-0.5 h-4">
      <span className="typing-dot" />
      <span className="typing-dot" />
      <span className="typing-dot" />
    </span>
  );
}

/* ─── Icons ─── */
const s = { fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const };

export function IconSearch({ size = 18 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" {...s}><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>;
}
export function IconSend() {
  return <svg width={20} height={20} viewBox="0 0 24 24" {...s}><path d="M22 2L11 13"/><path d="M22 2l-7 20-4-9-9-4 20-7z"/></svg>;
}
export function IconPaperclip() {
  return <svg width={20} height={20} viewBox="0 0 24 24" {...s}><path d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48"/></svg>;
}
export function IconMicrophone({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 2a3 3 0 013 3v6a3 3 0 01-6 0V5a3 3 0 013-3z"/>
      <path d="M19 10v2a7 7 0 01-14 0v-2"/>
      <line x1="12" y1="19" x2="12" y2="22"/>
      <line x1="8" y1="22" x2="16" y2="22"/>
    </svg>
  );
}
export function IconPlay({ size = 16 }: { size?: number } = {}) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor">
      <polygon points="7,5 19,12 7,19" />
    </svg>
  );
}
export function IconPause({ size = 16 }: { size?: number } = {}) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor">
      <rect x="6" y="5" width="4" height="14" rx="1" />
      <rect x="14" y="5" width="4" height="14" rx="1" />
    </svg>
  );
}
export function IconVolume({ size = 16 }: { size?: number } = {}) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
      <path d="M15 9a4 4 0 010 6" />
      <path d="M18 6a8 8 0 010 12" />
    </svg>
  );
}
export function IconPlus({ size = 18 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" {...s}><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>;
}
export function IconUsers({ size = 18 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" {...s}><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4-4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87"/><path d="M16 3.13a4 4 0 010 7.75"/></svg>;
}
export function IconX({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round">
      <path d="M1 1l12 12M13 1L1 13"/>
    </svg>
  );
}
export function IconCheck() {
  return <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5} strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>;
}
export function IconCheckDouble() {
  return (
    <svg width={16} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5} strokeLinecap="round" strokeLinejoin="round">
      <polyline points="18 6 7 17 2 12"/><polyline points="22 6 11 17"/>
    </svg>
  );
}
export function IconFile({ size = 18 }: { size?: number } = {}) {
  return <svg width={size} height={size} viewBox="0 0 24 24" {...s}><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>;
}
export function IconDownload({ size = 22 }: { size?: number } = {}) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" />
      <polyline points="7 10 12 15 17 10" />
      <line x1="12" y1="15" x2="12" y2="3" />
    </svg>
  );
}
export function IconLogout() {
  return <svg width={20} height={20} viewBox="0 0 24 24" {...s}><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>;
}
export function IconBack() {
  return <svg width={20} height={20} viewBox="0 0 24 24" {...s}><polyline points="15 18 9 12 15 6"/></svg>;
}
export function IconDotsVertical({ size = 20 }: { size?: number } = {}) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor">
      <circle cx="12" cy="5" r="1.5" />
      <circle cx="12" cy="12" r="1.5" />
      <circle cx="12" cy="19" r="1.5" />
    </svg>
  );
}
export function IconPhone({ size = 20, className = '' }: { size?: number; className?: string } = {}) {
  return <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...s}><path d="M22 16.92v3a2 2 0 01-2.18 2 19.79 19.79 0 01-8.63-3.07 19.5 19.5 0 01-6-6A19.79 19.79 0 012.12 4.18 2 2 0 014.11 2h3a2 2 0 012 1.72c.127.96.361 1.903.7 2.81a2 2 0 01-.45 2.11L8.09 9.91a16 16 0 006 6l1.27-1.27a2 2 0 012.11-.45c.907.339 1.85.573 2.81.7A2 2 0 0122 16.92z"/></svg>;
}
export function IconPhoneOff({ size = 20, className = '' }: { size?: number; className?: string } = {}) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...s}>
      <path d="M10.68 13.31a16 16 0 003.41 2.6l1.27-1.27a2 2 0 012.11-.45 19.79 19.79 0 005.25-2.52 2 2 0 001.72-2v-3a2 2 0 00-1.72-2 19.79 19.79 0 00-2.81-.7"/>
      <path d="M15.91 11.29l4.3-4.3a1 1 0 011.41 0l1.41 1.41a1 1 0 010 1.41l-4.3 4.3M2 2l20 20"/>
    </svg>
  );
}
export function IconMail({ size = 20 }: { size?: number } = {}) {
  return <svg width={size} height={size} viewBox="0 0 24 24" {...s}><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M22 7l-10 7L2 7"/></svg>;
}
export function IconUser({ size = 20 }: { size?: number } = {}) {
  return <svg width={size} height={size} viewBox="0 0 24 24" {...s}><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>;
}
export function IconStarOutline({ size = 18, className = '' }: { size?: number; className?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className={className}>
      <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
    </svg>
  );
}
export function IconStarFilled({ size = 18, className = '' }: { size?: number; className?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
    </svg>
  );
}
export function IconPin() {
  return <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M12 17v5"/><path d="M9 2h6l1 7H8l1-7z"/><path d="M8 9l-2 8h12l-2-8"/></svg>;
}
export function IconReply() {
  return <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><polyline points="9 17 4 12 9 7"/><path d="M20 18v-2a4 4 0 00-4-4H4"/></svg>;
}
export function IconEdit() {
  return <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>;
}
export function IconTrash() {
  return <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg>;
}

export function IconForward() {
  return <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><polyline points="15 17 20 12 15 7"/><path d="M4 18v-2a4 4 0 014-4h12"/></svg>;
}
export function IconBookmark() {
  return <svg width={22} height={22} viewBox="0 0 24 24" {...s}><path d="M19 21l-7-5-7 5V5a2 2 0 012-2h10a2 2 0 012 2z"/></svg>;
}
export function IconFilter() {
  return <svg width={16} height={16} viewBox="0 0 24 24" {...s}><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>;
}
export function IconSmile() {
  return <svg width={20} height={20} viewBox="0 0 24 24" {...s}><circle cx="12" cy="12" r="10"/><path d="M8 14s1.5 2 4 2 4-2 4-2"/><line x1="9" y1="9" x2="9.01" y2="9"/><line x1="15" y1="9" x2="15.01" y2="9"/></svg>;
}

/* Nav rail icons */
export function IconChat() {
  return <svg width={22} height={22} viewBox="0 0 24 24" {...s}><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>;
}
export function IconSettings() {
  return <svg width={22} height={22} viewBox="0 0 24 24" {...s}><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 010-2.83 2 2 0 012.83 0l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 0 2 2 0 010 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>;
}
export function IconCompass({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10"/>
      <polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76" fill="white" stroke="none" opacity="0.9"/>
    </svg>
  );
}
export function IconMessageCircle() {
  return (
    <svg width={56} height={56} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1} strokeLinecap="round" strokeLinejoin="round" className="text-txt-placeholder/40">
      <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/>
    </svg>
  );
}
export function IconChevronUp({ size = 20 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><polyline points="18 15 12 9 6 15"/></svg>;
}
export function IconChevronDown({ size = 20 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><polyline points="6 9 12 15 18 9"/></svg>;
}

/* ─── Format helpers ─── */
export function formatTime(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  if (d.toDateString() === now.toDateString()) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  const y = new Date(now); y.setDate(y.getDate() - 1);
  if (d.toDateString() === y.toDateString()) return 'Вчера';
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' });
}

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' КБ';
  return (bytes / (1024 * 1024)).toFixed(1) + ' МБ';
}
