package websocket

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type ChatService struct {
	DB *sql.DB
}

func NewChatService(db *sql.DB) *ChatService {
	return &ChatService{
		DB: db,
	}
}

// Add this helper function
func unmarshalData[T any](data interface{}) (*T, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var result T
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) handleMessage(message []byte) {
	var wsMsg WSMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		return
	}

	switch wsMsg.Type {
	case TypeChat:
		c.handleChatMessage(wsMsg.Data)
	case TypeTyping:
		c.handleTypingMessage(wsMsg.Data)
	case TypeGif:
		c.handleGifMessage(wsMsg.Data)
	case TypeMessagesRead:
		c.handleMessagesRead(wsMsg.Data)
	case TypeGroupInvitation:
		c.handleGroupInvitationMessage(wsMsg.Data)
	case TypeNotification:
		c.handleNotificationMessage(wsMsg.Data)
	case TypeUserStatusUpdate:
		c.handleUserStatusUpdate(wsMsg.Data)
	case TypeOnlineUsers:
		c.handleOnlineUsersRequest(wsMsg.Data)
	case TypeChatList:
		c.handleChatListRequest(wsMsg.Data)
	case TypeChatMessages: // New case
		c.handleChatMessagesRequest(wsMsg.Data)
	case "join_group": // handle group sync from frontend
		c.handleJoinGroup(wsMsg.Data)
	case "leave_group":
		c.handleLeaveGroup(wsMsg.Data)
	}
}

func (c *Client) handleChatMessage(data interface{}) {
	chatMsg, err := unmarshalData[ChatMessage](data)
	if err != nil {
		return
	}

	chatMsg.Timestamp = time.Now()
	chatMsg.SenderID = c.userID
	// DO NOT set chatMsg.ID here!

	// Validate message type
	if chatMsg.MessageType != "text" && chatMsg.MessageType != "emoji" &&
		chatMsg.MessageType != "media" && chatMsg.MessageType != "gif" {
		chatMsg.MessageType = "text"
	}

	// Get sender info
	var senderName, senderAvatar string
	err = c.hub.chatService.DB.QueryRow(
		"SELECT first_name || ' ' || last_name, COALESCE(avatar_path, '') FROM users WHERE id = ?",
		c.userID,
	).Scan(&senderName, &senderAvatar)
	if err != nil {
		return
	}
	chatMsg.SenderName = senderName
	chatMsg.SenderAvatar = senderAvatar

	// Save to DB and get chat_id and real message ID
	chatID, messageID, err := c.hub.chatService.SaveMessageAndGetIDs(chatMsg, chatMsg.GroupID)
	if err != nil {
		return
	}
	chatMsg.ChatID = strconv.FormatInt(chatID, 10)
	chatMsg.ID = strconv.FormatInt(messageID, 10) // Use the real DB ID

	// Send to recipients
	c.sendMessageToRecipients(chatMsg)
}

func (c *Client) handleTypingMessage(data interface{}) {
	typingMsg, err := unmarshalData[TypingMessage](data)
	if err != nil {
		return
	}

	c.hub.HandleTyping(typingMsg.ChatID, c.userID, typingMsg.NickName, typingMsg.IsTyping)
}

func (c *Client) handleMessagesRead(data interface{}) {
	readMsg, err := unmarshalData[MessagesReadMessage](data)
	if err != nil {
		return
	}

	readMsg.UserID = c.userID

	if err := c.updateReadMessages(*readMsg); err != nil {
		return
	}

	c.notifyChatParticipants(*readMsg)
}

