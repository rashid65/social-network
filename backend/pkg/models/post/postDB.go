package post

import (
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// handles database operations related to posts
type PostService struct {
	DB *sql.DB
}

// Create a new post in the database
func NewPostService(db *sql.DB) *PostService {
	return &PostService{DB: db}
}

func (s *PostService) CreatePost(req *CreatePostRequest, authorID string) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// For group posts, validate group membership
	if req.Privacy == PrivacyGroup && req.GroupID != nil {
		if err := s.validateGroupMembership(authorID, *req.GroupID); err != nil {
			return 0, err
		}
	}

	// Insert the post
	result, err := tx.Exec(
		"INSERT INTO posts (author_id, content, privacy, group_id) VALUES (?, ?, ?, ?)",
		authorID,
		req.Content,
		req.Privacy,
		req.GroupID,
	)
	if err != nil {
		return 0, err
	}

	postID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Insert custom privacy followers if applicable
	if req.Privacy == PrivacyCustom && len(req.AllowedFollowers) > 0 {
		for _, followerID := range req.AllowedFollowers {
			_, err = tx.Exec(
				"INSERT INTO post_allowed_followers (post_id, follower_id) VALUES (?, ?)",
				postID, followerID,
			)
			if err != nil {
				return 0, err
			}
		}
	}

	// Insert media if provided
	for _, media := range req.Media {
		_, err := tx.Exec(
			"INSERT INTO post_media (post_id, media_type, file_path) VALUES (?, ?, ?)",
			postID,
			media.MediaType,
			media.FilePath,
		)
		if err != nil {
			return 0, err
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}

	return postID, nil
}

// Add helper method to validate group membership
func (s *PostService) validateGroupMembership(userID string, groupID int64) error {
	var count int
	err := s.DB.QueryRow(
		"SELECT COUNT(*) FROM group_memberships WHERE user_id = ? AND group_id = ?",
		userID, groupID,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("user is not a member of the specified group")
	}
	return nil
}

// GetPosts retrieves posts from the database (including group posts for members)
func (s *PostService) GetPosts(userID string, offset, limit int) ([]Post, error) {
	query := `
		SELECT DISTINCT p.id, p.author_id, p.content, p.privacy, p.group_id, p.created_at, p.updated_at, p.liked,
			u.nickname, u.first_name, u.last_name, u.avatar_path,
			EXISTS(SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = ?) AS liked_by_current_user,
			(SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id) AS comment_count
		FROM posts p
		LEFT JOIN followers f ON p.author_id = f.followee_id AND f.follower_id = ?
		LEFT JOIN post_allowed_followers paf ON p.id = paf.post_id AND paf.follower_id = ?
		LEFT JOIN group_memberships gm ON p.group_id = gm.group_id AND gm.user_id = ?
		JOIN users u ON p.author_id = u.id
		WHERE
			p.privacy = 'public' OR
			(p.privacy = 'followers' AND (p.author_id = ? OR f.follower_id IS NOT NULL)) OR
			(p.privacy = 'custom' AND (p.author_id = ? OR paf.follower_id IS NOT NULL)) OR
			(p.privacy = 'group' AND (p.author_id = ? OR gm.user_id IS NOT NULL))
		ORDER BY p.created_at DESC
		LIMIT ? OFFSET ?
		`

	rows, err := s.DB.Query(query, userID, userID, userID, userID, userID, userID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var createdAtstr, updatedAtstr string

		err := rows.Scan(
			&post.ID,
			&post.AuthorID,
			&post.Content,
			&post.Privacy,
			&post.GroupID,
			&createdAtstr,
			&updatedAtstr,
			&post.Liked,
			&post.Author.Nickname,
			&post.Author.FirstName,
			&post.Author.LastName,
			&post.Author.Avatar,
			&post.LikedByCurrentUser,
			&post.CommentCount,
		)
		if err != nil {
			return nil, err
		}

		// parse the datetime strings
		post.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtstr)
		if err != nil {
			return nil, err
		}
		post.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAtstr)
		if err != nil {
			return nil, err
		}

		// Get media for each post
		mediaRows, err := s.DB.Query(
			"SELECT id, media_type, file_path, created_at FROM post_media WHERE post_id = ?",
			post.ID,
		)
		if err != nil {
			return nil, err
		}

		for mediaRows.Next() {
			var media PostMedia
			var mediaCreatedAtStr string
			media.PostID = strconv.FormatInt(post.ID, 10)
			err := mediaRows.Scan(
				&media.ID,
				&media.MediaType,
				&media.FilePath,
				&mediaCreatedAtStr,
			)
			if err != nil {
				mediaRows.Close()
				return nil, err
			}

			// parse the media created_at string
			media.CreatedAt, err = time.Parse("2006-01-02 15:04:05", mediaCreatedAtStr)
			if err != nil {
				mediaRows.Close()
				return nil, err
			}

			post.Media = append(post.Media, media)
		}
		mediaRows.Close()

		posts = append(posts, post)
	}

	return posts, nil
}

