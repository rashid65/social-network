package handlers

import (
	"encoding/json"
	"net/http"
	"social-network/pkg/models/user"
	"social-network/pkg/utils"
)

// dashboardHandler handles the dashboard page for authenticated users
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from context
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
		return
	}

	// get user data from database
	userData, err := user.GetUserByID(userID, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get user data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// dont return the password hash and ID
	userData.PasswordHash = ""
	userData.ID = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":   userData,
		"status": http.StatusOK,
	})
}