func (s *ChatService) SaveMessageAndGetChatID(msg *ChatMessage, groupID string) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var chatID int64
	if groupID != "" {
		// Group message
		chatID, err = s.getOrCreateGroupChatThread(tx, groupID)
	} else {
		// Private message
		chatID, err = s.getOrCreatePrivateChatThread(tx, msg.SenderID, msg.RecipientID)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to get or create chat thread: %w", err)
	}

	// Save message
	_, err = tx.Exec(`
        INSERT INTO messages (chat_id, sender_id, content, message_type, created_at)
        VALUES (?, ?, ?, ?, ?)
        `, chatID, msg.SenderID, msg.Content, msg.MessageType, msg.Timestamp.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("failed to save message: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return chatID, nil
}

func (s *ChatService) SaveMessageAndGetIDs(msg *ChatMessage, groupID string) (chatID int64, messageID int64, err error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if groupID != "" {
		chatID, err = s.getOrCreateGroupChatThread(tx, groupID)
	} else {
		chatID, err = s.getOrCreatePrivateChatThread(tx, msg.SenderID, msg.RecipientID)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get or create chat thread: %w", err)
	}

	result, err := tx.Exec(`
        INSERT INTO messages (chat_id, sender_id, content, message_type, created_at)
        VALUES (?, ?, ?, ?, ?)`,
		chatID, msg.SenderID, msg.Content, msg.MessageType, msg.Timestamp.Format(time.RFC3339))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to save message: %w", err)
	}
	messageID, err = result.LastInsertId()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get message ID: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return chatID, messageID, nil
}

func (s *ChatService) SavePrivateMessageAndGetChatID(msg *ChatMessage) (int64, error) {
	return s.SaveMessageAndGetChatID(msg, "")
}

func (s *ChatService) SaveGroupMessageAndGetChatID(msg *ChatMessage, groupID string) (int64, error) {
	return s.SaveMessageAndGetChatID(msg, groupID)
}

