package websocket

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

// send chat list to user
func (h *Hub) SendChatListToUser(userID string, chatList []ChatRoom) {
	message := WSMessage{
		Type: TypeChatList,
		Data: ChatListMessage{
			Chats: chatList,
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("[WS] Error marshaling chat list message: %v", err)
		return
	}

	h.SendToUser(userID, data)
}

func (h *Hub) SendNotificationToUser(userID string, notification NotificationMessage) {
	notification.SenderAvatar = GetSenderAvatar(h.chatService.DB, notification.SenderID, notification.Type) // <-- Add this

	message := WSMessage{
		Type:      TypeNotification,
		Data:      notification,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("error marshalling notification message: %v", err)
		return
	}

	h.SendToUser(userID, data)
}

func (h *Hub) SendOnlineUsersToUser(userID string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WS] Panic in SendOnlineUsersToUser for %s: %v", userID, r)
		}
	}()

	onlineUsers := h.GetOnlineUsers(userID)

	message := WSMessage{
		Type:      TypeOnlineUsers,
		Data:      onlineUsers,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("[WS] Error marshaling online users message: %v", err)
		return
	}

	h.SendToUser(userID, data)
}

func (h *Hub) SendChatMessagesToUser(userID string, chatMessagesResponse ChatMessagesResponse) {
	message := WSMessage{
		Type:      TypeChatMessages,
		Data:      chatMessagesResponse,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("[WS] Error marshaling chat messages response: %v", err)
		return
	}

	h.SendToUser(userID, data)
}

func GetSenderAvatar(db *sql.DB, senderID, notifType string) string {
	// Special cases for group_kick and group_event_created
	if notifType == "group_kick" || notifType == "group_event_created" {
		return "/images/default-group.png"
	}
	var avatar string
	_ = db.QueryRow("SELECT COALESCE(avatar_path, '') FROM users WHERE id = ?", senderID).Scan(&avatar)
	if avatar == "" {
		avatar = "/images/default-avatar.jpg"
	}
	return avatar
}
