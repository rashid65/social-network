package websocket

import (
	"encoding/json"
	"log"
	"social-network/pkg/db"
	"strconv"
	"time"
)

func (c *Client) handleGroupInvitationMessage(data interface{}) {
	jsonData, _ := json.Marshal(data)
	var inviteMsg GroupInvitationMessage
	if err := json.Unmarshal(jsonData, &inviteMsg); err != nil {
		log.Printf("Error unmarshalling group invitation message: %v", err)
		return
	}

	switch inviteMsg.Action {
	case "notify_invitation":
		c.handleNotifyGroupInvitation(inviteMsg)
	case "notify_response":
		c.handleNotifyInvitationResponse(inviteMsg)
	default:
		log.Printf("Unknown group invitation action: %s", inviteMsg.Action)
	}
}

func (c *Client) handleNotifyGroupInvitation(inviteMsg GroupInvitationMessage) {
	invitationMessage := WSMessage{
		Type:      TypeGroupInvitation,
		Data:      inviteMsg,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(invitationMessage)
	c.hub.SendToUser(inviteMsg.InviteeID, msgData)
}

func (c *Client) handleNotifyInvitationResponse(inviteMsg GroupInvitationMessage) {
	wsMessage := WSMessage{
		Type:      TypeGroupInvitation,
		Data:      inviteMsg,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(wsMessage)
	c.hub.SendToUser(inviteMsg.InviterID, msgData)
}

// ----------------- http function ---------------------
func (h *Hub) NotifyGroupInvitation(inviterID, inviteeID, groupID, groupName, inviterName string) {
	// Create notification in database and get the real ID
	notification := Notification{
		UserID:   inviteeID,
		SenderID: inviterID,
		Type:     "group_invitation",
		RefID:    groupID,
		IsRead:   false,
		Message:  inviterName + " has invited you to join the group " + groupName,
	}

	notificationID, err := CreateNotificationAndGetID(db.DB, notification)
	if err != nil {
		log.Printf("Error creating group invitation notification: %v", err)
		return
	}

	// Create and send notification using the actual database ID
	notificationMsg := NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     inviterID,
		RecipientID:  inviteeID,
		Type:         "group_invitation",
		RefID:        groupID,
		Message:      inviterName + " has invited you to join the group " + groupName,
		Timestamp:    time.Now(),
		SenderAvatar: GetSenderAvatar(db.DB, inviterID, "group_invitation"), // <-- Ensure avatar is set
	}

	// Use the standard notification sending mechanism
	h.SendNotificationToUser(inviteeID, notificationMsg)

	// Also send the detailed group invitation message for UI purposes
	inviteMsg := GroupInvitationMessage{
		ID:          strconv.Itoa(notificationID),
		GroupID:     groupID,
		GroupName:   groupName,
		InviterID:   inviterID,
		InviterName: inviterName,
		InviteeID:   inviteeID,
		Action:      "received",
		Message:     notificationMsg.Message,
		Timestamp:   time.Now(),
	}

	// Send the detailed invitation message via WebSocket
	wsMessage := WSMessage{
		Type:      TypeGroupInvitation,
		Data:      inviteMsg,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(wsMessage)
	h.SendToUser(inviteeID, msgData)
}

func (h *Hub) NotifyInvitationResponse(inviterID, inviteeID, groupID, groupName, inviteeName, action string) {
	var message string
	if action == "accepted" {
		message = inviteeName + " accepted your invitation to " + groupName
	} else {
		message = inviteeName + " declined your invitation to " + groupName
	}

	// Create notification in database and get the real ID
	notification := Notification{
		UserID:   inviterID,
		SenderID: inviteeID,
		Type:     "group_invitation_response",
		RefID:    groupID,
		IsRead:   false,
		Message:  message,
	}

	notificationID, err := CreateNotificationAndGetID(db.DB, notification)
	if err != nil {
		log.Printf("Error creating invitation response notification: %v", err)
		return
	}

	// Create and send notification using the actual database ID
	notificationMsg := NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     inviteeID,
		RecipientID:  inviterID,
		Type:         "group_invitation_response",
		RefID:        groupID,
		Message:      message,
		Timestamp:    time.Now(),
		SenderAvatar: GetSenderAvatar(db.DB, inviteeID, "group_invitation_response"), // <-- Ensure avatar is set
	}

	// Use the standard notification sending mechanism
	h.SendNotificationToUser(inviterID, notificationMsg)

	// Also send the detailed response message for UI purposes
	responseMsg := GroupInvitationMessage{
		ID:          strconv.Itoa(notificationID),
		GroupID:     groupID,
		GroupName:   groupName,
		InviterID:   inviterID,
		InviteeID:   inviteeID,
		InviteeName: inviteeName,
		Action:      "response_received",
		Message:     message,
		Timestamp:   time.Now(),
	}

	// Send the detailed response message via WebSocket
	wsMessage := WSMessage{
		Type:      TypeGroupInvitation,
		Data:      responseMsg,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(wsMessage)
	h.SendToUser(inviterID, msgData)
}

// SendGroupJoinRequestNotification sends a notification to the group admin when someone requests to join
func SendGroupJoinRequestNotification(hub *Hub, requesterID, requesterName, adminID, groupID, groupName string) error {
	// Create notification in database and get the real ID
	notification := Notification{
		UserID:   adminID,
		SenderID: requesterID,
		Type:     "group_join_request",
		RefID:    groupID,
		IsRead:   false,
		Message:  requesterName + " request to join your group '" + groupName + "'",
	}

	notificationID, err := CreateNotificationAndGetID(db.DB, notification)
	if err != nil {
		log.Printf("Error creating group join request notification: %v", err)
		return err
	}

	// Send via WebSocket
	notificationMsg := NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     requesterID,
		RecipientID:  adminID,
		Type:         "group_join_request",
		RefID:        groupID,
		Message:      requesterName + " request to join your group '" + groupName + "'",
		Timestamp:    time.Now(),
		SenderAvatar: GetSenderAvatar(db.DB, requesterID, "group_join_request"), // <-- Ensure avatar is set
	}

	go hub.SendNotificationToUser(adminID, notificationMsg)
	return nil
}

// SendGroupRequestResponseNotification sends a notification when admin approves/declines a join request
func SendGroupRequestResponseNotification(hub *Hub, requesterID, groupID, groupName string, approved bool, senderID string) error {
	var notificationType, message string

	if approved {
		notificationType = "group_request_approved"
		message = "Your request to join '" + groupName + "' has been approved"
	} else {
		notificationType = "group_request_declined"
		message = "Your request to join '" + groupName + "' has been declined"
	}

	// Create notification in database and get the real ID
	notification := Notification{
		UserID:   requesterID,
		SenderID: senderID, // No specific sender for system notifications
		Type:     notificationType,
		RefID:    groupID,
		IsRead:   false,
		Message:  message,
	}

	notificationID, err := CreateNotificationAndGetID(db.DB, notification)
	if err != nil {
		log.Printf("Error creating group request response notification: %v", err)
		return err
	}

	// Send via WebSocket
	notificationMsg := NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     senderID,
		RecipientID:  requesterID,
		Type:         notificationType,
		RefID:        groupID,
		Message:      message,
		Timestamp:    time.Now(),
		SenderAvatar: GetSenderAvatar(db.DB, senderID, notificationType), // <-- Ensure avatar is set
	}

	go hub.SendNotificationToUser(requesterID, notificationMsg)
	return nil
}

