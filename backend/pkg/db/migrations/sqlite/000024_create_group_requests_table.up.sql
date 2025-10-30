CREATE TABLE group_requests (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    requester_id    INTEGER NOT NULL,
    group_id        INTEGER NOT NULL,
    status          TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    responded_at    TEXT    NULL
);


