package handlers

import (
	"encoding/json"
	"net/http"
	"social-network/pkg/models/post"
	"social-network/pkg/utils"
	"strconv"
	"time"
)

type PostHandler struct {
	PostService *post.PostService
}

func NewPostHandler(postService *post.PostService) *PostHandler {
	return &PostHandler{PostService: postService}
}

// Handlers creation of a new post
func (h *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req post.CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if _, err := post.ValidateCreatePostRequest(&req); err != nil {
		response := post.CreatePostResponse{
			Success: false,
			Error:   "Validation error: " + err.Error(),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create post in database
	postID, err := h.PostService.CreatePost(&req, userID)
	if err != nil {
		response := post.CreatePostResponse{
			Success: false,
			Error:   "Falied to create post: Database error: " + err.Error(),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	authorData, err := h.PostService.GetAuthorData(userID)
	if err != nil {
		response := post.CreatePostResponse{
			Success: false,
			Error:   "Failed to retrieve author data: " + err.Error(),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Return success response
	response := post.CreatePostResponse{
		Success:   true,
		PostID:    postID,
		AuthorID:  userID,
		Author:    authorData,
		CreatedAt: time.Now().Format("2006-01-02T15:04:05Z07:00"), // Add created_at timestamp
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetPosts retrieves posts
func (h *PostHandler) GetPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Parse offset parameter (default to 0)
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			utils.WriteErrorJSON(w, "Invalid offset parameter: must be a non-negative integer", http.StatusBadRequest)
			return
		}
	}

	// Parse limit parameter (default to 20, max 100)
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			utils.WriteErrorJSON(w, "Invalid limit parameter: must be a positive integer", http.StatusBadRequest)
			return
		}
		if limit > 100 {
			limit = 100 // Cap at 100 to prevent excessive load
		}
	}

	// Get posts from the DB with pagination
	posts, err := h.PostService.GetPosts(userID, offset, limit)
	if err != nil {
		response := post.GetPostsResponse{
			Success: false,
			Error:   "Failed to retrieve posts: " + err.Error(),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Return success response with posts including author details
	response := map[string]interface{}{
		"success": true,
		"posts":   posts,
		"hasMore": len(posts) >= limit,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *PostHandler) GetPostByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user Id from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get Post ID from URL parameters
	postIDstr := r.URL.Query().Get("post_id")
	if postIDstr == "" {
		utils.WriteErrorJSON(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Get post from the database
	postObj, err := h.PostService.GetPostByID(postIDstr, userID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			utils.WriteErrorJSON(w, "Post not found", http.StatusNotFound)
			return
		}
		utils.WriteErrorJSON(w, "Failed to retrieve post: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response with post details
	response := map[string]interface{}{
		"success": true,
		"post":    postObj,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *PostHandler) GetUserPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Parse target user ID from request body
	var reqBody struct {
		UserID string `json:"user_id"`
		Offset int    `json:"offset,omitempty"`
		Limit  int    `json:"limit,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// validate target user ID
	if reqBody.UserID == "" {
		utils.WriteErrorJSON(w, "Target user ID is required", http.StatusBadRequest)
		return
	}

	isAllowed, err := h.PostService.IsUserAllowedToViewPosts(userID, reqBody.UserID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to check user permissions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !isAllowed {
		utils.WriteErrorJSON(w, "Unauthorized: You do not have permission to view this user's posts", http.StatusForbidden)
		return
	}

	// set default values for offset and limit
	offset := reqBody.Offset
	if offset < 0 {
		offset = 0
	}

	limit := reqBody.Limit
	if limit <= 0 {
		limit = 20 // default limit
	}
	if limit > 100 {
		limit = 100 // cap limit at 100
	}

	// Get user posts from DB
	posts, err := h.PostService.GetUserPosts(userID, reqBody.UserID, offset, limit)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to retrieve user posts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response with user posts
	response := map[string]interface{}{
		"success": true,
		"posts":   posts,
		"hasMore": len(posts) >= limit,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *PostHandler) EditPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get post ID from URL parameters
	postIDstr := r.URL.Query().Get("post_id")
	if postIDstr == "" {
		utils.WriteErrorJSON(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	postID, err := strconv.ParseInt(postIDstr, 10, 64)
	if err != nil {
		utils.WriteErrorJSON(w, "Invalid Post ID format: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Parse request body
	var req post.EditPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the request
	if _, err := post.ValidateEditPostRequest(&req); err != nil {
		response := post.EditPostResponse{
			Success: false,
			Error:   "validation error: " + err.Error(),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Edit post in database
	err = h.PostService.EditPost(postID, &req, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "unauthorized: you are not the author of this post" {
			status = http.StatusForbidden
		}
		response := post.EditPostResponse{
			Success: false,
			Error:   err.Error(),
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := post.EditPostResponse{
		Success: true,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get post ID from URL parameters
	postIDstr := r.URL.Query().Get("post_id")
	if postIDstr == "" {
		utils.WriteErrorJSON(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	postID, err := strconv.ParseInt(postIDstr, 10, 64)
	if err != nil {
		utils.WriteErrorJSON(w, "Invalid Post ID format: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Delete post in database
	err = h.PostService.DeletePost(postID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "unauthorized: you are not the author of this post" {
			status = http.StatusForbidden
		}

		response := post.DeletePostResponse{
			Success: false,
			Error:   err.Error(),
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(response)
		return
	}

	// return success response
	response := post.DeletePostResponse{
		Success: true,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *PostHandler) LikePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Get post ID from URL parameters
	postIDStr := r.URL.Query().Get("post_id")
	if postIDStr == "" {
		utils.WriteErrorJSON(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	postID, err := strconv.ParseInt(postIDStr, 10, 64)
	if err != nil {
		utils.WriteErrorJSON(w, "Invalid Post ID format: "+err.Error(), http.StatusBadRequest)
		return
	}

	isLiked, err, likeCount := h.PostService.LikePost(postID, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to like post: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"isliked":   isLiked,
		"likeCount": likeCount,
	}

	utils.WriteSuccessJSON(w, response, http.StatusOK)
}
