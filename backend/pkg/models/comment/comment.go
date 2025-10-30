package comment

import (
	"database/sql"
	"strconv"
	"time"
)

type Comment struct {
	ID        string         `json:"id"`
	PostID    string         `json:"post_id"`
	AuthorID  string         `json:"author_id"`
	Content   string         `json:"content"`
	CreatedAt string         `json:"created_at"`
	Liked     int            `json:"liked"`
	Media     []CommentMedia `json:"media"` // Add media field
	IsLiked   bool           `json:"isLiked"`
}

type CommentRequest struct {
	ID       string         `json:"id"`
	PostID   string         `json:"post_id"`
	AuthorID string         `json:"author_id"`
	Content  string         `json:"content"`
	Media    []CommentMedia `json:"media"` // Add media field
}

type UpdateCommentRequest struct {
	ID       string         `json:"id"`
	PostID   string         `json:"post_id"`
	AuthorID string         `json:"author_id"`
	Content  string         `json:"content"`
	Media    []CommentMedia `json:"media"` // Add media field
}

type DeleteCommentRequest struct {
	ID       string `json:"id"`
	AuthorID string `json:"author_id"`
}

type CommentLike struct {
	UserID    string `json:"user_id"`
	CommentID string `json:"id"`
	CreatedAt string `json:"created_at"`
}

// Add CommentMedia struct
type CommentMedia struct {
	ID        int64     `json:"id"`
	CommentID string    `json:"comment_id"`
	MediaType string    `json:"media_type"`
	FilePath  string    `json:"file_path"`
	CreatedAt time.Time `json:"created_at"`
}

