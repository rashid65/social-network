package event

import (
	"database/sql"
	"social-network/pkg/sockets/websocket"
	"strconv"
)

// -- Events in groups
// CREATE TABLE events (
//     id           INTEGER PRIMARY KEY AUTOINCREMENT,
//     group_id     INTEGER NOT NULL,
//     creator_id   TEXT    NOT NULL,
//     title        TEXT    NOT NULL,
//     description  TEXT    NOT NULL,
//     event_time   TEXT    NOT NULL,             -- ISO‑8601 datetime
//     created_at   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
//     FOREIGN KEY(group_id)   REFERENCES groups(id) ON DELETE CASCADE,
//     FOREIGN KEY(creator_id) REFERENCES users(id) ON DELETE CASCADE
// );

// CREATE TABLE event_responses (
//     id           INTEGER PRIMARY KEY AUTOINCREMENT,
//     event_id     INTEGER NOT NULL,
//     user_id      TEXT    NOT NULL,
//     response     TEXT    NOT NULL CHECK(response IN ('going','not_going')),
//     responded_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
//     FOREIGN KEY(event_id) REFERENCES events(id) ON DELETE CASCADE,
//     FOREIGN KEY(user_id)  REFERENCES users(id) ON DELETE CASCADE,
//     UNIQUE(event_id, user_id)
// );

type Event struct {
	ID          string `json:"id"`
	GroupID     string `json:"group_id"`
	CreatorID   string `json:"creator_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	EventTime   string `json:"event_time"`
	CreatedAt   string `json:"created_at"`
}

type EventResponse struct {
	ID          string `json:"id"`
	EventID     string `json:"event_id"`
	UserID      string `json:"user_id"`
	Response    string `json:"response"`
	RespondedAt string `json:"responded_at"`
}

func CreateEvent(db *sql.DB, e Event, hub *websocket.Hub) (Event, error) {
	// Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		return Event{}, err
	}

	// Defer a rollback in case anything fails
	defer tx.Rollback()

	query := `INSERT INTO events (group_id, creator_id, title, description, event_time)
              VALUES (?, ?, ?, ?, ?)`

	result, err := tx.Exec(query, e.GroupID, e.CreatorID, e.Title, e.Description, e.EventTime)
	if err != nil {
		return Event{}, err
	}

	// Get the last inserted ID
	lastID, err := result.LastInsertId()
	if err != nil {
		return Event{}, err
	}
	e.ID = strconv.Itoa(int(lastID))

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return Event{}, err
	}

	// Notify about the event creation
	go hub.NotifyGroupEventCreated(db, e.ID, e.GroupID, e.CreatorID, e.Title)

	return e, nil
}

func CreateEventResponse(db *sql.DB, er EventResponse) (EventResponse, error) {
	query := `INSERT INTO event_responses (event_id, user_id, response)
	          VALUES (?, ?, ?)`

	_, err := db.Exec(query, er.EventID, er.UserID, er.Response)
	if err != nil {
		return EventResponse{}, err
	}

	return er, nil
}

// GetEventsByGroupID retrieves all events for a group with response counts and user lists
func GetEventsByGroupID(db *sql.DB, groupID string, userID string) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            e.id, e.group_id, e.creator_id, e.title, e.description, e.event_time, e.created_at,
            COALESCE(u.nickname, u.first_name || ' ' || u.last_name) as creator_name,
            COALESCE(u.avatar_path, '') as creator_avatar
        FROM events e
        JOIN users u ON e.creator_id = u.id
        WHERE e.group_id = ?
        ORDER BY e.event_time ASC
    `

	rows, err := db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []map[string]interface{}

	for rows.Next() {
		var event Event
		var creatorName, creatorAvatar string

		err := rows.Scan(
			&event.ID, &event.GroupID, &event.CreatorID,
			&event.Title, &event.Description, &event.EventTime, &event.CreatedAt,
			&creatorName, &creatorAvatar,
		)
		if err != nil {
			return nil, err
		}

		// Get response counts and user lists
		goingUsers, notGoingUsers, err := getEventResponseUsers(db, event.ID)
		if err != nil {
			return nil, err
		}

		// Get user's response if userID is provided
		var userResponse *EventResponse
		if userID != "" {
			userResponse, _ = getUserEventResponse(db, event.ID, userID)
		}

		// Build response map
		eventData := map[string]interface{}{
			"id":          event.ID,
			"group_id":    event.GroupID,
			"creator_id":  event.CreatorID,
			"title":       event.Title,
			"description": event.Description,
			"event_time":  event.EventTime,
			"created_at":  event.CreatedAt,
			"creator": map[string]interface{}{
				"id":     event.CreatorID,
				"name":   creatorName,
				"avatar": creatorAvatar,
			},
			"going_count":     len(goingUsers),
			"not_going_count": len(notGoingUsers),
			"total_responses": len(goingUsers) + len(notGoingUsers),
			"going_users":     goingUsers,
			"not_going_users": notGoingUsers,
		}

		if userResponse != nil {
			eventData["user_response"] = userResponse.Response // Just "going" or "not_going"
		}

		events = append(events, eventData)
	}

	return events, nil
}

// getEventResponseUsers gets lists of user IDs for going and not going responses
func getEventResponseUsers(db *sql.DB, eventID string) ([]string, []string, error) {
	query := `
        SELECT user_id, response
        FROM event_responses 
        WHERE event_id = ?
        ORDER BY responded_at DESC
    `

	rows, err := db.Query(query, eventID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var goingUsers []string
	var notGoingUsers []string

	for rows.Next() {
		var userID, response string
		err := rows.Scan(&userID, &response)
		if err != nil {
			return nil, nil, err
		}

		if response == "going" {
			goingUsers = append(goingUsers, userID)
		} else if response == "not_going" {
			notGoingUsers = append(notGoingUsers, userID)
		}
	}

	return goingUsers, notGoingUsers, nil
}

// getUserEventResponse gets the current user's response for an event
func getUserEventResponse(db *sql.DB, eventID, userID string) (*EventResponse, error) {
	query := `
        SELECT id, event_id, user_id, response, responded_at 
        FROM event_responses 
        WHERE event_id = ? AND user_id = ?
    `

	var response EventResponse
	err := db.QueryRow(query, eventID, userID).Scan(
		&response.ID, &response.EventID, &response.UserID, &response.Response, &response.RespondedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &response, nil
}