func (s *ChatService) getOrCreatePrivateChatThread(tx *sql.Tx, userID1, userID2 string) (int64, error) {
	var chatID int64
	query := `
		SELECT ct.id
		FROM chat_threads ct
		JOIN chat_participants cp1 on ct.id = cp1.chat_id
		JOIN chat_participants cp2 on ct.id = cp2.chat_id
		WHERE ct.is_group = 0
		AND cp1.user_id = ?
		AND cp2.user_id = ?
		AND (
			SELECT COUNT(*)
			FROM chat_participants cp
			WHERE cp.chat_id = ct.id
		) = 2
	`

	err := tx.QueryRow(query, userID1, userID2).Scan(&chatID)
	if err == nil {
		// chat thread exists
		return chatID, nil
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to query chat thread: %w", err)
	}

	// create new chat thread
	results, err := tx.Exec(`
		INSERT INTO chat_threads (is_group, created_at)
		VALUES (0, datetime('now'))
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to create chat thread: %w", err)
	}

	chatID, err = results.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	// add users
	_, err = tx.Exec(`
		INSERT INTO chat_participants (chat_id, user_id)
		VALUES (?, ?), (?, ?)
	`, chatID, userID1, chatID, userID2)

	if err != nil {
		return 0, fmt.Errorf("failed to add participants: %w", err)
	}

	return chatID, nil
}

func (s *ChatService) getOrCreateGroupChatThread(tx *sql.Tx, groupID string) (int64, error) {
	var chatID int64
	err := tx.QueryRow(`
		SELECT id
		FROM chat_threads
		WHERE is_group = 1 AND group_id = ?
	`, groupID).Scan(&chatID)

	if err == nil {
		// chat thread exists
		return chatID, nil
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to query chat thread: %w", err)
	}

	result, err := tx.Exec(`
		INSERT INTO chat_threads (is_group, group_id, created_at)
		VALUES (1, ?, datetime('now'))
	`, groupID)
	if err != nil {
		return 0, fmt.Errorf("failed to create group chat thread: %w", err)
	}

	chatID, err = result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	// add members as participants(handling groups tables here)
	err = s.addGroupMembersToChat(tx, chatID, groupID)
	if err != nil {
		return 0, fmt.Errorf("failed to add group members to chat: %w", err)
	}

	return chatID, nil
}

func (s *ChatService) addGroupMembersToChat(tx *sql.Tx, chatID int64, groupID string) error {
	// First, get existing participants to avoid duplicates
	_, err := tx.Exec(`
        INSERT OR IGNORE INTO chat_participants (chat_id, user_id)
        SELECT ?, user_id
        FROM group_memberships
        WHERE group_id = ?
    `, chatID, groupID)

	return err
}

// Add function to add single user to group chat (for when users join later)
func (s *ChatService) AddUserToGroupChat(userID, groupID string) error {
	// Get the group's chat thread ID
	var chatID int64
	err := s.DB.QueryRow(`
        SELECT id FROM chat_threads 
        WHERE is_group = 1 AND group_id = ?
    `, groupID).Scan(&chatID)
	if err != nil {
		return fmt.Errorf("failed to find group chat thread: %w", err)
	}

	// Add user as participant
	_, err = s.DB.Exec(`
        INSERT OR IGNORE INTO chat_participants (chat_id, user_id)
        VALUES (?, ?)
    `, chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to add user to group chat: %w", err)
	}

	return nil
}

// RemoveUserFromGroupChat removes a user from a group's chat thread
func (s *ChatService) RemoveUserFromGroupChat(userID, groupID string) error {
	// Get the group's chat thread ID
	var chatID int64
	err := s.DB.QueryRow(`
        SELECT id FROM chat_threads 
        WHERE is_group = 1 AND group_id = ?
    `, groupID).Scan(&chatID)
	if err != nil {
		return fmt.Errorf("failed to find group chat thread: %w", err)
	}

	// Remove user as participant
	_, err = s.DB.Exec(`
        DELETE FROM chat_participants 
        WHERE chat_id = ? AND user_id = ?
    `, chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user from group chat: %w", err)
	}

	return nil
}

// AddUserToGroupChatTx adds a user to a group's chat thread within a transaction
func (s *ChatService) AddUserToGroupChatTx(tx *sql.Tx, userID, groupID string) error {
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

// RemoveUserFromGroupChatTx removes a user from a group's chat thread within a transaction
func (s *ChatService) RemoveUserFromGroupChatTx(tx *sql.Tx, userID, groupID string) error {
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

// SyncGroupChatParticipants synchronizes all group members with chat participants
func (s *ChatService) SyncGroupChatParticipants(groupID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the group's chat thread ID
	var chatID int64
	err = tx.QueryRow(`
        SELECT id FROM chat_threads 
        WHERE is_group = 1 AND group_id = ?
    `, groupID).Scan(&chatID)
	if err != nil {
		return fmt.Errorf("failed to find group chat thread: %w", err)
	}

	// Clear existing participants
	_, err = tx.Exec(`
        DELETE FROM chat_participants WHERE chat_id = ?
    `, chatID)
	if err != nil {
		return fmt.Errorf("failed to clear existing chat participants: %w", err)
	}

	// Add all current group members (including creator)
	_, err = tx.Exec(`
        INSERT INTO chat_participants (chat_id, user_id)
        SELECT ?, user_id FROM group_memberships WHERE group_id = ?
        UNION
        SELECT ?, creator_id FROM groups WHERE id = ?
    `, chatID, groupID, chatID, groupID)
	if err != nil {
		return fmt.Errorf("failed to sync chat participants: %w", err)
	}

	return tx.Commit()
}

func (s *ChatService) GetChatMessages(chatID string, limit int, offset int) ([]ChatMessage, error) {
	query := `
		SELECT m.id, m.chat_id, m.sender_id, u.first_name || ' ' || u.last_name as sender_name,
			COALESCE(u.avatar_path, '') as sender_avatar, m.content, m.message_type, m.created_at,
			CASE WHEN mr.message_id IS NOT NULL THEN 1 ELSE 0 END as is_read
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		LEFT JOIN message_reads mr ON m.id = mr.message_id
		WHERE m.chat_id = ?
		ORDER BY m.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.DB.Query(query, chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat messages: %w", err)
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		var createdAt string
		var isRead int

		err := rows.Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &msg.SenderName,
			&msg.SenderAvatar, &msg.Content, &msg.MessageType, &createdAt, &isRead)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat message: %w", err)
		}

		//Handle both old format and new RFC3339 format
		if timestamp, err := time.Parse(time.RFC3339, createdAt); err == nil {
			// New format with timezone: 2025-09-19T19:37:47+03:00
			msg.Timestamp = timestamp
		} else if timestamp, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			// Old format without timezone: 2025-09-19 01:26:38
			// Assume it's in UTC+3 timezone to match WebSocket format
			loc, _ := time.LoadLocation("UTC")
			timestamp = timestamp.In(loc).Add(3 * time.Hour) // Add 3 hours for UTC+3
			msg.Timestamp = timestamp
		} else {
			return nil, fmt.Errorf("failed to parse timestamp: %s", createdAt)
		}

		msg.IsRead = isRead == 1
		messages = append(messages, msg)
	}
	return messages, nil
}

