package event

import (
	"database/sql"
	"errors"
)

// Function to validate event creation
func (e *Event) ValidateEventCreation(db *sql.DB) error {
	// general validation
	if e.GroupID == "" || e.CreatorID == "" || e.Title == "" || e.Description == "" || e.EventTime == "" {
		return errors.New("all fields must be provided")
	}
	// validate title and description length
	if len(e.Title) < 10 || len(e.Title) > 200 {
		return errors.New("title must be between 10 and 200 characters")
	}

	if len(e.Description) < 10 || len(e.Description) > 500 {
		return errors.New("description must be between 10 and 500 characters")
	}

	// Check if group exists
	var groupCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM groups WHERE id = ?", e.GroupID).Scan(&groupCount); err != nil || groupCount == 0 {
		return errors.New("group does not exist")
	}

	// Check if creator is a member of the group
	var memberCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM group_memberships WHERE group_id = ? AND user_id = ?", e.GroupID, e.CreatorID).Scan(&memberCount); err != nil || memberCount == 0 {
		return errors.New("creator is not a member of the group")
	}

	return nil
}

// function to validate event response
func (eventRes *EventResponse) ValidateEventResponse(db *sql.DB) error {
	if eventRes.EventID == "" || eventRes.UserID == "" || eventRes.Response == "" {
		return errors.New("all fields must be provided")
	}

	if eventRes.Response != "going" && eventRes.Response != "not_going" {
		return errors.New("response must be either 'going' or 'not going'")
	}

	// Check if event exists and get its group_id
	var groupID int
	if err := db.QueryRow("SELECT group_id FROM events WHERE id = ?", eventRes.EventID).Scan(&groupID); err != nil {
		return errors.New("event does not exist")
	}

	// Check if user is a member of the event's group
	var userCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM group_memberships WHERE group_id = ? AND user_id = ?", groupID, eventRes.UserID).Scan(&userCount); err != nil || userCount == 0 {
		return errors.New("user is not a member of the event's group")
	}

	return nil
}