func CreateComment(db *sql.DB, c Comment) (Comment, error) {
	tx, err := db.Begin()
	if err != nil {
		return Comment{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert the comment
	query := `INSERT INTO comments (post_id, author_id, content)
                VALUES (?, ?, ?)`

	result, err := tx.Exec(query, c.PostID, c.AuthorID, c.Content)
	if err != nil {
		return Comment{}, err
	}

	// Get the ID of the newly inserted comment
	commentID, err := result.LastInsertId()
	if err != nil {
		return Comment{}, err
	}

	// Insert media if provided
	for _, media := range c.Media {
		_, err := tx.Exec(
			"INSERT INTO comment_media (comment_id, media_type, file_path) VALUES (?, ?, ?)",
			commentID,
			media.MediaType,
			media.FilePath,
		)
		if err != nil {
			return Comment{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return Comment{}, err
	}

	// Retrieve the newly created comment with media
	var newComment Comment
	selectQuery := `SELECT id, post_id, author_id, content, created_at, COALESCE(liked, 0) as liked
                    FROM comments WHERE id = ?`

	err = db.QueryRow(selectQuery, commentID).Scan(
		&newComment.ID,
		&newComment.PostID,
		&newComment.AuthorID,
		&newComment.Content,
		&newComment.CreatedAt,
		&newComment.Liked,
	)

	if err != nil {
		return Comment{}, err
	}

	// Get media for the comment
	mediaRows, err := db.Query(
		"SELECT id, media_type, file_path, created_at FROM comment_media WHERE comment_id = ?",
		commentID,
	)
	if err != nil {
		return Comment{}, err
	}
	defer mediaRows.Close()

	for mediaRows.Next() {
		var media CommentMedia
		var mediaCreatedAtStr string
		media.CommentID = strconv.FormatInt(commentID, 10)
		err := mediaRows.Scan(
			&media.ID,
			&media.MediaType,
			&media.FilePath,
			&mediaCreatedAtStr,
		)
		if err != nil {
			return Comment{}, err
		}

		media.CreatedAt, err = time.Parse("2006-01-02 15:04:05", mediaCreatedAtStr)
		if err != nil {
			return Comment{}, err
		}

		newComment.Media = append(newComment.Media, media)
	}

	return newComment, nil
}

func DeleteComment(db *sql.DB, C Comment) error {
	query := `DELETE FROM comments WHERE id = ?`

	_, err := db.Exec(query, C.ID)
	if err != nil {
		return err
	}
	return nil
}

func UpdateComment(db *sql.DB, C Comment) (Comment, error) {
	tx, err := db.Begin()
	if err != nil {
		return Comment{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Update the comment
	query := `UPDATE comments 
                SET post_id = ?, author_id = ?, content = ?, created_at = CURRENT_TIMESTAMP
                WHERE id = ?`

	_, err = tx.Exec(query, C.PostID, C.AuthorID, C.Content, C.ID)
	if err != nil {
		return Comment{}, err
	}

	// Update media if provided
	if len(C.Media) > 0 {
		// Delete existing media
		_, err = tx.Exec("DELETE FROM comment_media WHERE comment_id = ?", C.ID)
		if err != nil {
			return Comment{}, err
		}

		// Insert new media
		for _, media := range C.Media {
			_, err = tx.Exec(
				"INSERT INTO comment_media (comment_id, media_type, file_path) VALUES (?, ?, ?)",
				C.ID,
				media.MediaType,
				media.FilePath,
			)
			if err != nil {
				return Comment{}, err
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return Comment{}, err
	}

	// Retrieve the updated comment with media
	var updatedComment Comment
	selectQuery := `SELECT id, post_id, author_id, content, created_at, COALESCE(liked, 0) as liked
                    FROM comments WHERE id = ?`

	err = db.QueryRow(selectQuery, C.ID).Scan(
		&updatedComment.ID,
		&updatedComment.PostID,
		&updatedComment.AuthorID,
		&updatedComment.Content,
		&updatedComment.CreatedAt,
		&updatedComment.Liked,
	)

	if err != nil {
		return Comment{}, err
	}

	// Get media for the comment
	mediaRows, err := db.Query(
		"SELECT id, media_type, file_path, created_at FROM comment_media WHERE comment_id = ?",
		C.ID,
	)
	if err != nil {
		return Comment{}, err
	}
	defer mediaRows.Close()

	for mediaRows.Next() {
		var media CommentMedia
		var mediaCreatedAtStr string
		media.CommentID = C.ID
		err := mediaRows.Scan(
			&media.ID,
			&media.MediaType,
			&media.FilePath,
			&mediaCreatedAtStr,
		)
		if err != nil {
			return Comment{}, err
		}

		media.CreatedAt, err = time.Parse("2006-01-02 15:04:05", mediaCreatedAtStr)
		if err != nil {
			return Comment{}, err
		}

		updatedComment.Media = append(updatedComment.Media, media)
	}

	return updatedComment, nil
}

func GetComment(db *sql.DB, postID string, userID string, offset, limit int) ([]Comment, error) {
	query := `SELECT id, post_id, author_id, content, created_at, liked
                FROM comments
                WHERE post_id = ?
                ORDER BY created_at DESC
                LIMIT ? OFFSET ?`

	rows, err := db.Query(query, postID, limit, offset)
	if err != nil {
		return []Comment{}, err
	}
	defer rows.Close()

	var comments []Comment

	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.CreatedAt, &c.Liked); err != nil {
			return []Comment{}, err
		}

		// Check if liked by current user
		var likedByUser bool
		err = db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM comment_likes WHERE comment_id = ? AND user_id = ?)",
			c.ID, userID,
		).Scan(&likedByUser)
		if err != nil {
			return []Comment{}, err
		}
		c.IsLiked = likedByUser // Add this field to your Comment struct: IsLiked bool `json:"isLiked"`

		// Get media for each comment
		mediaRows, err := db.Query(
			"SELECT id, media_type, file_path, created_at FROM comment_media WHERE comment_id = ?",
			c.ID,
		)
		if err != nil {
			return []Comment{}, err
		}

		for mediaRows.Next() {
			var media CommentMedia
			var mediaCreatedAtStr string
			media.CommentID = c.ID
			err := mediaRows.Scan(
				&media.ID,
				&media.MediaType,
				&media.FilePath,
				&mediaCreatedAtStr,
			)
			if err != nil {
				mediaRows.Close()
				return []Comment{}, err
			}

			media.CreatedAt, err = time.Parse("2006-01-02 15:04:05", mediaCreatedAtStr)
			if err != nil {
				mediaRows.Close()
				return []Comment{}, err
			}

			c.Media = append(c.Media, media)
		}
		mediaRows.Close()

		comments = append(comments, c)
	}

	if len(comments) == 0 {
		return []Comment{}, sql.ErrNoRows
	}

	return comments, nil
}

func LikeComment(db *sql.DB, commentID string, userID string) (bool, error, int) {
	tx, err := db.Begin()
	if err != nil {
		return false, err, 0
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// check if already liked
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM comment_likes WHERE comment_id = ? AND user_id = ?)",
		commentID, userID).Scan(&exists)
	if err != nil {
		return false, err, 0
	}

	var newLikeCount int
	var isLiked bool

	if exists {
		// unlike the comment
		_, err = tx.Exec("DELETE FROM comment_likes WHERE comment_id = ? AND user_id = ?", commentID, userID)
		if err != nil {
			return false, err, 0
		}

		// decrement the like count
		err = tx.QueryRow("UPDATE comments SET liked = liked - 1 WHERE id = ? RETURNING liked", commentID).Scan(&newLikeCount)
		if err != nil {
			return false, err, 0
		}
		isLiked = false
	} else {
		// like the comment
		_, err = tx.Exec("INSERT INTO comment_likes (comment_id, user_id) VALUES (?, ?)", commentID, userID)
		if err != nil {
			return false, err, 0
		}

		// increment the like count
		err = tx.QueryRow("UPDATE comments SET liked = liked + 1 WHERE id = ? RETURNING liked", commentID).Scan(&newLikeCount)
		if err != nil {
			return false, err, 0
		}
		isLiked = true
	}

	if err = tx.Commit(); err != nil {
		return false, err, 0
	}

	return isLiked, nil, newLikeCount
}