// SendGroupKickNotification notifies a user that they have been removed from a group
func SendGroupKickNotification(hub *Hub, kickedUserID, groupID, senderID string) error {
	var groupName string
	err := db.DB.QueryRow("SELECT name FROM groups WHERE id = ?", groupID).Scan(&groupName)
	if err != nil {
		groupName = "The group"
	}

	notification := Notification{
		UserID:   kickedUserID,
		SenderID: senderID, // Use the actual sender's user ID
		Type:     "group_kick",
		RefID:    groupID,
		IsRead:   false,
		Message:  "You have been removed from " + groupName,
	}

	notificationID, err := CreateNotificationAndGetID(db.DB, notification)
	if err != nil {
		log.Printf("Error creating group kick notification: %v", err)
		return err
	}

	// Send via WebSocket
	notificationMsg := NotificationMessage{
		ID:           strconv.Itoa(notificationID),
		SenderID:     senderID,
		RecipientID:  kickedUserID,
		Type:         "group_kick",
		RefID:        groupID,
		Message:      "You have been removed from " + groupName,
		Timestamp:    time.Now(),
		SenderAvatar: GetSenderAvatar(db.DB, senderID, "group_kick"), // <-- Ensure avatar is set
	}

	go hub.SendNotificationToUser(kickedUserID, notificationMsg)
	return nil
}
