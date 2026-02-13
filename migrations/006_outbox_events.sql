CREATE TABLE IF NOT EXISTS outbox_events (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    event_key TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    lock_owner TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox_events (available_at, created_at)
    WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_outbox_locked
    ON outbox_events (locked_at)
    WHERE published_at IS NULL;
