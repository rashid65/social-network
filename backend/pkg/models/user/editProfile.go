package user

import (
	"fmt"
	"social-network/pkg/db"
	"social-network/pkg/models/follow"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type EditProfileRequest struct {
	FirstName          *string `json:"first_name,omitempty"`
	LastName           *string `json:"last_name,omitempty"`
	Nickname           *string `json:"nickname,omitempty"`
	Email              *string `json:"email,omitempty"`
	IsPublic           *bool   `json:"is_public,omitempty"`
	AboutMe            *string `json:"about_me,omitempty"`
	AvatarPath         *string `json:"avatar_path,omitempty"` // Changed from Avatar to AvatarPath
	DOB                *string `json:"dob,omitempty"`
	OldPassword        *string `json:"old_password,omitempty"`
	NewPassword        *string `json:"new_password,omitempty"`
	ConfirmNewPassword *string `json:"confirm_new_password,omitempty"`
}

func UpdateUserProfile(userID string, req *EditProfileRequest, followService *follow.FollowService) error {
	// Build dynamic query based on provided fields
	var setParts []string
	var args []interface{}
	var changingToPublic bool
	changingToPublic = false

	if req.FirstName != nil {
		setParts = append(setParts, "first_name = ?")
		args = append(args, *req.FirstName)
	}

	if req.LastName != nil {
		setParts = append(setParts, "last_name = ?")
		args = append(args, *req.LastName)
	}

	if req.AboutMe != nil {
		setParts = append(setParts, "about_me = ?")
		args = append(args, *req.AboutMe)
	}

	if req.AvatarPath != nil { // Changed from req.Avatar to req.AvatarPath
		setParts = append(setParts, "avatar_path = ?")
		args = append(args, *req.AvatarPath) // Changed from *req.Avatar to *req.AvatarPath
	}

	if req.IsPublic != nil {
		setParts = append(setParts, "is_public = ?")
		var publicValue int
		if *req.IsPublic {
			publicValue = 1
			changingToPublic = true
		} else {
			publicValue = 0
		}
		args = append(args, publicValue)
	}

	if req.Nickname != nil {
		setParts = append(setParts, "nickname = ?")
		args = append(args, *req.Nickname)
	}

	if req.Email != nil {
		// Validate email
		if valid, err := ValidateEmail(*req.Email); !valid {
			return fmt.Errorf("invalid email: %v", err)
		}
		setParts = append(setParts, "email = ?")
		args = append(args, *req.Email)
	}

	if req.NewPassword != nil {
		// Validate password
		if valid, err := ValidatePassword(*req.NewPassword); !valid {
			return fmt.Errorf("invalid password: %v", err)
		}
		// Hash the password before saving!
		hashed, err := bcrypt.GenerateFromPassword([]byte(*req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %v", err)
		}
		setParts = append(setParts, "password = ?")
		args = append(args, string(hashed))
	}

	if req.DOB != nil {
		setParts = append(setParts, "dob = ?")
		args = append(args, *req.DOB)
	}

	// Password change logic
	if req.OldPassword != nil && req.NewPassword != nil && req.ConfirmNewPassword != nil {
		// Fetch current password hash
		var currentHash string
		err := db.DB.QueryRow("SELECT password FROM users WHERE id = ?", userID).Scan(&currentHash)
		if err != nil {
			return fmt.Errorf("failed to verify old password: %v", err)
		}
		// Check old password
		if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(*req.OldPassword)); err != nil {
			return fmt.Errorf("old password is incorrect")
		}
		// Hash new password
		hashed, err := bcrypt.GenerateFromPassword([]byte(*req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash new password: %v", err)
		}
		setParts = append(setParts, "password = ?")
		args = append(args, string(hashed))
	}

	// If no fields to update, return
	if len(setParts) == 0 {
		return fmt.Errorf("no fields provided to update")
	}

	// Add userID to args for WHERE clause
	args = append(args, userID)

	// Build and execute query
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = ?", strings.Join(setParts, ", "))

	result, err := db.DB.Exec(query, args...)
	if err != nil {
		return err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found or no changes made")
	}

	if changingToPublic {
		if err := AcceptAllPendingFollowRequests(userID, followService); err != nil {
			return fmt.Errorf("profile updated but failed to accept follow requests: %v", err)
		}
	}

	return nil
}

func AcceptAllPendingFollowRequests(userID string, followService *follow.FollowService) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get all pending follow requests for the user
	rows, err := tx.Query(`
        SELECT requester_id, recipient_id, created_at 
        FROM follow_requests 
        WHERE recipient_id = ? AND status = 'pending'
    `, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Store requester IDs for notifications
	var requesterIDs []string

	// Prepare statement to insert into followers table
	insertStmt, err := tx.Prepare(`
        INSERT INTO followers (follower_id, followee_id, created_at)
        VALUES (?, ?, ?)
    `)
	if err != nil {
		return err
	}
	defer insertStmt.Close()

	// Prepare statement to update follow_requests status
	updateStmt, err := tx.Prepare(`
        UPDATE follow_requests 
        SET status = 'accepted', responded_at = datetime('now')
        WHERE requester_id = ? AND recipient_id = ?
    `)
	if err != nil {
		return err
	}
	defer updateStmt.Close()

	// Process each pending request
	for rows.Next() {
		var recipientID string
		var followerID string
		var createdAt string

		if err := rows.Scan(&followerID, &recipientID, &createdAt); err != nil {
			return err
		}

		// Store requester ID for notifications
		requesterIDs = append(requesterIDs, followerID)

		// Add to followers
		_, err = insertStmt.Exec(followerID, userID, createdAt)
		if err != nil {
			return err
		}

		// Update follow request status
		_, err = updateStmt.Exec(followerID, userID)
		if err != nil {
			return err
		}
	}

	if err = rows.Err(); err != nil {
		return err
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return err
	}

	// Send notifications to all requesters after successful commit
	for _, requesterID := range requesterIDs {
		// Use the follow service to send accept notification
		followService.SendAcceptNotification(requesterID, userID)
	}

	return nil
}
