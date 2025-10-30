package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"social-network/pkg/db"
	"social-network/pkg/db/sqlite"
	"social-network/pkg/models/user"
	"social-network/pkg/sockets/websocket"
	"social-network/pkg/utils"
	"strconv"
	"strings"
)

func GetUserByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user ID from context
	authenticatedUserID, ok := r.Context().Value("userID").(string)
	if !ok || authenticatedUserID == "" {
		utils.WriteErrorJSON(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req struct {
		Id string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID := req.Id
	if userID == "" {
		// If no userID provided, return authenticated user's profile
		userID = authenticatedUserID
	}

	// Validate UUID format (optional but recommended)
	if !isValidUUID(userID) {
		utils.WriteErrorJSON(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	// Get user data from database
	userData, err := user.GetUserByID(userID, authenticatedUserID)
	if err != nil {
		if err.Error() == "user not found" || err.Error() == "sql: no rows in result set" {
			utils.WriteErrorJSON(w, "User not found", http.StatusNotFound)
			return
		}
		utils.WriteErrorJSON(w, "Failed to get user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return user data as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userData)
}

// Helper function to validate UUID format
func isValidUUID(uuid string) bool {
	// UUID v4 regex pattern
	uuidPattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`
	matched, _ := regexp.MatchString(uuidPattern, uuid)
	return matched
}

// DevRollbackHandler handles rolling back migrations (development only)
func DevRollbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get number of steps from query parameters
	stepsStr := r.URL.Query().Get("steps")
	if stepsStr == "" {
		stepsStr = "1" // Default to 1 step if not provided
	}

	steps, err := strconv.Atoi(stepsStr)
	if err != nil {
		http.Error(w, "Invalid steps parameter", http.StatusBadRequest)
		return
	}

	dbPath := "./social-network.db"
	migrationsDir := "./pkg/db/migrations/sqlite"

	err = sqlite.RollbackMigrations(dbPath, migrationsDir, steps)
	if err != nil {
		http.Error(w, "Failed to roll back migrations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"message": fmt.Sprintf("Successfully rolled back %d migration(s)", steps),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DevMigrationstatsuHandler shows current migration status
func DevMigrationStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dbPath := "./social-network.db"
	migrationsDir := "./pkg/db/migrations/sqlite"

	version, dirty, err := sqlite.GetMigrationVersion(dbPath, migrationsDir)
	if err != nil {
		http.Error(w, "Failed to get migration version: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"current_version": version,
		"dirty":           dirty,
		"status":          "healthy",
	}

	if dirty {
		response["status"] = "dirty - manual intervention required"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func DevClearDbHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Delete all sessions and users
	_, err := db.DB.Exec("DELETE FROM sessions")
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to clear sessions", http.StatusInternalServerError)
		return
	}

	_, err = db.DB.Exec("DELETE FROM users")
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to clear users", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Database cleared successfully"))
}

type AuthTestResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	SessionInfo string `json:"session_info,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	Nickname    string `json:"nickname,omitempty"`
	Email       string `json:"email,omitempty"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	DOB         string `json:"dob,omitempty"` // Date of Birth
	AboutMe     string `json:"about_me,omitempty"`
	Avatar      string `json:"avatar,omitempty"`     // Avatar path
	CreatedAt   string `json:"created_at,omitempty"` // Creation date
}

func AuthTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		response := AuthTestResponse{
			Success: false,
			Message: "Authentication failed: UserID not found in context",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get user data from database to verify session is valid
	userData, err := user.GetUserByID(userID, userID)
	if err != nil {
		response := AuthTestResponse{
			Success: false,
			Message: "Failed to get user data: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	tokenString := r.Header.Get("Authorization")
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	// Success response with user info
	response := AuthTestResponse{
		Success:     true,
		Message:     "Authentication successful! Session is valid.",
		SessionInfo: "Token validated successfully",
		UserID:      userID,
		Nickname:    userData.Nickname,
		Email:       userData.Email,
		FirstName:   userData.FirstName,
		LastName:    userData.LastName,
		DOB:         userData.DOB, // Date of Birth
		AboutMe:     userData.AboutMe,
		Avatar:      userData.Avatar,    // Avatar path
		CreatedAt:   userData.CreatedAt, // Creation date
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func UpdateNotificationMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		ID      int    `json:"id"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate notification ID
	if req.ID <= 0 {
		utils.WriteErrorJSON(w, "Valid notification ID is required", http.StatusBadRequest)
		return
	}

	// Validate message
	if strings.TrimSpace(req.Message) == "" {
		utils.WriteErrorJSON(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}

	if len(req.Message) > 500 {
		utils.WriteErrorJSON(w, "Message too long (max 500 characters)", http.StatusBadRequest)
		return
	}

	// Check if notification exists
	_, err := websocket.GetNotificationByID(db.DB, req.ID)
	if err != nil {
		if err.Error() == "notification not found" {
			utils.WriteErrorJSON(w, "Notification not found", http.StatusNotFound)
			return
		}
		utils.WriteErrorJSON(w, "Error retrieving notification: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update the notification message
	err = websocket.UpdateNotificationMessage(db.DB, req.ID, strings.TrimSpace(req.Message))
	if err != nil {
		utils.WriteErrorJSON(w, "Error updating notification message: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get updated notification
	updatedNotification, err := websocket.GetNotificationByID(db.DB, req.ID)
	if err != nil {
		utils.WriteErrorJSON(w, "Error retrieving updated notification: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response with updated notification
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"message":      "Notification message updated successfully",
		"notification": updatedNotification,
	})
}

func GetBatchUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user ID from context
	authenticatedUserID, ok := r.Context().Value("userID").(string)
	if !ok || authenticatedUserID == "" {
		utils.WriteErrorJSON(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req struct {
		UserIDs []string `json:"user_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.UserIDs) == 0 {
		utils.WriteErrorJSON(w, "user_ids array cannot be empty", http.StatusBadRequest)
		return
	}

	// Validate UUID format for all user IDs
	for _, userID := range req.UserIDs {
		if !isValidUUID(userID) {
			utils.WriteErrorJSON(w, fmt.Sprintf("Invalid user ID format: %s", userID), http.StatusBadRequest)
			return
		}
	}

	// Get user data from database for each user ID
	var users []interface{}
	for _, userID := range req.UserIDs {
		userData, err := user.GetUserByID(userID, authenticatedUserID)
		if err != nil {
			if err.Error() == "user not found" || err.Error() == "sql: no rows in result set" {
				log.Printf("User not found: %s", userID)
				// Skip users that don't exist instead of returning an error
				continue
			}
			utils.WriteErrorJSON(w, "Failed to get user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Add user data to results
		users = append(users, userData)
	}

	// Return users data as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(users)
}
