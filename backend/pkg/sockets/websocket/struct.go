package websocket

import "time"

type MessageType string

const (
	TypeChat              MessageType = "chat"
	TypeTyping            MessageType = "typing"
	TypeGif               MessageType = "gif"
	TypeUserStatusUpdate  MessageType = "user_status_update"
	TypeChatList          MessageType = "chat_list"
	TypeMessagesRead      MessageType = "messages_read"
	TypeFollow            MessageType = "follow"
	TypeUnfollow          MessageType = "unfollow"
	TypeNotification      MessageType = "notification"
	TypeOnlineUsers       MessageType = "online_users"
	TypeGroupInvitation   MessageType = "group_invitation"
	TypeGroupEventCreated MessageType = "group_event_created"
	TypeChatMessages      MessageType = "chat_messages" // New message type
)

type WSMessage struct {
	Type      MessageType `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

type ChatMessage struct {
	ID           string    `json:"id"`
	ChatID       string    `json:"chat_id"`
	SenderID     string    `json:"sender_id"`
	SenderName   string    `json:"sender_name"`
	SenderAvatar string    `json:"sender_avatar"`
	Content      string    `json:"content"`
	MessageType  string    `json:"message_type"` // text, image, Emoji
	Timestamp    time.Time `json:"timestamp"`
	IsRead       bool      `json:"is_read"`
	RecipientID  string    `json:"recipient_id,omitempty"`
	GroupID      string    `json:"group_id,omitempty"`
}

type TypingMessage struct {
	UserID   string `json:"user_id"`
	NickName string `json:"user_name"`
	ChatID   string `json:"chat_id"`
	IsTyping bool   `json:"is_typing"`
}

type UserStatusMessage struct {
	UserID   string    `json:"user_id"`
	IsOnline bool      `json:"is_online"`
	LastSeen time.Time `json:"last_seen,omitempty"` // Optional, only if user is offline
}

type ChatListMessage struct {
	Chats []ChatRoom `json:"chats"`
}

type ChatRoom struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"` // private, group
	Name         string       `json:"name"`
	Avatar       string       `json:"avatar"`
	Participants []string     `json:"participants"` // User IDs
	LastMessage  *ChatMessage `json:"last_message,omitempty"`
	UnreadCount  int          `json:"unread_count"`
	IsOnline     bool         `json:"is_online"`
	MemberCount  int          `json:"member_count,omitempty"`
	GroupID      string       `json:"group_id,omitempty"`
}

type MessagesReadMessage struct {
	ChatID     string   `json:"chat_id"`
	MessageIDs []string `json:"message_ids"`
	UserID     string   `json:"user_id"`
}

//! Not used?
type FollowMessage struct {
	FollowerID   string `json:"follower_id"`
	FollowingID  string `json:"following_id"`
	FollowerName string `json:"follower_name"`
	Action       string `json:"action"`            // request, accept, reject, unfollow
	Message      string `json:"message,omitempty"` // For success/error messages
}

type NotificationMessage struct {
	ID           string    `json:"id"`
	SenderID     string    `json:"sender_id"`
	RecipientID  string    `json:"recipient_id"`
	Type         string    `json:"type"`
	RefID        string    `json:"ref_id"`
	Message      string    `json:"message"`
	IsRead       bool      `json:"is_read"`
	Timestamp    time.Time `json:"timestamp"`
	SenderAvatar string    `json:"sender_avatar"` // <-- Add this
}

type GroupInvitationMessage struct {
	ID          string    `json:"id"`
	GroupID     string    `json:"group_id"`
	GroupName   string    `json:"group_name"`
	InviterID   string    `json:"inviter_id"`
	InviterName string    `json:"inviter_name"`
	InviteeID   string    `json:"invitee_id"`
	InviteeName string    `json:"invitee_name"`
	Action      string    `json:"action"`            // send, accept, decline
	Message     string    `json:"message,omitempty"` // For success/error messages
	Timestamp   time.Time `json:"timestamp"`
}

type GroupEventCreatedMessage struct {
	Type        MessageType `json:"type"`
	EventID     string      `json:"event_id"`
	GroupID     string      `json:"group_id"`
	CreatorID   string      `json:"creator_id"`
	CreatorName string      `json:"creator_name"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	EventTime   string      `json:"event_time"`
	Timestamp   string      `json:"timestamp"`
}

// New structs for chat messages request/response
type ChatMessagesRequest struct {
	ChatID string `json:"chat_id"`
	Limit  int    `json:"limit,omitempty"`  // Optional, default 50
	Offset int    `json:"offset,omitempty"` // Optional, default 0
}

type ChatMessagesResponse struct {
	ChatID   string        `json:"chat_id"`
	Messages []ChatMessage `json:"messages"`
	HasMore  bool          `json:"has_more"`
	Total    int           `json:"total"`
}
