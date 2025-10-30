-- Group memberships
CREATE TABLE group_memberships (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id   INTEGER NOT NULL,
    user_id    TEXT    NOT NULL,
    role       TEXT    NOT NULL CHECK(role IN ('member','admin')),
    joined_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id)  REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(group_id, user_id)
);