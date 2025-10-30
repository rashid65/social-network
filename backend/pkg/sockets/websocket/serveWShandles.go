package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Increased limits and timeouts
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 2048 // Increased from 512 to 2048
)

type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	userID      string
	chatService *ChatService
}

// client to server
func (c *Client) readPump() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WS] Panic in readPump for user %s: %v", c.userID, r)
		}

		// Ensure unregistration happens
		select {
		case c.hub.unregister <- c:
		default:
			log.Printf("[WS] Failed to unregister client %s - channel full", c.userID)
		}

		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Unexpected close error for user %s: %v", c.userID, err)
			} else {
				log.Printf("[WS] Connection closed for user %s: %v", c.userID, err)
			}
			break
		}

		log.Printf("[WS] Message received from user %s: %s", c.userID, string(message))

		// Handle message in a goroutine to prevent blocking
		go func(msg []byte) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[WS] Panic in handleMessage for user %s: %v", c.userID, r)
				}
			}()
			c.handleMessage(msg)
		}(message)
	}
}

// server to client
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WS] Panic in writePump for user %s: %v", c.userID, r)
		}
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("[WS] Error getting writer for user %s: %v", c.userID, err)
				return
			}

			if _, err := w.Write(message); err != nil {
				log.Printf("[WS] Error writing message for user %s: %v", c.userID, err)
				w.Close()
				return
			}

			// Add queued messages to the current message
			n := len(c.send)
			for i := 0; i < n; i++ {
				select {
				case additionalMessage := <-c.send:
					w.Write([]byte{'\n'})
					w.Write(additionalMessage)
				default:
					break
				}
			}

			if err := w.Close(); err != nil {
				log.Printf("[WS] Error closing writer for user %s: %v", c.userID, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[WS] Error sending ping to user %s: %v", c.userID, err)
				return
			}
		}
	}
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, userID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 512), // Increased buffer size
		userID:      userID,
		chatService: hub.chatService,
	}

	// Register client with timeout
	select {
	case client.hub.register <- client:
		log.Printf("[WS] Client registered successfully: %s", userID)
	case <-time.After(5 * time.Second):
		log.Printf("[WS] Failed to register client - timeout for user: %s", userID)
		conn.Close()
		return
	}

	// Start the pumps
	go client.writePump()
	client.readPump() // This blocks until connection closes
}

func (c *Client) handleUserStatusUpdate(data interface{}) {
	statusMsg, err := unmarshalData[UserStatusMessage](data)
	if err != nil {
		log.Printf("[WS] Error unmarshaling user status update: %v", err)
		return
	}

	statusMsg.UserID = c.userID
	statusMsg.LastSeen = time.Now()

	c.hub.updateUserStatus(c.userID, statusMsg.IsOnline)
	c.hub.broadcastUserStatus(c.userID, statusMsg.IsOnline)
}

func (c *Client) handleOnlineUsersRequest(data interface{}) {
	// Send current online users to the client
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[WS] Panic in handleOnlineUsersRequest for user %s: %v", c.userID, r)
			}
		}()
		c.hub.SendOnlineUsersToUser(c.userID)
	}()
}

func (c *Client) handleChatListRequest(data interface{}) {
	// Get user's chat list from database
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[WS] Panic in handleChatListRequest for user %s: %v", c.userID, r)
			}
		}()

		chats, err := c.chatService.GetUserChats(c.userID)
		if err != nil {
			log.Printf("[WS] Error getting user chats for %s: %v", c.userID, err)
			return
		}

		// Update online status for each chat
		c.hub.updateChatsWithOnlineStatus(chats, c.userID)

		c.hub.SendChatListToUser(c.userID, chats)
	}()
}

func (c *Client) handleChatMessagesRequest(data interface{}) {
	// Get chat messages for a specific chat
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[WS] Panic in handleChatMessagesRequest for user %s: %v", c.userID, r)
			}
		}()

		req, err := unmarshalData[ChatMessagesRequest](data)
		if err != nil {
			log.Printf("[WS] Error unmarshaling chat messages request for user %s: %v", c.userID, err)
			c.sendChatMessagesError("Invalid request format")
			return
		}

		// Validate required fields
		if req.ChatID == "" {
			log.Printf("[WS] Chat ID missing in request from user %s", c.userID)
			c.sendChatMessagesError("Chat ID is required")
			return
		}

		// Set default values
		if req.Limit <= 0 || req.Limit > 100 {
			req.Limit = 50 // Default limit
		}
		if req.Offset < 0 {
			req.Offset = 0
		}

		// Check if user is a participant of the chat
		isParticipant, err := c.chatService.IsUserChatParticipant(c.userID, req.ChatID)
		if err != nil {
			log.Printf("[WS] Error checking chat participation for user %s, chat %s: %v", c.userID, req.ChatID, err)
			c.sendChatMessagesError("Error checking chat access")
			return
		}

		if !isParticipant {
			log.Printf("[WS] User %s is not a participant of chat %s", c.userID, req.ChatID)
			c.sendChatMessagesError("Access denied: You are not a participant of this chat")
			return
		}

		// Get chat messages
		messages, err := c.chatService.GetChatMessages(req.ChatID, req.Limit, req.Offset)
		if err != nil {
			log.Printf("[WS] Error getting chat messages for user %s, chat %s: %v", c.userID, req.ChatID, err)
			c.sendChatMessagesError("Error retrieving chat messages")
			return
		}

		// Get total message count for pagination
		total, err := c.chatService.GetChatMessageCount(req.ChatID)
		if err != nil {
			log.Printf("[WS] Error getting message count for chat %s: %v", req.ChatID, err)
			total = len(messages) // Fallback to current message count
		}

		// Determine if there are more messages
		hasMore := req.Offset+len(messages) < total

		// Create response
		response := ChatMessagesResponse{
			ChatID:   req.ChatID,
			Messages: messages,
			HasMore:  hasMore,
			Total:    total,
		}

		// Send response
		c.hub.SendChatMessagesToUser(c.userID, response)
	}()
}

func (c *Client) sendChatMessagesError(message string) {
	errorResponse := map[string]interface{}{
		"error":   true,
		"message": message,
		"type":    "chat_messages_error",
	}

	wsMessage := WSMessage{
		Type:      TypeChatMessages,
		Data:      errorResponse,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(wsMessage)
	c.hub.SendToUser(c.userID, msgData)
}
