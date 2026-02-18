CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    endpoint TEXT,
    method TEXT,
    ip TEXT,
    user_agent TEXT,
    request_id TEXT,
    status_code INTEGER,
    response_time_ms INTEGER,
    metadata TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_logs_created_at
    ON logs (created_at);

CREATE INDEX IF NOT EXISTS idx_logs_level
    ON logs (level);

CREATE INDEX IF NOT EXISTS idx_logs_request_id
    ON logs (request_id);