// Add method to get posts for a specific group
func (s *PostService) GetGroupPosts(userID string, groupID int64, offset, limit int) ([]Post, error) {
	// Check if group is public
	var isPublic bool
	err := s.DB.QueryRow("SELECT is_public FROM groups WHERE id = ?", groupID).Scan(&isPublic)
	if err != nil {
		return nil, err
	}

	// If not public, check membership
	if !isPublic {
		if err := s.validateGroupMembership(userID, groupID); err != nil {
			return nil, errors.New("unauthorized: user is not a member of this group")
		}
	}

	query := `
        SELECT p.id, p.author_id, p.content, p.privacy, p.group_id, p.created_at, p.updated_at, p.liked,
               u.nickname, u.first_name, u.last_name, u.avatar_path
        FROM posts p
        JOIN users u ON p.author_id = u.id
        WHERE p.group_id = ? AND p.privacy = 'group'
        ORDER BY p.created_at DESC
        LIMIT ? OFFSET ?
    `

	rows, err := s.DB.Query(query, groupID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var createdAtStr, updatedAtStr string

		err := rows.Scan(
			&post.ID,
			&post.AuthorID,
			&post.Content,
			&post.Privacy,
			&post.GroupID,
			&createdAtStr,
			&updatedAtStr,
			&post.Liked,
			&post.Author.Nickname,
			&post.Author.FirstName,
			&post.Author.LastName,
			&post.Author.Avatar,
		)
		if err != nil {
			return nil, err
		}

		// Parse datetime strings
		post.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			return nil, err
		}
		post.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAtStr)
		if err != nil {
			return nil, err
		}

		// Get media for each post
		mediaRows, err := s.DB.Query(
			"SELECT id, media_type, file_path, created_at FROM post_media WHERE post_id = ?",
			post.ID,
		)
		if err != nil {
			return nil, err
		}

		for mediaRows.Next() {
			var media PostMedia
			var mediaCreatedAtStr string
			media.PostID = strconv.FormatInt(post.ID, 10)
			err := mediaRows.Scan(
				&media.ID,
				&media.MediaType,
				&media.FilePath,
				&mediaCreatedAtStr,
			)
			if err != nil {
				mediaRows.Close()
				return nil, err
			}

			media.CreatedAt, err = time.Parse("2006-01-02 15:04:05", mediaCreatedAtStr)
			if err != nil {
				mediaRows.Close()
				return nil, err
			}

			post.Media = append(post.Media, media)
		}
		mediaRows.Close()

		posts = append(posts, post)
	}

	return posts, nil
}

