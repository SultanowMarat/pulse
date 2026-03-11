-- Unified baseline schema migration.
-- Includes all changes that were previously split across 001..016.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    avatar_url TEXT DEFAULT '',
    last_seen_at TIMESTAMPTZ DEFAULT NOW(),
    is_online BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    phone VARCHAR(20) DEFAULT '',
    disabled_at TIMESTAMPTZ DEFAULT NULL
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(20) DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ DEFAULT NULL;

-- Chats
CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_type VARCHAR(10) NOT NULL,
    name VARCHAR(100) DEFAULT '',
    avatar_url TEXT DEFAULT '',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    description TEXT DEFAULT ''
);
ALTER TABLE chats ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';

DO $$
DECLARE
  conname text;
BEGIN
  FOR conname IN
    SELECT c.conname
    FROM pg_constraint c
    JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey) AND NOT a.attisdropped
    WHERE c.conrelid = 'chats'::regclass AND c.contype = 'c' AND a.attname = 'chat_type'
  LOOP
    EXECUTE format('ALTER TABLE chats DROP CONSTRAINT %I', conname);
  END LOOP;
END $$;

ALTER TABLE chats
  ADD CONSTRAINT chats_chat_type_check CHECK (chat_type IN ('personal', 'group', 'notes'));

-- Chat members
CREATE TABLE IF NOT EXISTS chat_members (
    chat_id UUID REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(10) DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    last_read_at TIMESTAMPTZ DEFAULT '1970-01-01T00:00:00Z',
    muted BOOLEAN DEFAULT FALSE,
    cleared_at TIMESTAMPTZ DEFAULT '1970-01-01T00:00:00Z',
    PRIMARY KEY (chat_id, user_id)
);
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS last_read_at TIMESTAMPTZ DEFAULT '1970-01-01T00:00:00Z';
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS muted BOOLEAN DEFAULT FALSE;
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS cleared_at TIMESTAMPTZ DEFAULT '1970-01-01T00:00:00Z';

-- Messages
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID REFERENCES chats(id) ON DELETE CASCADE,
    sender_id UUID REFERENCES users(id) ON DELETE SET NULL,
    content TEXT DEFAULT '',
    content_type VARCHAR(20) DEFAULT 'text',
    file_url TEXT DEFAULT '',
    file_name VARCHAR(255) DEFAULT '',
    file_size BIGINT DEFAULT 0,
    status VARCHAR(10) DEFAULT 'sent' CHECK (status IN ('sent', 'delivered', 'read')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    reply_to_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    edited_at TIMESTAMPTZ,
    is_deleted BOOLEAN DEFAULT FALSE
);
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_to_id UUID REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS edited_at TIMESTAMPTZ;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN DEFAULT FALSE;

ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_content_type_check;
UPDATE messages SET content_type = 'text' WHERE content_type IS NULL OR content_type = '';
ALTER TABLE messages
  ADD CONSTRAINT messages_content_type_check CHECK (content_type IN ('text', 'image', 'file', 'system', 'voice'));

-- Message reactions
CREATE TABLE IF NOT EXISTS message_reactions (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id, emoji)
);

-- Pinned messages
CREATE TABLE IF NOT EXISTS pinned_messages (
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    pinned_by UUID NOT NULL REFERENCES users(id),
    pinned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, message_id)
);

-- Favorites
CREATE TABLE IF NOT EXISTS user_favorite_chats (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, chat_id)
);

-- Device sessions
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) NOT NULL,
    device_name VARCHAR(255) DEFAULT '',
    secret_hash VARCHAR(64) NOT NULL,
    last_seen_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ DEFAULT NULL,
    session_secret TEXT DEFAULT NULL
);
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS session_secret TEXT DEFAULT NULL;

-- User permissions
CREATE TABLE IF NOT EXISTS user_permissions (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    administrator BOOLEAN NOT NULL DEFAULT FALSE,
    member BOOLEAN NOT NULL DEFAULT TRUE,
    delete_others_messages BOOLEAN NOT NULL DEFAULT FALSE,
    manage_bots BOOLEAN NOT NULL DEFAULT FALSE,
    edit_others_profile BOOLEAN NOT NULL DEFAULT FALSE,
    invite_to_team BOOLEAN NOT NULL DEFAULT FALSE,
    remove_from_team BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS administrator BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS member BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE user_permissions DROP COLUMN IF EXISTS admin_all_groups;

-- Global mail (SMTP) settings used by OTP auth and admin test-mail.
CREATE TABLE IF NOT EXISTS app_mail_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    host TEXT NOT NULL DEFAULT '',
    port INT NOT NULL DEFAULT 0,
    username TEXT NOT NULL DEFAULT '',
    password TEXT NOT NULL DEFAULT '',
    from_email TEXT NOT NULL DEFAULT '',
    from_name TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Global file settings for uploads from client.
CREATE TABLE IF NOT EXISTS app_file_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    max_file_size_mb INT NOT NULL DEFAULT 20,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Data cleanup from old versions
UPDATE messages
SET file_name = REPLACE(TRIM(file_name), '+', ' ')
WHERE file_name LIKE '%+%' AND file_name IS NOT NULL AND file_name != '';

-- Indexes
CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_chat_members_user_id ON chat_members(user_id);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_reactions_message ON message_reactions(message_id);
CREATE INDEX IF NOT EXISTS idx_user_favorite_chats_user_id ON user_favorite_chats(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_user_device ON sessions(user_id, device_id);
CREATE INDEX IF NOT EXISTS idx_sessions_id_revoked ON sessions(id, revoked_at);
CREATE INDEX IF NOT EXISTS idx_user_permissions_user_id ON user_permissions(user_id);
