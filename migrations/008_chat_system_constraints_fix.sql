-- Normalize chats system fields for compatibility with legacy deployments.
-- Regular chats must keep system_key = NULL.
-- System chats (general/notes) use non-NULL system_key and is_system = true.

ALTER TABLE chats
  ADD COLUMN IF NOT EXISTS system_key TEXT;

-- Legacy schema had NOT NULL + empty string values.
ALTER TABLE chats
  ALTER COLUMN system_key DROP NOT NULL;

ALTER TABLE chats
  ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE chats
SET system_key = NULL
WHERE system_key IS NOT NULL
  AND btrim(system_key) = '';

-- Keep only known system keys.
UPDATE chats
SET system_key = NULL
WHERE system_key IS NOT NULL
  AND system_key NOT IN ('notes', 'general');

UPDATE chats
SET is_system = (system_key IS NOT NULL);

ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_system_key_key;
ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_system_key_allowed_check;
ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_system_key_requires_system_check;

ALTER TABLE chats
  ADD CONSTRAINT chats_system_key_allowed_check
  CHECK (system_key IS NULL OR system_key IN ('notes', 'general'));

ALTER TABLE chats
  ADD CONSTRAINT chats_system_key_requires_system_check
  CHECK (system_key IS NULL OR is_system = TRUE);

DROP INDEX IF EXISTS idx_chats_system_key_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_chats_system_key_unique
  ON chats(system_key)
  WHERE system_key IS NOT NULL;
