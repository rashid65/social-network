-- Group invitations
CREATE TABLE group_invitations (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id     INTEGER NOT NULL,
    inviter_id   TEXT    NOT NULL,
    invitee_id   TEXT    NOT NULL,
    status       TEXT    NOT NULL CHECK(status IN ('pending','accepted','declined')),
    created_at   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    responded_at TEXT    NULL,
    FOREIGN KEY(group_id)   REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY(inviter_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(invitee_id) REFERENCES users(id) ON DELETE CASCADE
);