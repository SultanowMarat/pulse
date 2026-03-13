import type { ChatWithLastMessage } from '../../types';

export function getChatName(chat: ChatWithLastMessage, myId: string): string {
  if (chat.chat.chat_type === 'notes') return chat.chat.name;
  if (chat.chat.chat_type === 'group') return chat.chat.name;
  return chat.members.find((member) => member.id !== myId)?.username || '\u0427\u0430\u0442';
}

export function getChatOnline(
  chat: ChatWithLastMessage,
  myId: string,
  onlineUsers: Record<string, boolean>
): boolean | undefined {
  if (chat.chat.chat_type === 'group' || chat.chat.chat_type === 'notes') return undefined;
  const other = chat.members.find((member) => member.id !== myId);
  return other ? (onlineUsers[other.id] ?? other.is_online) : undefined;
}

export function getChatAvatar(chat: ChatWithLastMessage, myId: string): string | undefined {
  if (chat.chat.chat_type === 'group' || chat.chat.chat_type === 'notes') return chat.chat.avatar_url || undefined;
  const other = chat.members.find((member) => member.id !== myId);
  return other?.avatar_url || chat.chat.avatar_url || undefined;
}
