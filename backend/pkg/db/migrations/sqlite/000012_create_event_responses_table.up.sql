-- Event responses (RSVP)
CREATE TABLE event_responses (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id     INTEGER NOT NULL,
    user_id      TEXT    NOT NULL,
    response     TEXT    NOT NULL CHECK(response IN ('going','not_going')),
    responded_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(event_id) REFERENCES events(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id)  REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(event_id, user_id)
);