// GetPostByID retrieves a post by its ID
func (s *PostService) GetPostByID(postID string, userID string) (*Post, error) {
	post := &Post{}
	var createdAtStr, updatedAtStr string

	err := s.DB.QueryRow(`
        SELECT p.id, p.author_id, p.content, p.privacy, p.created_at, p.updated_at,
               u.nickname, u.first_name, u.last_name, u.avatar_path,
               EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = ?) AS liked_by_current_user,
               (SELECT COUNT(*) FROM comments WHERE post_id = p.id) AS comment_count
        FROM posts p
        JOIN users u ON p.author_id = u.id
        WHERE p.id = ?`,
		userID, postID,
	).Scan(
		&post.ID,
		&post.AuthorID,
		&post.Content,
		&post.Privacy,
		&createdAtStr,
		&updatedAtStr,
		&post.Author.Nickname,
		&post.Author.FirstName,
		&post.Author.LastName,
		&post.Author.Avatar,
		&post.LikedByCurrentUser,
		&post.CommentCount,
	)

	if err != nil {
		return nil, err
	}

	// Parse the datetime strings
	post.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		return nil, err
	}

	post.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAtStr)
	if err != nil {
		return nil, err
	}

	// Get media for the post
	mediaRows, err := s.DB.Query(
		"SELECT id, media_type, file_path, created_at FROM post_media WHERE post_id = ?",
		post.ID,
	)
	if err != nil {
		return nil, err
	}
	defer mediaRows.Close()

	for mediaRows.Next() {
		var media PostMedia
		var mediaCreatedAtStr string
		media.PostID = strconv.FormatInt(post.ID, 10)
		err := mediaRows.Scan(
			&media.ID,
			&media.MediaType,
			&media.FilePath,
			&mediaCreatedAtStr,
		)
		if err != nil {
			return nil, err
		}

		// parse the media created_at string
		media.CreatedAt, err = time.Parse("2006-01-02 15:04:05", mediaCreatedAtStr)
		if err != nil {
			return nil, err
		}

		post.Media = append(post.Media, media)
	}

	return post, nil
}

func (s *PostService) GetUserPosts(userID, targetUserID string, offset, limit int) ([]Post, error) {
	query := `
        SELECT DISTINCT p.id, p.author_id, p.content, p.privacy, p.created_at, p.updated_at,
            u.nickname, u.first_name, u.last_name, u.avatar_path,
            EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = ?) AS liked_by_current_user,
            (SELECT COUNT(*) FROM comments WHERE post_id = p.id) AS comment_count
        FROM posts p
        LEFT JOIN followers f ON p.author_id = f.followee_id AND f.follower_id = ?
        LEFT JOIN post_allowed_followers paf ON p.id = paf.post_id AND paf.follower_id = ?
        JOIN users u ON p.author_id = u.id
        WHERE p.author_id = ? AND (
            p.privacy = 'public' OR
            p.author_id = ? OR
            (p.privacy = 'followers' AND f.follower_id IS NOT NULL) OR
            (p.privacy = 'custom' AND paf.follower_id IS NOT NULL)
        )
        ORDER BY p.created_at DESC
        LIMIT ? OFFSET ?
    `

	rows, err := s.DB.Query(query, userID, userID, userID, targetUserID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var createdAtStr, updatedAtStr string

		err := rows.Scan(
			&post.ID,
			&post.AuthorID,
			&post.Content,
			&post.Privacy,
			&createdAtStr,
			&updatedAtStr,
			&post.Author.Nickname,
			&post.Author.FirstName,
			&post.Author.LastName,
			&post.Author.Avatar,
			&post.LikedByCurrentUser,
			&post.CommentCount,
		)
		if err != nil {
			return nil, err
		}

		// parse the datetime strings
		post.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			return nil, err
		}
		post.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAtStr)
		if err != nil {
			return nil, err
		}

		// Get media for each post
		mediaRows, err := s.DB.Query(
			"SELECT id, media_type, file_path, created_at FROM post_media WHERE post_id = ?",
			post.ID,
		)
		if err != nil {
			return nil, err
		}

		for mediaRows.Next() {
			var media PostMedia
			var mediaCreatedAtStr string
			media.PostID = strconv.FormatInt(post.ID, 10)
			err := mediaRows.Scan(
				&media.ID,
				&media.MediaType,
				&media.FilePath,
				&mediaCreatedAtStr,
			)
			if err != nil {
				mediaRows.Close()
				return nil, err
			}

			// Parse the media created_at string
			media.CreatedAt, err = time.Parse("2006-01-02 15:04:05", mediaCreatedAtStr)
			if err != nil {
				mediaRows.Close()
				return nil, err
			}

			post.Media = append(post.Media, media)
		}
		mediaRows.Close()

		posts = append(posts, post)
	}

	return posts, nil
}

