-- Messages
CREATE TABLE messages (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id      INTEGER NOT NULL,
    sender_id    TEXT    NOT NULL,
    content      TEXT    NOT NULL,
    message_type TEXT    NOT NULL CHECK(message_type IN ('text','emoji','media')),
    created_at   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(chat_id)   REFERENCES chat_threads(id) ON DELETE CASCADE,
    FOREIGN KEY(sender_id) REFERENCES users(id) ON DELETE CASCADE
);