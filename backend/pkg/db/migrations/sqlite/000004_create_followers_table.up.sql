-- Followers (accepted relationships)
CREATE TABLE followers (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    follower_id  TEXT    NOT NULL,
    followee_id  TEXT    NOT NULL,
    created_at   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(follower_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(followee_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(follower_id, followee_id)
);