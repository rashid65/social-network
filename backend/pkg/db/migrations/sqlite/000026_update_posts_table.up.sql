ALTER TABLE posts ADD COLUMN group_id INTEGER NULL;

-- Add foreign key constraint (SQLite doesn't support adding constraints directly)
-- Create new table with constraint
CREATE TABLE posts_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    author_id   TEXT    NOT NULL,
    content     TEXT    NOT NULL,
    privacy     TEXT    NOT NULL CHECK(privacy IN ('public','followers','custom','group')),
    group_id    INTEGER NULL,
    liked       INTEGER DEFAULT 0, 
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(author_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
);

-- Copy data from old table
INSERT INTO posts_new SELECT id, author_id, content, privacy, NULL as group_id, liked, created_at, updated_at FROM posts;

-- Drop old table and rename new one
DROP TABLE posts;
ALTER TABLE posts_new RENAME TO posts;