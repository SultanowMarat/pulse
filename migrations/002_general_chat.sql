-- Add system_key to chats to support service/system chats (e.g. "Общий чат").

ALTER TABLE chats
  ADD COLUMN IF NOT EXISTS system_key TEXT NOT NULL DEFAULT '';

-- Only one chat per system_key.
CREATE UNIQUE INDEX IF NOT EXISTS idx_chats_system_key_unique
  ON chats(system_key)
  WHERE system_key <> '';

