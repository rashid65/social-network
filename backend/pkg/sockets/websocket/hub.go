package websocket

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Hub works as the central point for websocket connections
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Register requests from the clients
	register chan *Client

	// Unregister requests from the clients
	unregister chan *Client

	// Inbond message from the clients
	broadcast chan []byte

	// user-specific connections
	userConnections map[string][]*Client

	// Typing staus tracker
	typingUsers map[string]map[string]*TypingMessage // map[chatID]map[userID]*TypingMessage

	// User status tracker
	userStatus map[string]*UserStatusMessage // map[userID]*UserStatusMessage

	// database service
	chatService *ChatService

	// Mutex to protect the server
	mutex sync.RWMutex

	// Channel to stop the hub
	stop chan struct{}
}

// Function to create a new Hub with better channel sizes
func NewHub(db *sql.DB) *Hub {
	return &Hub{
		clients:         make(map[*Client]bool),
		register:        make(chan *Client, 1000), // Increased buffer size
		unregister:      make(chan *Client, 1000), // Increased buffer size
		broadcast:       make(chan []byte, 10000), // Increased buffer size
		chatService:     NewChatService(db),
		userConnections: make(map[string][]*Client),
		typingUsers:     make(map[string]map[string]*TypingMessage),
		userStatus:      make(map[string]*UserStatusMessage),
		stop:            make(chan struct{}),
	}
}

// main function to run the Hub ongoing loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case message := <-h.broadcast:
			h.handleBroadcast(message)

		case <-h.stop:
			log.Println("[WS] Hub stopping...")
			return
		}
	}
}

func (h *Hub) handleRegister(client *Client) {
	h.mutex.Lock()
	h.clients[client] = true
	h.addUserConnectionUnsafe(client)
	h.mutex.Unlock()

	h.updateUserStatus(client.userID, true)

	log.Printf("[WS] Client registered: %s (Total clients: %d)", client.userID, len(h.clients))

	// Send chat list with online status to the newly connected user (non-blocking)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[WS] Panic in sending chat list: %v", r)
			}
		}()

		chats, err := h.chatService.GetUserChats(client.userID)
		if err != nil {
			log.Printf("[WS] Error getting user chats for %s: %v", client.userID, err)
			return
		}

		// Update online status for each chat
		h.updateChatsWithOnlineStatus(chats, client.userID)

		h.SendChatListToUser(client.userID, chats)
	}()

	// Broadcast user status to related users
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[WS] Panic in broadcasting user status: %v", r)
			}
		}()
		h.broadcastUserStatus(client.userID, true)
	}()
}

// Add this simple method to hub.go
func (h *Hub) updateChatsWithOnlineStatus(chats []ChatRoom, currentUserID string) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for i := range chats {
		// Reset online status
		chats[i].IsOnline = false

		if chats[i].Type == "private" {
			// For private chats, check if the other participant is online
			for _, participantID := range chats[i].Participants {
				if participantID != currentUserID {
					if status, exists := h.userStatus[participantID]; exists && status.IsOnline {
						chats[i].IsOnline = true
						break
					}
				}
			}
		} else if chats[i].Type == "group" {
			// For group chats, check if any participant (except current user) is online
			onlineCount := 0
			for _, participantID := range chats[i].Participants {
				if participantID != currentUserID {
					if status, exists := h.userStatus[participantID]; exists && status.IsOnline {
						onlineCount++
					}
				}
			}
			chats[i].IsOnline = onlineCount > 0
			chats[i].MemberCount = len(chats[i].Participants)
		}
	}
}

func (h *Hub) handleUnregister(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		h.removeUserConnectionUnsafe(client)

		// Close client channel safely
		select {
		case <-client.send:
			// Channel already closed
		default:
			// Only close if not already closed
			close(client.send)
		}

		// Check if the user has any other connections
		if len(h.userConnections[client.userID]) == 0 {
			go func() {
				h.updateUserStatus(client.userID, false)
				h.broadcastUserStatus(client.userID, false)
			}()
		}

		log.Printf("[WS] Client unregistered: %s (Total clients: %d)", client.userID, len(h.clients))
	}
}

func (h *Hub) handleBroadcast(message []byte) {
	h.mutex.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mutex.RUnlock()

	// Send to clients without holding the main lock
	for _, client := range clients {
		select {
		case client.send <- message:
			// Message sent successfully
		default:
			// Channel is full or closed, unregister client
			log.Printf("[WS] Failed to send broadcast message - channel blocked for user: %s", client.userID)
			go func(c *Client) {
				select {
				case h.unregister <- c:
				default:
					log.Printf("[WS] Failed to unregister client %s - unregister channel full", c.userID)
				}
			}(client)
		}
	}
}

func (h *Hub) addUserConnectionUnsafe(client *Client) {
	h.userConnections[client.userID] = append(h.userConnections[client.userID], client)
}

func (h *Hub) addUserConnection(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.addUserConnectionUnsafe(client)
}

func (h *Hub) removeUserConnectionUnsafe(client *Client) {
	connections := h.userConnections[client.userID]
	for i, conn := range connections {
		if conn == client {
			h.userConnections[client.userID] = append(connections[:i], connections[i+1:]...)
			break
		}
	}

	if len(h.userConnections[client.userID]) == 0 {
		delete(h.userConnections, client.userID)
	}
}

