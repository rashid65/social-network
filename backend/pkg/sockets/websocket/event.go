package websocket

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"
)

func (h *Hub) NotifyGroupEventCreated(db *sql.DB, eventID, groupID, creatorID, title string) {
	var creatorName string
	err := db.QueryRow("SELECT first_name || ' ' || last_name FROM users WHERE id = ?", creatorID).Scan(&creatorName)
	if err != nil {
		log.Printf("error getting creator name: %v", err)
		return
	}

	rows, err := db.Query("SELECT user_id FROM group_memberships WHERE group_id = ?", groupID)
	if err != nil {
		log.Printf("error getting group members: %v", err)
		return
	}
	defer rows.Close()

	// Collect all userIDs that need notifications
	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			log.Printf("error scanning user ID: %v", err)
			continue
		}

		if userID == creatorID {
			continue // Skip the creator
		}

		userIDs = append(userIDs, userID)
	}

	// Check for errors from iterating over rows
	if err = rows.Err(); err != nil {
		log.Printf("error iterating group members: %v", err)
		return
	}

	messageText := fmt.Sprintf("%s created a new event: %s", creatorName, title)

	// Send notifications to all collected users
	for _, userID := range userIDs {
		// Create notification in database and get the real ID
		notification := Notification{
			UserID:   userID,
			SenderID: creatorID,
			Type:     "group_event_created",
			RefID:    eventID,
			IsRead:   false,
			Message:  messageText,
		}

		notificationID, err := CreateNotificationAndGetID(db, notification)
		if err != nil {
			log.Printf("Error creating group event notification for user %s: %v", userID, err)
			continue
		}

		message := NotificationMessage{
			ID:           strconv.Itoa(notificationID),
			SenderID:     creatorID,
			RecipientID:  userID,
			Type:         "group_event_created",
			RefID:        eventID,
			Message:      messageText,
			Timestamp:    time.Now(),
			SenderAvatar: GetSenderAvatar(db, creatorID, "group_event_created"),
		}

		h.SendNotificationToUser(userID, message)
	}
}
