package post


type CreatePostRequest struct {
	Content   string          `json:"content"` 
	Privacy   PrivacyType     `json:"privacy" oneof:"public followers custom group"` 
	GroupID   *int64          `json:"group_id,omitempty"` // Add group ID for group posts
	Media     []MediaItem     `json:"media"` 
	// for custom privacy, this will be a list of user IDs
	AllowedFollowers []string `json:"allowed_followers,omitempty"`
}
// CreatePostResponse represents the response after creating a post.
type CreatePostResponse struct {
	Success   bool        `json:"success"`
	PostID    int64       `json:"post_id,omitempty"` // ID of the created post, if successful
	Error     string      `json:"error,omitempty"`   // Error message, if any
	AuthorID  string      `json:"author_id,omitempty"` // ID of the author, if successful
	Author    AuthorData  `json:"author,omitempty"` // Author of the post, if successful
	CreatedAt string      `json:"created_at,omitempty"` // Timestamp of post creation
}

type GetPostsResponse struct {
	Success bool   `json:"success"`
	Posts   []Post `json:"posts,omitempty"` // List of posts if successful
	Error   string `json:"error,omitempty"` // Error message, if any
}

// MediaItem represents a media file associated with a post.
type MediaItem struct {
	MediaType string `json:"media_type"`
	FilePath  string `json:"file_path"`
}


// Edit ===============================================
type EditPostRequest struct {
	Content           string           `json:"content"`
	Privacy           PrivacyType      `json:"privacy" oneof:"public followers custom group"`
	GroupID           *int64           `json:"group_id,omitempty"` // Add group ID support
	Media             []MediaItem      `json:"media"`
	// For custome privacy
	AllowedFollowers  []string         `json:"allowed_followers,omitempty"` 	
}

type EditPostResponse struct {
	Success    bool    `json:"success"`
	Error      string   `json:"error,omitempty"` 
}

// Delete ===============================================
type DeletePostResponse struct {
	Success   bool     `json:"success"`
	Error     string   `json:"error,omitempty"` // Error message, if any
}