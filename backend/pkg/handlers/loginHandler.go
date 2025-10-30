package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"social-network/pkg/auth"
	"social-network/pkg/models/user"
	"social-network/pkg/utils"
)

// LoginHandler handles user login
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req user.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	userData, token, err := user.Login(req)
	if err != nil {
		// default error status
		status := http.StatusBadRequest
		switch err {
		// ovride when needed
		case user.ErrInvalidCredentials:
			status = http.StatusUnauthorized
		case user.ErrUserNotFound:
			status = http.StatusNotFound
		default:
			log.Printf("Error during login: %v", err)
			utils.WriteErrorJSON(w, "Internal server error", http.StatusInternalServerError)
		}

		utils.WriteErrorJSON(w, err.Error(), status)
		return
	}

	// dont return the password hash (security best practice)
	userData.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":   userData,
		"token":  token,
		"status": http.StatusOK,
	})
}

// logouthandler handlers user logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// get the token from authorization header
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		utils.WriteErrorJSON(w, "No token provided", http.StatusUnauthorized)
		return
	}

	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:] // remove "Bearer " prefix
	}

	// Invalidate the token (delete from sessions table)
	err := auth.InvalidateToken(tokenString)
	if err != nil {
		log.Printf("Error invalidating token: %v", err)
		utils.WriteErrorJSON(w, "Failed to log out", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	utils.WriteSuccessJSON(w, map[string]string{"message": "Logged out successfully"}, http.StatusOK)
}
