package handlers

import (
	"net/http"
	"social-network/pkg/models/user"
	"social-network/pkg/utils"
)

func protectedHandler(w http.ResponseWriter, r *http.Request) {
	// verify the user is authenticated

	userID := r.Context().Value("userID").(string)

	// get user data from database
	userData, err := user.GetUserByID(userID, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get user data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// dont return the password hash
	userData.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	utils.WriteSuccessJSON(w, userData, http.StatusOK)

}
