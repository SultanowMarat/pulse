-- Performance indexes for hot chat/message paths.

-- Faster unread counts and active message listing inside chats.
CREATE INDEX IF NOT EXISTS idx_messages_chat_created_active
  ON messages (chat_id, created_at DESC)
  WHERE is_deleted = false;

-- Faster mark-as-read / per-chat sender scans.
CREATE INDEX IF NOT EXISTS idx_messages_chat_sender_status
  ON messages (chat_id, sender_id, status);

-- Faster attachment reference checks when deleting messages/chats.
CREATE INDEX IF NOT EXISTS idx_messages_file_url_active
  ON messages (file_url)
  WHERE COALESCE(file_url, '') <> '' AND is_deleted = false;

-- Faster ordered members listing for chat card/group info rendering.
CREATE INDEX IF NOT EXISTS idx_chat_members_chat_joined
  ON chat_members (chat_id, joined_at);