func (s *ChatService) GetUserChats(userID string) ([]ChatRoom, error) {
	query := `
        SELECT 
            ct.id, 
            ct.is_group,
            ct.group_id,
            COALESCE(g.title, 
                (SELECT u.first_name || ' ' || u.last_name
                 FROM chat_participants cp
                 JOIN users u ON cp.user_id = u.id
                 WHERE cp.chat_id = ct.id AND cp.user_id != ?
                 LIMIT 1)
            ) as chat_name,
            CASE 
                WHEN ct.is_group = 1 THEN '/images/default-group.png'
                ELSE (
                    SELECT u.avatar_path
                    FROM chat_participants cp
                    JOIN users u ON cp.user_id = u.id
                    WHERE cp.chat_id = ct.id AND cp.user_id != ?
                    LIMIT 1
                )
            END as chat_avatar,
            -- Get last message data
            lm.id as last_msg_id,
            lm.sender_id as last_msg_sender_id,
            lm.content as last_msg_content,
            lm.message_type as last_msg_type,
            lm.created_at as last_msg_timestamp,
            u_sender.first_name || ' ' || u_sender.last_name as last_msg_sender_name,
            u_sender.avatar_path as last_msg_sender_avatar,
            -- Unread count
            COALESCE(unread_count.count, 0) as unread_count
        FROM chat_threads ct
        LEFT JOIN groups g ON ct.group_id = g.id
        JOIN chat_participants cp ON ct.id = cp.chat_id
        -- Get last message
        LEFT JOIN (
            SELECT m1.chat_id, m1.id, m1.sender_id, m1.content, m1.message_type, m1.created_at
            FROM messages m1
            INNER JOIN (
                SELECT chat_id, MAX(created_at) as max_created_at
                FROM messages
                GROUP BY chat_id
            ) m2 ON m1.chat_id = m2.chat_id AND m1.created_at = m2.max_created_at
        ) lm ON ct.id = lm.chat_id
        LEFT JOIN users u_sender ON lm.sender_id = u_sender.id
        -- Get unread count
        LEFT JOIN (
            SELECT m.chat_id, COUNT(*) as count
            FROM messages m
            LEFT JOIN message_reads mr ON m.id = mr.message_id AND mr.user_id = ?
            WHERE mr.message_id IS NULL AND m.sender_id != ?
            GROUP BY m.chat_id
        ) unread_count ON ct.id = unread_count.chat_id
        WHERE cp.user_id = ?
        -- NOTE: removed filter that excluded chats without any messages
        ORDER BY COALESCE(lm.created_at, ct.created_at) DESC
    `

	rows, err := s.DB.Query(query, userID, userID, userID, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user chats: %w", err)
	}
	defer rows.Close()

	var chats []ChatRoom
	for rows.Next() {
		var chat ChatRoom
		var isGroup int
		var groupID sql.NullString
		var avatar sql.NullString
		var lastMsgID, lastMsgSenderID, lastMsgContent, lastMsgType, lastMsgTimestamp sql.NullString
		var lastMsgSenderName, lastMsgSenderAvatar sql.NullString
		var unreadCount int

		err := rows.Scan(&chat.ID, &isGroup, &groupID, &chat.Name, &avatar,
			&lastMsgID, &lastMsgSenderID, &lastMsgContent, &lastMsgType, &lastMsgTimestamp,
			&lastMsgSenderName, &lastMsgSenderAvatar, &unreadCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat room: %w", err)
		}

		// SET THE TYPE AND GROUP_ID FIELDS
		if isGroup == 1 {
			chat.Type = "group"
			if groupID.Valid {
				chat.GroupID = groupID.String
			}
		} else {
			chat.Type = "private"
			chat.GroupID = "" // Ensure it's empty for private chats
		}

		// Set avatar
		chat.Avatar = ""
		if avatar.Valid {
			chat.Avatar = avatar.String
		}

		// Set unread count
		chat.UnreadCount = unreadCount

		// Set last message if exists
		if lastMsgID.Valid {
			//Handle both timestamp formats
			var timestamp time.Time
			if t, err := time.Parse(time.RFC3339, lastMsgTimestamp.String); err == nil {
				// New format with timezone
				timestamp = t
			} else if t, err := time.Parse("2006-01-02 15:04:05", lastMsgTimestamp.String); err == nil {
				// Old format without timezone - assume UTC+3
				loc, _ := time.LoadLocation("UTC")
				timestamp = t.In(loc).Add(3 * time.Hour)
			} else {
				// Fallback to current time if parsing fails
				timestamp = time.Now()
			}

			chat.LastMessage = &ChatMessage{
				ID:           lastMsgID.String,
				ChatID:       chat.ID,
				SenderID:     lastMsgSenderID.String,
				SenderName:   lastMsgSenderName.String,
				SenderAvatar: lastMsgSenderAvatar.String,
				Content:      lastMsgContent.String,
				MessageType:  lastMsgType.String,
				Timestamp:    timestamp,
				RecipientID:  "",
				GroupID:      chat.GroupID,
			}
		}

		// Get participants
		participants, err := s.getChatParticipants(chat.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get chat participants: %w", err)
		}
		chat.Participants = participants

		// Set member count for groups
		if chat.Type == "group" {
			chat.MemberCount = len(participants)
		}

		chats = append(chats, chat)
	}

	return chats, nil
}

