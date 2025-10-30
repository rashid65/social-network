package follow

import (
	"log"
	"social-network/pkg/sockets/websocket"
	"strconv"
	"time"
)

func (s *FollowService) sendFollowRequestNotification(followerID, followeeID string) {
	// Get follower name for notification
	var followerName string
	err := s.DB.QueryRow(
		"SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
		followerID,
	).Scan(&followerName)
	if err != nil {
		log.Printf("error getting follower name: %v", err)
		followerName = "Unknown User"
	}

	// Create notification in database and get the real ID
	notification := websocket.Notification{
		UserID:   followeeID,
		SenderID: followerID,
		Type:     "follow_request",
		RefID:    followerID, // Using followerID as reference
		IsRead:   false,
		Message:  followerName + " wants to follow you",
	}

	notificationID, err := websocket.CreateNotificationAndGetID(s.DB, notification)
	if err != nil {
		log.Printf("Error creating follow request notification: %v", err)
		return
	}

	notificationMsg := websocket.NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     followerID,
		RecipientID:  followeeID,
		Type:         "follow_request",
		RefID:        followerID, // Using followerID as reference
		Message:      followerName + " wants to follow you",
		Timestamp:    time.Now(),
		SenderAvatar: websocket.GetSenderAvatar(s.DB, followerID, "follow_request"),
	}

	s.Hub.SendNotificationToUser(followeeID, notificationMsg)
}

func (s *FollowService) sendFollowNotification(followerID, followeeID string) {
	// Get follower name for notification
	var followerName string
	err := s.DB.QueryRow(
		"SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
		followerID,
	).Scan(&followerName)
	if err != nil {
		log.Printf("error getting follower name: %v", err)
		followerName = "Unknown User"
	}

	// Notify followee - create notification in database
	notification := websocket.Notification{
		UserID:   followeeID,
		SenderID: followerID,
		Type:     "follow",
		RefID:    followerID,
		IsRead:   false,
		Message:  followerName + " is now following you",
	}

	notificationID, err := websocket.CreateNotificationAndGetID(s.DB, notification)
	if err != nil {
		log.Printf("Error creating follow notification: %v", err)
		return
	}

	notificationMsg := websocket.NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     followerID,
		RecipientID:  followeeID,
		Type:         "follow",
		RefID:        followerID, // Using followerID as reference
		Message:      followerName + " is now following you",
		Timestamp:    time.Now(),
		SenderAvatar: websocket.GetSenderAvatar(s.DB, followerID, "follow"),
	}

	s.Hub.SendNotificationToUser(followeeID, notificationMsg)

	// Notify follower (confirmation) - create another notification in database
	confirmationNotification := websocket.Notification{
		UserID:   followerID,
		SenderID: followeeID,
		Type:     "follow_success",
		RefID:    followeeID,
		IsRead:   false,
		Message:  "Your follow request has been accepted",
	}

	confirmationNotificationID, err := websocket.CreateNotificationAndGetID(s.DB, confirmationNotification)
	if err != nil {
		log.Printf("Error creating follow confirmation notification: %v", err)
		return
	}

	confirmationNotificationMsg := websocket.NotificationMessage{
		ID:           strconv.Itoa(confirmationNotificationID),
		SenderID:     followeeID,
		RecipientID:  followerID,
		Type:         "follow_success",
		RefID:        followeeID, // Using followeeID as reference
		Message:      "Your follow request has been accepted",
		Timestamp:    time.Now(),
		SenderAvatar: websocket.GetSenderAvatar(s.DB, followeeID, "follow_success"),
	}

	s.Hub.SendNotificationToUser(followerID, confirmationNotificationMsg)
}

func (s *FollowService) sendAcceptNotification(followerID, followeeID string) {
	// Get followee name for notification
	var followeeName string
	err := s.DB.QueryRow(
		"SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
		followeeID,
	).Scan(&followeeName)
	if err != nil {
		log.Printf("error getting followee name: %v", err)
		followeeName = "Unknown User"
	}

	// Create notification in database and get the real ID
	notification := websocket.Notification{
		UserID:   followerID,
		SenderID: followeeID,
		Type:     "follow_accepted",
		RefID:    followeeID,
		IsRead:   false,
		Message:  followeeName + " accepted your follow request",
	}

	notificationID, err := websocket.CreateNotificationAndGetID(s.DB, notification)
	if err != nil {
		log.Printf("Error creating follow accept notification: %v", err)
		return
	}

	notificationMsg := websocket.NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     followeeID,
		RecipientID:  followerID,
		Type:         "follow_accepted",
		RefID:        followeeID, // Using followeeID as reference
		Message:      followeeName + " accepted your follow request",
		Timestamp:    time.Now(),
		SenderAvatar: websocket.GetSenderAvatar(s.DB, followeeID, "follow_accepted"),
	}

	s.Hub.SendNotificationToUser(followerID, notificationMsg)
}

func (s *FollowService) sendRejectNotification(followerID, followeeID string) {
	// Get followee name for notification
	var followeeName string
	err := s.DB.QueryRow(
		"SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
		followeeID,
	).Scan(&followeeName)
	if err != nil {
		log.Printf("error getting followee name: %v", err)
		followeeName = "Unknown User"
	}

	// Create notification in database and get the real ID
	notification := websocket.Notification{
		UserID:   followerID,
		SenderID: followeeID,
		Type:     "follow_rejected",
		RefID:    followeeID,
		IsRead:   false,
		Message:  followeeName + " declined your follow request",
	}

	notificationID, err := websocket.CreateNotificationAndGetID(s.DB, notification)
	if err != nil {
		log.Printf("Error creating follow reject notification: %v", err)
		return
	}

	notificationMsg := websocket.NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     followeeID,
		RecipientID:  followerID,
		Type:         "follow_rejected",
		RefID:        followeeID, // Using followeeID as reference
		Message:      followeeName + " declined your follow request",
		Timestamp:    time.Now(),
		SenderAvatar: websocket.GetSenderAvatar(s.DB, followeeID, "follow_rejected"),
	}

	s.Hub.SendNotificationToUser(followerID, notificationMsg)
}

func (s *FollowService) sendUnfollowNotification(followerID, followeeID string) {
	// Get followee name for notification
	var followeeName string
	err := s.DB.QueryRow(
		"SELECT first_name || ' ' || last_name FROM users WHERE id = ?",
		followeeID,
	).Scan(&followeeName)
	if err != nil {
		log.Printf("error getting followee name: %v", err)
		followeeName = "Unknown User"
	}

	// Create notification in database and get the real ID
	notification := websocket.Notification{
		UserID:   followerID,
		SenderID: followerID,
		Type:     "unfollow",
		RefID:    followeeID,
		IsRead:   false,
		Message:  "You have unfollowed " + followeeName,
	}

	notificationID, err := websocket.CreateNotificationAndGetID(s.DB, notification)
	if err != nil {
		log.Printf("Error creating unfollow notification: %v", err)
		return
	}

	notificationMsg := websocket.NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     followerID,
		RecipientID:  followerID, // Sending to self as confirmation
		Type:         "unfollow",
		RefID:        followeeID, // Using followeeID as reference
		Message:      "You have unfollowed " + followeeName,
		Timestamp:    time.Now(),
		SenderAvatar: websocket.GetSenderAvatar(s.DB, followerID, "unfollow"),
	}

	s.Hub.SendNotificationToUser(followerID, notificationMsg)
}
