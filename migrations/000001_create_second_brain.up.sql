CREATE TABLE IF NOT EXISTS second_brain (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    title TEXT NOT NULL,
    context TEXT NOT NULL,
    project TEXT NOT NULL,
    commits TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_second_brain_created_at
    ON second_brain (created_at);