func (s *ChatService) getChatParticipants(chatID string) ([]string, error) {
	rows, err := s.DB.Query(`
	    SELECT user_id
		FROM chat_participants
		WHERE chat_id = ?
	`, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat participants: %w", err)
	}
	defer rows.Close()

	var participants []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan participant user ID: %w", err)
		}
		participants = append(participants, userID)
	}
	return participants, nil
}

// Add method to get users that are related (following/followed by) to a specific user
func (s *ChatService) getRelatedUsers(userID string) ([]string, error) {
	query := `
        SELECT DISTINCT 
            CASE 
                WHEN follower_id = ? THEN followee_id
                ELSE follower_id
            END as related_user_id
        FROM followers 
        WHERE (follower_id = ? OR followee_id = ?) 
    `

	rows, err := s.DB.Query(query, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relatedUsers []string
	for rows.Next() {
		var relatedUserID string
		if err := rows.Scan(&relatedUserID); err != nil {
			continue
		}
		relatedUsers = append(relatedUsers, relatedUserID)
	}

	return relatedUsers, nil
}

// Add method to check if user is a participant of a chat
func (s *ChatService) IsUserChatParticipant(userID, chatID string) (bool, error) {
	var count int
	err := s.DB.QueryRow(`
        SELECT COUNT(*)
        FROM chat_participants
        WHERE chat_id = ? AND user_id = ?
    `, chatID, userID).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check chat participation: %w", err)
	}

	return count > 0, nil
}

// Add method to get total message count for a chat
func (s *ChatService) GetChatMessageCount(chatID string) (int, error) {
	var count int
	err := s.DB.QueryRow(`
        SELECT COUNT(*)
        FROM messages
        WHERE chat_id = ?
    `, chatID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to get message count: %w", err)
	}

	return count, nil
}

