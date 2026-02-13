import { useState, useEffect, useLayoutEffect, useRef, useCallback, useMemo } from 'react';
import { useAuthStore, useChatStore } from '../store';
import { Avatar, Modal, IconSend, IconPaperclip, IconMicrophone, IconCheck, IconCheckDouble, IconFile, IconDownload, IconReply, IconEdit, IconTrash, IconPin, IconForward, IconX, IconBack, IconSearch, IconStarOutline, IconStarFilled, IconSmile, IconChevronUp, IconChevronDown, TypingDots, formatTime, formatFileSize, IconPlay, IconPause, IconVolume } from './ui';
import UserCard from './UserCard';
import type { Message, ChatWithLastMessage } from '../types';

/** Для отображения и скачивания: "+" в имени часто приходит вместо пробела (URL-кодирование). */
function normalizeFileDisplayName(name: string | undefined): string {
  return name ? name.replace(/\+/g, ' ').trim() : '';
}

/** Парсит системное сообщение и возвращает сегменты: текст и имена для клика по профилю. */
function parseSystemMessageContent(content: string): { prefix: string; names: string[]; suffix: string; isGroupAdd?: boolean } {
  const groupMatch = content.match(/^(.+?) добавил\(а\) (.+?) в группу$/);
  if (groupMatch) {
    return { prefix: '', names: [groupMatch[1].trim(), groupMatch[2].trim()], suffix: '', isGroupAdd: true };
  }
  const addPrefix = 'Добавлен пользователь ';
  if (content.startsWith(addPrefix)) {
    const name = content.slice(addPrefix.length).trim();
    return { prefix: addPrefix, names: name ? [name] : [], suffix: '' };
  }
  const removedPrefix = 'Удалён пользователь ';
  if (content.startsWith(removedPrefix)) {
    const name = content.slice(removedPrefix.length).trim();
    return { prefix: removedPrefix, names: name ? [name] : [], suffix: '' };
  }
  return { prefix: content, names: [], suffix: '' };
}

/** Рендерит системное сообщение с кликабельными именами (открытие профиля). */
function SystemMessageContent({
  content,
  members,
  onUserClick,
}: {
  content: string;
  members: { id: string; username: string }[];
  onUserClick: (userId: string) => void;
}) {
  const { prefix, names, suffix, isGroupAdd } = parseSystemMessageContent(content);
  if (names.length === 0) {
    return <>{content}</>;
  }
  if (isGroupAdd && names.length === 2) {
    const [actorName, addedName] = names;
    const actor = members.find((m) => m.username === actorName);
    const added = members.find((m) => m.username === addedName);
    return (
      <>
        {actor ? (
          <button type="button" className="font-medium hover:underline focus:outline-none focus:underline" onClick={() => onUserClick(actor.id)}>
            {actorName}
          </button>
        ) : (
          actorName
        )}
        {' добавил(а) '}
        {added ? (
          <button type="button" className="font-medium hover:underline focus:outline-none focus:underline" onClick={() => onUserClick(added.id)}>
            {addedName}
          </button>
        ) : (
          addedName
        )}
        {' в группу'}
      </>
    );
  }
  const name = names[0];
  const member = members.find((m) => m.username === name);
  return (
    <>
      {prefix}
      {member ? (
        <button type="button" className="font-medium hover:underline focus:outline-none focus:underline" onClick={() => onUserClick(member.id)}>
          {name}
        </button>
      ) : (
        name
      )}
      {suffix}
    </>
  );
}

const EMOJI_CATEGORIES: { label: string; emojis: string[] }[] = [
  { label: 'Часто', emojis: ['👍', '❤️', '😂', '😮', '😢', '🔥', '👎', '🎉'] },
  { label: 'Лица', emojis: ['😀', '😃', '😄', '😁', '😆', '🤣', '😅', '😊', '😇', '🙂', '😉', '😌', '😍', '🥰', '😘', '😗', '🤗', '🤔', '🤫', '😶', '😏', '😒', '🙄', '😬', '🤥', '😌', '😴', '🤒', '🤮', '🥵', '🥶', '😱', '😡', '🤬'] },
  { label: 'Жесты', emojis: ['👋', '🤚', '🖐️', '✋', '🤙', '👌', '🤏', '✌️', '🤞', '🤟', '🤘', '🤙', '👈', '👉', '👆', '👇', '☝️', '👍', '👎', '👊', '✊', '🤛', '🤜', '👏', '🙌', '🤝', '🙏'] },
  { label: 'Символы', emojis: ['❤️', '🧡', '💛', '💚', '💙', '💜', '🖤', '🤍', '💯', '💥', '💫', '⭐', '🌟', '✨', '⚡', '🔥', '💧', '🌊', '🎵', '🎶', '✅', '❌', '⚠️', '🚫', '💡', '🔔', '📌', '📎'] },
  { label: 'Еда', emojis: ['🍎', '🍐', '🍊', '🍋', '🍌', '🍉', '🍇', '🍓', '🫐', '🍒', '🍑', '🥭', '🍍', '🥥', '🥝', '🍅', '🥑', '🌽', '🌶️', '🍕', '🍔', '🍟', '🌮', '🍣', '🍱', '☕', '🍺', '🍷'] },
];

const RECORDING_WAVE = [6, 10, 14, 9, 16, 12, 8, 14, 10, 6];
const VOICE_WAVE = [4, 8, 6, 10, 7, 12, 9, 5, 8, 6, 10, 7, 12, 9, 5, 8, 6, 10, 7, 12, 9, 5];

function IconLock({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path d="M7.5 10V8.6C7.5 6.1 9.5 4 12 4s4.5 2.1 4.5 4.6V10" stroke="currentColor" strokeWidth="2" strokeLinecap="round"/>
      <path d="M7 10h10a2 2 0 0 1 2 2v6.5A2.5 2.5 0 0 1 16.5 21h-9A2.5 2.5 0 0 1 5 18.5V12a2 2 0 0 1 2-2Z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round"/>
      <path d="M12 14v3" stroke="currentColor" strokeWidth="2" strokeLinecap="round"/>
    </svg>
  );
}

function formatAudioTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0:00';
  const s = Math.floor(seconds);
  const m = Math.floor(s / 60);
  const r = s % 60;
  return `${m}:${String(r).padStart(2, '0')}`;
}

function mentionTokenFromUsername(username: string): string {
  return username.trim().replace(/\s+/g, '_');
}

