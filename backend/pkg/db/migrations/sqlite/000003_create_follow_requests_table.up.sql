-- Follow requests
CREATE TABLE follow_requests (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    requester_id  TEXT    NOT NULL,
    recipient_id  TEXT    NOT NULL,
    status        TEXT    NOT NULL CHECK(status IN ('pending','accepted','declined')),
    created_at    TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    responded_at  TEXT    NULL,
    FOREIGN KEY(requester_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(recipient_id) REFERENCES users(id) ON DELETE CASCADE
);