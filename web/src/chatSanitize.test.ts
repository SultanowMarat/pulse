import { describe, expect, it } from 'vitest';
import { sanitizeChatList, sanitizeChatWithLastMessage, sanitizeMembers } from './chatSanitize';

describe('chatSanitize', () => {
  it('returns empty members for non-array values', () => {
    expect(sanitizeMembers(null)).toEqual([]);
    expect(sanitizeMembers(undefined)).toEqual([]);
    expect(sanitizeMembers({})).toEqual([]);
  });

  it('filters invalid members and keeps valid ones', () => {
    const members = sanitizeMembers([
      null,
      { id: 1, username: 'bad' },
      { id: 'u1' },
      { id: 'u2', username: 'Alice', is_online: 1 },
    ]);
    expect(members).toHaveLength(1);
    expect(members[0]).toMatchObject({ id: 'u2', username: 'Alice', is_online: true });
  });

  it('sanitizes chat and normalizes members array', () => {
    const chat = sanitizeChatWithLastMessage({
      chat: { id: 'c1', chat_type: 'personal', name: 'A', description: '', avatar_url: '', created_by: 'u1', created_at: '2026-01-01T00:00:00Z' },
      members: null,
      unread_count: 3,
      muted: undefined,
    });
    expect(chat).not.toBeNull();
    expect(chat?.members).toEqual([]);
    expect(chat?.muted).toBe(false);
    expect(chat?.unread_count).toBe(3);
  });

  it('drops malformed chats from list', () => {
    const chats = sanitizeChatList([
      { chat: { id: '', chat_type: 'personal' } },
      { foo: 'bar' },
      {
        chat: { id: 'ok', chat_type: 'group', name: 'G', description: '', avatar_url: '', created_by: 'u1', created_at: '2026-01-01T00:00:00Z' },
        members: [],
        unread_count: 0,
      },
    ]);
    expect(chats).toHaveLength(1);
    expect(chats[0]?.chat.id).toBe('ok');
  });
});
