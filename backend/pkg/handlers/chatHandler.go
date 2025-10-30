package handlers

import (
	"encoding/json"
	"net/http"
	"social-network/pkg/db"
	"social-network/pkg/sockets/websocket"
	"social-network/pkg/utils"
)

func CreatePrivateChatHandler(w http.ResponseWriter, r *http.Request) {
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
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.UserID == "" {
		utils.WriteErrorJSON(w, "Target userId is required", http.StatusBadRequest)
		return
	}
	if req.UserID == userID {
		utils.WriteErrorJSON(w, "Cannot create chat with yourself", http.StatusBadRequest)
		return
	}

	chatService := websocket.NewChatService(db.DB)
	chatRoom, err := chatService.GetOrCreatePrivateChat(userID, req.UserID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to create/fetch private chat: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatRoom)
}
