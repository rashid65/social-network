-- Posts
CREATE TABLE posts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    author_id   TEXT    NOT NULL,
    content     TEXT    NOT NULL,
    privacy     TEXT    NOT NULL CHECK(privacy IN ('public','followers','custom')),
    liked       INTEGER DEFAULT 0, 
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(author_id) REFERENCES users(id) ON DELETE CASCADE
);