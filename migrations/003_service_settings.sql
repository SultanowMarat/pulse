-- Service/runtime settings editable from admin panel (single row id=1).
-- These mostly mirror env/yaml config but can be changed from UI.

CREATE TABLE IF NOT EXISTS app_service_settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  maintenance BOOLEAN NOT NULL DEFAULT FALSE,
  read_only BOOLEAN NOT NULL DEFAULT FALSE,
  degradation BOOLEAN NOT NULL DEFAULT FALSE,
  status_message TEXT NOT NULL DEFAULT '',

  cors_allowed_origins TEXT NOT NULL DEFAULT '*',

  max_ws_connections INTEGER NOT NULL DEFAULT 10000,
  ws_send_buffer_size INTEGER NOT NULL DEFAULT 256,
  ws_write_timeout INTEGER NOT NULL DEFAULT 10,
  ws_pong_timeout INTEGER NOT NULL DEFAULT 60,
  ws_max_message_size INTEGER NOT NULL DEFAULT 4096,

  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Ensure singleton row exists (so backups always contain settings).
INSERT INTO app_service_settings (id) VALUES (1) ON CONFLICT (id) DO NOTHING;
