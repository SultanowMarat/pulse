ALTER TABLE users ADD COLUMN IF NOT EXISTS login_key_hash VARCHAR(128);
ALTER TABLE users ADD COLUMN IF NOT EXISTS login_key_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS login_key_active BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS login_key_generated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_login_key_hash_active
  ON users (login_key_hash)
  WHERE login_key_active = TRUE AND login_key_hash IS NOT NULL;