func (s *PostService) GetPostAuthor(postID int64) (string, error) {
	var authorID string
	err := s.DB.QueryRow(
		"SELECT author_id FROM posts WHERE id = ?",
		postID).Scan(&authorID)
	if err != nil {
		return "", err
	}
	return authorID, nil
}

func (s *PostService) GetAuthorData(authorID string) (AuthorData, error) {
	var author AuthorData
	err := s.DB.QueryRow(
		"SELECT nickname, first_name, last_name, avatar_path FROM users WHERE id = ?",
		authorID,
	).Scan(
		&author.Nickname,
		&author.FirstName,
		&author.LastName,
		&author.Avatar,
	)
	if err != nil {
		return author, err
	}
	return author, nil
}

// Edit post functions ================================================
func (s *PostService) EditPost(postID int64, req *EditPostRequest, authorID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Verify if the post author
	var currentAuthorID string
	var currentGroupID *int64
	err = tx.QueryRow(
		"SELECT author_id, group_id FROM posts WHERE id = ?", postID).Scan(&currentAuthorID, &currentGroupID)
	if err != nil {
		return err
	}
	if currentAuthorID != authorID {
		return errors.New("unauthorized: you are not the author of this post")
	}

	// For group posts, validate group membership
	if req.Privacy == PrivacyGroup && req.GroupID != nil {
		if err := s.validateGroupMembership(authorID, *req.GroupID); err != nil {
			return err
		}
	}

	// Update the post
	_, err = tx.Exec(
		"UPDATE posts SET content = ?, privacy = ?, group_id = ? WHERE id = ?",
		req.Content,
		req.Privacy,
		req.GroupID,
		postID,
	)
	if err != nil {
		return err
	}

	// check if the edit request has media
	if len(req.Media) > 0 {
		// Delete media
		_, err = tx.Exec("DELETE FROM post_media WHERE post_id = ?", postID)
		if err != nil {
			return err
		}

		for _, media := range req.Media {
			_, err = tx.Exec(
				"INSERT INTO post_media (post_id, media_type, file_path) VALUES (?, ?, ?)",
				postID,
				media.MediaType,
				media.FilePath,
			)
			if err != nil {
				return err
			}
		}
	}

	// check if the privacy is custom
	if req.Privacy == PrivacyCustom {
		_, err = tx.Exec("DELETE FROM post_allowed_followers WHERE post_id = ?", postID)
		if err != nil {
			return err
		}

		if len(req.AllowedFollowers) > 0 {
			for _, followerID := range req.AllowedFollowers {
				_, err = tx.Exec(
					"INSERT INTO post_allowed_followers (post_id, follower_id) VALUES (?, ?)",
					postID, followerID,
				)
				if err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

func (s *PostService) DeletePost(postID int64, authorID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Verify if the post author
	var currentAuthorID string
	err = tx.QueryRow("SELECT author_id FROM posts WHERE id = ?", postID).Scan(&currentAuthorID)
	if err != nil {
		return err
	}
	if currentAuthorID != authorID {
		return errors.New("unauthorized: you are not the author of this post")
	}

	// Delete the post
	_, err = tx.Exec("DELETE FROM posts WHERE id = ?", postID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// LikePost adds a like to a post
func (s *PostService) LikePost(postID int64, userID string) (bool, error, int) {
	// First check if user can access this post
	var privacy string
	var groupID *int64
	err := s.DB.QueryRow("SELECT privacy, group_id FROM posts WHERE id = ?", postID).Scan(&privacy, &groupID)
	if err != nil {
		return false, err, 0
	}

	// If it's a group post, check group membership
	if privacy == "group" && groupID != nil {
		if err := s.validateGroupMembership(userID, *groupID); err != nil {
			return false, errors.New("unauthorized: cannot like group post - not a member"), 0
		}
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return false, err, 0
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Check if user has already liked post
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM post_likes WHERE post_id = ? AND user_id = ?)",
		postID, userID).Scan(&exists)
	if err != nil {
		return false, err, 0
	}

	var newLikeCount int
	var isLiked bool

	if exists {
		//------------------------------------ unlike the post
		_, err = tx.Exec("DELETE FROM post_likes WHERE post_id = ? AND user_id = ?", postID, userID)
		if err != nil {
			return false, err, 0
		}

		// Decrease the like count
		err = tx.QueryRow("UPDATE posts SET liked = liked - 1 WHERE id = ? RETURNING liked", postID).Scan(&newLikeCount)
		if err != nil {
			return false, err, 0
		}
		isLiked = false
	} else {
		//----------------------------------- like the post
		_, err = tx.Exec("INSERT INTO post_likes (post_id, user_id) VALUES (?, ?)", postID, userID)
		if err != nil {
			return false, err, 0
		}

		// Increase the like count
		err = tx.QueryRow("UPDATE posts SET liked = liked + 1 WHERE id = ? RETURNING liked", postID).Scan(&newLikeCount)
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

func (s *PostService) IsUserAllowedToViewPosts(requesterUserID, targetUserID string) (bool, error) {
	// user can view their own data
	if requesterUserID == targetUserID {
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
		targetUserID, requesterUserID,
	).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// SearchPosts searches for posts by content (only posts user can see)
func (s *PostService) SearchPosts(query, userID string, limit, offset int) ([]map[string]interface{}, error) {
	searchPattern := "%" + query + "%"
	rows, err := s.DB.Query(`
        SELECT DISTINCT p.id, p.author_id, p.content, p.privacy, p.group_id, p.created_at, p.updated_at,
            u.nickname, u.first_name, u.last_name, u.avatar_path
        FROM posts p
        JOIN users u ON p.author_id = u.id
        LEFT JOIN group_memberships gm ON p.group_id = gm.group_id AND gm.user_id = ?
        LEFT JOIN groups g ON p.group_id = g.id
        WHERE p.content LIKE ?
        AND (
            -- Public posts
            p.privacy = 'public'
            -- User's own posts
            OR p.author_id = ?
            -- Group posts (user is member or group is public)
            OR (p.privacy = 'group' AND (gm.user_id IS NOT NULL OR g.is_public = 1))
        )
        ORDER BY p.created_at DESC
        LIMIT ? OFFSET ?
    `, userID, searchPattern, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []map[string]interface{}
	for rows.Next() {
		var postID, authorID, content, privacy, createdAt, updatedAt string
		var groupID sql.NullString
		var nickname, firstName, lastName, avatarPath string

		if err := rows.Scan(&postID, &authorID, &content, &privacy, &groupID, &createdAt, &updatedAt,
			&nickname, &firstName, &lastName, &avatarPath); err != nil {
			return nil, err
		}

		post := map[string]interface{}{
			"id":         postID,
			"author_id":  authorID,
			"content":    content,
			"privacy":    privacy,
			"created_at": createdAt,
			"updated_at": updatedAt,
			"author": map[string]interface{}{
				"id":         authorID,
				"nickname":   nickname,
				"first_name": firstName,
				"last_name":  lastName,
				"avatar":     avatarPath,
			},
		}

		if groupID.Valid {
			post["group_id"] = groupID.String
		}

		// Get post media
		mediaRows, err := s.DB.Query(
			"SELECT id, media_type, file_path, created_at FROM post_media WHERE post_id = ?",
			postID,
		)
		if err == nil {
			var media []map[string]interface{}
			for mediaRows.Next() {
				var mediaID, mediaType, filePath, mediaCreatedAt string
				if err := mediaRows.Scan(&mediaID, &mediaType, &filePath, &mediaCreatedAt); err == nil {
					media = append(media, map[string]interface{}{
						"id":         mediaID,
						"media_type": mediaType,
						"file_path":  filePath,
						"created_at": mediaCreatedAt,
					})
				}
			}
			mediaRows.Close()
			post["media"] = media
		}

		posts = append(posts, post)
	}

	return posts, nil
}
