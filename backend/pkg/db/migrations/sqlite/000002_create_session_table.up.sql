-- Sessions table
CREATE TABLE sessions (
    id           TEXT    PRIMARY KEY,
    user_id      TEXT    NOT NULL,
    token        TEXT    NOT NULL UNIQUE,
    created_at   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at   TEXT    NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);