function getMentionQueryContext(value: string, caret: number): { start: number; query: string } | null {
  const left = value.slice(0, caret);
  const at = left.lastIndexOf('@');
  if (at < 0) return null;
  const prev = at > 0 ? left[at - 1] : ' ';
  if (prev && !/\s|[([{>"'`]/.test(prev)) return null;
  const query = left.slice(at + 1);
  if (query.includes(' ') || query.includes('\n') || query.includes('\t')) return null;
  return { start: at, query };
}

interface ChatProps { onBack: () => void; onOpenInfo?: () => void; onOpenSearch?: () => void; onOpenProfile?: () => void; }

export default function Chat({ onBack, onOpenInfo, onOpenSearch, onOpenProfile }: ChatProps) {
  const { user } = useAuthStore();
  const {
    activeChatId, chats, messages, typingUsers, onlineUsers, pinnedMessages,
    favoriteChatIds, toggleFavorite,
    sendMessage, sendTyping, uploadFile, uploadVoice,
    addOptimisticVoiceMessage, removeOptimisticMessage, updateOptimisticVoiceMessage, sendMessageWsOnly,
    replyTo, editingMessage,
    setReplyTo, setEditingMessage, editMessage, deleteMessage,
    addReaction, pinMessage, unpinMessage, setActiveChat,
  } = useChatStore();

  const chat = useMemo(() => chats.find((c) => c.chat.id === activeChatId), [chats, activeChatId]);
  const chatMessages = activeChatId ? messages[activeChatId] || [] : [];
  const typing = activeChatId ? typingUsers[activeChatId] || [] : [];
  const pinned = activeChatId ? pinnedMessages[activeChatId] || [] : [];
  const wsConnected = useChatStore((s) => s.ws?.readyState === WebSocket.OPEN);

  const [text, setText] = useState('');
  const [uploading, setUploading] = useState(false);
  const [recording, setRecording] = useState(false);
  const [recordingLocked, setRecordingLocked] = useState(false);
  const [recordingCancelArmed, setRecordingCancelArmed] = useState(false);
  const [recordingLockArmed, setRecordingLockArmed] = useState(false);
  const [recordingSec, setRecordingSec] = useState(0);
  const [voiceError, setVoiceError] = useState<string | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const recordedChunksRef = useRef<Blob[]>([]);
  const recordingTimerRef = useRef<ReturnType<typeof setInterval>>();
  const recordingChatIdRef = useRef<string | null>(null);
  const recordingCancelledRef = useRef(false);
  const recordingPointerIdRef = useRef<number | null>(null);
  const recordingStartPosRef = useRef<{ x: number; y: number } | null>(null);
  const recordingLastPosRef = useRef<{ x: number; y: number } | null>(null);
  const suppressMicClickRef = useRef(false);
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; msg: Message } | null>(null);
  const [forwardMsg, setForwardMsg] = useState<Message | null>(null);
  const [emojiPickerFor, setEmojiPickerFor] = useState<string | null>(null);
  const [showInputEmojiPicker, setShowInputEmojiPicker] = useState(false);
  const [mentionOpen, setMentionOpen] = useState(false);
  const [mentionQuery, setMentionQuery] = useState('');
  const [mentionStart, setMentionStart] = useState(-1);
  const [mentionActiveIndex, setMentionActiveIndex] = useState(0);
  const [userCardId, setUserCardId] = useState<string | null>(null);
  const [inChatSearchOpen, setInChatSearchOpen] = useState(false);
  const [inChatSearchQuery, setInChatSearchQuery] = useState('');
  const [inChatSearchIndex, setInChatSearchIndex] = useState(0);
  const inChatSearchInputRef = useRef<HTMLInputElement>(null);

  const endRef = useRef<HTMLDivElement>(null);
  const messagesScrollRef = useRef<HTMLDivElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const textRef = useRef<HTMLTextAreaElement>(null);
  const inputEmojiPickerRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLDivElement>(null);
  const typingT = useRef<ReturnType<typeof setTimeout>>();
  const prevLen = useRef(0);
  const prevChatIdRef = useRef<string | null>(null);
  const didInitialScrollForChatRef = useRef<string | null>(null);
  const stickToBottomRef = useRef(true);
  const keyboardSettlingUntilRef = useRef(0);
  const [highlightMsgId, setHighlightMsgId] = useState<string | null>(null);
  const BOTTOM_STICK_THRESHOLD = 72;

  const isScrollNearBottom = useCallback((el: HTMLDivElement | null): boolean => {
    if (!el) return true;
    return el.scrollHeight - (el.scrollTop + el.clientHeight) <= BOTTOM_STICK_THRESHOLD;
  }, []);

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'auto') => {
    const el = messagesScrollRef.current;
    if (!el) {
      endRef.current?.scrollIntoView({ behavior, block: 'end' });
      return;
    }
    if (behavior === 'smooth') {
      endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
      return;
    }
    const prevBehavior = el.style.scrollBehavior;
    el.style.scrollBehavior = 'auto';
    el.scrollTop = el.scrollHeight;
    el.style.scrollBehavior = prevBehavior;
  }, []);

  const stabilizeBottom = useCallback(() => {
    if (!stickToBottomRef.current && Date.now() >= keyboardSettlingUntilRef.current) return;
    scrollToBottom('auto');
    requestAnimationFrame(() => {
      if (stickToBottomRef.current || Date.now() < keyboardSettlingUntilRef.current) scrollToBottom('auto');
    });
    setTimeout(() => {
      if (stickToBottomRef.current || Date.now() < keyboardSettlingUntilRef.current) scrollToBottom('auto');
    }, 120);
  }, [scrollToBottom]);

  const isKeyboardSettling = useCallback((): boolean => {
    return Date.now() < keyboardSettlingUntilRef.current;
  }, []);

  const mentionCandidates = useMemo(() => {
    const members = chat?.members || [];
    const q = mentionQuery.trim().toLowerCase();
    if (!q) return members.slice(0, 8);
    return members
      .filter((m) => {
        const uname = m.username.toLowerCase();
        const token = mentionTokenFromUsername(m.username).toLowerCase();
        return uname.includes(q) || token.includes(q);
      })
      .slice(0, 8);
  }, [chat?.members, mentionQuery]);

  // При смене чата показываем пустое поле/ответ, чтобы не было вспышки старого контента внизу
  const chatIdChanged = prevChatIdRef.current !== activeChatId;
  if (chatIdChanged && activeChatId) {
    prevChatIdRef.current = activeChatId;
  } else if (!activeChatId) {
    prevChatIdRef.current = null;
  }
  const displayText = chatIdChanged ? '' : text;
  const displayForwardMsg = chatIdChanged ? null : forwardMsg;
  const displayReplyTo = chatIdChanged ? null : replyTo;
  const displayEditingMessage = chatIdChanged ? null : editingMessage;

  // При открытии чата — сразу показываем конец без какой-либо видимой прокрутки (как в Telegram)
  useLayoutEffect(() => {
    if (!activeChatId || chatMessages.length === 0) return;
    if (didInitialScrollForChatRef.current === activeChatId) return;
    didInitialScrollForChatRef.current = activeChatId;
    stickToBottomRef.current = true;
    prevLen.current = chatMessages.length;
    scrollToBottom('auto');
  }, [activeChatId, chatMessages.length, scrollToBottom]);

  // Сброс «начального скролла» при смене чата, чтобы новый чат открылся сразу с конца
  useEffect(() => {
    didInitialScrollForChatRef.current = null;
    stickToBottomRef.current = true;
  }, [activeChatId]);

  // Telegram-style sticky bottom: если пользователь у нижнего края, новые сообщения удерживают чат внизу.
  useEffect(() => {
    const el = messagesScrollRef.current;
    if (!el) return;
    const onScroll = () => {
      if (isKeyboardSettling()) {
        stickToBottomRef.current = true;
        return;
      }
      stickToBottomRef.current = isScrollNearBottom(el);
    };
    onScroll();
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => el.removeEventListener('scroll', onScroll);
  }, [activeChatId, isKeyboardSettling, isScrollNearBottom]);

  // Плавный скролл вниз только при новом сообщении в уже открытом чате (не при первой загрузке)
  useEffect(() => {
    const prevCount = prevLen.current;
    if (chatMessages.length <= prevCount) {
      prevLen.current = chatMessages.length;
      return;
    }
    const isInitialLoad = prevCount === 0;
    prevLen.current = chatMessages.length;
    const last = chatMessages[chatMessages.length - 1];
    const lastFromMe = !!last && last.sender_id === user?.id;
    if (!(isInitialLoad || stickToBottomRef.current || lastFromMe)) return;

    requestAnimationFrame(() => {
      scrollToBottom(isInitialLoad ? 'auto' : 'smooth');
      // Добиваемся точного прилипания после перерасчёта высот (изображения/аватар/voice metadata).
      setTimeout(() => scrollToBottom('auto'), 120);
    });
  }, [chatMessages, scrollToBottom, user?.id]);

  // На изменениях размеров (клавиатура, safe-area, композер) сохраняем прилипание внизу.
  useLayoutEffect(() => {
    const scrollEl = messagesScrollRef.current;
    if (!scrollEl || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(() => {
      if (stickToBottomRef.current || isKeyboardSettling()) stabilizeBottom();
    });
    observer.observe(scrollEl);
    if (composerRef.current) observer.observe(composerRef.current);
    const content = scrollEl.firstElementChild;
    if (content) observer.observe(content);
    return () => observer.disconnect();
  }, [activeChatId, isKeyboardSettling, stabilizeBottom]);

  // Dynamic bottom offset for different platforms: iOS/Android/PWA/Desktop.
  // Keeps last messages visible above composer regardless of keyboard/safe-area differences.
  useLayoutEffect(() => {
    const scrollEl = messagesScrollRef.current;
    const composerEl = composerRef.current;
    if (!scrollEl || !composerEl) return;

    const applyComposerHeight = () => {
      const h = Math.max(56, Math.ceil(composerEl.getBoundingClientRect().height));
      scrollEl.style.setProperty('--chat-composer-height', `${h}px`);
    };

    applyComposerHeight();

    let observer: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(() => applyComposerHeight());
      observer.observe(composerEl);
    }

    const onViewportResize = () => applyComposerHeight();
    window.addEventListener('resize', onViewportResize, { passive: true });
    if (window.visualViewport) {
      window.visualViewport.addEventListener('resize', onViewportResize, { passive: true });
    }

    return () => {
      observer?.disconnect();
      window.removeEventListener('resize', onViewportResize);
      if (window.visualViewport) {
        window.visualViewport.removeEventListener('resize', onViewportResize);
      }
      scrollEl.style.removeProperty('--chat-composer-height');
    };
  }, [activeChatId]);

  useEffect(() => {
    const onViewportResize = () => {
      if (stickToBottomRef.current || isKeyboardSettling()) stabilizeBottom();
    };
    window.addEventListener('resize', onViewportResize, { passive: true });
    if (window.visualViewport) {
      window.visualViewport.addEventListener('resize', onViewportResize, { passive: true });
    }
    return () => {
      window.removeEventListener('resize', onViewportResize);
      if (window.visualViewport) {
        window.visualViewport.removeEventListener('resize', onViewportResize);
      }
    };
  }, [isKeyboardSettling, stabilizeBottom]);

  // Сброс локального состояния при смене чата, чтобы не было вспышки старого контента (ответ/перес send/поле ввода)
  useEffect(() => {
    setText('');
    setForwardMsg(null);
    setCtxMenu(null);
    setEmojiPickerFor(null);
    setShowInputEmojiPicker(false);
    setMentionOpen(false);
    setMentionQuery('');
    setMentionStart(-1);
    setMentionActiveIndex(0);
    setUserCardId(null);
    setInChatSearchOpen(false);
    setInChatSearchQuery('');
    setHighlightMsgId(null);
    setVoiceError(null);
  }, [activeChatId]);

  const scrollToMessage = useCallback((msgId: string) => {
    const el = document.getElementById(`msg-${msgId}`);
    if (!el) return;
    el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    setHighlightMsgId(msgId);
    setTimeout(() => setHighlightMsgId(null), 2000);
  }, []);

  const insertEmojiAtCursor = useCallback((emoji: string) => {
    const ta = textRef.current;
    if (!ta) return;
    const start = ta.selectionStart ?? text.length;
    const end = ta.selectionEnd ?? text.length;
    const newText = text.slice(0, start) + emoji + text.slice(end);
    setText(newText);
    setShowInputEmojiPicker(false);
    requestAnimationFrame(() => {
      const pos = start + emoji.length;
      ta.focus();
      ta.setSelectionRange(pos, pos);
    });
  }, [text]);

  const updateMentionState = useCallback((value: string, caret: number) => {
    const ctx = getMentionQueryContext(value, caret);
    if (!ctx) {
      setMentionOpen(false);
      setMentionQuery('');
      setMentionStart(-1);
      setMentionActiveIndex(0);
      return;
    }
    setMentionStart(ctx.start);
    setMentionQuery(ctx.query);
    setMentionOpen(true);
    setMentionActiveIndex(0);
  }, []);

  const applyMention = useCallback((username: string) => {
    if (mentionStart < 0) return;
    const ta = textRef.current;
    const caret = ta?.selectionStart ?? text.length;
    const before = text.slice(0, mentionStart);
    const after = text.slice(caret);
    const inserted = `@${mentionTokenFromUsername(username)} `;
    const next = before + inserted + after;
    setText(next);
    setMentionOpen(false);
    setMentionQuery('');
    setMentionStart(-1);
    setMentionActiveIndex(0);
    requestAnimationFrame(() => {
      const pos = before.length + inserted.length;
      ta?.focus();
      ta?.setSelectionRange(pos, pos);
    });
  }, [mentionStart, text]);

  useEffect(() => {
    if (!showInputEmojiPicker) return;
    const onMouseDown = (e: MouseEvent) => {
      const el = inputEmojiPickerRef.current;
      if (el && !el.contains(e.target as Node)) setShowInputEmojiPicker(false);
    };
    document.addEventListener('mousedown', onMouseDown);
    return () => document.removeEventListener('mousedown', onMouseDown);
  }, [showInputEmojiPicker]);

  // Поиск только по текущему чату: совпадения по тексту сообщений (уже загруженных)
  const inChatSearchResultIds = useMemo(() => {
    if (!inChatSearchQuery.trim()) return [];
    const q = inChatSearchQuery.toLowerCase().trim();
    return chatMessages
      .filter((m) => !m.is_deleted && m.content_type === 'text' && m.content.toLowerCase().includes(q))
      .map((m) => m.id);
  }, [chatMessages, inChatSearchQuery]);

  const goToSearchResult = useCallback((index: number) => {
    const id = inChatSearchResultIds[index];
    if (id) scrollToMessage(id);
    setInChatSearchIndex(index);
  }, [inChatSearchResultIds, scrollToMessage]);

  useEffect(() => {
    if (inChatSearchOpen) inChatSearchInputRef.current?.focus();
  }, [inChatSearchOpen]);

  useEffect(() => {
    setInChatSearchIndex(0);
  }, [inChatSearchQuery]);

  useEffect(() => {
    if (inChatSearchResultIds.length > 0 && inChatSearchIndex >= 0 && inChatSearchIndex < inChatSearchResultIds.length) {
      scrollToMessage(inChatSearchResultIds[inChatSearchIndex]);
    }
  }, [inChatSearchResultIds, inChatSearchIndex, scrollToMessage]);

  const handleInChatSearchPrev = useCallback(() => {
    if (inChatSearchResultIds.length === 0) return;
    const next = inChatSearchIndex <= 0 ? inChatSearchResultIds.length - 1 : inChatSearchIndex - 1;
    setInChatSearchIndex(next);
    scrollToMessage(inChatSearchResultIds[next]);
  }, [inChatSearchResultIds, inChatSearchIndex, scrollToMessage]);

  const handleInChatSearchNext = useCallback(() => {
    if (inChatSearchResultIds.length === 0) return;
    const next = inChatSearchIndex >= inChatSearchResultIds.length - 1 ? 0 : inChatSearchIndex + 1;
    setInChatSearchIndex(next);
    scrollToMessage(inChatSearchResultIds[next]);
  }, [inChatSearchResultIds, inChatSearchIndex, scrollToMessage]);

  // Populate on edit
  useEffect(() => { if (editingMessage) setText(editingMessage.content); }, [editingMessage]);

  // Auto-focus on reply/edit
  useEffect(() => {
    if (replyTo || editingMessage) requestAnimationFrame(() => textRef.current?.focus());
  }, [replyTo, editingMessage]);

  // Close context menu on any click
  useEffect(() => {
    const h = () => setCtxMenu(null);
    document.addEventListener('click', h);
    return () => document.removeEventListener('click', h);
  }, []);

  const handleSend = useCallback(() => {
    if (!text.trim() || !activeChatId) return;
    if (editingMessage) { editMessage(editingMessage.id, text.trim()); setText(''); return; }
    sendMessage(activeChatId, text.trim());
    setText('');
  }, [text, activeChatId, sendMessage, editMessage, editingMessage]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (mentionOpen && mentionCandidates.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setMentionActiveIndex((i) => (i + 1) % mentionCandidates.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setMentionActiveIndex((i) => (i <= 0 ? mentionCandidates.length - 1 : i - 1));
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        applyMention(mentionCandidates[mentionActiveIndex].username);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionOpen(false);
        return;
      }
    }
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
    if (e.key === 'Escape') { setReplyTo(null); setEditingMessage(null); setText(''); }
  }, [mentionOpen, mentionCandidates, mentionActiveIndex, applyMention, handleSend, setReplyTo, setEditingMessage]);

  const lastTypingSentRef = useRef(0);
  const TYPING_SEND_INTERVAL_MS = 1500;
  const handleTyping = useCallback(() => {
    if (!activeChatId) return;
    if (typingT.current) clearTimeout(typingT.current);
    const now = Date.now();
    if (now - lastTypingSentRef.current < TYPING_SEND_INTERVAL_MS) {
      typingT.current = setTimeout(() => {}, 3000);
      return;
    }
    lastTypingSentRef.current = now;
    sendTyping(activeChatId);
    typingT.current = setTimeout(() => {}, 3000);
  }, [activeChatId, sendTyping]);

  const handleFile = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !activeChatId) return;
    setUploading(true);
    try {
      const r = await uploadFile(file);
      const displayName = normalizeFileDisplayName(r.file_name) || file.name.replace(/\+/g, ' ').trim() || file.name;
      sendMessage(activeChatId, file.name, { contentType: r.content_type, fileUrl: r.url, fileName: displayName, fileSize: r.file_size });
    } catch { /* */ }
    setUploading(false);
    if (fileRef.current) fileRef.current.value = '';
  }, [activeChatId, uploadFile, sendMessage]);

  const startRecording = useCallback(async () => {
    if (!activeChatId || !navigator.mediaDevices?.getUserMedia) return;
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const mime = MediaRecorder.isTypeSupported('audio/webm;codecs=opus') ? 'audio/webm;codecs=opus' : 'audio/webm';
      const recorder = new MediaRecorder(stream);
      recordingCancelledRef.current = false;
      recordedChunksRef.current = [];
      recordingChatIdRef.current = activeChatId;
      recorder.ondataavailable = (e) => { if (e.data.size > 0) recordedChunksRef.current.push(e.data); };
      recorder.onstop = async () => {
        stream.getTracks().forEach((t) => t.stop());
        const wasCancelled = recordingCancelledRef.current;
        recordingCancelledRef.current = false;
        const chatId = recordingChatIdRef.current;
        recordingChatIdRef.current = null;
        if (wasCancelled) {
          recordedChunksRef.current = [];
          setUploading(false);
          return;
        }
        const blob = new Blob(recordedChunksRef.current, { type: mime });
        try {
          if (blob.size === 0 || !chatId) return;
          const ext = mime.includes('webm') ? '.webm' : '.ogg';
          const file = new File([blob], `voice-${Date.now()}${ext}`, { type: mime.split(';')[0] });
          const { optId, clientMsgId } = addOptimisticVoiceMessage(chatId);
          try {
            const r = await uploadVoice(file);
            updateOptimisticVoiceMessage(chatId, optId, { fileUrl: r.url, fileName: r.file_name || 'voice', fileSize: r.file_size });
            sendMessageWsOnly(chatId, 'Голосовое сообщение', { contentType: 'voice', fileUrl: r.url, fileName: r.file_name || 'voice', fileSize: r.file_size, clientMsgId });
          } catch (e: unknown) {
            removeOptimisticMessage(chatId, optId);
            const msg = e instanceof Error ? e.message : 'Не удалось отправить голосовое';
            setVoiceError(msg);
            console.error('uploadVoice failed:', e);
          }
        } finally {
          setUploading(false);
        }
      };
      mediaRecorderRef.current = recorder;
      recorder.start(100);
      setRecording(true);
      setRecordingSec(0);
      recordingTimerRef.current = setInterval(() => setRecordingSec((s) => s + 1), 1000);
    } catch { /* */ }
  }, [activeChatId, uploadVoice, addOptimisticVoiceMessage, updateOptimisticVoiceMessage, sendMessageWsOnly, removeOptimisticMessage]);

  const stopRecording = useCallback((send = true) => {
    if (recordingTimerRef.current) clearInterval(recordingTimerRef.current);
    setRecording(false);
    setRecordingLocked(false);
    setRecordingCancelArmed(false);
    setRecordingLockArmed(false);
    setRecordingSec(0);
    recordingPointerIdRef.current = null;
    recordingStartPosRef.current = null;
    recordingLastPosRef.current = null;
    const recorder = mediaRecorderRef.current;
    mediaRecorderRef.current = null;
    if (recorder?.state === 'recording') {
      recordingCancelledRef.current = !send;
      if (send) setUploading(true);
      try {
        recorder.requestData();
      } catch { /* */ }
      recorder.stop();
    }
  }, []);

  const cancelRecording = useCallback(() => {
    stopRecording(false);
  }, [stopRecording]);

  const RECORD_CANCEL_PX = 90;
  const RECORD_LOCK_ARM_PX = 80;
  const RECORD_LOCK_COMMIT_PX = 130;

  const handleMicPointerDown = useCallback(async (e: React.PointerEvent<HTMLButtonElement>) => {
    if (uploading) return;
    if (recording) return;
    if (e.button != null && e.button !== 0) return;
    e.preventDefault();
    e.stopPropagation();
    setShowInputEmojiPicker(false);
    setMentionOpen(false);
    setRecordingLocked(false);
    setRecordingCancelArmed(false);
    setRecordingLockArmed(false);
    recordingPointerIdRef.current = e.pointerId;
    recordingStartPosRef.current = { x: e.clientX, y: e.clientY };
    recordingLastPosRef.current = { x: e.clientX, y: e.clientY };
    try { e.currentTarget.setPointerCapture(e.pointerId); } catch { /* */ }
    await startRecording();
  }, [uploading, recording, startRecording]);

  const handleMicPointerMove = useCallback((e: React.PointerEvent<HTMLButtonElement>) => {
    if (!recording) return;
    if (recordingLocked) return;
    if (recordingPointerIdRef.current !== e.pointerId) return;
    const start = recordingStartPosRef.current;
    if (!start) return;
    recordingLastPosRef.current = { x: e.clientX, y: e.clientY };
    const dx = e.clientX - start.x;
    const dy = e.clientY - start.y;
    const cancelArmed = dx < -RECORD_CANCEL_PX;
    const lockArmed = dy < -RECORD_LOCK_ARM_PX;
    if (cancelArmed !== recordingCancelArmed) setRecordingCancelArmed(cancelArmed);
    if (lockArmed !== recordingLockArmed) setRecordingLockArmed(lockArmed);
    if (dy < -RECORD_LOCK_COMMIT_PX) {
      setRecordingLocked(true);
      setRecordingLockArmed(false);
      try { (navigator as any)?.vibrate?.(10); } catch { /* */ }
    }
  }, [recording, recordingLocked, recordingCancelArmed, recordingLockArmed]);

  const handleMicPointerUp = useCallback((e: React.PointerEvent<HTMLButtonElement>) => {
    if (recordingPointerIdRef.current !== e.pointerId) return;
    recordingLastPosRef.current = { x: e.clientX, y: e.clientY };
    try { e.currentTarget.releasePointerCapture(e.pointerId); } catch { /* */ }
    // Prevent "click" after a hold gesture.
    suppressMicClickRef.current = true;
    setTimeout(() => { suppressMicClickRef.current = false; }, 0);

    if (!recording) return;
    if (recordingLocked) return; // locked mode: user sends/cancels via buttons

    const last = recordingLastPosRef.current;
    const rect = composerRef.current?.getBoundingClientRect();
    const releasedOutside = !!(last && rect && (
      last.x < rect.left || last.x > rect.right || last.y < rect.top || last.y > rect.bottom
    ));

    const shouldCancel = recordingCancelArmed || releasedOutside;
    stopRecording(!shouldCancel);
  }, [recording, recordingLocked, recordingCancelArmed, stopRecording]);

  const handleMicPointerCancel = useCallback((e: React.PointerEvent<HTMLButtonElement>) => {
    if (recordingPointerIdRef.current !== e.pointerId) return;
    try { e.currentTarget.releasePointerCapture(e.pointerId); } catch { /* */ }
    suppressMicClickRef.current = true;
    setTimeout(() => { suppressMicClickRef.current = false; }, 0);
    if (recording && !recordingLocked) stopRecording(false);
  }, [recording, recordingLocked, stopRecording]);

  useEffect(() => {
    return () => { if (recordingTimerRef.current) clearInterval(recordingTimerRef.current); };
  }, []);

  useEffect(() => {
    if (!recording) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        stopRecording(true);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        cancelRecording();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [recording, cancelRecording, stopRecording]);

  useEffect(() => {
    if (!voiceError) return;
    const t = setTimeout(() => setVoiceError(null), 5000);
    return () => clearTimeout(t);
  }, [voiceError]);

  const onCtx = useCallback((e: React.MouseEvent, msg: Message) => {
    e.preventDefault(); e.stopPropagation();
    setCtxMenu({ x: e.clientX, y: e.clientY, msg });
  }, []);

  const handleForward = useCallback((targetChatId: string) => {
    if (!forwardMsg) return;
    const fwdContent = forwardMsg.content_type === 'text'
      ? `⤷ ${forwardMsg.sender?.username || 'Пользователь'}:\n${forwardMsg.content}`
      : forwardMsg.content;
    sendMessage(targetChatId, fwdContent, {
      contentType: forwardMsg.content_type,
      fileUrl: forwardMsg.file_url,
      fileName: normalizeFileDisplayName(forwardMsg.file_name) || forwardMsg.file_name,
      fileSize: forwardMsg.file_size,
    });
    setForwardMsg(null);
  }, [forwardMsg, sendMessage]);

  if (!activeChatId) return null;
  if (!chat) {
    return (
      <div className="h-full flex flex-col items-center justify-center bg-white dark:bg-dark-bg min-w-0">
        <div className="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin mb-2" />
        <p className="text-[13px] text-txt-secondary dark:text-[#8b98a5]">Загрузка чата...</p>
      </div>
    );
  }

  const chatName = getName(chat, user?.id || '');
  const chatOnline = getOnline(chat, user?.id || '', onlineUsers);
  const typNames = typing.map((uid) => chat.members.find((m) => m.id === uid)?.username).filter(Boolean);
  const hasTypingInHeader = typNames.length > 0;
  const [showTypingInHeader, setShowTypingInHeader] = useState(hasTypingInHeader);
  useEffect(() => {
    if (hasTypingInHeader) setShowTypingInHeader(true);
    else {
      const id = setTimeout(() => setShowTypingInHeader(false), 1200);
      return () => clearTimeout(id);
    }
  }, [hasTypingInHeader]);

  return (
    <div className="h-[var(--app-height)] md:h-full min-h-0 flex flex-col bg-white dark:bg-dark-bg safe-x min-w-0 overflow-hidden overscroll-none" onClick={() => setCtxMenu(null)}>
      {/* ── Header ── */}
      <div className="shrink-0 sticky top-0 z-20 flex items-center gap-2.5 sm:gap-3 px-3 sm:px-4 py-1.5 sm:py-2.5 pt-[max(0.25rem,env(safe-area-inset-top))] sm:pt-[max(0.625rem,env(safe-area-inset-top))] border-b border-surface-border dark:border-dark-border bg-white dark:bg-dark-bg min-w-0 overflow-hidden">
        {onOpenProfile && (
          <button type="button" onClick={(e) => { e.stopPropagation(); onOpenProfile(); }} className="md:hidden p-0.5 rounded-full hover:bg-surface dark:hover:bg-dark-elevated transition-colors shrink-0" title="Профиль" aria-label="Профиль">
            <Avatar name={user?.username ?? ''} url={user?.avatar_url} size={32} />
          </button>
        )}
        <button onClick={(e) => { e.stopPropagation(); onBack(); }} className="md:hidden p-1 rounded-compass hover:bg-surface dark:hover:bg-dark-elevated transition-colors">
          <IconBack />
        </button>
        <div onClick={(e) => {
          if (chat.chat.chat_type === 'personal') {
            e.stopPropagation();
            const other = chat.members.find((m) => m.id !== user?.id);
            if (other) setUserCardId(other.id);
            return;
          }
          if (chat.chat.chat_type === 'group') {
            e.stopPropagation();
            onOpenInfo?.();
          }
        }}>
          <Avatar name={chatName} url={chat.chat.avatar_url || undefined} size={34} online={chatOnline} className="sm:!w-10 sm:!h-10" />
        </div>
        <div className="flex-1 min-w-0" onClick={(e) => {
          if (chat.chat.chat_type === 'personal') {
            e.stopPropagation();
            const other = chat.members.find((m) => m.id !== user?.id);
            if (other) setUserCardId(other.id);
            return;
          }
          if (chat.chat.chat_type === 'group') {
            e.stopPropagation();
            onOpenInfo?.();
          }
        }}>
          <h2 className="text-[13px] sm:text-[14px] font-semibold text-txt dark:text-[#e7e9ea] truncate leading-tight">{chatName}</h2>
          <p className="text-[11px] sm:text-[12px] text-txt-secondary dark:text-[#8b98a5] leading-tight mt-0.5 transition-opacity duration-200">
            {showTypingInHeader ? (
              <span className="text-primary inline-flex items-center gap-1">{typNames.length > 0 ? typNames.join(', ') : chatName} печатает <TypingDots /></span>
            ) : chatOnline !== undefined ? (
              chatOnline ? <span className="text-green">В сети</span> : 'Не в сети'
            ) : `${chat.members.length} участников`}
          </p>
        </div>
        {activeChatId && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              toggleFavorite(activeChatId);
            }}
            className="p-1.5 sm:p-2 rounded-full hover:bg-surface dark:hover:bg-dark-elevated transition-all duration-200 ease-out text-txt-secondary hover:text-txt dark:text-dark-muted dark:hover:text-[#e7e9ea]"
            title={favoriteChatIds.includes(activeChatId) ? 'Убрать из избранного' : 'В избранное'}
          >
            {favoriteChatIds.includes(activeChatId) ? (
              <IconStarFilled size={20} className="text-[#ff8a00]" />
            ) : (
              <IconStarOutline size={20} className="text-txt-secondary dark:text-dark-muted" />
            )}
          </button>
        )}
        <button onClick={(e) => { e.stopPropagation(); setInChatSearchOpen(true); }}
          className="p-1.5 sm:p-2 rounded-full hover:bg-surface dark:hover:bg-dark-elevated transition-all duration-200 ease-out text-txt-secondary hover:text-txt dark:text-[#8b98a5] dark:hover:text-[#e7e9ea]" title="Поиск в чате">
          <IconSearch size={20} />
        </button>
      </div>

      {/* ── Поиск по чату (графа в данном чате) ── */}
      {inChatSearchOpen && (
        <div className="shrink-0 flex items-center gap-2 px-4 py-2.5 bg-surface dark:bg-dark-elevated border-b border-surface-border dark:border-dark-border">
          <button
            type="button"
            onClick={handleInChatSearchPrev}
            disabled={inChatSearchResultIds.length === 0}
            className="p-1.5 rounded-compass hover:bg-surface-light dark:hover:bg-dark-hover text-txt-secondary dark:text-[#8b98a5] disabled:opacity-30 disabled:pointer-events-none transition-colors"
            title="Предыдущее совпадение"
          >
            <IconChevronUp size={20} />
          </button>
          <button
            type="button"
            onClick={handleInChatSearchNext}
            disabled={inChatSearchResultIds.length === 0}
            className="p-1.5 rounded-compass hover:bg-surface-light dark:hover:bg-dark-hover text-txt-secondary dark:text-[#8b98a5] disabled:opacity-30 disabled:pointer-events-none transition-colors"
            title="Следующее совпадение"
          >
            <IconChevronDown size={20} />
          </button>
          <div className="flex-1 relative flex items-center">
            <span className="absolute left-3 text-txt-placeholder dark:text-[#8b98a5] pointer-events-none">
              <IconSearch size={18} />
            </span>
            <input
              ref={inChatSearchInputRef}
              type="text"
              value={inChatSearchQuery}
              onChange={(e) => setInChatSearchQuery(e.target.value)}
              placeholder="Поиск"
              className="w-full pl-10 pr-9 py-2 bg-white dark:bg-dark-bg border border-surface-border dark:border-dark-border rounded-compass text-[14px] text-txt dark:text-[#e7e9ea] placeholder:text-txt-placeholder dark:placeholder:text-[#8b98a5] focus:border-primary/40 focus:ring-1 focus:ring-primary/20 outline-none transition-colors"
            />
            {inChatSearchQuery && (
              <button
                type="button"
                onClick={() => setInChatSearchQuery('')}
                className="absolute right-2 w-6 h-6 flex items-center justify-center rounded-full hover:bg-surface dark:hover:bg-dark-hover text-txt-placeholder hover:text-txt dark:text-[#8b98a5] transition-colors"
                title="Очистить"
              >
                <IconX size={14} />
              </button>
            )}
          </div>
          {inChatSearchResultIds.length > 0 && (
            <span className="text-[12px] text-txt-secondary dark:text-[#8b98a5] shrink-0 tabular-nums">
              {inChatSearchIndex + 1} из {inChatSearchResultIds.length}
            </span>
          )}
          <button
            type="button"
            onClick={() => { setInChatSearchOpen(false); setInChatSearchQuery(''); }}
            className="p-2 rounded-full hover:bg-surface-light dark:hover:bg-dark-hover text-txt-secondary hover:text-txt dark:text-[#8b98a5] transition-colors"
            title="Закрыть поиск"
          >
            <IconX size={18} />
          </button>
        </div>
      )}

      {/* ── Pinned bar ── */}
      {pinned.length > 0 && (
        <div
          className="shrink-0 px-4 py-2 bg-primary/5 dark:bg-primary/10 border-b border-primary/10 dark:border-primary/20 flex items-center gap-2 cursor-pointer hover:bg-primary/10 dark:hover:bg-primary/15 transition-colors"
          onClick={() => pinned[0]?.message_id && scrollToMessage(pinned[0].message_id)}
        >
          <IconPin />
          <span className="text-[12px] text-primary font-medium truncate flex-1">
            {pinned[0].message?.sender?.username}: {pinned[0].message?.content}
          </span>
          {pinned.length > 1 && <span className="text-[11px] text-primary/50">+{pinned.length - 1}</span>}
        </div>
      )}

      {/* ── Notes chat description ── */}
      {chat?.chat.chat_type === 'notes' && chat.chat.description && (
        <div className="shrink-0 px-4 py-4 bg-surface/50 dark:bg-dark-elevated/50 border-b border-surface-border dark:border-dark-border">
          <p className="text-[13px] text-txt dark:text-[#e7e9ea] whitespace-pre-line leading-relaxed">
            {chat.chat.description}
          </p>
        </div>
      )}

      {/* ── No connection banner ── */}
      {!wsConnected && (
        <div className="shrink-0 px-4 py-2 bg-danger/10 dark:bg-danger/20 border-b border-danger/30 flex items-center gap-2">
          <span className="text-[13px] text-danger font-medium">Нет соединения с сервером. Сообщения не отправляются.</span>
          <button type="button" onClick={() => useChatStore.getState().connectWS()} className="text-[12px] text-danger hover:underline shrink-0">Повторить</button>
        </div>
      )}

      {/* ── Messages ── */}
      <div ref={messagesScrollRef} className="chat-messages-scroll flex-1 min-h-0 min-w-0 overflow-y-auto overflow-x-hidden overscroll-y-contain px-2.5 sm:px-4 pt-2 pb-2 sm:pt-3 sm:pb-3 scroll-smooth touch-pan-y flex flex-col">
        <div className="min-h-0 flex-1 flex flex-col justify-end gap-0.5 pb-[var(--chat-composer-height,72px)] md:pb-0">
          {/* Spacer: забирает верхнее пространство, прижимает сообщения к низу (важно на мобильных). */}
          <div className="min-h-[1px] flex-1 shrink-0" aria-hidden />
          {chatMessages.map((msg, i) => {
            const prev = chatMessages[i - 1];
            const showDate = !prev || new Date(msg.created_at).toDateString() !== new Date(prev.created_at).toDateString();
            const isOwn = msg.sender_id === user?.id;
            const showAvatar = !isOwn && (!chatMessages[i + 1] || chatMessages[i + 1].sender_id !== msg.sender_id);

            return (
              <div key={msg.id} id={`msg-${msg.id}`} className={`min-w-0 overflow-hidden ${highlightMsgId === msg.id ? 'animate-msg-highlight rounded-compass' : ''}`}>
                {showDate && (
                  <div className="flex justify-center my-2.5 sm:my-3">
                    <span className="px-3 py-1 bg-surface dark:bg-dark-elevated rounded-full text-[11px] text-txt-secondary dark:text-[#8b98a5] font-medium">
                      {new Date(msg.created_at).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })}
                    </span>
                  </div>
                )}
                {msg.content_type === 'system' ? (
                  <div className="flex justify-center my-1.5 sm:my-2">
                    <span className="text-[12px] text-txt-secondary dark:text-[#8b98a5] bg-surface/90 dark:bg-dark-elevated px-3 py-1.5 rounded-full max-w-[85%] text-center">
                      <SystemMessageContent content={msg.content} members={chat.members} onUserClick={(uid) => setUserCardId(uid)} />
                    </span>
                  </div>
                ) : (
                  <MsgBubble msg={msg} isOwn={isOwn} showAvatar={showAvatar} isGroup={chat.chat.chat_type === 'group'}
                    onCtx={(e) => onCtx(e, msg)} onReply={() => setReplyTo(msg)}
                    onReact={(emoji) => addReaction(msg.id, emoji)} myId={user?.id || ''}
                    onScrollTo={scrollToMessage} onUserClick={(uid) => setUserCardId(uid)} />
                )}
              </div>
            );
          })}
          <div ref={endRef} className="chat-scroll-anchor" />
        </div>
      </div>

      {/* ── Context Menu (ПКМ по сообщению) ── */}
      {ctxMenu && !ctxMenu.msg.is_deleted && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setCtxMenu(null)} aria-hidden />
          <div className="fixed z-50" style={{ left: Math.min(ctxMenu.x, innerWidth - 180), top: Math.min(ctxMenu.y, innerHeight - 220) }}
            onClick={(e) => e.stopPropagation()}>
            <div className="bg-white dark:bg-dark-elevated rounded-compass shadow-compass border border-surface-border dark:border-dark-border py-1 min-w-[160px] animate-dialog">
              <CtxItem icon={<IconReply />} label="Ответить" onClick={() => { setReplyTo(ctxMenu.msg); setCtxMenu(null); }} />
              <CtxItem icon={<IconForward />} label="Переслать" onClick={() => { setForwardMsg(ctxMenu.msg); setCtxMenu(null); }} />
              {ctxMenu.msg.sender_id === user?.id && ctxMenu.msg.content_type === 'text' && (
                <CtxItem icon={<IconEdit />} label="Редактировать" onClick={() => { setEditingMessage(ctxMenu.msg); setCtxMenu(null); }} />
              )}
              {pinned.some((p) => p.message_id === ctxMenu.msg.id)
                ? <CtxItem icon={<IconPin />} label="Открепить" onClick={() => { unpinMessage(activeChatId, ctxMenu.msg.id); setCtxMenu(null); }} />
                : <CtxItem icon={<IconPin />} label="Закрепить" onClick={() => { pinMessage(activeChatId, ctxMenu.msg.id); setCtxMenu(null); }} />
              }
              {ctxMenu.msg.sender_id === user?.id && (
                <CtxItem icon={<IconTrash />} label="Удалить" danger onClick={() => { deleteMessage(ctxMenu.msg.id); setCtxMenu(null); }} />
              )}
            </div>
          </div>
        </>
      )}

      {/* ── Reply/Edit bar ── */}
      {(displayReplyTo || displayEditingMessage) && (
        <div className="shrink-0 px-4 py-2 bg-white dark:bg-dark-elevated border-t border-surface-border dark:border-dark-border flex items-center gap-2 min-h-0">
          <div className="w-0.5 h-8 bg-primary rounded-full shrink-0" />
          <div className="flex-1 min-w-0">
            <p className="text-[11px] font-semibold text-primary leading-tight">
              {displayEditingMessage ? 'Редактирование' : `Ответ для ${displayReplyTo?.sender?.username || ''}`}
            </p>
            <p className="text-[12px] text-txt-secondary dark:text-[#8b98a5] truncate leading-tight">{(displayEditingMessage || displayReplyTo)?.content}</p>
          </div>
          <button onClick={() => { setReplyTo(null); setEditingMessage(null); setText(''); }}
            className="w-6 h-6 flex items-center justify-center rounded-full hover:bg-surface dark:hover:bg-dark-hover transition-colors text-txt-placeholder hover:text-txt dark:text-[#8b98a5] dark:hover:text-[#e7e9ea]">
            <IconX size={10} />
          </button>
        </div>
      )}

      {/* Sending voice overlay */}
      {uploading && !recording && (
        <div className="shrink-0 px-4 py-2 flex items-center gap-2 bg-surface dark:bg-dark-elevated border-t border-surface-border dark:border-dark-border">
          <div className="w-5 h-5 border-2 border-primary border-t-transparent rounded-full animate-spin shrink-0" />
          <span className="text-[13px] text-txt-secondary dark:text-[#8b98a5]">Отправка голосового...</span>
        </div>
      )}

      {/* Voice send error */}
      {voiceError && (
        <div className="shrink-0 px-4 py-2 flex items-center justify-between gap-2 bg-danger/10 dark:bg-danger/20 border-t border-danger/30">
          <span className="text-[13px] text-danger">{voiceError}</span>
          <button type="button" onClick={() => setVoiceError(null)} className="text-danger hover:underline text-[12px]">Закрыть</button>
        </div>
      )}

      {/* ── Input ── */}
      {/* На мобильных: композер максимально внизу (fixed по low visualViewport); при вводе — вплотную к клавиатуре. На десктопе — sticky. */}
      <div
        ref={composerRef}
        className="shrink-0 z-20 px-2.5 sm:px-4 py-1 sm:py-2.5 pb-[var(--composer-padding-bottom,max(2px,env(safe-area-inset-bottom)))] sm:pb-[max(0.625rem,env(safe-area-inset-bottom))] bg-white dark:bg-dark-bg border-t border-surface-border dark:border-dark-border min-w-0 max-w-full overflow-visible fixed left-0 right-0 bottom-[var(--composer-bottom)] md:static md:sticky md:bottom-0"
      >
        <div className="relative min-w-0 max-w-full overflow-visible" ref={inputEmojiPickerRef}>
          <div className="flex items-end gap-1.5 sm:gap-2 min-w-0 max-w-full">
            <input ref={fileRef} type="file" className="hidden" onChange={handleFile} />

            <div className="flex-1 min-w-0">
              {!recording ? (
                <div className="flex items-end gap-1.5 sm:gap-2 min-w-0 max-w-full">
                  <button onClick={() => fileRef.current?.click()} disabled={uploading}
                    className="p-1.5 sm:p-2 rounded-compass text-txt-secondary hover:text-primary hover:bg-primary/5 dark:text-[#8b98a5] dark:hover:text-primary dark:hover:bg-primary/10 transition-colors disabled:opacity-50"
                    title="Прикрепить файл">
                    <IconPaperclip />
                  </button>
                  <button type="button" onClick={() => setShowInputEmojiPicker((s) => !s)} disabled={uploading}
                    className={`p-1.5 sm:p-2 rounded-compass transition-colors disabled:opacity-50 ${showInputEmojiPicker ? 'text-primary bg-primary/10' : 'text-txt-secondary hover:text-primary hover:bg-primary/5 dark:text-[#8b98a5] dark:hover:text-primary dark:hover:bg-primary/10'}`}
                    title="Эмодзи">
                    <IconSmile />
                  </button>
                  <textarea ref={textRef} value={displayText}
                  onChange={(e) => {
                    setText(e.target.value);
                    handleTyping();
                    updateMentionState(e.target.value, e.target.selectionStart ?? e.target.value.length);
                    if (stickToBottomRef.current) requestAnimationFrame(() => scrollToBottom('auto'));
                  }}
                  onClick={(e) => {
                    const t = e.currentTarget;
                    updateMentionState(t.value, t.selectionStart ?? t.value.length);
                  }}
                  onFocus={() => {
                    stickToBottomRef.current = true;
                    keyboardSettlingUntilRef.current = Date.now() + 2000;
                    stabilizeBottom();
                  }}
                  onBlur={() => {
                    keyboardSettlingUntilRef.current = 0;
                    stickToBottomRef.current = isScrollNearBottom(messagesScrollRef.current);
                  }}
                  onKeyDown={handleKeyDown} placeholder="Написать сообщение..." rows={1}
                  className="flex-1 min-w-0 resize-none px-3 py-1.5 sm:py-2 bg-surface dark:bg-dark-elevated rounded-compass text-[13px] sm:text-[14px] text-txt dark:text-[#e7e9ea] placeholder:text-txt-placeholder dark:placeholder:text-[#8b98a5] border border-transparent focus:border-primary/30 focus:ring-1 focus:ring-primary/15 outline-none transition-all max-h-28 sm:max-h-32 overflow-y-auto overflow-x-hidden break-words"
                  style={{ minHeight: 34, maxWidth: '100%' }} />
                  <button onClick={handleSend} disabled={!displayText.trim()}
                    className="p-1.5 sm:p-2 rounded-compass bg-primary text-white hover:bg-primary-hover transition-colors disabled:opacity-30">
                    <IconSend />
                  </button>
                </div>
              ) : (
                <div className="flex items-center gap-3 min-w-0">
                  <button
                    type="button"
                    onClick={cancelRecording}
                    className="p-2 rounded-compass text-danger hover:bg-danger/10 transition-colors shrink-0"
                    title="Отменить (Esc)"
                    aria-label="Отменить"
                  >
                    <IconTrash />
                  </button>

                  <div className="flex-1 min-w-0 flex items-center gap-3">
                    <span className="w-2 h-2 rounded-full bg-danger animate-pulse shrink-0" aria-hidden />
                    <span className="text-[13px] font-medium text-txt dark:text-[#e7e9ea] tabular-nums shrink-0">
                      {Math.floor(recordingSec / 60)}:{(recordingSec % 60).toString().padStart(2, '0')}
                    </span>
                    <div className="hidden sm:flex items-end gap-0.5 h-6 shrink-0" aria-hidden>
                      {RECORDING_WAVE.map((h, i) => (
                        <span
                          key={i}
                          className="w-1 rounded-full bg-primary dark:bg-primary/90 animate-voice-bar"
                          style={{ height: `${h}px`, animationDelay: `${i * 0.07}s` }}
                        />
                      ))}
                    </div>
                    <div className="min-w-0">
                      <div className={`text-[12px] font-medium truncate ${recordingCancelArmed ? 'text-danger' : 'text-txt-secondary dark:text-[#8b98a5]'}`}>
                        {recordingLocked ? 'Запись зафиксирована' : recordingCancelArmed ? 'Отпустите, чтобы отменить' : 'Свайп влево для отмены'}
                      </div>
                      {!recordingLocked && (
                        <div className="text-[11px] text-txt-placeholder dark:text-[#8b98a5] truncate">
                          Для отмены можно отпустить курсор вне поля
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>

            {typeof navigator !== 'undefined' && navigator.mediaDevices != null && (
              <div className="relative shrink-0">
                {recording && !recordingLocked && (
                  <div className={`absolute -top-[76px] right-0 w-[44px] rounded-[16px] border border-primary/25 bg-surface dark:bg-dark-elevated shadow-compass p-2 flex flex-col items-center gap-2 ${
                    recordingLockArmed ? 'text-primary border-primary/50' : 'text-txt-secondary dark:text-[#8b98a5]'
                  }`} aria-hidden>
                    <IconLock size={18} />
                    <IconChevronUp size={16} />
                  </div>
                )}
                <button
                  type="button"
                  onClick={(e) => {
                    if (suppressMicClickRef.current) { e.preventDefault(); e.stopPropagation(); return; }
                    if (recordingLocked) stopRecording(true);
                  }}
                  onPointerDown={handleMicPointerDown}
                  onPointerMove={handleMicPointerMove}
                  onPointerUp={handleMicPointerUp}
                  onPointerCancel={handleMicPointerCancel}
                  disabled={uploading}
                  className={recording
                    ? 'w-14 h-14 rounded-full bg-primary text-white flex items-center justify-center hover:bg-primary-hover active:scale-95 transition-all touch-none disabled:opacity-50'
                    : 'p-1.5 sm:p-2 rounded-compass transition-colors disabled:opacity-50 text-txt-secondary hover:text-primary hover:bg-primary/5 dark:text-[#8b98a5] dark:hover:text-primary dark:hover:bg-primary/10 touch-none'
                  }
                  title={recording ? (recordingLocked ? 'Отправить голосовое' : 'Запись...') : 'Удерживайте для записи голосового'}
                  aria-label={recording ? (recordingLocked ? 'Отправить голосовое' : 'Запись') : 'Запись голосового'}
                >
                  {recording ? (recordingLocked ? <IconSend /> : <IconMicrophone size={22} />) : <IconMicrophone size={20} />}
                </button>
              </div>
            )}
          </div>

          {!recording && showInputEmojiPicker && (
            <div className="absolute bottom-full right-0 mb-1 z-20" onClick={(e) => e.stopPropagation()}>
              <EmojiPicker isOwn={true} topClass="-top-6" onPick={insertEmojiAtCursor} onClose={() => setShowInputEmojiPicker(false)} />
            </div>
          )}
          {!recording && mentionOpen && mentionCandidates.length > 0 && (
            <div className="absolute bottom-full left-0 sm:left-[120px] mb-1 z-20 w-[min(260px,calc(100vw-1rem))] max-h-[220px] overflow-y-auto rounded-[12px] bg-white dark:bg-dark-elevated border border-surface-border dark:border-dark-border shadow-compass-dialog">
              {mentionCandidates.map((m, i) => (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => applyMention(m.username)}
                  className={`w-full flex items-center gap-2 px-3 py-2 text-left transition-colors ${
                    i === mentionActiveIndex ? 'bg-surface dark:bg-dark-hover' : 'hover:bg-surface dark:hover:bg-dark-hover'
                  }`}
                >
                  <Avatar name={m.username} url={m.avatar_url || undefined} size={24} />
                  <span className="text-[13px] text-txt dark:text-[#e7e9ea] truncate">{m.username}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* ── Forward Modal ── */}
      {displayForwardMsg && (
        <ForwardModal
          message={displayForwardMsg}
          chats={chats}
          myId={user?.id || ''}
          onForward={handleForward}
          onClose={() => setForwardMsg(null)}
        />
      )}

      {/* ── User Card ── */}
      {userCardId && (
        <UserCard userId={userCardId} onClose={() => setUserCardId(null)}
          onOpenChat={(uid) => {
            setUserCardId(null);
            // Find or start personal chat with this user
            const existing = chats.find((c) =>
              c.chat.chat_type === 'personal' && c.members.some((m) => m.id === uid)
            );
            if (existing) {
              setActiveChat(existing.chat.id);
            }
          }}
        />
      )}
    </div>
  );
}

function VoiceMessage({ url, isOwn }: { url: string; isOwn: boolean }) {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const [duration, setDuration] = useState(0);
  const [current, setCurrent] = useState(0);
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    const onLoaded = () => setDuration(Number.isFinite(audio.duration) ? audio.duration : 0);
    const onTime = () => setCurrent(audio.currentTime || 0);
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    const onEnded = () => { setPlaying(false); setCurrent(audio.duration || 0); };
    audio.addEventListener('loadedmetadata', onLoaded);
    audio.addEventListener('timeupdate', onTime);
    audio.addEventListener('play', onPlay);
    audio.addEventListener('pause', onPause);
    audio.addEventListener('ended', onEnded);
    return () => {
      audio.removeEventListener('loadedmetadata', onLoaded);
      audio.removeEventListener('timeupdate', onTime);
      audio.removeEventListener('play', onPlay);
      audio.removeEventListener('pause', onPause);
      audio.removeEventListener('ended', onEnded);
    };
  }, []);

  const toggle = useCallback(() => {
    const audio = audioRef.current;
    if (!audio) return;
    if (audio.paused) {
      audio.play().catch(() => { /* autoplay may be blocked */ });
    } else {
      audio.pause();
    }
  }, []);

  const seek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const audio = audioRef.current;
    if (!audio || !duration) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const x = Math.min(Math.max(0, e.clientX - rect.left), rect.width);
    const next = (x / rect.width) * duration;
    audio.currentTime = next;
    setCurrent(next);
  }, [duration]);

  const progress = duration > 0 ? Math.min(1, current / duration) : 0;
  const rawActiveBars = Math.max(0, Math.round(progress * VOICE_WAVE.length));
  // Telegram-like: while playing, show at least a couple of active/animated bars so it feels "alive" from 0:00.
  const activeBars = playing ? Math.max(2, rawActiveBars) : rawActiveBars;
  const animBars = playing ? Math.min(activeBars, VOICE_WAVE.length) : 0;

  return (
    <div className="voice-msg">
      <button
        type="button"
        onClick={toggle}
        className={`voice-play ${isOwn ? 'bg-white/20 text-white' : 'bg-primary/10 text-primary'}`}
        aria-label={playing ? 'Пауза' : 'Воспроизвести'}
        title={playing ? 'Пауза' : 'Воспроизвести'}
      >
        {playing ? <IconPause size={14} /> : <IconPlay size={14} />}
      </button>
      <div
        className={`voice-wave ${isOwn ? 'text-white' : 'text-primary'}`}
        onClick={seek}
        role="slider"
        aria-valuemin={0}
        aria-valuemax={Math.round(duration)}
        aria-valuenow={Math.round(current)}
      >
        {VOICE_WAVE.map((h, i) => (
          <span
            key={i}
            className={`voice-bar ${i < activeBars ? 'voice-bar-active' : ''} ${i < animBars ? 'voice-bar-playing' : ''}`}
            style={{
              height: `${h}px`,
              ...(i < animBars ? { animationDelay: `${(i % 7) * 0.06}s`, animationDuration: `${0.85 + (i % 5) * 0.09}s` } : null),
            }}
          />
        ))}
      </div>
      <span className={`voice-time ${isOwn ? 'text-white/80' : 'text-txt-secondary dark:text-[#8b98a5]'}`}>
        {formatAudioTime(playing ? current : duration)}
      </span>
      <button
        type="button"
        className={`voice-vol ${isOwn ? 'text-white/70 hover:bg-white/15' : 'text-txt-secondary hover:bg-primary/10'}`}
        aria-label="Громкость"
        title="Громкость"
      >
        <IconVolume size={14} />
      </button>
      <audio ref={audioRef} src={url} preload="metadata" />
    </div>
  );
}

/* ── Context menu item ── */
function CtxItem({ icon, label, onClick, danger }: { icon: React.ReactNode; label: string; onClick: () => void; danger?: boolean }) {
  return (
    <button onClick={onClick}
      className={`w-full flex items-center gap-2.5 px-3.5 py-2 text-[13px] font-medium hover:bg-surface dark:hover:bg-dark-hover transition-colors ${danger ? 'text-danger' : 'text-txt dark:text-[#e7e9ea]'}`}>
      <span className={danger ? 'text-danger' : 'text-txt-secondary dark:text-[#8b98a5]'}>{icon}</span>
      {label}
    </button>
  );
}

/* ── Message Bubble ── */
function MsgBubble({ msg, isOwn, showAvatar, isGroup, onCtx, onReply, onReact, myId, onScrollTo, onUserClick }: {
  msg: Message; isOwn: boolean; showAvatar: boolean; isGroup: boolean;
  onCtx: (e: React.MouseEvent) => void; onReply: () => void;
  onReact: (emoji: string) => void; myId: string; onScrollTo?: (msgId: string) => void;
  onUserClick?: (userId: string) => void;
}) {
  const [showEmoji, setShowEmoji] = useState(false);

  const groups = useMemo(() => {
    if (msg.is_deleted) return [];
    const g: Record<string, { emoji: string; users: string[] }> = {};
    for (const r of msg.reactions || []) {
      if (!g[r.emoji]) g[r.emoji] = { emoji: r.emoji, users: [] };
      g[r.emoji].users.push(r.user_id);
    }
    return Object.values(g);
  }, [msg.reactions, msg.is_deleted]);

  const msgAppearClass = isOwn ? 'msg-appear-own' : 'msg-appear-incoming';
  if (msg.is_deleted) {
    return (
      <div className={`flex items-end gap-2 ${msgAppearClass} ${isOwn ? 'justify-end' : 'justify-start'}`}>
        {!isOwn && <div className="w-8 shrink-0" />}
        <div className="px-3 py-1.5 rounded-compass bg-surface dark:bg-dark-elevated text-txt-placeholder dark:text-[#8b98a5] text-[13px] italic">Сообщение удалено</div>
      </div>
    );
  }

  return (
    <div className={`flex items-end gap-2 ${msgAppearClass} group relative min-w-0 ${isOwn ? 'justify-end' : 'justify-start'}`} onContextMenu={onCtx}>
      {!isOwn && (
        <div className="w-8 shrink-0">
          {showAvatar && msg.sender && (
            <div className="cursor-pointer" onClick={() => msg.sender && onUserClick?.(msg.sender.id)}>
              <Avatar name={msg.sender.username} url={msg.sender.avatar_url || undefined} size={32} />
            </div>
          )}
        </div>
      )}
      <div className={`max-w-[85%] min-w-0 w-fit ${isOwn ? 'order-first' : ''}`}>
        {!isOwn && isGroup && msg.sender && (
          <p className="text-[11px] font-semibold text-primary dark:text-[#58a6ff] mb-0.5 ml-1 cursor-pointer hover:underline"
            onClick={() => msg.sender && onUserClick?.(msg.sender.id)}>{msg.sender.username}</p>
        )}

        <div className={`rounded-[14px] px-3 py-2 inline-block max-w-full ${isOwn ? 'bg-primary text-white rounded-br-[4px]' : 'bg-surface dark:bg-dark-elevated text-txt dark:text-[#e7e9ea] rounded-bl-[4px]'}`}>
          {/* Reply quote */}
          {msg.reply_to && (
            <div
              className={`mb-1.5 px-2.5 py-1.5 rounded-compass border-l-2 cursor-pointer transition-colors ${isOwn ? 'bg-white/15 border-white/50 hover:bg-white/25' : 'bg-primary/8 dark:bg-primary/15 border-primary hover:bg-primary/15 dark:hover:bg-primary/20'}`}
              onClick={(e) => { e.stopPropagation(); msg.reply_to?.id && onScrollTo?.(msg.reply_to.id); }}
            >
              <p className={`text-[10px] font-bold ${isOwn ? 'text-white/90' : 'text-primary'}`}>{msg.reply_to.sender?.username}</p>
              <p className={`text-[11px] truncate ${isOwn ? 'text-white/65' : 'text-txt-secondary dark:text-[#8b98a5]'}`}>{msg.reply_to.content}</p>
            </div>
          )}

          {msg.content_type === 'image' && msg.file_url && (
            <a href={msg.file_url} target="_blank" rel="noopener noreferrer" className="block mb-1.5">
              <img src={msg.file_url} alt={normalizeFileDisplayName(msg.file_name) || 'image'} className="rounded-compass max-w-full max-h-60 object-cover" loading="lazy" />
            </a>
          )}
          {msg.content_type === 'voice' && (
            <div className="mb-1.5">
              {msg.file_url ? (
                <VoiceMessage url={msg.file_url} isOwn={isOwn} />
              ) : (
                <span className="flex items-center gap-2 text-[13px] text-txt-secondary dark:text-[#8b98a5]">
                  <span className="w-4 h-4 border-2 border-primary border-t-transparent rounded-full animate-spin shrink-0" />
                  Отправка…
                </span>
              )}
            </div>
          )}
          {msg.content_type === 'file' && msg.file_url && (
            <div className={`rounded-[12px] overflow-hidden mb-1.5 w-full max-w-[260px] ${isOwn ? 'bg-white/15' : 'bg-surface-light dark:bg-dark-hover'}`}>
              <div className="flex items-center gap-3 px-3.5 py-3">
                <div className={`flex-shrink-0 w-11 h-11 rounded-[10px] flex items-center justify-center ${isOwn ? 'bg-white/20' : 'bg-primary/10'}`}>
                  <IconFile size={22} />
                </div>
                <div className="min-w-0 flex-1">
                  <p className={`text-[13px] font-semibold truncate leading-tight ${isOwn ? 'text-white' : 'text-txt dark:text-[#e7e9ea]'}`}>{normalizeFileDisplayName(msg.file_name) || 'Файл'}</p>
                  {msg.file_size ? <p className={`text-[11px] mt-0.5 ${isOwn ? 'text-white/60' : 'text-txt-secondary dark:text-[#8b98a5]'}`}>{formatFileSize(msg.file_size)}</p> : null}
                </div>
              </div>
              <a href={`${msg.file_url}?name=${encodeURIComponent(normalizeFileDisplayName(msg.file_name) || 'file')}`} download={normalizeFileDisplayName(msg.file_name) || 'file'}
                className={`flex items-center justify-center gap-2 py-2.5 border-t transition-colors cursor-pointer ${
                  isOwn
                    ? 'border-white/15 text-white/90 hover:bg-white/10'
                    : 'border-border text-primary hover:bg-primary/5'
                }`}>
                <IconDownload size={16} />
                <span className="text-[13px] font-semibold">Загрузить</span>
              </a>
            </div>
          )}
          {msg.content && msg.content_type === 'text' && (
            <p className="text-[13px] whitespace-pre-wrap break-words leading-[18px]">{msg.content}</p>
          )}
          <div className="flex items-center gap-1.5 mt-1 justify-end flex-shrink-0">
            {msg.edited_at && <span className={`text-[9px] ${isOwn ? 'text-white/35' : 'text-txt-placeholder dark:text-[#8b98a5]'}`}>ред.</span>}
            <span className={`text-[10px] whitespace-nowrap ${isOwn ? 'text-white/55' : 'text-txt-placeholder dark:text-[#8b98a5]'}`}>{formatTime(msg.created_at)}</span>
            {isOwn && (
              <span className={msg.status === 'read' ? 'text-white/80' : 'text-white/45'}>
                {msg.status === 'read' ? <IconCheckDouble /> : <IconCheck />}
              </span>
            )}
          </div>
        </div>

        {/* Reactions */}
        {groups.length > 0 && (
          <div className="flex flex-wrap gap-1 mt-1 mx-1">
            {groups.map((g) => (
              <button key={g.emoji} onClick={() => onReact(g.emoji)}
                className={`inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded-full text-[11px] border transition-colors ${
                  g.users.includes(myId) ? 'border-primary bg-primary/10 text-primary' : 'border-surface-border bg-white text-txt-secondary hover:border-primary/30'
                }`}>
                <span>{g.emoji}</span><span className="text-[10px]">{g.users.length}</span>
              </button>
            ))}
          </div>
        )}

        {/* Hover actions */}
        <div className={`absolute top-0 ${isOwn ? 'left-0 -translate-x-full' : 'right-0 translate-x-full'} opacity-0 group-hover:opacity-100 transition-opacity flex gap-0.5 px-1`}>
          <HoverBtn tip="Ответить" onClick={(e) => { e.stopPropagation(); onReply(); }}><IconReply /></HoverBtn>
          <HoverBtn tip="Реакция" onClick={(e) => { e.stopPropagation(); setShowEmoji(!showEmoji); }}>
            <span className="text-[12px]">😀</span>
          </HoverBtn>
        </div>

        {showEmoji && (
          <EmojiPicker
            isOwn={isOwn}
            onPick={(e) => { onReact(e); setShowEmoji(false); }}
            onClose={() => setShowEmoji(false)}
          />
        )}
      </div>
    </div>
  );
}

function HoverBtn({ tip, onClick, children }: { tip: string; onClick: (e: React.MouseEvent) => void; children: React.ReactNode }) {
  return (
    <button onClick={onClick} title={tip}
      className="p-1 rounded-compass bg-white dark:bg-dark-elevated shadow-compass border border-surface-border dark:border-dark-border text-txt-secondary hover:text-primary dark:text-[#8b98a5] dark:hover:text-primary transition-colors">
      {children}
    </button>
  );
}

/* ── Enhanced Emoji Picker ── */
function EmojiPicker({ isOwn, onPick, onClose, topClass = '-top-2' }: { isOwn: boolean; onPick: (emoji: string) => void; onClose: () => void; topClass?: string }) {
  const [tab, setTab] = useState(0);
  return (
    <div className={`absolute ${isOwn ? 'right-0' : 'left-10'} ${topClass} -translate-y-full bg-white dark:bg-dark-elevated rounded-[12px] shadow-compass-lg border border-surface-border dark:border-dark-border z-20 w-[280px]`}
      onClick={(e) => e.stopPropagation()}>
      {/* Tabs */}
      <div className="flex border-b border-surface-border dark:border-dark-border px-1 pt-1">
        {EMOJI_CATEGORIES.map((cat, i) => (
          <button key={cat.label} onClick={() => setTab(i)}
            className={`px-2 py-1.5 text-[11px] font-medium rounded-t-compass transition-colors ${
              tab === i ? 'text-primary border-b-2 border-primary' : 'text-txt-secondary hover:text-txt dark:text-[#8b98a5] dark:hover:text-[#e7e9ea]'
            }`}>
            {cat.label}
          </button>
        ))}
      </div>
      {/* Grid */}
      <div className="p-2 grid grid-cols-8 gap-0.5 max-h-[160px] overflow-y-auto">
        {EMOJI_CATEGORIES[tab].emojis.map((e) => (
          <button key={e} onClick={() => onPick(e)}
            className="text-[18px] w-8 h-8 flex items-center justify-center hover:bg-surface dark:hover:bg-dark-hover rounded-compass transition-colors">{e}</button>
        ))}
      </div>
    </div>
  );
}

/* ── Forward Modal ── */
function ForwardModal({ message, chats, myId, onForward, onClose }: {
  message: Message; chats: ChatWithLastMessage[]; myId: string;
  onForward: (chatId: string) => void; onClose: () => void;
}) {
  const [search, setSearch] = useState('');
  const filtered = useMemo(() => {
    if (!search.trim()) return chats;
    const q = search.toLowerCase();
    return chats.filter((c) => getName(c, myId).toLowerCase().includes(q));
  }, [chats, search, myId]);

  return (
    <Modal open={true} onClose={onClose} title="Переслать сообщение" size="md">
      {/* Preview */}
      <div className="mb-3 px-3 py-2 bg-surface dark:bg-dark-hover rounded-compass border-l-2 border-primary">
        <p className="text-[11px] font-semibold text-primary">{message.sender?.username}</p>
        <p className="text-[12px] text-txt-secondary dark:text-[#8b98a5] truncate">{message.content || (message.content_type === 'image' ? '📷 Фото' : message.content_type === 'voice' ? '🎤 Голосовое' : '📎 Файл')}</p>
      </div>

      <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} autoFocus
        placeholder="Найти чат..." className="compass-input mb-3" />

      <div className="max-h-64 overflow-y-auto space-y-0.5">
        {filtered.map((c) => {
          const name = getName(c, myId);
          return (
            <button key={c.chat.id} onClick={() => onForward(c.chat.id)}
              className="w-full flex items-center gap-3 px-3 py-2.5 rounded-compass hover:bg-surface dark:hover:bg-dark-hover transition-colors">
              <Avatar name={name} url={c.chat.avatar_url || undefined} size={36} />
              <div className="flex-1 min-w-0">
                <span className="text-[14px] font-medium text-txt dark:text-[#e7e9ea]">{name}</span>
                <p className="text-[11px] text-txt-secondary dark:text-[#8b98a5]">{c.chat.chat_type === 'group' ? `${c.members.length} участников` : c.chat.chat_type === 'notes' ? 'Персональный чат' : 'Личный чат'}</p>
              </div>
              <IconForward />
            </button>
          );
        })}
        {filtered.length === 0 && <p className="text-center text-txt-secondary dark:text-[#8b98a5] text-[13px] py-4">Ничего не найдено</p>}
      </div>
    </Modal>
  );
}

/* Helpers */
function getName(c: ChatWithLastMessage, myId: string) {
  if (c.chat.chat_type === 'group' || c.chat.chat_type === 'notes') return c.chat.name;
  return c.members.find((m) => m.id !== myId)?.username || 'Чат';
}
function getOnline(c: ChatWithLastMessage, myId: string, o: Record<string, boolean>) {
  if (c.chat.chat_type === 'group' || c.chat.chat_type === 'notes') return undefined;
  const other = c.members.find((m) => m.id !== myId);
  return other ? (o[other.id] ?? other.is_online) : undefined;
}
