package websocket

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
)

type Notification struct {
	ID        int       `json:"id"` // Changed to int to match database
	UserID    string    `json:"user_id"`
	SenderID  string    `json:"sender_id"`
	Type      string    `json:"type"`
	RefID     string    `json:"ref_id"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message"`
}

func (c *Client) handleNotificationMessage(data interface{}) {
	jsonData, _ := json.Marshal(data)
	var notifMsg NotificationMessage
	if err := json.Unmarshal(jsonData, &notifMsg); err != nil {
		log.Printf("error unmarshalling notification message: %v", err)
		return
	}

	// Set sender ID from authenticated user
	notifMsg.SenderID = c.userID
	notifMsg.Timestamp = time.Now()
	notifMsg.SenderAvatar = GetSenderAvatar(c.hub.chatService.DB, notifMsg.SenderID, notifMsg.Type) // <-- Ensure avatar is set

	// Validate notification data
	if notifMsg.RecipientID == "" {
		log.Printf("notification missing recipient ID")
		return
	}

	if notifMsg.Type == "" {
		log.Printf("notification missing type")
		return
	}

	// Save notification to database and get the generated ID
	dbNotification := Notification{
		UserID:   notifMsg.RecipientID,
		SenderID: notifMsg.SenderID,
		Type:     notifMsg.Type,
		RefID:    notifMsg.RefID,
		IsRead:   false,
		Message:  notifMsg.Message,
	}

	notificationID, err := CreateNotificationAndGetID(c.hub.chatService.DB, dbNotification)
	if err != nil {
		log.Printf("error saving notification to database: %v", err)
		return
	}

	// Set the actual database ID
	notifMsg.ID = strconv.Itoa(notificationID)

	// Send notification via WebSocket to recipient
	c.hub.SendNotificationToUser(notifMsg.RecipientID, notifMsg)

	// Send acknowledgment back to sender
	ackMsg := WSMessage{
		Type: TypeNotification,
		Data: map[string]interface{}{
			"status":  "sent",
			"message": "Notification sent successfully",
			"id":      notificationID, // Include the actual ID in response
		},
		Timestamp: time.Now(),
	}

	ackData, _ := json.Marshal(ackMsg)
	c.hub.SendToUser(c.userID, ackData)
}

// New function that returns the inserted ID
func CreateNotificationAndGetID(db *sql.DB, notification Notification) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO notifications (user_id, sender_id, type, ref_id, is_read, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
	`
	result, err := tx.Exec(query, notification.UserID, notification.SenderID, notification.Type, notification.RefID, 0, notification.Message)
	if err != nil {
		return 0, err
	}

	// Get the last inserted ID
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return int(lastInsertID), nil
}

// Keep the old function for compatibility
func CreateNotification(db *sql.DB, notification Notification) error {
	_, err := CreateNotificationAndGetID(db, notification)
	return err
}

func GetNotificationsByUserID(db *sql.DB, userID string) ([]NotificationMessage, error) {
	query := `
		SELECT id, user_id, COALESCE(sender_id, ''), type, ref_id, is_read, created_at, message
		FROM notifications
		WHERE user_id = ?
		ORDER BY created_at DESC
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []NotificationMessage
	for rows.Next() {
		var n Notification
		var createdAt string

		err := rows.Scan(&n.ID, &n.UserID, &n.SenderID, &n.Type, &n.RefID, &n.IsRead, &createdAt, &n.Message)
		if err != nil {
			return nil, err
		}

		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

		notifications = append(notifications, NotificationMessage{
			ID:           strconv.Itoa(n.ID),
			SenderID:     n.SenderID,
			RecipientID:  n.UserID,
			Type:         n.Type,
			RefID:        n.RefID,
			IsRead:       n.IsRead,
			Message:      n.Message,
			Timestamp:    n.CreatedAt,
			SenderAvatar: GetSenderAvatar(db, n.SenderID, n.Type), // <-- Ensure avatar is set
		})
	}
	return notifications, nil
}

func MarkAsRead(db *sql.DB, notificationID int) error {
	query := `UPDATE notifications SET is_read = 1 WHERE id = ?`
	_, err := db.Exec(query, notificationID)
	return err
}

// Remove the fake ID generator - we don't need this anymore
// func GenerateNotificationID() string {
//     return "notif-" + generateMessageID()
// }

// Helper function to notify chat participants
func (c *Client) notifyChatParticipants(readMsg MessagesReadMessage) {
	// Get chat participants
	participants, err := c.chatService.getChatParticipants(readMsg.ChatID)
	if err != nil {
		log.Printf("error getting chat participants: %v", err)
		return
	}

	// Create WebSocket message
	message := WSMessage{
		Type:      TypeMessagesRead,
		Data:      readMsg,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(message)

	// Send to all chat participants
	c.hub.SendToUsers(participants, msgData)
}

func UpdateNotificationMessage(db *sql.DB, notificationID int, newMessage string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `UPDATE notifications SET message = ? WHERE id = ?`
	result, err := tx.Exec(query, newMessage, notificationID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return tx.Commit()
}

func GetNotificationByID(db *sql.DB, notificationID int) (*Notification, error) {
	query := `
        SELECT id, user_id, sender_id, type, ref_id, is_read, created_at, message 
        FROM notifications 
        WHERE id = ?
    `
	var notification Notification
	var createdAtStr string

	err := db.QueryRow(query, notificationID).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.SenderID,
		&notification.Type,
		&notification.RefID,
		&notification.IsRead,
		&createdAtStr,
		&notification.Message,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("notification not found")
		}
		return nil, err
	}

	// Parse the timestamp
	if createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
		notification.CreatedAt = createdAt
	}

	return &notification, nil
}
