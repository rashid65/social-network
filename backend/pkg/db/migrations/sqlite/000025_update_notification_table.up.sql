-- Add check constraint for notification types
CREATE TABLE notifications_new (
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
    message TEXT
);

-- Copy existing data
INSERT INTO notifications_new (id, user_id, type, ref_id, is_read, created_at)
SELECT id, user_id, type, ref_id, is_read, created_at
FROM notifications;

-- Drop old table and rename new one
DROP TABLE notifications;
ALTER TABLE notifications_new RENAME TO notifications;