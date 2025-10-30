package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"social-network/pkg/db"
	"social-network/pkg/models/group"
	"social-network/pkg/models/user"
	"social-network/pkg/sockets/websocket"
	"social-network/pkg/utils"
)

// Helper function to add user to group chat within a transaction
func addUserToGroupChatTx(tx *sql.Tx, userID, groupID string) error {
	// Get the group's chat thread ID
	var chatID int64
	err := tx.QueryRow(`
        SELECT id FROM chat_threads 
        WHERE is_group = 1 AND group_id = ?
    `, groupID).Scan(&chatID)
	if err != nil {
		return fmt.Errorf("failed to find group chat thread: %w", err)
	}

	// Add user as participant
	_, err = tx.Exec(`
        INSERT OR IGNORE INTO chat_participants (chat_id, user_id)
        VALUES (?, ?)
    `, chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to add user to group chat: %w", err)
	}

	return nil
}

// Helper function to remove user from group chat within a transaction
func removeUserFromGroupChatTx(tx *sql.Tx, userID, groupID string) error {
	// Get the group's chat thread ID
	var chatID int64
	err := tx.QueryRow(`
        SELECT id FROM chat_threads 
        WHERE is_group = 1 AND group_id = ?
    `, groupID).Scan(&chatID)
	if err != nil {
		return fmt.Errorf("failed to find group chat thread: %w", err)
	}

	// Remove user as participant
	_, err = tx.Exec(`
        DELETE FROM chat_participants 
        WHERE chat_id = ? AND user_id = ?
    `, chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user from group chat: %w", err)
	}

	return nil
}

// Handler for creating groups
func GroupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var newGroup group.Group
	if err := json.NewDecoder(r.Body).Decode(&newGroup); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	newGroup.CreatorID = userID // CreatorID = authenticated userID

	if err := newGroup.ValidateGroupCreation(); err != nil {
		utils.WriteErrorJSON(w, "Invalid group: "+err.Error(), http.StatusBadRequest)
		return
	}

	createGroup, err := group.CreateGroup(db.DB, newGroup)
	if err != nil {
		// Log the full error for debugging
		log.Printf("Group creation failed for user %s: %v", userID, err)

		// Return user-friendly error message
		if strings.Contains(err.Error(), "chat thread") {
			utils.WriteErrorJSON(w, "Failed to create group chat. Please try again.", http.StatusInternalServerError)
		} else if strings.Contains(err.Error(), "group memberships") {
			utils.WriteErrorJSON(w, "Failed to set up group membership. Please try again.", http.StatusInternalServerError)
		} else {
			utils.WriteErrorJSON(w, "Failed to create group. Please try again.", http.StatusInternalServerError)
		}
		return
	}

	// response including chat thread information
	response := map[string]interface{}{
		"message": "Group created successfully",
		"group": map[string]interface{}{
			"id":          createGroup.ID,
			"creator_id":  createGroup.CreatorID,
			"title":       createGroup.Title,
			"description": createGroup.Description,
			"is_public":   createGroup.IsPublic,
			"created_at":  createGroup.CreatedAt,
			"chat_id":     createGroup.ChatID,
		},
	}

	utils.WriteSuccessJSON(w, response, http.StatusCreated)
}

// Add hub parameter to handlers that need WebSocket notifications
func GroupInvitationHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get the user ID from the context (set by auth middleware)
		userID := r.Context().Value("userID").(string)
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
			return
		}

		var groupInv group.GroupInvitation
		if err := json.NewDecoder(r.Body).Decode(&groupInv); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		groupInv.InviterID = userID // InviterID = authenticated userID
		groupInv.Status = "pending"

		if err := groupInv.ValidateGroupInvitation(db.DB); err != nil {
			utils.WriteErrorJSON(w, "Invalid group invitation: "+err.Error(), http.StatusBadRequest)
			return
		}

		newGroupInv, err := group.CreateGroupInvitation(db.DB, groupInv)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to create group invitation: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get inviter name and group name for notification
		var inviterName, groupName string
		err = db.DB.QueryRow("SELECT first_name || ' ' || last_name FROM users WHERE id = ?", userID).Scan(&inviterName)
		if err != nil {
			inviterName = "Unknown User"
		}

		err = db.DB.QueryRow("SELECT title FROM groups WHERE id = ?", groupInv.GroupID).Scan(&groupName)
		if err != nil {
			groupName = "Unknown Group"
		}

		// Send WebSocket notification after successful DB update
		go hub.NotifyGroupInvitation(userID, groupInv.InviteeID, groupInv.GroupID, groupName, inviterName)
		log.Printf("Group invitation sent from %s to %s for group %s", inviterName, groupInv.InviteeID, groupName)

		utils.WriteSuccessJSON(w, newGroupInv, http.StatusCreated)
	}
}

