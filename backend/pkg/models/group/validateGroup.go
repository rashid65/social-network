package group

import (
	"database/sql"
	"errors"
)

// Function to validate group
func (g *Group) ValidateGroupCreation() error {

	// Check if all required fields are provided
	if g.Title == "" || g.Description == "" {
		return errors.New("all fields must be provided")
	}

	//validate title length
	if len(g.Title) < 10 || len(g.Title) > 200 {
		return errors.New("title must be between 10 and 200 characters")
	}

	//validate description length
	if len(g.Description) < 10 || len(g.Description) > 500 {
		return errors.New("description must be between 10 and 500 characters")
	}

	return nil
}

// Function to validate GroupInvitation - UPDATED to allow re-inviting
func (gi *GroupInvitation) ValidateGroupInvitation(db *sql.DB) error {
	if gi.GroupID == "" || gi.InviterID == "" || gi.InviteeID == "" || gi.Status == "" {
		return errors.New("all fields must be provided")
	}

	// Check if user exists
	var userExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", gi.InviteeID).Scan(&userExists)
	if err != nil {
		return err
	}
	if !userExists {
		return errors.New("invitee does not exist")
	}

	// Check if group exists
	var groupExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM groups WHERE id = ?)", gi.GroupID).Scan(&groupExists)
	if err != nil {
		return err
	}
	if !groupExists {
		return errors.New("group does not exist")
	}

	// Check if user is already a member or the creator
	var isMember bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM group_memberships WHERE group_id = ? AND user_id = ?)", gi.GroupID, gi.InviteeID).Scan(&isMember)
	if err != nil {
		return err
	}
	if isMember {
		return errors.New("user is already a member of this group")
	}

	// Check if user is the creator
	var isCreator bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM groups WHERE id = ? AND creator_id = ?)", gi.GroupID, gi.InviteeID).Scan(&isCreator)
	if err != nil {
		return err
	}
	if isCreator {
		return errors.New("user is the creator of this group")
	}

	// Check if there's already a PENDING invitation (not declined/accepted)
	var hasPendingInvitation bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM group_invitations WHERE group_id = ? AND invitee_id = ? AND status = 'pending')", gi.GroupID, gi.InviteeID).Scan(&hasPendingInvitation)
	if err != nil {
		return err
	}
	if hasPendingInvitation {
		return errors.New("an invitation to this user already exists for this group")
	}

	// Check if inviter is a member or creator of the group
	var inviterIsMember bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM group_memberships WHERE group_id = ? AND user_id = ?)", gi.GroupID, gi.InviterID).Scan(&inviterIsMember)
	if err != nil {
		return err
	}

	var inviterIsCreator bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM groups WHERE id = ? AND creator_id = ?)", gi.GroupID, gi.InviterID).Scan(&inviterIsCreator)
	if err != nil {
		return err
	}

	if !inviterIsMember && !inviterIsCreator {
		return errors.New("inviter is not a member of the group")
	}

	if !isValidStatus(gi.Status) {
		return errors.New("invalid status: " + gi.Status)
	}

	return nil
}

// Function to validate GroupRequest
func (gr *GroupRequest) ValidateGroupRequest(db *sql.DB) error {
	if gr.RequesterID == "" || gr.GroupID == "" || gr.Status == "" {
		return errors.New("all fields must be provided")
	}

	// Check if requester is already a member of the group
	var isMember bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM group_memberships WHERE group_id = ? AND user_id = ?)",
		gr.GroupID, gr.RequesterID,
	).Scan(&isMember)
	if err != nil {
		return err
	}
	if isMember {
		return errors.New("you are already a member of this group")
	}

	// Check if user is the creator
	var isCreator bool
	err = db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM groups WHERE id = ? AND creator_id = ?)",
		gr.GroupID, gr.RequesterID,
	).Scan(&isCreator)
	if err != nil {
		return err
	}
	if isCreator {
		return errors.New("you are the creator of this group")
	}

	// Prevent duplicate PENDING requests only
	var requestExists bool
	err = db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM group_requests WHERE requester_id = ? AND group_id = ? AND status = 'pending')",
		gr.RequesterID, gr.GroupID,
	).Scan(&requestExists)
	if err != nil {
		return err
	}
	if requestExists {
		return errors.New("you have already sent a pending request to this group")
	}

	if !isValidStatus(gr.Status) {
		return errors.New("invalid status: " + gr.Status)
	}
	return nil
}

// Function to validate a response to a group invitation (accept/decline)
func (gi *GroupInvitation) ValidateGroupInvitationResponse(db *sql.DB) error {
	if gi.ID == "" || gi.InviteeID == "" {
		return errors.New("invitation ID and invitee ID must be provided")
	}

	// Check if the invitation exists and is pending
	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM group_invitations WHERE id = ? AND invitee_id = ? AND status = 'pending')",
		gi.ID, gi.InviteeID,
	).Scan(&exists)

	if err != nil {
		return err
	}

	if !exists {
		return errors.New("no pending invitation found with this ID for this user")
	}

	return nil
}

func isValidStatus(status string) bool {
	allowedStatuses := map[string]bool{
		"pending":  true,
		"accepted": true,
		"declined": true,
	}
	return allowedStatuses[status]
}

// Function to validate a response to a group request (accept/decline)
func (gr *GroupRequest) ValidateGroupRequestResponse(db *sql.DB) error {
	if gr.ID == "" {
		return errors.New("request ID must be provided")
	}

	// Check if the group request exists and is pending, and get request details
	var requesterID, groupID string
	err := db.QueryRow(
		"SELECT requester_id, group_id FROM group_requests WHERE id = ? AND status = 'pending'",
		gr.ID,
	).Scan(&requesterID, &groupID)

	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("no pending group request found with this ID")
		}
		return err
	}

	// Set the values from database
	gr.RequesterID = requesterID
	gr.GroupID = groupID

	return nil
}
