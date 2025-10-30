-- Post media (images/GIFs)
CREATE TABLE post_media (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id     INTEGER NOT NULL,
    media_type  TEXT    NOT NULL,   -- e.g. 'image/jpeg', 'image/png', 'image/gif'
    file_path   TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE
);