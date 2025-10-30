package handlers

import (
	"encoding/json"
	"net/http"
	"social-network/pkg/db"
	"social-network/pkg/sockets/websocket"
	"social-network/pkg/utils"
	"strconv"
	"time"
)

func HandleWebSocket(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get userID from session
		userID, ok := r.Context().Value("userID").(string)
		if !ok || userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
			return
		}

		websocket.ServeWS(hub, w, r, userID)
	}
}

func CreateNotificationHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := r.Context().Value("userID").(string)
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
			return
		}

		var req struct {
			RecipientID string `json:"recipient_id"`
			Type        string `json:"type"`
			RefID       string `json:"ref_id"`
			Message     string `json:"message"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// validate
		if req.RecipientID == "" {
			utils.WriteErrorJSON(w, "Recipient ID is required", http.StatusBadRequest)
			return
		}
		if req.Type == "" {
			utils.WriteErrorJSON(w, "Notification type is required", http.StatusBadRequest)
			return
		}
		if req.RefID == "" {
			utils.WriteErrorJSON(w, "Reference ID is required", http.StatusBadRequest)
			return
		}

		validTypes := map[string]bool{
			"follow_request":            true,
			"follow_success":            true,
			"follow":                    true,
			"follow_accepted":           true,
			"follow_rejected":           true,
			"unfollow":                  true,
			"group_invitation":          true,
			"group_invitation_response": true,
			"group_event_created":       true,
			"group_join_request":        true,
			"group_request_approved":    true,
			"group_request_declined":    true,
			"message":                   true,
		}

		if !validTypes[req.Type] {
			utils.WriteErrorJSON(w, "Invalid notification type", http.StatusBadRequest)
			return
		}

		notification := websocket.Notification{
			UserID:   req.RecipientID,
			SenderID: userID,
			Type:     req.Type,
			RefID:    req.RefID,
			IsRead:   false,
			Message:  req.Message,
		}

		// Create notification and get the database-generated ID
		notificationID, err := websocket.CreateNotificationAndGetID(db.DB, notification)
		if err != nil {
			utils.WriteErrorJSON(w, "Error creating notification: "+err.Error(), http.StatusInternalServerError)
			return
		}

		//send via websocket as well with actual database ID
		notificationMsg := websocket.NotificationMessage{
			ID:          strconv.Itoa(notificationID), // Use actual database ID
			SenderID:    userID,
			RecipientID: req.RecipientID,
			Type:        req.Type,
			RefID:       req.RefID,
			Message:     req.Message,
			Timestamp:   time.Now(),
		}

		go hub.SendNotificationToUser(req.RecipientID, notificationMsg)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":         "Notification created successfully",
			"notification_id": notificationID, // Return actual database ID
			"recipient_id":    req.RecipientID,
			"type":            req.Type,
		})
	}

}

func GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	notifications, err := websocket.GetNotificationsByUserID(db.DB, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Error fetching notifications", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(notifications)
}

func MarkNotificationAsReadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	notificationIDStr := r.URL.Query().Get("id")
	if notificationIDStr == "" {
		utils.WriteErrorJSON(w, "Notification ID is required", http.StatusBadRequest)
		return
	}

	notificationID, err := strconv.Atoi(notificationIDStr)
	if err != nil {
		utils.WriteErrorJSON(w, "Invalid notification ID format", http.StatusBadRequest)
		return
	}

	err = websocket.MarkAsRead(db.DB, notificationID)
	if err != nil {
		utils.WriteErrorJSON(w, "Error marking notification as read: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Notification marked as read"})
}

func GetUserChatsHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := r.Context().Value("userID").(string)
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
			return
		}

		// Create chat service instance
		chatService := websocket.NewChatService(db.DB)

		chats, err := chatService.GetUserChats(userID)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to get user chats: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(chats)
	}
}
