-- Chat participants
CREATE TABLE chat_participants (
    chat_id  INTEGER NOT NULL,
    user_id  TEXT    NOT NULL,
    PRIMARY KEY(chat_id, user_id),
    FOREIGN KEY(chat_id) REFERENCES chat_threads(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);