-- Events in groups
CREATE TABLE events (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id     INTEGER NOT NULL,
    creator_id   TEXT    NOT NULL,
    title        TEXT    NOT NULL,
    description  TEXT    NOT NULL,
    event_time   TEXT    NOT NULL,             -- ISO‑8601 datetime
    created_at   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(group_id)   REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY(creator_id) REFERENCES users(id) ON DELETE CASCADE
);