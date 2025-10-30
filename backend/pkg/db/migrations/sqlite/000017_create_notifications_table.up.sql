-- Notifications
CREATE TABLE notifications (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    TEXT    NOT NULL,  -- who is notified
    type       TEXT    NOT NULL CHECK(type IN ('follow_request','group_invite','group_join_request','event','chat_message','other')),
    ref_id     TEXT    NOT NULL,  -- references the triggering record’s PK
    is_read    INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);