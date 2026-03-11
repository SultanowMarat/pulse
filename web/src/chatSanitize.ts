import type { ChatWithLastMessage, UserPublic } from './types';

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

export function sanitizeMembers(members: unknown): UserPublic[] {
  if (!Array.isArray(members)) return [];
  const out: UserPublic[] = [];
  for (const raw of members) {
    if (!raw || typeof raw !== 'object') continue;
    const r = raw as Partial<UserPublic>;
    const id = asString(r.id);
    const username = asString(r.username);
    if (!id || !username) continue;
    out.push({
      id,
      username,
      email: asString(r.email),
      phone: asString(r.phone),
      position: asString(r.position),
      avatar_url: asString(r.avatar_url),
      is_online: !!r.is_online,
      last_seen_at: asString(r.last_seen_at),
      disabled_at: r.disabled_at ?? null,
    });
  }
  return out;
}

export function sanitizeChatWithLastMessage(input: unknown): ChatWithLastMessage | null {
  if (!input || typeof input !== 'object') return null;
  const row = input as ChatWithLastMessage;
  const chat = row.chat as ChatWithLastMessage['chat'] | undefined;
  if (!chat || typeof chat !== 'object' || typeof chat.id !== 'string' || !chat.id) return null;

  return {
    ...row,
    muted: row.muted ?? false,
    unread_count: Number.isFinite(row.unread_count) ? Math.max(0, row.unread_count || 0) : 0,
    members: sanitizeMembers((row as { members?: unknown }).members),
  };
}

export function sanitizeChatList(input: unknown): ChatWithLastMessage[] {
  if (!Array.isArray(input)) return [];
  const out: ChatWithLastMessage[] = [];
  for (const row of input) {
    const clean = sanitizeChatWithLastMessage(row);
    if (clean) out.push(clean);
  }
  return out;
}
