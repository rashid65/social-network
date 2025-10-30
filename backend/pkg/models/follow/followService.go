package follow

import (
	"database/sql"
	"errors"
	"log"
)

func NewFollowService(db *sql.DB, hub WebSocketHub) *FollowService {
	return &FollowService{
		DB:  db,
		Hub: hub,
	}
}

// Add the missing IsFollowing method
func (s *FollowService) IsFollowing(followerID, followeeID string) (bool, error) {
	var count int
	err := s.DB.QueryRow(
		"SELECT COUNT(*) FROM followers WHERE follower_id = ? AND followee_id = ?",
		followerID, followeeID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Add the missing followRequestExists method
func (s *FollowService) followRequestExists(followerID, followeeID string) (bool, error) {
	var count int
	err := s.DB.QueryRow(
		"SELECT COUNT(*) FROM follow_requests WHERE requester_id = ? AND recipient_id = ? AND status = 'pending'",
		followerID, followeeID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *FollowService) SendFollowRequest(followerID, followeeID string) error {
	// Check if already following
	isFollowing, err := s.IsFollowing(followerID, followeeID)
	if err != nil {
		return err
	}
	if isFollowing {
		return errors.New("you are already following this user")
	}

	// Check if follow request already exists
	exists, err := s.followRequestExists(followerID, followeeID)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("follow request already exists")
	}

	// check if the user has a public or private profile
	var isPublic bool
	err = s.DB.QueryRow(
		"SELECT is_public FROM users WHERE id = ?",
		followeeID,
	).Scan(&isPublic)
	if err != nil {
		return err
	}

	if isPublic {
		return s.followImmediately(followerID, followeeID)
	}

	// Insert the follow request only for private profiles
	_, err = s.DB.Exec(
		"INSERT INTO follow_requests (requester_id, recipient_id, status, created_at) VALUES (?, ?, 'pending', datetime('now'))",
		followerID, followeeID,
	)
	if err != nil {
		return err
	}

	// Send real-time notification via WebSocket
	s.sendFollowRequestNotification(followerID, followeeID)

	log.Printf("Follow request sent from %s to %s", followerID, followeeID)
	return nil
}

func (s *FollowService) followImmediately(followerID, followeeID string) error {
	_, err := s.DB.Exec(
		"INSERT INTO followers (follower_id, followee_id, created_at) VALUES (?, ?, datetime('now'))",
		followerID, followeeID,
	)
	if err != nil {
		return err
	}

	// Send notifications via WebSocket
	s.sendFollowNotification(followerID, followeeID)

	log.Printf("Followed immediately from %s to %s", followerID, followeeID)
	return nil
}

func (s *FollowService) AcceptFollowRequest(followerID, followeeID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Update request status
	_, err = tx.Exec(
		"UPDATE follow_requests SET status = 'accepted', responded_at = datetime('now') WHERE requester_id = ? AND recipient_id = ?",
		followerID, followeeID,
	)
	if err != nil {
		return err
	}

	// Add to followers table
	_, err = tx.Exec(
		"INSERT INTO followers (follower_id, followee_id, created_at) VALUES (?, ?, datetime('now'))",
		followerID, followeeID,
	)
	if err != nil {
		return err
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return err
	}

	// Send real-time notification via WebSocket
	s.sendAcceptNotification(followerID, followeeID)

	log.Printf("Follow request accepted from %s to %s", followerID, followeeID)
	return nil
}

func (s *FollowService) RejectFollowRequest(followerID, followeeID string) error {
	_, err := s.DB.Exec(
		"UPDATE follow_requests SET status = 'declined', responded_at = datetime('now') WHERE requester_id = ? AND recipient_id = ?",
		followerID, followeeID,
	)
	if err != nil {
		return err
	}

	// Send real-time notification via WebSocket
	s.sendRejectNotification(followerID, followeeID)

	return nil
}

func (s *FollowService) GetPendingRequests(userID string) ([]FollowRequest, error) {

	query := `
		SELECT requester_id, recipient_id, status, created_at
		FROM follow_requests
		WHERE requester_id = ? AND status = 'pending'
		ORDER BY created_at DESC
	`

	rows, err := s.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []FollowRequest
	for rows.Next() {
		var req FollowRequest
		err := rows.Scan(&req.FollowerID, &req.FolloweeID, &req.Status, &req.CreatedAt)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}

	return requests, nil
}

func (s *FollowService) CanViewUserData(requestinguserID, targetUserID string) (bool, error) {
	// user can view their own data
	if requestinguserID == targetUserID {
		return true, nil
	}

	// Check if the target user is private
	var isPublic bool
	err := s.DB.QueryRow("SELECT is_public FROM users WHERE id = ?", targetUserID).Scan(&isPublic)
	if err != nil {
		return false, err
	}

	// If the target user is public, allow access
	if isPublic {
		return true, nil
	}

	// if targte is private
	var count int
	err = s.DB.QueryRow(
		"SELECT COUNT(*) FROM followers WHERE followee_id = ? AND follower_id = ?",
		targetUserID, requestinguserID,
	).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *FollowService) GetUserFollowers(requestingUserID, userID string, offset, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT u.id, u.nickname, u.first_name, u.last_name, u.avatar_path, f.created_at
		FROM followers f
		JOIN users u ON f.follower_id = u.id
		WHERE f.followee_id = ?
		ORDER BY f.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.DB.Query(query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var followers []map[string]interface{}
	for rows.Next() {
		var follower struct {
			ID         string
			Nickname   string
			FirstName  string
			LastName   string
			AvatarPath string
			CreatedAt  string
		}

		err := rows.Scan(
			&follower.ID,
			&follower.Nickname,
			&follower.FirstName,
			&follower.LastName,
			&follower.AvatarPath,
			&follower.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Check if requestingUserID follows this follower
		var isFollowed bool
		checkErr := s.DB.QueryRow(
			"SELECT COUNT(*) > 0 FROM followers WHERE follower_id = ? AND followee_id = ?",
			requestingUserID, follower.ID,
		).Scan(&isFollowed)
		if checkErr != nil {
			isFollowed = false
		}

		followerData := map[string]interface{}{
			"id":          follower.ID,
			"nickname":    follower.Nickname,
			"first_name":  follower.FirstName,
			"last_name":   follower.LastName,
			"avatar_path": follower.AvatarPath,
			"created_at":  follower.CreatedAt,
			"isFollowed":  isFollowed,
		}

		followers = append(followers, followerData)
	}

	return followers, nil
}

func (s *FollowService) GetUserFollowing(requestingUserID, userID string, offset, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT u.id, u.nickname, u.first_name, u.last_name, u.avatar_path, f.created_at
		FROM followers f
		JOIN users u ON f.followee_id = u.id
		WHERE f.follower_id = ?
		ORDER BY f.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.DB.Query(query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var following []map[string]interface{}
	for rows.Next() {
		var followee struct {
			ID         string
			Nickname   string
			FirstName  string
			LastName   string
			AvatarPath string
			CreatedAt  string
		}

		err := rows.Scan(
			&followee.ID,
			&followee.Nickname,
			&followee.FirstName,
			&followee.LastName,
			&followee.AvatarPath,
			&followee.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Check if requestingUserID follows this followee
		var isFollowed bool
		checkErr := s.DB.QueryRow(
			"SELECT COUNT(*) > 0 FROM followers WHERE follower_id = ? AND followee_id = ?",
			requestingUserID, followee.ID,
		).Scan(&isFollowed)
		if checkErr != nil {
			isFollowed = false
		}

		followeeData := map[string]interface{}{
			"id":          followee.ID,
			"nickname":    followee.Nickname,
			"first_name":  followee.FirstName,
			"last_name":   followee.LastName,
			"avatar_path": followee.AvatarPath,
			"created_at":  followee.CreatedAt,
			"isFollowed":  isFollowed,
		}

		following = append(following, followeeData)
	}
	return following, nil
}

func (s *FollowService) Unfollow(followerID, followeeID string) error {
	// check the follow relationship exists
	var count int
	err := s.DB.QueryRow(
		"SELECT COUNT(*) FROM followers WHERE follower_id = ? AND followee_id = ?",
		followerID, followeeID,
	).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		return errors.New("you are not following this user")
	}

	// unfollow on the database
	_, err = s.DB.Exec(
		"DELETE FROM followers WHERE follower_id = ? AND followee_id = ?",
		followerID, followeeID,
	)
	if err != nil {
		return err
	}

	// notify the user via WebSocket
	s.sendUnfollowNotification(followerID, followeeID)

	log.Printf("Unfollowed %s from %s", followeeID, followerID)

	// Also remove any existing follow request records to prevent conflicts
	// when the user wants to send a new follow request later
	err = s.removeFollowRequest(followerID, followeeID)
	if err != nil {
		// Log the error but don't fail the unfollow operation
		// as the main relationship has already been removed
		log.Printf("Warning: Failed to clean up follow request records: %v", err)
	}

	return nil
}

// Helper method to remove follow request records
func (s *FollowService) removeFollowRequest(followerID, followeeID string) error {
	query := `DELETE FROM follow_requests WHERE requester_id = ? AND recipient_id = ?`
	_, err := s.DB.Exec(query, followerID, followeeID)
	return err
}

// Add this public method to expose the notification functionality
func (s *FollowService) SendAcceptNotification(followerID, followeeID string) {
	s.sendAcceptNotification(followerID, followeeID)
}
