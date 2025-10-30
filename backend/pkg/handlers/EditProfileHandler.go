package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"social-network/pkg/models/user"
	"social-network/pkg/utils"
	"strings"
	"social-network/pkg/models/follow"
)

type EditProfileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

func EditProfileHandler(w http.ResponseWriter, r *http.Request, fs follow.FollowService) {
	if r.Method != http.MethodPut {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context (set by auth middleware)
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req user.EditProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if err := validateEditProfileRequest(&req); err != nil {
		utils.WriteErrorJSON(w, "Validation error: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Update the user profile
	err := user.UpdateUserProfile(userID, &req, &fs)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to update profile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response
	response := EditProfileResponse{
		Success: true,
		Message: "Profile updated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// validateEditProfileRequest validates the profile edit request
func validateEditProfileRequest(req *user.EditProfileRequest) error {
	if req.FirstName != nil {
		if valid, err := user.ValidateName(*req.FirstName, "dummy"); !valid {
			return err
		}
	}
	if req.LastName != nil {
		if valid, err := user.ValidateName("dummy", *req.LastName); !valid {
			return err
		}
	}
	if req.Nickname != nil {
		if valid, err := user.ValidateNickname(*req.Nickname); !valid {
			return err
		}
	}
	if req.Email != nil {
		if valid, err := user.ValidateEmail(*req.Email); !valid {
			return err
		}
	}
	if req.AboutMe != nil {
		if valid, err := user.ValidateAboutMe(*req.AboutMe); !valid {
			return err
		}
	}
	if req.DOB != nil {
		if valid, err := user.ValidateDOB(*req.DOB); !valid {
			return err
		}
	}

	// Password change validation
	if req.NewPassword != nil || req.ConfirmNewPassword != nil || req.OldPassword != nil {
		if req.OldPassword == nil || req.NewPassword == nil || req.ConfirmNewPassword == nil {
			return fmt.Errorf("all password fields (old_password, new_password, confirm_new_password) are required to change password")
		}
		if *req.NewPassword != *req.ConfirmNewPassword {
			return fmt.Errorf("new password and confirm new password do not match")
		}
		if valid, err := user.ValidatePassword(*req.NewPassword); !valid {
			return err
		}
	}

	if req.AvatarPath != nil {
		// Basic validation for avatar path
		if *req.AvatarPath != "" && !strings.HasPrefix(*req.AvatarPath, "/uploads/") {
			return fmt.Errorf("invalid avatar path format")
		}
	}

	// is_public is a bool, no validation needed unless you want to restrict values
	return nil
}
