-- Reverse the group posts support migration
CREATE TABLE posts_old (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    author_id   TEXT    NOT NULL,
    content     TEXT    NOT NULL,
    privacy     TEXT    NOT NULL CHECK(privacy IN ('public','followers','custom')),
    liked       INTEGER DEFAULT 0, 
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(author_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Copy data (excluding group posts)
INSERT INTO posts_old SELECT id, author_id, content, privacy, liked, created_at, updated_at FROM posts WHERE privacy != 'group';

-- Drop current table and rename old one
DROP TABLE posts;
ALTER TABLE posts_old RENAME TO posts;