// Handler for creating group requests - ADD HUB PARAMETER
func GroupRequestHandler(hub *websocket.Hub) http.HandlerFunc {
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

		var groupReq group.GroupRequest
		if err := json.NewDecoder(r.Body).Decode(&groupReq); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		groupReq.RequesterID = userID // RequesterID = authenticated userID
		groupReq.Status = "pending"

		if err := groupReq.ValidateGroupRequest(db.DB); err != nil {
			utils.WriteErrorJSON(w, "Invalid group request: "+err.Error(), http.StatusBadRequest)
			return
		}

		groupRequest, err := group.CreateGroupRequest(db.DB, groupReq)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to create group request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		user, err := user.GetUserByID(userID, userID)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to get user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Send WebSocket notification after successful DB update
		go websocket.SendGroupJoinRequestNotification(hub, userID, user.Nickname, groupRequest.AdminID, groupRequest.GroupID, groupRequest.GroupName)
		log.Printf("Group request sent from %s for group %s", userID, groupRequest.GroupID)

		groupRequest.AdminID = ""

		utils.WriteSuccessJSON(w, groupRequest, http.StatusCreated)
	}
}

// Handler for accepting group invitations
func AcceptGroupInvitationHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := r.Context().Value("userID").(string)
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
			return
		}

		var groupInv group.GroupInvitation
		if err := json.NewDecoder(r.Body).Decode(&groupInv); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Begin transaction for atomic operations
		tx, err := db.DB.Begin()
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to begin transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Find invitation by group_id and invitee_id
		var invitationID, inviterID, groupName string
		err = tx.QueryRow(`
            SELECT gi.id, gi.inviter_id, g.title 
            FROM group_invitations gi 
            JOIN groups g ON gi.group_id = g.id 
            WHERE gi.group_id = ? AND gi.invitee_id = ? AND gi.status = 'pending'
        `, groupInv.GroupID, groupInv.InviteeID).Scan(&invitationID, &inviterID, &groupName)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to find invitation: "+err.Error(), http.StatusNotFound)
			return
		}

		groupInv.ID = invitationID

		// Accept invitation and add to group
		_, err = tx.Exec(`
            UPDATE group_invitations 
            SET status = 'accepted', responded_at = datetime('now') 
            WHERE id = ?
        `, groupInv.ID)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to accept invitation: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Add user to group_memberships
		// Check if user is already a member (defensive check)
		var exists int
		err = tx.QueryRow(`
    SELECT COUNT(*) FROM group_memberships WHERE group_id = ? AND user_id = ?
`, groupInv.GroupID, userID).Scan(&exists)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to check group membership: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if exists == 0 {
			_, err = tx.Exec(`
        INSERT INTO group_memberships (group_id, user_id, role, joined_at)
        VALUES (?, ?, 'member', datetime('now'))
    `, groupInv.GroupID, userID)
			if err != nil {
				utils.WriteErrorJSON(w, "Failed to add user to group: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Add user to group chat
		if err := addUserToGroupChatTx(tx, userID, groupInv.GroupID); err != nil {
			utils.WriteErrorJSON(w, "Failed to add user to group chat: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			utils.WriteErrorJSON(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get invitee name for notification
		var inviteeName string
		err = db.DB.QueryRow("SELECT first_name || ' ' || last_name FROM users WHERE id = ?", userID).Scan(&inviteeName)
		if err != nil {
			inviteeName = "Unknown User"
		}

		// Send WebSocket notification after successful DB update
		go hub.NotifyInvitationResponse(inviterID, userID, groupInv.GroupID, groupName, inviteeName, "accepted")

		utils.WriteSuccessJSON(w, "Group invitation accepted successfully", http.StatusOK)
	}
}

// Handler for declining group invitations
func DeclineGroupInvitationHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := r.Context().Value("userID").(string)
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
			return
		}

		var groupInv group.GroupInvitation
		if err := json.NewDecoder(r.Body).Decode(&groupInv); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Find invitation by group_id and invitee_id (userID)
		var invitationID, inviterID, groupName string
		err := db.DB.QueryRow(`
            SELECT gi.id, gi.inviter_id, g.title
            FROM group_invitations gi
            JOIN groups g ON gi.group_id = g.id
            WHERE gi.group_id = ? AND gi.invitee_id = ? AND gi.status = 'pending'
        `, groupInv.GroupID, groupInv.InviteeID).Scan(&invitationID, &inviterID, &groupName)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to find invitation: "+err.Error(), http.StatusNotFound)
			return
		}

		groupInv.ID = invitationID

		if err := group.DeclineGroupInvitation(db.DB, groupInv); err != nil {
			utils.WriteErrorJSON(w, "Failed to decline group invitation: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get invitee name for notification
		var inviteeName string
		err = db.DB.QueryRow("SELECT first_name || ' ' || last_name FROM users WHERE id = ?", userID).Scan(&inviteeName)
		if err != nil {
			inviteeName = "Unknown User"
		}

		go hub.NotifyInvitationResponse(inviterID, userID, groupInv.GroupID, groupName, inviteeName, "declined")

		utils.WriteSuccessJSON(w, "Group invitation declined successfully", http.StatusOK)
	}
}

// Handler for accepting group requests - UPDATED
func AcceptGroupRequestHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// get the user ID from the context
		userID := r.Context().Value("userID").(string)
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
			return
		}

		var requestBody struct {
			GroupID     string `json:"group_id"`
			RequesterID string `json:"requester_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		if requestBody.GroupID == "" || requestBody.RequesterID == "" {
			utils.WriteErrorJSON(w, "group_id and requester_id are required", http.StatusBadRequest)
			return
		}

		// Find the pending group request using group_id and requester_id
		var requestID string
		err := db.DB.QueryRow(`
            SELECT id FROM group_requests 
            WHERE group_id = ? AND requester_id = ? AND status = 'pending'
        `, requestBody.GroupID, requestBody.RequesterID).Scan(&requestID)
		if err != nil {
			if err == sql.ErrNoRows {
				utils.WriteErrorJSON(w, "No pending group request found", http.StatusNotFound)
				return
			}
			utils.WriteErrorJSON(w, "Failed to find group request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Retrieve the group creator ID (creator_id)
		var creatorID string
		err = db.DB.QueryRow("SELECT creator_id FROM groups WHERE id = ?", requestBody.GroupID).Scan(&creatorID)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to find group creator: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Check if the user is the group creator
		if userID != creatorID {
			utils.WriteErrorJSON(w, "Unauthorized: Only the group creator can accept or decline requests", http.StatusForbidden)
			return
		}

		// Create GroupRequest struct for the AcceptGroupRequest function
		var groupReq group.GroupRequest
		groupReq.ID = requestID
		groupReq.GroupID = requestBody.GroupID
		groupReq.RequesterID = requestBody.RequesterID

		// Accept the group request
		if err := group.AcceptGroupRequest(db.DB, groupReq); err != nil {
			utils.WriteErrorJSON(w, "Failed to accept group request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// get groupName
		var groupName string
		err = db.DB.QueryRow("SELECT title FROM groups WHERE id = ?", requestBody.GroupID).Scan(&groupName)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to find group: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Update request status and add user to group in transaction
		tx, err := db.DB.Begin()
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to begin transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Update request status
		_, err = tx.Exec(`
            UPDATE group_requests 
            SET status = 'accepted', responded_at = datetime('now') 
            WHERE id = ?
        `, requestID)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to update request status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Check if user is already a member (defensive check)
		var exists int
		err = tx.QueryRow(`
            SELECT COUNT(*) FROM group_memberships WHERE group_id = ? AND user_id = ?
        `, requestBody.GroupID, requestBody.RequesterID).Scan(&exists)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to check group membership: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if exists == 0 {
			// Add user to group
			_, err = tx.Exec(`
                INSERT INTO group_memberships (group_id, user_id, role, joined_at)
                VALUES (?, ?, 'member', datetime('now'))
            `, requestBody.GroupID, requestBody.RequesterID)
			if err != nil {
				utils.WriteErrorJSON(w, "Failed to add user to group: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Add user to group chat
		if err := addUserToGroupChatTx(tx, requestBody.RequesterID, requestBody.GroupID); err != nil {
			utils.WriteErrorJSON(w, "Failed to add user to group chat: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			utils.WriteErrorJSON(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Send success notification
		go websocket.SendGroupRequestResponseNotification(hub, requestBody.RequesterID, requestBody.GroupID, groupName, true, userID)

		utils.WriteSuccessJSON(w, "Group request accepted successfully", http.StatusOK)
	}
}

// Handler for declining group requests - UPDATED
func DeclineGroupRequestHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// get the user ID from the context
		userID := r.Context().Value("userID").(string)
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
			return
		}

		var requestBody struct {
			GroupID     string `json:"group_id"`
			RequesterID string `json:"requester_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		if requestBody.GroupID == "" || requestBody.RequesterID == "" {
			utils.WriteErrorJSON(w, "group_id and requester_id are required", http.StatusBadRequest)
			return
		}

		// Find the pending group request using group_id and requester_id
		var requestID string
		err := db.DB.QueryRow(`
            SELECT id FROM group_requests 
            WHERE group_id = ? AND requester_id = ? AND status = 'pending'
        `, requestBody.GroupID, requestBody.RequesterID).Scan(&requestID)
		if err != nil {
			if err == sql.ErrNoRows {
				utils.WriteErrorJSON(w, "No pending group request found", http.StatusNotFound)
				return
			}
			utils.WriteErrorJSON(w, "Failed to find group request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Check if the user is a member of the group and retrieve their role
		var role sql.NullString
		err = db.DB.QueryRow("SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?", requestBody.GroupID, userID).Scan(&role)
		if err != nil {
			if err == sql.ErrNoRows {
				// Check if user is the group creator
				var creatorID string
				err = db.DB.QueryRow("SELECT creator_id FROM groups WHERE id = ?", requestBody.GroupID).Scan(&creatorID)
				if err != nil || userID != creatorID {
					utils.WriteErrorJSON(w, "Unauthorized: User is not a member of the group", http.StatusForbidden)
					return
				}
				// Creator can decline requests
			} else {
				utils.WriteErrorJSON(w, "Failed to check user membership and role: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else if !role.Valid || role.String != "admin" {
			// Check if user is the group creator
			var creatorID string
			err = db.DB.QueryRow("SELECT creator_id FROM groups WHERE id = ?", requestBody.GroupID).Scan(&creatorID)
			if err != nil || userID != creatorID {
				utils.WriteErrorJSON(w, "Unauthorized: Only group admins or creator can decline requests", http.StatusForbidden)
				return
			}
		}

		// Create GroupRequest struct for the DeclineGroupRequest function
		var groupReq group.GroupRequest
		groupReq.ID = requestID
		groupReq.GroupID = requestBody.GroupID
		groupReq.RequesterID = requestBody.RequesterID

		// Decline the group request
		if err := group.DeclineGroupRequest(db.DB, groupReq); err != nil {
			utils.WriteErrorJSON(w, "Failed to decline group request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// get groupName
		var groupName string
		err = db.DB.QueryRow("SELECT title FROM groups WHERE id = ?", requestBody.GroupID).Scan(&groupName)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to find group: "+err.Error(), http.StatusInternalServerError)
			return
		}

		go websocket.SendGroupRequestResponseNotification(hub, requestBody.RequesterID, requestBody.GroupID, groupName, false, userID)

		utils.WriteSuccessJSON(w, "Group request declined successfully", http.StatusOK)
	}
}

// GetGroupPosts retrieves posts for a specific group
func (h *PostHandler) GetGroupPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context (may be empty for unauthenticated)
	userID, _ := r.Context().Value("userID").(string)

	// Get group ID from URL parameters
	groupIDStr := r.URL.Query().Get("group_id")
	if groupIDStr == "" {
		utils.WriteErrorJSON(w, "Group ID is required", http.StatusBadRequest)
		return
	}

	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		utils.WriteErrorJSON(w, "Invalid Group ID format: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Check if group is public
	var isPublic bool
	err = db.DB.QueryRow("SELECT is_public FROM groups WHERE id = ?", groupID).Scan(&isPublic)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to check group privacy: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If not public, check if user is a member
	isMember := false
	if !isPublic {
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: Private group", http.StatusUnauthorized)
			return
		}
		var dbRole sql.NullString
		err := db.DB.QueryRow(
			"SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?",
			groupID, userID,
		).Scan(&dbRole)
		if err == nil && dbRole.Valid {
			isMember = true
		}
		if !isMember {
			utils.WriteErrorJSON(w, "Unauthorized: Private group", http.StatusUnauthorized)
			return
		}
	}

	// Parse offset parameter (default to 0)
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			utils.WriteErrorJSON(w, "Invalid offset parameter: must be a non-negative integer", http.StatusBadRequest)
			return
		}
	}

	// Parse limit parameter (default to 20, max 100)
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			utils.WriteErrorJSON(w, "Invalid limit parameter: must be a positive integer", http.StatusBadRequest)
			return
		}
		if limit > 100 {
			limit = 100
		}
	}

	// Get group posts from the DB
	posts, err := h.PostService.GetGroupPosts(userID, groupID, offset, limit)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to retrieve group posts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response
	response := map[string]interface{}{
		"success": true,
		"posts":   posts,
		"hasMore": len(posts) >= limit,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Handler to get all groups the user is a member of
func GetUserGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	groups, err := group.GetGroupsByUserID(db.DB, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get user groups: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"groups": groups,
	})
}