func (c *Client) saveMessageToDatabase(chatMsg *ChatMessage) (int64, error) {
	if chatMsg.RecipientID != "" {
		return c.chatService.SavePrivateMessageAndGetChatID(chatMsg)
	} else if chatMsg.GroupID != "" {
		return c.chatService.SaveGroupMessageAndGetChatID(chatMsg, chatMsg.GroupID)
	}
	return 0, fmt.Errorf("no valid recipient or group specified")
}

func (c *Client) sendMessageToRecipients(chatMsg *ChatMessage) {
	message := WSMessage{
		Type:      TypeChat,
		Data:      *chatMsg,
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(message)

	if chatMsg.RecipientID != "" {
		// Private message
		c.hub.SendToUser(chatMsg.RecipientID, msgData)
		c.hub.SendToUser(chatMsg.SenderID, msgData) // ack
	} else if chatMsg.GroupID != "" {
		// Group message
		participants, err := c.chatService.getChatParticipants(chatMsg.ChatID)
		if err != nil {
			return
		}
		c.hub.SendToUsers(participants, msgData)
	}
}

// Helper function to update read messages in database
func (c *Client) updateReadMessages(readMsg MessagesReadMessage) error {
	// Start transaction for batch update
	tx, err := c.hub.chatService.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update messages as read for this user
	for _, messageID := range readMsg.MessageIDs {
		// Check if read record already exists
		var count int
		err = tx.QueryRow(
			"SELECT COUNT(*) FROM message_reads WHERE message_id = ? AND user_id = ?",
			messageID, readMsg.UserID,
		).Scan(&count)
		if err != nil {
			return err
		}

		// Only insert if not already marked as read
		if count == 0 {
			// Use RFC3339 format instead of datetime('now')
			now := time.Now().Format(time.RFC3339)
			_, err = tx.Exec(
				"INSERT INTO message_reads (message_id, user_id, read_at) VALUES (?, ?, ?)",
				messageID, readMsg.UserID, now,
			)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *ChatService) GetOrCreatePrivateChat(userID1, userID2 string) (*ChatRoom, error) {
	// Always order user IDs to avoid duplicate chats
	if userID1 > userID2 {
		userID1, userID2 = userID2, userID1
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var chatID int64
	query := `
        SELECT ct.id
        FROM chat_threads ct
        JOIN chat_participants cp1 ON ct.id = cp1.chat_id
        JOIN chat_participants cp2 ON ct.id = cp2.chat_id
        WHERE ct.is_group = 0
        AND cp1.user_id = ?
        AND cp2.user_id = ?
        AND (
            SELECT COUNT(*) FROM chat_participants cp WHERE cp.chat_id = ct.id
        ) = 2
        LIMIT 1
    `
	err = tx.QueryRow(query, userID1, userID2).Scan(&chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create new chat thread
			result, err := tx.Exec(`
                INSERT INTO chat_threads (is_group, created_at)
                VALUES (0, datetime('now'))
            `)
			if err != nil {
				return nil, err
			}
			chatID, err = result.LastInsertId()
			if err != nil {
				return nil, err
			}
			// Add both users as participants
			_, err = tx.Exec(`
                INSERT INTO chat_participants (chat_id, user_id)
                VALUES (?, ?), (?, ?)
            `, chatID, userID1, chatID, userID2)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Now fetch the chat room info
	chatRoom, err := s.getChatRoomByID(chatID, userID1)
	if err != nil {
		return nil, err
	}
	return chatRoom, nil
}

// Helper to fetch chat room info by ID
func (s *ChatService) getChatRoomByID(chatID int64, currentUserID string) (*ChatRoom, error) {
	query := `
        SELECT 
            ct.id, 
            ct.is_group,
            ct.group_id,
            COALESCE(
                (SELECT u.first_name || ' ' || u.last_name
                 FROM chat_participants cp
                 JOIN users u ON cp.user_id = u.id
                 WHERE cp.chat_id = ct.id AND cp.user_id != ?
                 LIMIT 1
                ), 'Private Chat'
            ) as chat_name,
            (
                SELECT u.avatar_path
                FROM chat_participants cp
                JOIN users u ON cp.user_id = u.id
                WHERE cp.chat_id = ct.id AND cp.user_id != ?
                LIMIT 1
            ) as chat_avatar
        FROM chat_threads ct
        WHERE ct.id = ?
    `
	var chat ChatRoom
	var isGroup int
	var groupID sql.NullString
	var avatar sql.NullString

	err := s.DB.QueryRow(query, currentUserID, currentUserID, chatID).Scan(
		&chat.ID, &isGroup, &groupID, &chat.Name, &avatar,
	)
	if err != nil {
		return nil, err
	}

	if isGroup == 1 {
		chat.Type = "group"
		if groupID.Valid {
			chat.GroupID = groupID.String
		}
	} else {
		chat.Type = "private"
		chat.GroupID = ""
	}
	chat.Avatar = ""
	if avatar.Valid {
		chat.Avatar = avatar.String
	}

	// Get participants
	participants, err := s.getChatParticipants(chat.ID)
	if err != nil {
		return nil, err
	}
	chat.Participants = participants

	// Set unread count to 0 (optional, or you can fetch real count)
	chat.UnreadCount = 0

	return &chat, nil
}

// Minimal payload for join/leave messages
type groupSyncPayload struct {
	GroupID string `json:"group_id"`
}

func (c *Client) handleJoinGroup(data interface{}) {
	payload, err := unmarshalData[groupSyncPayload](data)
	if err != nil || payload.GroupID == "" {
		return
	}

	// Ensure a group chat thread exists, and add current user as participant
	tx, err := c.hub.chatService.DB.Begin()
	if err == nil {
		// Try to find existing chat thread
		var chatID int64
		errFind := tx.QueryRow(`SELECT id FROM chat_threads WHERE is_group = 1 AND group_id = ?`, payload.GroupID).Scan(&chatID)
		if errFind == sql.ErrNoRows {
			// Create chat thread for the group
			res, errInsert := tx.Exec(`INSERT INTO chat_threads (is_group, group_id, created_at) VALUES (1, ?, datetime('now'))`, payload.GroupID)
			if errInsert == nil {
				chatID, _ = res.LastInsertId()
				// Add all current members and the creator (covers existing members)
				_, _ = tx.Exec(`
                    INSERT OR IGNORE INTO chat_participants (chat_id, user_id)
                    SELECT ?, user_id FROM group_memberships WHERE group_id = ?`,
					chatID, payload.GroupID)
				_, _ = tx.Exec(`
                    INSERT OR IGNORE INTO chat_participants (chat_id, user_id)
                    SELECT ?, creator_id FROM groups WHERE id = ?`,
					chatID, payload.GroupID)
			}
		}
		_ = tx.Commit()
	}

	// Ensure this user is in the group chat participants (in case they joined after)
	_ = c.hub.chatService.AddUserToGroupChat(c.userID, payload.GroupID)

	// Send back updated chat list
	c.sendChatList()
}

func (c *Client) handleLeaveGroup(data interface{}) {
	payload, err := unmarshalData[groupSyncPayload](data)
	if err != nil || payload.GroupID == "" {
		return
	}
	_ = c.hub.chatService.RemoveUserFromGroupChat(c.userID, payload.GroupID)

	// Send back updated chat list
	c.sendChatList()
}

func (c *Client) sendChatList() {
	chats, err := c.hub.chatService.GetUserChats(c.userID)
	if err != nil {
		return
	}
	resp := WSMessage{
		Type:      TypeChatList,
		Data:      map[string]interface{}{"chats": chats},
		Timestamp: time.Now(),
	}
	b, _ := json.Marshal(resp)
	// Send only to this client
	c.send <- b
}
