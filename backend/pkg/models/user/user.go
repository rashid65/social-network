package user

import (
	"database/sql"
	"errors"
	"log"
	"social-network/pkg/db"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

// use represents a user in the system
type User struct {
	ID             string `json:"id"` // uuid
	Nickname       string `json:"nickname"`
	Email          string `json:"email"`
	PasswordHash   string
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	DOB            string `json:"dob"`
	AboutMe        string `json:"about_me"`
	Avatar         string `json:"avatar_path"`
	CreatedAt      string `json:"created_at"`
	IsPublic       bool   `json:"is_public"` // true if public profile, false if private
	FollowersCount int    `json:"followers_count"`
	FollowingCount int    `json:"following_count"`
	PostsCount     int    `json:"posts_count"`  // <-- Add this line
	IsFollowed     bool   `json:"is_followed"`  // <--- Add this
	IsFollowing    bool   `json:"is_following"` // <--- Add this
}

// CreateUser adds a new user to the database
func CreateUser(user User) (string, error) {
	query := `
	    INSERT INTO users (id, email, password_hash, first_name, last_name, date_of_birth, nickname, about_me, avatar_path, is_public)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Generate a UUID for id
	id := uuid.New().String()

	_, err := db.DB.Exec(
		query,
		id,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.DOB,
		user.Nickname,
		user.AboutMe,
		user.Avatar,
		user.IsPublic,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetUserByEmail retrieves a user by their Email with follower counts
func GetUserByEmail(email string) (User, error) {
	// First get the basic user data
	query := `
        SELECT id, nickname, email, password_hash, first_name, last_name, 
                about_me, avatar_path, is_public, created_at
        FROM users 
        WHERE email = ?
    `

	var user User
	var isPublicInt int
	err := db.DB.QueryRow(query, email).Scan(
		&user.ID,
		&user.Nickname,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.AboutMe,
		&user.Avatar,
		&isPublicInt, // <-- scan as int
		&user.CreatedAt,
	)

	if err != nil {
		return User{}, ErrUserNotFound
	}

	user.IsPublic = isPublicInt == 1

	// Initialize counts to 0 explicitly
	user.FollowersCount = 0
	user.FollowingCount = 0

	// Get follower count
	err = db.DB.QueryRow("SELECT COUNT(*) FROM followers WHERE followee_id = ?", user.ID).Scan(&user.FollowersCount)
	if err != nil {
		log.Printf("Error getting follower count: %v", err)
		user.FollowersCount = 0
	}

	// Get following count
	err = db.DB.QueryRow("SELECT COUNT(*) FROM followers WHERE follower_id = ?", user.ID).Scan(&user.FollowingCount)
	if err != nil {
		log.Printf("Error getting following count: %v", err)
		user.FollowingCount = 0
	}

	return user, nil
}

// GetUserByNickname retrieves a user by their Nickname with follower counts
func GetUserByNickname(nickname string) (User, error) {
	// First get the basic user data
	query := `
        SELECT id, email, password_hash, first_name, last_name, date_of_birth,
                nickname, about_me, avatar_path, is_public, created_at
        FROM users 
        WHERE nickname = ?
    `

	var user User
	var isPublicInt int
	err := db.DB.QueryRow(query, nickname).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.DOB,
		&user.Nickname,
		&user.AboutMe,
		&user.Avatar,
		&isPublicInt, // <-- scan as int
		&user.CreatedAt,
	)

	if err != nil {
		log.Printf("Error retrieving user by nickname inside function: %v", err)
		return User{}, ErrUserNotFound
	}

	user.IsPublic = isPublicInt == 1

	// Initialize counts to 0 explicitly
	user.FollowersCount = 0
	user.FollowingCount = 0

	// Get follower count
	err = db.DB.QueryRow("SELECT COUNT(*) FROM followers WHERE followee_id = ?", user.ID).Scan(&user.FollowersCount)
	if err != nil {
		log.Printf("Error getting follower count: %v", err)
		user.FollowersCount = 0
	}

	// Get following count
	err = db.DB.QueryRow("SELECT COUNT(*) FROM followers WHERE follower_id = ?", user.ID).Scan(&user.FollowingCount)
	if err != nil {
		log.Printf("Error getting following count: %v", err)
		user.FollowingCount = 0
	}

	return user, nil
}

// GetUserByID retrieves a user by their ID with follower counts
func GetUserByID(id string, currentUserID string) (User, error) {
	query := `
        SELECT id, email, first_name, last_name, date_of_birth,
                nickname, about_me, avatar_path, is_public, created_at
        FROM users 
        WHERE id = ?
    `

	var user User
	var isPublicInt int
	err := db.DB.QueryRow(query, id).Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.DOB,
		&user.Nickname,
		&user.AboutMe,
		&user.Avatar,
		&isPublicInt,
		&user.CreatedAt,
	)
	if err != nil {
		return User{}, ErrUserNotFound
	}
	user.IsPublic = isPublicInt == 1

	// Initialize counts to 0 explicitly
	user.FollowersCount = 0
	user.FollowingCount = 0
	user.PostsCount = 0

	// Get follower count
	err = db.DB.QueryRow("SELECT COUNT(*) FROM followers WHERE followee_id = ?", user.ID).Scan(&user.FollowersCount)
	if err != nil {
		log.Printf("Error getting follower count: %v", err)
		user.FollowersCount = 0
	}

	// Get following count
	err = db.DB.QueryRow("SELECT COUNT(*) FROM followers WHERE follower_id = ?", user.ID).Scan(&user.FollowingCount)
	if err != nil {
		log.Printf("Error getting following count: %v", err)
		user.FollowingCount = 0
	}

	// Get posts count
	err = db.DB.QueryRow("SELECT COUNT(*) FROM posts WHERE author_id = ?", user.ID).Scan(&user.PostsCount)
	if err != nil {
		log.Printf("Error getting posts count: %v", err)
		user.PostsCount = 0
	}

	// Check if current user follows this user
	if currentUserID != "" && currentUserID != user.ID {
		var count int
		// IsFollowed: does currentUser follow this user?
		err = db.DB.QueryRow("SELECT COUNT(*) FROM followers WHERE follower_id = ? AND followee_id = ?", currentUserID, user.ID).Scan(&count)
		user.IsFollowed = count > 0

		// IsFollowing: does this user follow currentUser?
		err = db.DB.QueryRow("SELECT COUNT(*) FROM followers WHERE follower_id = ? AND followee_id = ?", user.ID, currentUserID).Scan(&count)
		user.IsFollowing = count > 0
	} else {
		user.IsFollowed = false
		user.IsFollowing = false
	}

	return user, nil
}

// SearchUsers searches for users by nickname, first name, or last name
func SearchUsers(db *sql.DB, query, currentUserID string, limit, offset int) ([]map[string]interface{}, error) {
	searchPattern := "%" + query + "%"
	rows, err := db.Query(`
        SELECT id, nickname, first_name, last_name, avatar_path
        FROM users 
        WHERE (nickname LIKE ? OR first_name LIKE ? OR last_name LIKE ?)
        AND id != ?
        ORDER BY 
            CASE 
                WHEN nickname LIKE ? THEN 1 
                WHEN first_name LIKE ? THEN 2
                WHEN last_name LIKE ? THEN 3
                ELSE 4
            END
        LIMIT ? OFFSET ?
    `, searchPattern, searchPattern, searchPattern, currentUserID, searchPattern, searchPattern, searchPattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, nickname, firstName, lastName string
		var avatarPath sql.NullString
		if err := rows.Scan(&id, &nickname, &firstName, &lastName, &avatarPath); err != nil {
			return nil, err
		}

		avatar := ""
		if avatarPath.Valid {
			avatar = avatarPath.String
		}

		users = append(users, map[string]interface{}{
			"id":         id,
			"nickname":   nickname,
			"first_name": firstName,
			"last_name":  lastName,
			"avatar":     avatar,
		})
	}

	return users, nil
}
