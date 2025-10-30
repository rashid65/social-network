package post

import (
	"errors"
	"strings"
)

func ValidateCreatePostRequest(req *CreatePostRequest) (bool, error) {
	if req == nil {
		return false, errors.New("request cannot be null")
	}

	if strings.TrimSpace(req.Content) == "" {
		return false, errors.New("content cannot be empty")
	}

	if len(req.Content) > 500 {
		return false, errors.New("content cannot exceed 500 characters")
	}

	// Validate privacy setting
	validPrivacyTypes := []PrivacyType{PrivacyPublic, PrivacyFollowers, PrivacyCustom, PrivacyGroup}
	isValidPrivacy := false
	for _, validType := range validPrivacyTypes {
		if req.Privacy == validType {
			isValidPrivacy = true
			break
		}
	}
	if !isValidPrivacy {
		return false, errors.New("invalid privacy setting")
	}

	// Validate group post requirements
	if req.Privacy == PrivacyGroup {
		if req.GroupID == nil || *req.GroupID <= 0 {
			return false, errors.New("group_id is required for group posts")
		}
	} else {
		if req.GroupID != nil {
			return false, errors.New("group_id should only be provided for group posts")
		}
	}

	// req custom privacy, ensure allowed followers are provided
	if req.Privacy == PrivacyCustom && len(req.AllowedFollowers) == 0 {
		return false, errors.New("allowed followers cannot be empty for custom privacy")
	}

	// Validate each media item
	for i, media := range req.Media {
		if err := validateMediaItem(media, i); err != nil {
			return false, err
		}
	}

	return true, nil
}

func validateMediaItem(media MediaItem, index int) error {
	// Validate media type
	validMediaTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
	}

	if !validMediaTypes[media.MediaType] {
		return errors.New("invalid media type at index " + string(index) + "it must be either of these options (image/jpeg, image/png, image/gif)")
	}

	if media.FilePath == "" {
		return errors.New("file path cannot be empty at index " + string(index))
	}

	return nil
}

func ValidateEditPostRequest(req *EditPostRequest) (bool, error) {
	if req == nil {
		return false, errors.New("request cannot be null")
	}

	if strings.TrimSpace(req.Content) == "" {
		return false, errors.New("content cannot be empty")
	}

	if len(req.Content) > 500 {
		return false, errors.New("content cannot exceed 500 characters")
	}

	// Validate privacy setting
	validPrivacyTypes := []PrivacyType{PrivacyPublic, PrivacyFollowers, PrivacyCustom, PrivacyGroup}
	isValidPrivacy := false
	for _, validType := range validPrivacyTypes {
		if req.Privacy == validType {
			isValidPrivacy = true
			break
		}
	}
	if !isValidPrivacy {
		return false, errors.New("invalid privacy setting")
	}

	// Validate group post requirements
	if req.Privacy == PrivacyGroup {
		if req.GroupID == nil || *req.GroupID <= 0 {
			return false, errors.New("group_id is required for group posts")
		}
	}

	// for custome ensure allowed followers are provided
	if req.Privacy == PrivacyCustom && len(req.AllowedFollowers) == 0 {
		return false, errors.New("allowed followers cannot be empty for custom privacy")
	}

	// Validate each media item
	for i, media := range req.Media {
		if err := validateMediaItem(media, i); err != nil {
			return false, err
		}
	}

	return true, nil
}