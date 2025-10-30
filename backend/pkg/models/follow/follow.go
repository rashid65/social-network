package follow

import (
	"database/sql"
	"social-network/pkg/sockets/websocket"
)

type FollowRequest struct {
	FollowerID   string   `json:"follower_id"`
	FolloweeID   string   `json:"followee_id"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at"`
}

type FollowNotification struct {
	Type         string   `json:"type"`
	FollowerID   string   `json:"follower_is"`
	FolloweeID   string   `json:"followee_id"`
	Status       string   `json:"status"`
	Message      string   `json:"message"`
}

type FollowService struct {
	DB    *sql.DB
	Hub   WebSocketHub
}

type WebSocketHub interface {
	SendNotificationToUser(userID string, notification websocket.NotificationMessage)
}
