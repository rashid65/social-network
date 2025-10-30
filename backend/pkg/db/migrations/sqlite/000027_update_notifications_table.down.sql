-- Revert sender_id column addition
CREATE TABLE notifications_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN (
        'follow_request',
        'follow_success', 
        'follow',
        'follow_accepted',
        'follow_rejected',
        'unfollow',
        'group_invitation',
        'group_invitation_response',
        'group_event_created',
        'group_join_request',
        'group_request_approved',
        'group_request_declined',
        'message'
    )),
    ref_id TEXT,
    is_read INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    message TEXT,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Copy data back without sender_id
INSERT INTO notifications_old (id, user_id, type, ref_id, is_read, created_at, message)
SELECT id, user_id, type, ref_id, is_read, created_at, message
FROM notifications;

-- Drop new table and rename old one back
DROP TABLE notifications;
ALTER TABLE notifications_old RENAME TO notifications;