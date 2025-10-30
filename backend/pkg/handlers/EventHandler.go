package handlers

import (
	"encoding/json"
	"net/http"

	"social-network/pkg/db"
	"social-network/pkg/models/event"
	"social-network/pkg/sockets/websocket"
	"social-network/pkg/utils"
)

// Handler for Creating Group Requests
func CreateEventHandler(hub *websocket.Hub) http.HandlerFunc {
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

		var newEvent event.Event
		if err := json.NewDecoder(r.Body).Decode(&newEvent); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		newEvent.CreatorID = userID // Set the creator ID to the authenticated user ID

		if err := newEvent.ValidateEventCreation(db.DB); err != nil {
			utils.WriteErrorJSON(w, "Invalid event: "+err.Error(), http.StatusBadRequest)
			return
		}

		event, err := event.CreateEvent(db.DB, newEvent, hub)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to create event: "+err.Error(), http.StatusInternalServerError)
			return
		}

		utils.WriteSuccessJSON(w, event, http.StatusCreated)
	}
}

// Handler for Creating/Updating Event Responses
func CreateEventResponseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId := r.Context().Value("userID").(string)
	if userId == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var newEventResponse event.EventResponse
	if err := json.NewDecoder(r.Body).Decode(&newEventResponse); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	newEventResponse.UserID = userId // Set the user ID to the authenticated user ID

	// Validate the event response (but skip duplicate check since we'll handle updates)
	if err := newEventResponse.ValidateEventResponse(db.DB); err != nil {
		utils.WriteErrorJSON(w, "Invalid event response: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Use REPLACE to update existing response or create new one
	query := `
        REPLACE INTO event_responses (event_id, user_id, response) 
        VALUES (?, ?, ?)
    `

	_, err := db.DB.Exec(query, newEventResponse.EventID, newEventResponse.UserID, newEventResponse.Response)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to record event response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteSuccessJSON(w, "Event response recorded successfully", http.StatusCreated)
}

// Handler for Getting Events for a Group
func GetGroupEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get groupId from query parameter: /api/event/group?groupId=123
	groupID := r.URL.Query().Get("groupId")
	if groupID == "" {
		utils.WriteErrorJSON(w, "Missing groupId query parameter", http.StatusBadRequest)
		return
	}

	// Get user ID from context (optional - if user is authenticated)
	userID := ""
	if userIDFromContext := r.Context().Value("userID"); userIDFromContext != nil {
		userID = userIDFromContext.(string)
	}

	events, err := event.GetEventsByGroupID(db.DB, groupID, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"events": events,
		"total":  len(events),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
