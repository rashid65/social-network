package post

import (
	"time"
)

// PrivacyType represents the privacy settings for a post.
type PrivacyType string

type AuthorData struct {
	Nickname  string `json:"nickname"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Avatar    string `json:"avatar_path"`
}

const (
	PrivacyPublic    PrivacyType = "public"    // visible to all users
	PrivacyFollowers PrivacyType = "followers" // visible to followers only
	PrivacyCustom    PrivacyType = "custom"    // visible to a custom list of users
	PrivacyGroup     PrivacyType = "group"     // Add group privacy type (group posts)
)

type Post struct {
	ID        int64       `json:"id"`                 // Unique identifier for the post
	AuthorID  string      `json:"author_id"`          // ID of the user who created the post
	Content   string      `json:"content"`            // Text content of the post
	Privacy   PrivacyType `json:"privacy"`            // Privacy setting of the post
	GroupID   *int64      `json:"group_id,omitempty"` // ID of the group (for group posts)
	CreatedAt time.Time   `json:"created_at"`         // Timestamp of when the post was created
	UpdatedAt time.Time   `json:"updated_at"`         // Timestamp of when the post was last updated
	Media     []PostMedia `json:"media"`              // List of media URLs associated with the post
	Liked     int         `json:"liked"`
	// Author details (populated when fetching posts)
	Author             AuthorData `json:"author,omitempty"`
	LikedByCurrentUser bool       `json:"liked_by_current_user"`
	CommentCount       int        `json:"comment_count"`
}

type PostMedia struct {
	ID        int64     `json:"id"`
	PostID    string    `json:"post_id"`    // ID of the post this media belongs to
	MediaType string    `json:"media_type"` // Type of media (e.g., image, GIF)
	FilePath  string    `json:"file_path"`  // Path to the media file
	CreatedAt time.Time `json:"created_at"` // Timestamp of when the media was added
}

type PostLike struct {
	UserID    string    `json:"user_id"`
	Nickname  string    `json:"nickname"`
	PostID    int64     `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
}