func (h *Hub) removeUserConnection(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.removeUserConnectionUnsafe(client)
}

func (h *Hub) updateUserStatus(userID string, isOnline bool) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.userStatus[userID] = &UserStatusMessage{
		UserID:   userID,
		IsOnline: isOnline,
		LastSeen: time.Now(),
	}
}

func (h *Hub) broadcastUserStatus(userID string, isOnline bool) {
	// Get users who should be notified about this user's status change
	relatedUsers, err := h.chatService.getRelatedUsers(userID)
	if err != nil {
		log.Printf("[WS] Error getting related users for status broadcast: %v", err)
		return
	}

	message := WSMessage{
		Type: TypeUserStatusUpdate,
		Data: UserStatusMessage{
			UserID:   userID,
			IsOnline: isOnline,
			LastSeen: time.Now(),
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("[WS] Error marshaling user status message: %v", err)
		return
	}

	// Send status update to related users (non-blocking)
	go h.SendToUsers(relatedUsers, data)

	// Send updated chat list to each related user to show updated online status
	for _, relatedUserID := range relatedUsers {
		go func(targetUserID string) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[WS] Panic in sending updated chat list to %s: %v", targetUserID, r)
				}
			}()

			// Small delay to prevent message storm
			time.Sleep(100 * time.Millisecond)

			// Get updated chat list for the related user
			chats, err := h.chatService.GetUserChats(targetUserID)
			if err != nil {
				log.Printf("[WS] Error getting chats for user %s: %v", targetUserID, err)
				return
			}

			// Update online status for each chat
			h.updateChatsWithOnlineStatus(chats, targetUserID)

			// Send updated chat list
			h.SendChatListToUser(targetUserID, chats)
		}(relatedUserID)
	}
}

// Send message to a specific user (non-blocking)
func (h *Hub) SendToUser(userID string, message []byte) {
	h.mutex.RLock()
	connections := make([]*Client, len(h.userConnections[userID]))
	copy(connections, h.userConnections[userID])
	h.mutex.RUnlock()

	if len(connections) == 0 {
		return
	}

	log.Printf("[WS] Sending message to user: %s", userID)

	for _, client := range connections {
		// Check if client is still registered before sending
		h.mutex.RLock()
		_, exists := h.clients[client]
		h.mutex.RUnlock()

		if !exists {
			continue // Skip clients that are no longer registered
		}

		select {
		case client.send <- message:
			// Message sent successfully
		default:
			log.Printf("[WS] Failed to send message - channel blocked for user: %s", userID)
			// Don't block here, just skip this client
			go func(c *Client) {
				select {
				case h.unregister <- c:
				default:
					log.Printf("[WS] Failed to unregister client %s - unregister channel full", c.userID)
				}
			}(client)
		}
	}
}

func (h *Hub) SendToUsers(userIDs []string, message []byte) {
	for _, userID := range userIDs {
		go h.SendToUser(userID, message)
	}
}

func (h *Hub) HandleTyping(chatID, userID, nickName string, isTyping bool) {
	log.Printf("[WS] HandleTyping: user=%s, chat=%s, isTyping=%v", userID, chatID, isTyping)

	// Create the typing message that we'll broadcast
	typingMessage := TypingMessage{
		UserID:   userID,
		NickName: nickName,
		ChatID:   chatID,
		IsTyping: isTyping,
	}

	// Create array with the single typing message (whether true or false)
	typingData := []TypingMessage{typingMessage}

	message := WSMessage{
		Type:      TypeTyping,
		Data:      typingData,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("[WS] Error marshaling typing message: %v", err)
		return
	}

	log.Printf("[WS] Sending typing data: %s", string(data))

	participants, err := h.chatService.getChatParticipants(chatID)
	if err != nil {
		log.Printf("[WS] Error getting chat participants: %v", err)
		return
	}

	// Send the typing message to all participants
	h.SendToUsers(participants, data)

	// Now update the internal typing state AFTER broadcasting
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.typingUsers[chatID] == nil {
		h.typingUsers[chatID] = make(map[string]*TypingMessage)
	}

	if isTyping {
		h.typingUsers[chatID][userID] = &typingMessage
		log.Printf("[WS] User %s started typing in chat %s", userID, chatID)
	} else {
		delete(h.typingUsers[chatID], userID)
		if len(h.typingUsers[chatID]) == 0 {
			delete(h.typingUsers, chatID)
		}
		log.Printf("[WS] User %s stopped typing in chat %s", userID, chatID)
	}
}

func (h *Hub) GetOnlineUsers(requestingUserID string) []string {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var onlineUsers []string

	// Get the list of users that the requesting user follows or is followed by
	relatedUsers, err := h.chatService.getRelatedUsers(requestingUserID)
	if err != nil {
		log.Printf("[WS] Error getting related users for %s: %v", requestingUserID, err)
		return onlineUsers
	}

	// Create a map for faster lookup
	relatedUsersMap := make(map[string]bool)
	for _, userID := range relatedUsers {
		relatedUsersMap[userID] = true
	}

	// Only include online users who are related to the requesting user
	for userID, status := range h.userStatus {
		if status.IsOnline && userID != requestingUserID && relatedUsersMap[userID] {
			onlineUsers = append(onlineUsers, userID)
		}
	}

	return onlineUsers
}

// Stop the hub gracefully
func (h *Hub) Stop() {
	close(h.stop)
}