// Handler to get info for a specific group, including membership and role
func GetGroupByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	groupID := r.URL.Query().Get("group_id")
	if groupID == "" {
		utils.WriteErrorJSON(w, "Group ID is required", http.StatusBadRequest)
		return
	}

	// Get user ID from context (if authenticated)
	userID, _ := r.Context().Value("userID").(string)

	groupInfo, err := group.GetGroupByID(db.DB, groupID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get group: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Default membership info
	isMember := false
	role := ""

	// If user is authenticated, check membership and role
	if userID != "" {
		var dbRole sql.NullString
		err := db.DB.QueryRow(
			"SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?",
			groupID, userID,
		).Scan(&dbRole)
		if err == nil && dbRole.Valid {
			isMember = true
			role = dbRole.String
		}
	}

	// Build response
	resp := map[string]interface{}{
		"group":    groupInfo,
		"isMember": isMember,
		"role":     role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetPendingGroupRequestsHandler retrieves pending group join requests for a group
func GetPendingGroupRequestsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	groupID := r.URL.Query().Get("group_id")
	if groupID == "" {
		utils.WriteErrorJSON(w, "Group ID is required", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get group info to check creator
	var creatorID string
	err := db.DB.QueryRow("SELECT creator_id FROM groups WHERE id = ?", groupID).Scan(&creatorID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get group info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user is admin in the group
	var role sql.NullString
	err = db.DB.QueryRow(
		"SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?",
		groupID, userID,
	).Scan(&role)
	isAdmin := err == nil && role.Valid && role.String == "admin"

	// Allow if user is admin or creator
	if !isAdmin && userID != creatorID {
		utils.WriteErrorJSON(w, "Unauthorized: Only group admins or the creator can view pending requests", http.StatusForbidden)
		return
	}

	// Get pending requests
	rows, err := db.DB.Query(`
        SELECT gr.id, gr.requester_id, u.nickname, u.first_name, u.last_name, u.avatar_path, gr.created_at
        FROM group_requests gr
        JOIN users u ON gr.requester_id = u.id
        WHERE gr.group_id = ? AND gr.status = 'pending'
        ORDER BY gr.created_at ASC
    `, groupID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get pending requests: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var requests []map[string]interface{}
	for rows.Next() {
		var reqID, requesterID, nickname, firstName, lastName, avatarPath string
		var createdAt string
		if err := rows.Scan(&reqID, &requesterID, &nickname, &firstName, &lastName, &avatarPath, &createdAt); err != nil {
			utils.WriteErrorJSON(w, "Failed to scan request: "+err.Error(), http.StatusInternalServerError)
			return
		}
		requests = append(requests, map[string]interface{}{
			"id":           reqID,
			"requester_id": requesterID,
			"nickname":     nickname,
			"first_name":   firstName,
			"last_name":    lastName,
			"avatar":       avatarPath,
			"created_at":   createdAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"requests": requests,
	})
}

// GetGroupMembersHandler retrieves all members of a group
func GetGroupMembersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	groupID := r.URL.Query().Get("group_id")
	if groupID == "" {
		utils.WriteErrorJSON(w, "Group ID is required", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Check if user is a member of the group
	var role sql.NullString
	err := db.DB.QueryRow(
		"SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?",
		groupID, userID,
	).Scan(&role)
	if err != nil || !role.Valid {
		utils.WriteErrorJSON(w, "Unauthorized: User is not a member of this group", http.StatusForbidden)
		return
	}

	// Get all group members
	rows, err := db.DB.Query(`
        SELECT gm.user_id, gm.role, u.nickname, u.first_name, u.last_name, u.avatar_path, gm.joined_at
        FROM group_memberships gm
        JOIN users u ON gm.user_id = u.id
        WHERE gm.group_id = ?
        ORDER BY gm.joined_at ASC
    `, groupID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get group members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var members []map[string]interface{}
	for rows.Next() {
		var memberID, memberRole, nickname, firstName, lastName, avatarPath, joinedAt string
		if err := rows.Scan(&memberID, &memberRole, &nickname, &firstName, &lastName, &avatarPath, &joinedAt); err != nil {
			utils.WriteErrorJSON(w, "Failed to scan member: "+err.Error(), http.StatusInternalServerError)
			return
		}
		members = append(members, map[string]interface{}{
			"id":         memberID,
			"role":       memberRole,
			"nickname":   nickname,
			"first_name": firstName,
			"last_name":  lastName,
			"avatar":     avatarPath,
			"joined_at":  joinedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"members": members,
	})
}

// GrantAdminHandler grants admin role to a group member
func GrantAdminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req struct {
		GroupID  string `json:"group_id"`
		MemberID string `json:"member_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get group creator ID
	var creatorID string
	err := db.DB.QueryRow("SELECT creator_id FROM groups WHERE id = ?", req.GroupID).Scan(&creatorID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get group info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user is admin or creator
	var role sql.NullString
	err = db.DB.QueryRow(
		"SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?",
		req.GroupID, userID,
	).Scan(&role)
	isAdmin := err == nil && role.Valid && role.String == "admin"

	if !isAdmin && userID != creatorID {
		utils.WriteErrorJSON(w, "Unauthorized: Only group admins or creator can grant admin role", http.StatusForbidden)
		return
	}

	// Update member role to admin
	_, err = db.DB.Exec(
		"UPDATE group_memberships SET role = 'admin' WHERE group_id = ? AND user_id = ?",
		req.GroupID, req.MemberID,
	)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to grant admin role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteSuccessJSON(w, "Admin role granted successfully", http.StatusOK)
}

// RevokeAdminHandler revokes admin role from a group member
func RevokeAdminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req struct {
		GroupID  string `json:"group_id"`
		MemberID string `json:"member_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get group creator ID
	var creatorID string
	err := db.DB.QueryRow("SELECT creator_id FROM groups WHERE id = ?", req.GroupID).Scan(&creatorID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get group info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Only creator can revoke admin role
	if userID != creatorID {
		utils.WriteErrorJSON(w, "Unauthorized: Only the group creator can revoke admin role", http.StatusForbidden)
		return
	}

	// Update member role to member
	_, err = db.DB.Exec(
		"UPDATE group_memberships SET role = 'member' WHERE group_id = ? AND user_id = ?",
		req.GroupID, req.MemberID,
	)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to revoke admin role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteSuccessJSON(w, "Admin role revoked successfully", http.StatusOK)
}

// GrantCreatorHandler transfers creator ownership to another member
func GrantCreatorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req struct {
		GroupID  string `json:"group_id"`
		MemberID string `json:"member_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get group creator ID
	var creatorID string
	err := db.DB.QueryRow("SELECT creator_id FROM groups WHERE id = ?", req.GroupID).Scan(&creatorID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get group info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Only current creator can transfer ownership
	if userID != creatorID {
		utils.WriteErrorJSON(w, "Unauthorized: Only the current creator can grant creator role", http.StatusForbidden)
		return
	}

	// Start transaction
	tx, err := db.DB.Begin()
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to start transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Update group creator_id
	_, err = tx.Exec("UPDATE groups SET creator_id = ? WHERE id = ?", req.MemberID, req.GroupID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to update group creator: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Make new creator an admin if they weren't already
	_, err = tx.Exec(`
        INSERT OR REPLACE INTO group_memberships (group_id, user_id, role, joined_at)
        VALUES (?, ?, 'admin', COALESCE(
            (SELECT joined_at FROM group_memberships WHERE group_id = ? AND user_id = ?),
            datetime('now')
        ))
    `, req.GroupID, req.MemberID, req.GroupID, req.MemberID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to grant admin role to new creator: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Previous creator stays as admin (no role change needed if already admin)
	_, err = tx.Exec(`
        UPDATE group_memberships 
        SET role = 'admin' 
        WHERE group_id = ? AND user_id = ? AND role != 'admin'
    `, req.GroupID, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to update previous creator role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		utils.WriteErrorJSON(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteSuccessJSON(w, "Creator role transferred successfully", http.StatusOK)
}

// KickMemberHandler removes a member from the group
func KickMemberHandler(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := r.Context().Value("userID").(string)
		if userID == "" {
			utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
			return
		}

		var req struct {
			GroupID  string `json:"group_id"`
			MemberID string `json:"member_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Begin transaction
		tx, err := db.DB.Begin()
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to begin transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Get group creator ID
		var creatorID string
		err = tx.QueryRow("SELECT creator_id FROM groups WHERE id = ?", req.GroupID).Scan(&creatorID)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to get group info: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get target member's role
		var targetRole sql.NullString
		err = tx.QueryRow(
			"SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?",
			req.GroupID, req.MemberID,
		).Scan(&targetRole)
		if err != nil || !targetRole.Valid {
			utils.WriteErrorJSON(w, "Target user is not a member of this group", http.StatusBadRequest)
			return
		}

		// Check permissions
		var userRole sql.NullString
		err = tx.QueryRow(
			"SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?",
			req.GroupID, userID,
		).Scan(&userRole)
		isAdmin := err == nil && userRole.Valid && userRole.String == "admin"
		isCreator := userID == creatorID

		// Only creator can kick admins, admins can kick members
		if targetRole.String == "admin" && !isCreator {
			utils.WriteErrorJSON(w, "Unauthorized: Only the creator can kick admins", http.StatusForbidden)
			return
		}
		if !isAdmin && !isCreator {
			utils.WriteErrorJSON(w, "Unauthorized: Only admins or creator can kick members", http.StatusForbidden)
			return
		}

		// Cannot kick the creator
		if req.MemberID == creatorID {
			utils.WriteErrorJSON(w, "Cannot kick the group creator", http.StatusBadRequest)
			return
		}

		// Remove member from group
		_, err = tx.Exec(
			"DELETE FROM group_memberships WHERE group_id = ? AND user_id = ?",
			req.GroupID, req.MemberID,
		)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to kick member: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Remove member from group chat
		if err := removeUserFromGroupChatTx(tx, req.MemberID, req.GroupID); err != nil {
			utils.WriteErrorJSON(w, "Failed to remove member from group chat: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Clean up any invitation records for the kicked user
		_, err = tx.Exec(`
			DELETE FROM group_invitations 
			WHERE group_id = ? AND invitee_id = ?
		`, req.GroupID, req.MemberID)
		if err != nil {
			log.Printf("Warning: Failed to clean up invitation records for kicked user %s from group %s: %v", req.MemberID, req.GroupID, err)
		}

		// Clean up any group request records for the kicked user
		_, err = tx.Exec(`
			DELETE FROM group_requests 
			WHERE group_id = ? AND requester_id = ?
		`, req.GroupID, req.MemberID)
		if err != nil {
			log.Printf("Warning: Failed to clean up group request records for kicked user %s from group %s: %v", req.MemberID, req.GroupID, err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			utils.WriteErrorJSON(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

		go websocket.SendGroupKickNotification(hub, req.MemberID, req.GroupID, userID)

		utils.WriteSuccessJSON(w, "Member kicked successfully", http.StatusOK)
	}

}

// EditGroupHandler allows admins to edit group title, description, and privacy setting
func EditGroupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req struct {
		GroupID     string `json:"group_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		IsPublic    bool   `json:"is_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.GroupID == "" {
		utils.WriteErrorJSON(w, "Group ID is required", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		utils.WriteErrorJSON(w, "Group title is required", http.StatusBadRequest)
		return
	}

	// Get group creator ID
	var creatorID string
	err := db.DB.QueryRow("SELECT creator_id FROM groups WHERE id = ?", req.GroupID).Scan(&creatorID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get group info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user is admin or creator
	var role sql.NullString
	err = db.DB.QueryRow(
		"SELECT role FROM group_memberships WHERE group_id = ? AND user_id = ?",
		req.GroupID, userID,
	).Scan(&role)
	isAdmin := err == nil && role.Valid && role.String == "admin"

	if !isAdmin && userID != creatorID {
		utils.WriteErrorJSON(w, "Unauthorized: Only group admins or creator can edit group settings", http.StatusForbidden)
		return
	}

	// Update group settings (removed updated_at since column doesn't exist)
	_, err = db.DB.Exec(`
        UPDATE groups 
        SET title = ?, description = ?, is_public = ?
        WHERE id = ?
    `, req.Title, req.Description, req.IsPublic, req.GroupID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to update group settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteSuccessJSON(w, "Group settings updated successfully", http.StatusOK)
}

// Handler for Joining a Public Group
func JoinPublicGroupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var requestBody struct {
		GroupID string `json:"group_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if requestBody.GroupID == "" {
		utils.WriteErrorJSON(w, "Missing group_id", http.StatusBadRequest)
		return
	}

	// Check if group exists and is public
	var isPublic bool
	var groupTitle string
	query := `SELECT is_public, title FROM groups WHERE id = ?`
	err := db.DB.QueryRow(query, requestBody.GroupID).Scan(&isPublic, &groupTitle)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.WriteErrorJSON(w, "Group not found", http.StatusNotFound)
			return
		}
		utils.WriteErrorJSON(w, "Failed to check group: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !isPublic {
		utils.WriteErrorJSON(w, "Can only join public groups directly", http.StatusForbidden)
		return
	}

	// Check if user is already a member (defensive check)
	// Check if user is already a member (defensive check for both membership and creator)
	var existingMemberCount int
	memberQuery := `
    SELECT COUNT(*) FROM (
        SELECT user_id FROM group_memberships WHERE group_id = ? AND user_id = ?
        UNION
        SELECT creator_id FROM groups WHERE id = ? AND creator_id = ?
    )
`
	err = db.DB.QueryRow(memberQuery, requestBody.GroupID, userID, requestBody.GroupID, userID).Scan(&existingMemberCount)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to check membership: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if existingMemberCount > 0 {
		utils.WriteErrorJSON(w, "You are already a member of this group", http.StatusConflict)
		return
	}

	// Add user as member using group_memberships table
	insertQuery := `
    INSERT INTO group_memberships (group_id, user_id, role, joined_at)
    VALUES (?, ?, 'member', datetime('now'))
`
	_, err = db.DB.Exec(insertQuery, requestBody.GroupID, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to join group: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Add user to group chat
	chatService := websocket.NewChatService(db.DB)
	if err := chatService.AddUserToGroupChat(userID, requestBody.GroupID); err != nil {
		log.Printf("Warning: Failed to add user to group chat: %v", err)
		// Don't fail the request, just log the warning
	}

	resp := map[string]interface{}{
		"message":    "Successfully joined group",
		"group_id":   requestBody.GroupID,
		"group_name": groupTitle,
	}

	utils.WriteSuccessJSON(w, resp, http.StatusOK)
}

// Handler for Leaving a Group
func LeaveGroupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var requestBody struct {
		GroupID string `json:"group_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if requestBody.GroupID == "" {
		utils.WriteErrorJSON(w, "Missing group_id", http.StatusBadRequest)
		return
	}

	// Begin transaction for complex operations
	tx, err := db.DB.Begin()
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to begin transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Check if user is a member and get group info
	var creatorID string
	var groupTitle string
	var memberRole sql.NullString
	query := `
        SELECT g.creator_id, g.title, COALESCE(gm.role, '') as member_role
        FROM groups g
        LEFT JOIN group_memberships gm ON g.id = gm.group_id AND gm.user_id = ?
        WHERE g.id = ?
    `
	err = tx.QueryRow(query, userID, requestBody.GroupID).Scan(&creatorID, &groupTitle, &memberRole)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.WriteErrorJSON(w, "Group not found", http.StatusNotFound)
			return
		}
		utils.WriteErrorJSON(w, "Failed to check group membership: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user is a member (creator or member in group_memberships)
	isCreator := creatorID == userID
	isMember := memberRole.Valid && (memberRole.String == "member" || memberRole.String == "admin")

	if !isMember && !isCreator {
		utils.WriteErrorJSON(w, "You are not a member of this group", http.StatusForbidden)
		return
	}

	// Count total members (excluding creator from group_memberships count)
	var memberCount int
	countQuery := `SELECT COUNT(*) FROM group_memberships WHERE group_id = ?`
	err = tx.QueryRow(countQuery, requestBody.GroupID).Scan(&memberCount)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to count members: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle different scenarios
	if isCreator {
		// If creator is the only member (memberCount = 1 and it's just the creator)
		if memberCount == 1 {
			// Creator is the only member - delete the entire group
			if err := deleteGroupCompletely(tx, requestBody.GroupID); err != nil {
				utils.WriteErrorJSON(w, "Failed to delete group: "+err.Error(), http.StatusInternalServerError)
				return
			}

			if err := tx.Commit(); err != nil {
				utils.WriteErrorJSON(w, "Failed to commit deletion: "+err.Error(), http.StatusInternalServerError)
				return
			}

			resp := map[string]interface{}{
				"message":       "Group deleted successfully (you were the only member)",
				"group_id":      requestBody.GroupID,
				"group_name":    groupTitle,
				"group_deleted": true,
			}
			utils.WriteSuccessJSON(w, resp, http.StatusOK)
			return
		} else {
			// Creator has other members - cannot leave without transferring ownership
			utils.WriteErrorJSON(w, "As the group creator, you cannot leave until you transfer ownership to another member", http.StatusForbidden)
			return
		}
	} else {
		// Regular member leaving - remove from group and chat
		deleteQuery := `DELETE FROM group_memberships WHERE group_id = ? AND user_id = ?`
		result, err := tx.Exec(deleteQuery, requestBody.GroupID, userID)
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to leave group: "+err.Error(), http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			utils.WriteErrorJSON(w, "Failed to check operation result: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if rowsAffected == 0 {
			utils.WriteErrorJSON(w, "Failed to leave group: no membership found", http.StatusBadRequest)
			return
		}

		// Remove user from group chat
		if err := removeUserFromGroupChatTx(tx, userID, requestBody.GroupID); err != nil {
			utils.WriteErrorJSON(w, "Failed to remove user from group chat: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Clean up any invitation records for this user when they leave
		// This allows them to be invited again later
		_, err = tx.Exec(`
            DELETE FROM group_invitations 
            WHERE group_id = ? AND invitee_id = ?
        `, requestBody.GroupID, userID)
		if err != nil {
			log.Printf("Warning: Failed to clean up invitation records for user %s leaving group %s: %v", userID, requestBody.GroupID, err)
			// Don't fail the leave operation for this
		}

		// Clean up any group request records for this user when they leave
		// This allows them to send new requests later
		_, err = tx.Exec(`
            DELETE FROM group_requests 
            WHERE group_id = ? AND requester_id = ?
        `, requestBody.GroupID, userID)
		if err != nil {
			log.Printf("Warning: Failed to clean up group request records for user %s leaving group %s: %v", userID, requestBody.GroupID, err)
			// Don't fail the leave operation for this
		}

		if err := tx.Commit(); err != nil {
			utils.WriteErrorJSON(w, "Failed to commit leave operation: "+err.Error(), http.StatusInternalServerError)
			return
		}

		resp := map[string]interface{}{
			"message":    "Successfully left group",
			"group_id":   requestBody.GroupID,
			"group_name": groupTitle,
		}
		utils.WriteSuccessJSON(w, resp, http.StatusOK)
		return
	}
}

// Helper function to delete group and all related data
func deleteGroupCompletely(tx *sql.Tx, groupID string) error {
	// Delete chat participants first
	_, err := tx.Exec(`
        DELETE FROM chat_participants 
        WHERE chat_id IN (
            SELECT id FROM chat_threads 
            WHERE is_group = 1 AND group_id = ?
        )
    `, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete chat participants: %v", err)
	}

	// Delete chat messages
	_, err = tx.Exec(`
        DELETE FROM messages 
        WHERE chat_id IN (
            SELECT id FROM chat_threads 
            WHERE is_group = 1 AND group_id = ?
        )
    `, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete chat messages: %v", err)
	}

	// Delete chat threads
	_, err = tx.Exec(`
        DELETE FROM chat_threads 
        WHERE is_group = 1 AND group_id = ?
    `, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete chat threads: %v", err)
	}

	// Delete event responses first
	_, err = tx.Exec(`
        DELETE FROM event_responses 
        WHERE event_id IN (SELECT id FROM events WHERE group_id = ?)
    `, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete event responses: %v", err)
	}

	// Delete events
	_, err = tx.Exec(`DELETE FROM events WHERE group_id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete events: %v", err)
	}

	// Delete group memberships
	_, err = tx.Exec(`DELETE FROM group_memberships WHERE group_id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete group members: %v", err)
	}

	// Delete group requests
	_, err = tx.Exec(`DELETE FROM group_requests WHERE group_id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete group requests: %v", err)
	}

	// Delete group invitations
	_, err = tx.Exec(`DELETE FROM group_invitations WHERE group_id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete group invitations: %v", err)
	}

	// Delete posts in the group
	_, err = tx.Exec(`DELETE FROM posts WHERE group_id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete group posts: %v", err)
	}

	// Finally delete the group itself
	_, err = tx.Exec(`DELETE FROM groups WHERE id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete group: %v", err)
	}

	return nil
}
