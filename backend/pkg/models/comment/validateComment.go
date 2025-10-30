package comment

import (
	"errors"
	"html"
	"strings"
)

func ValidateComment(c Comment) error {
	const (
		minContentLength = 1
		maxContentLength = 300
		maxMediaCount    = 1
	)

	if c.PostID == "" {
		return errors.New("post ID cannot be empty")
	}

	if c.AuthorID == "" {
		return errors.New("author ID cannot be empty")
	}

	// validate content (allow empty if media is provided)
	if strings.TrimSpace(c.Content) == "" && len(c.Media) == 0 {
		return errors.New("comment must have either content or media")
	}

	if c.Content != "" {
		if len(c.Content) < minContentLength {
			return errors.New("comment content must be at least 1 character long")
		}

		if len(c.Content) > maxContentLength {
			return errors.New("comment content must not exceed 300 characters")
		}
		
		safeContent := html.EscapeString(c.Content)
		c.Content = safeContent
	}

	// validate media
	if len(c.Media) > maxMediaCount {
		return errors.New("too many media files, maximum allowed is 1")
	}

	for _, media := range c.Media {
		if media.MediaType == "" {
			return errors.New("media type cannot be empty")
		}
		if media.FilePath == "" {
			return errors.New("media file path cannot be empty")
		}

		// validate media type
		allowedTypes := []string{"image/jpeg", "image/jpg", "image/png", "image/gif"}
		isValidType := false
		for _, allowedType := range allowedTypes {
			if media.MediaType == allowedType {
				isValidType = true
				break
			}
		}
		if !isValidType {
			return errors.New("invalid media type, only JPEG, PNG, and GIF are allowed")
		}
	}

	return nil
}

