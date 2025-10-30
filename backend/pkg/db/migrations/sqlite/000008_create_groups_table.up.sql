-- Groups
CREATE TABLE groups (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    creator_id  TEXT    NOT NULL,
    title       TEXT    NOT NULL,
    description TEXT    NOT NULL,
    is_public   INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(creator_id) REFERENCES users(id) ON DELETE CASCADE
);