-- Comment media (images/GIFs)
CREATE TABLE comment_media (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    comment_id  INTEGER NOT NULL,
    media_type  TEXT    NOT NULL,   -- e.g. 'image/jpeg', 'image/png', 'image/gif'
    file_path   TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(comment_id) REFERENCES comments(id) ON DELETE CASCADE
);