-- Chat threads (1:1 or group)
CREATE TABLE chat_threads (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    is_group   INTEGER NOT NULL DEFAULT 0,
    group_id   INTEGER NULL,   -- if group chat
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
);