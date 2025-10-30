package handlers

import (
	"encoding/json"
	"net/http"
	"social-network/pkg/db"
	"social-network/pkg/models/group"
	"social-network/pkg/models/post"
	"social-network/pkg/models/user"
	"strconv"
	"strings"
)

// writeErrorJSON writes an error response in JSON format
func writeErrorJSON(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":  message,
		"status": statusCode,
	})
}

// SearchUsersHandler searches for users by nickname, first name, or last name
func SearchUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user ID from context
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		writeErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get search query
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeErrorJSON(w, "Search query is required", http.StatusBadRequest)
		return
	}

	// Parse limit parameter (default to 20, max 50)
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 20
		}
		if limit > 50 {
			limit = 50
		}
	}

	// Parse offset parameter (default to 0)
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}
	}

	// Use user model to search users
	users, err := user.SearchUsers(db.DB, query, userID, limit, offset)
	if err != nil {
		writeErrorJSON(w, "Failed to search users: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
	})
}

// SearchGroupsHandler searches for public groups by title or description
func SearchGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user ID from context
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		writeErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get search query
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeErrorJSON(w, "Search query is required", http.StatusBadRequest)
		return
	}

	// Parse limit parameter (default to 20, max 50)
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 20
		}
		if limit > 50 {
			limit = 50
		}
	}

	// Parse offset parameter (default to 0)
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}
	}

	// Use group model to search groups
	groups, err := group.SearchGroups(db.DB, query, userID, limit, offset)
	if err != nil {
		writeErrorJSON(w, "Failed to search groups: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"groups": groups,
	})
}

// SearchPostsHandler searches for posts by content (only posts user can see)
func SearchPostsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user ID from context
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		writeErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get search query
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeErrorJSON(w, "Search query is required", http.StatusBadRequest)
		return
	}

	// Parse limit parameter (default to 20, max 50)
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 20
		}
		if limit > 50 {
			limit = 50
		}
	}

	// Parse offset parameter (default to 0)
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}
	}

	// Create post service and search posts
	postService := post.NewPostService(db.DB)
	posts, err := postService.SearchPosts(query, userID, limit, offset)
	if err != nil {
		writeErrorJSON(w, "Failed to search posts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"posts":   posts,
		"hasMore": len(posts) >= limit,
	})
}

// GlobalSearchHandler performs a combined search across users, groups, and posts
func GlobalSearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user ID from context
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		writeErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get search query
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeErrorJSON(w, "Search query is required", http.StatusBadRequest)
		return
	}

	// Parse type parameter (users, groups, posts, or all)
	searchType := r.URL.Query().Get("type")
	if searchType == "" {
		searchType = "all"
	}

	// Parse limit parameter (default to 10 for each type when all, 20 for specific type)
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if searchType != "all" {
		limit = 20
	}
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 10
		}
		if limit > 50 {
			limit = 50
		}
	}

	result := make(map[string]interface{})

	// Search users
	if searchType == "all" || searchType == "users" {
		users, err := user.SearchUsers(db.DB, query, userID, limit, 0)
		if err == nil {
			result["users"] = users
		} else {
			result["users"] = []interface{}{}
		}
	}

	// Search groups
	if searchType == "all" || searchType == "groups" {
		groups, err := group.SearchGroups(db.DB, query, userID, limit, 0)
		if err == nil {
			result["groups"] = groups
		} else {
			result["groups"] = []interface{}{}
		}
	}

	// Search posts
	if searchType == "all" || searchType == "posts" {
		postService := post.NewPostService(db.DB)
		posts, err := postService.SearchPosts(query, userID, limit, 0)
		if err == nil {
			result["posts"] = posts
		} else {
			result["posts"] = []interface{}{}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
