package group

import (
	"database/sql"
	"fmt"
	"strconv"
)

type Group struct {
	ID          string `json:"id"`
	CreatorID   string `json:"creator_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"` // true if public group, false if private
	CreatedAt   string `json:"created_at"`
	ChatID      int64  `json:"chat_id,omitempty"`
}

type GroupInvitation struct {
	ID        string `json:"id"`
	GroupID   string `json:"group_id"`
	InviterID string `json:"inviter_id"`
	InviteeID string `json:"invitee_id"`
	Status    string `json:"status"` // e.g., "pending", "accepted", "declined"
	CreatedAt string `json:"created_at"`
}

type GroupRequest struct {
	ID          string `json:"id"`
	RequesterID string `json:"requester_id"`
	AdminID     string `json:"admin_id"`
	GroupID     string `json:"group_id"`
	GroupName   string `json:"group_name"`
	Status      string `json:"status"` // e.g., "pending", "accepted", "declined"
	CreatedAt   string `json:"created_at"`
}

func CreateGroup(db *sql.DB, g Group) (Group, error) {
    tx, err := db.Begin()
    if err != nil {
        return Group{}, fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    // 1. Insert group
    query := `INSERT INTO groups (creator_id, title, description, is_public) VALUES (?, ?, ?, ?)`
    result, err := tx.Exec(query, g.CreatorID, g.Title, g.Description, g.IsPublic)
    if err != nil {
        return Group{}, fmt.Errorf("failed to create group: %w", err)
    }

    lastID, err := result.LastInsertId()
    if err != nil {
        return Group{}, fmt.Errorf("failed to get last insert ID: %w", err)
    }

    // 2. Fetch the newly created group (including created_at)
    var created Group
    getQuery := `SELECT id, creator_id, title, description, is_public, created_at FROM groups WHERE id = ?`
    err = tx.QueryRow(getQuery, lastID).Scan(
        &created.ID,
        &created.CreatorID,
        &created.Title,
        &created.Description,
        &created.IsPublic,
        &created.CreatedAt,
    )
    if err != nil {
        return Group{}, fmt.Errorf("failed to fetch created group: %w", err)
    }

    // 3. Create chat thread FIRST (before adding members)
    chatID, err := createGroupChatThread(tx, lastID, created.CreatorID)
    if err != nil {
        return Group{}, fmt.Errorf("failed to create group chat thread: %w", err)
    }

    // 4. Add the creator as admin to group_memberships AFTER chat thread exists
    err = AddUserToGroupTx(tx, lastID, created.CreatorID, "admin")
    if err != nil {
        return Group{}, fmt.Errorf("failed to add creator as admin: %w", err)
    }

    // Store chat_id in the created group struct for response
    created.ChatID = chatID

    // Commit transaction
    if err = tx.Commit(); err != nil {
        return Group{}, fmt.Errorf("failed to commit transaction: %w", err)
    }

    return created, nil
}

// Helper function to create group chat thread and add creator as participant
func createGroupChatThread(tx *sql.Tx, groupID int64, creatorID string) (int64, error) {
    // Create chat thread for the group
    result, err := tx.Exec(`
        INSERT INTO chat_threads (is_group, group_id, created_at)
        VALUES (1, ?, datetime('now'))
    `, groupID)
    if err != nil {
        return 0, fmt.Errorf("failed to create group chat thread: %w", err)
    }

    chatID, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("failed to get chat thread ID: %w", err)
    }

    // Add creator as chat participant
    _, err = tx.Exec(`
        INSERT INTO chat_participants (chat_id, user_id)
        VALUES (?, ?)
    `, chatID, creatorID)
    if err != nil {
        return 0, fmt.Errorf("failed to add creator to chat: %w", err)
    }

    return chatID, nil
}

