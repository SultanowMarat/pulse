import type { Message, UserPublic } from './types';

function norm(v: string | null | undefined): string {
  return String(v || '').trim().toLowerCase();
}

function same(a: string | null | undefined, b: string | null | undefined): boolean {
  const na = norm(a);
  const nb = norm(b);
  return na !== '' && na === nb;
}

type SenderLike = Pick<Message, 'sender_id' | 'sender'>;

export function isMessageFromUserId(msg: SenderLike, userId: string | null | undefined): boolean {
  if (same(msg.sender_id, userId)) return true;
  if (msg.sender && same(msg.sender.id, userId)) return true;
  return false;
}

export function isMessageFromUser(msg: SenderLike, user: UserPublic | null | undefined): boolean {
  if (!user) return false;
  if (isMessageFromUserId(msg, user.id)) return true;
  if (msg.sender) {
    if (same(msg.sender.username, user.username)) return true;
    if (same(msg.sender.email, user.email)) return true;
  }
  return false;
}

export function areMessagesFromSameSender(a: SenderLike, b: SenderLike): boolean {
  if (same(a.sender_id, b.sender_id)) return true;
  if (a.sender && b.sender) {
    if (same(a.sender.id, b.sender.id)) return true;
    if (same(a.sender.username, b.sender.username)) return true;
    if (same(a.sender.email, b.sender.email)) return true;
  }
  if (a.sender && same(a.sender.id, b.sender_id)) return true;
  if (b.sender && same(b.sender.id, a.sender_id)) return true;
  return false;
}
