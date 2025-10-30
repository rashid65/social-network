package websocket

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func (c *Client) handleGifMessage(data interface{}) {
	// convert data to gifMessage type
	jsonData, _ := json.Marshal(data)
	var gifMsg ChatMessage
	if err := json.Unmarshal(jsonData, &gifMsg); err != nil {
		return
	}

	// Validate that this is actually a tenor URL
	if !c.isValidGifMessage(gifMsg) {
		c.sendGifError("Invalid GIF message")
		return
	}

	gifMsg.Timestamp = time.Now()
	gifMsg.SenderID = c.userID
	// DO NOT set gifMsg.ID here!
	gifMsg.MessageType = "media"

	// Get sender information from database
	var senderName, senderAvatar string
	err := c.hub.chatService.DB.QueryRow(
		"SELECT first_name || ' ' || last_name, COALESCE(avatar_path, '') FROM users WHERE id = ?",
		c.userID,
	).Scan(&senderName, &senderAvatar)
	if err != nil {
		return
	}
	gifMsg.SenderName = senderName
	gifMsg.SenderAvatar = senderAvatar

	// Save to DB and get chat_id and real message ID
	var chatID, messageID int64
	if gifMsg.RecipientID != "" {
		// private gif message
		chatID, messageID, err = c.hub.chatService.SaveMessageAndGetIDs(&gifMsg, "")
	} else if gifMsg.GroupID != "" {
		// group gif message
		chatID, messageID, err = c.hub.chatService.SaveMessageAndGetIDs(&gifMsg, gifMsg.GroupID)
	} else {
		return
	}
	if err != nil {
		return
	}

	// set the chat_id and real DB message ID
	gifMsg.ChatID = strconv.FormatInt(chatID, 10)
	gifMsg.ID = strconv.FormatInt(messageID, 10)

	message := WSMessage{
		Type:      TypeGif,
		Data:      gifMsg,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(message)

	if gifMsg.RecipientID != "" {
		// private: send to both users
		c.hub.SendToUser(gifMsg.RecipientID, msgData)
		c.hub.SendToUser(c.userID, msgData)
	} else if gifMsg.GroupID != "" {
		// group: send to all group participants (implement as needed)
		// Example: c.hub.SendToGroup(gifMsg.GroupID, msgData)
	}
}

func (c *Client) isValidGifMessage(gifMsg ChatMessage) bool {
	// check content if includes a valid tenor URL
	if gifMsg.Content == "" {
		return false
	}

	if strings.Contains(gifMsg.Content, "tenor.com") {
		return true
	}
	return false
}

func (c *Client) sendGifError(message string) {
	response := map[string]interface{}{
		"error":   true,
		"message": message,
		"type":    "media_error",
	}

	WSMessage := WSMessage{
		Type:      TypeGif,
		Data:      response,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(WSMessage)
	c.hub.SendToUser(c.userID, msgData)
}