// Transaction-safe version of AddUserToGroup with chat sync
func AddUserToGroupTx(tx *sql.Tx, groupID int64, userID, role string) error {
    // Add to group_memberships first
    _, err := tx.Exec(`
        INSERT INTO group_memberships (group_id, user_id, role, joined_at)
        VALUES (?, ?, ?, datetime('now'))
    `, groupID, userID, role)
    if err != nil {
        return fmt.Errorf("failed to add user to group memberships: %w", err)
    }

    // Find the group's chat thread
    var chatID int64
    err = tx.QueryRow(`
        SELECT id FROM chat_threads 
        WHERE is_group = 1 AND group_id = ?
    `, groupID).Scan(&chatID)
    if err != nil {
        if err == sql.ErrNoRows {
            return fmt.Errorf("group chat thread not found for group %d - this indicates a data integrity issue", groupID)
        }
        return fmt.Errorf("failed to find group chat thread: %w", err)
    }

    // Add user to chat participants
    _, err = tx.Exec(`
        INSERT OR IGNORE INTO chat_participants (chat_id, user_id)
        VALUES (?, ?)
    `, chatID, userID)
    if err != nil {
        return fmt.Errorf("failed to add user to group chat: %w", err)
    }

    return nil
}

func CreateGroupInvitation(db *sql.DB, groupInv GroupInvitation) (GroupInvitation, error) {
	// First, clean up any old invitations for this user-group pair
	// This allows re-inviting users who previously declined or were kicked
	_, err := db.Exec(`
        DELETE FROM group_invitations 
        WHERE group_id = ? AND invitee_id = ? AND status != 'pending'
    `, groupInv.GroupID, groupInv.InviteeID)
	if err != nil {
		return GroupInvitation{}, err
	}

	// Now create the new invitation
	query := `
        INSERT INTO group_invitations (group_id, inviter_id, invitee_id, status, created_at) 
        VALUES (?, ?, ?, ?, datetime('now'))
    `
	result, err := db.Exec(query, groupInv.GroupID, groupInv.InviterID, groupInv.InviteeID, groupInv.Status)
	if err != nil {
		return GroupInvitation{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return GroupInvitation{}, err
	}

	groupInv.ID = strconv.FormatInt(id, 10)
	return groupInv, nil
}

func CreateGroupRequest(db *sql.DB, gr GroupRequest) (GroupRequest, error) {
	if err := gr.ValidateGroupRequest(db); err != nil {
		return GroupRequest{}, err
	}

	query := `INSERT INTO group_requests (requester_id, group_id, status)
			  VALUES (?, ?, ?)`

	result, err := db.Exec(query, gr.RequesterID, gr.GroupID, gr.Status)
	if err != nil {
		return GroupRequest{}, err
	}

	//get last inserted ID and assign it to gr.ID
	lastID, err := result.LastInsertId()
	if err != nil {
		return GroupRequest{}, err
	}
	gr.ID = strconv.Itoa(int(lastID))

	// get group admin and group name
	err = db.QueryRow("SELECT creator_id, title FROM groups WHERE id = ?", gr.GroupID).Scan(&gr.AdminID, &gr.GroupName)
	if err != nil {
		return GroupRequest{}, err
	}

	return gr, nil
}

// Function to accept a group invitation
func AcceptGroupInvitation(db *sql.DB, gi GroupInvitation) error {
	// Validate the invitation response
	if err := gi.ValidateGroupInvitationResponse(db); err != nil {
		return err
	}

	// Get the group ID for this invitation
	var groupID string
	err := db.QueryRow("SELECT group_id FROM group_invitations WHERE id = ?", gi.ID).Scan(&groupID)
	if err != nil {
		return err
	}

	query := `UPDATE group_invitations SET status = 'accepted', responded_at = datetime('now')
              WHERE id = ? AND invitee_id = ?`

	_, err = db.Exec(query, gi.ID, gi.InviteeID)
	if err != nil {
		return err
	}

	// Add user to the group
	return AddUserToGroup(db, groupID, gi.InviteeID, "member")
}

// Function to decline a group invitation
func DeclineGroupInvitation(db *sql.DB, gi GroupInvitation) error {
	// Validate the invitation response
	if err := gi.ValidateGroupInvitationResponse(db); err != nil {
		return err
	}

	query := `UPDATE group_invitations SET status = 'declined', responded_at = datetime('now')
              WHERE id = ? AND invitee_id = ?`

	_, err := db.Exec(query, gi.ID, gi.InviteeID)
	if err != nil {
		return err
	}

	return nil
}

// Function to accept a group request
func AcceptGroupRequest(db *sql.DB, gr GroupRequest) error {
	if err := gr.ValidateGroupRequestResponse(db); err != nil {
		return err
	}

	var groupID string
	err := db.QueryRow("SELECT group_id FROM group_requests WHERE id = ?", gr.ID).Scan(&groupID)
	if err != nil {
		return err
	}

	query := `UPDATE group_requests SET status = 'accepted', responded_at = datetime('now')
              WHERE id = ? AND requester_id = ?`

	_, err = db.Exec(query, gr.ID, gr.RequesterID)
	if err != nil {
		return err
	}

	AddUserToGroup(db, gr.GroupID, gr.RequesterID, "member")

	return nil
}

// Function to decline a group request
func DeclineGroupRequest(db *sql.DB, gr GroupRequest) error {
	if err := gr.ValidateGroupRequestResponse(db); err != nil {
		return err
	}

	query := `UPDATE group_requests SET status = 'declined', responded_at = datetime('now')
			  WHERE id = ? AND requester_id = ?`

	_, err := db.Exec(query, gr.ID, gr.RequesterID)
	if err != nil {
		return err
	}

	return nil
}

// function to add a user to a group
func AddUserToGroup(db *sql.DB, groupID string, userID, role string) error {
	query := `INSERT INTO group_memberships (group_id, user_id, role) VALUES (?, ?, ?)`
	_, err := db.Exec(query, groupID, userID, role)
	return err
}

// GetGroupsByUserID retrieves all groups for a specific user ID
func GetGroupsByUserID(db *sql.DB, userID string) ([]Group, error) {
	rows, err := db.Query(`
        SELECT g.id, g.creator_id, g.title, g.description, g.is_public, g.created_at
        FROM groups g
        INNER JOIN group_memberships gm ON g.id = gm.group_id
        WHERE gm.user_id = ?
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.CreatorID, &g.Title, &g.Description, &g.IsPublic, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func GetGroupByID(db *sql.DB, groupID string) (*Group, error) {
	var g Group
	err := db.QueryRow(`
        SELECT id, creator_id, title, description, is_public, created_at
        FROM groups
        WHERE id = ?
    `, groupID).Scan(&g.ID, &g.CreatorID, &g.Title, &g.Description, &g.IsPublic, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// SearchGroups searches for groups by title or description
func SearchGroups(db *sql.DB, query, userID string, limit, offset int) ([]map[string]interface{}, error) {
	searchPattern := "%" + query + "%"
	rows, err := db.Query(`
        SELECT DISTINCT g.id, g.title, g.description, g.is_public, g.creator_id, g.created_at,
            CASE WHEN gm.user_id IS NOT NULL THEN 1 ELSE 0 END as is_member,
            COALESCE(gm.role, '') as role
        FROM groups g
        LEFT JOIN group_memberships gm ON g.id = gm.group_id AND gm.user_id = ?
        WHERE g.title LIKE ?
        ORDER BY 
            is_member DESC,
            CASE 
                WHEN g.title LIKE ? THEN 1 
                ELSE 2
            END
        LIMIT ? OFFSET ?
    `, userID, searchPattern, searchPattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []map[string]interface{}
	for rows.Next() {
		var id, title, description, creatorID, createdAt, role string
		var isPublic, isMember int
		if err := rows.Scan(&id, &title, &description, &isPublic, &creatorID, &createdAt, &isMember, &role); err != nil {
			return nil, err
		}

		groups = append(groups, map[string]interface{}{
			"id":          id,
			"title":       title,
			"description": description,
			"is_public":   isPublic == 1,
			"creator_id":  creatorID,
			"created_at":  createdAt,
			"is_member":   isMember == 1,
			"role":        role,
		})
	}

	return groups, nil
}
