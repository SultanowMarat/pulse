-- Optional user profile fields: company and position.

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS company TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS position TEXT NOT NULL DEFAULT '';

