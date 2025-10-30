package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"social-network/pkg/db"
	"social-network/pkg/models/comment"
	"social-network/pkg/utils"
)

// handler for creating a new comment
func CommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context (set by auth middleware)
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var newComment comment.Comment
	if err := json.NewDecoder(r.Body).Decode(&newComment); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Set the author ID from the authenticated user
	newComment.AuthorID = userID

	// validate the comment
	if err := comment.ValidateComment(newComment); err != nil {
		utils.WriteErrorJSON(w, "Invalid comment: "+err.Error(), http.StatusBadRequest)
		return
	}

	createdComment, err := comment.CreateComment(db.DB, newComment)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to create comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdComment)
}

// handler for updating an existing comment
func UpdateCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context (set by auth middleware)
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var updatedComment comment.Comment
	if err := json.NewDecoder(r.Body).Decode(&updatedComment); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Ensure the user can only update their own comments
	updatedComment.AuthorID = userID

	// validate the updated comment
	if err := comment.ValidateComment(updatedComment); err != nil {
		utils.WriteErrorJSON(w, "Invalid comment: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Update the comment in the database
	updated, err := comment.UpdateComment(db.DB, updatedComment)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to update comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updated)
}

// handler for deleting an existing comment
func DeleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context (set by auth middleware)
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	var deletedComment comment.Comment
	if err := json.NewDecoder(r.Body).Decode(&deletedComment); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Ensure the user can only delete their own comments
	deletedComment.AuthorID = userID

	// Delete the comment from the database
	err := comment.DeleteComment(db.DB, deletedComment)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to delete comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "comment deleted successfully"})
}

// handler for getting comments by post ID
func GetCommentsByPostIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		utils.WriteErrorJSON(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		utils.WriteErrorJSON(w, "Offset is required", http.StatusBadRequest)
		return
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		utils.WriteErrorJSON(w, "Invalid offset value", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		utils.WriteErrorJSON(w, "Limit is required", http.StatusBadRequest)
		return
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		utils.WriteErrorJSON(w, "Invalid limit value", http.StatusBadRequest)
		return
	}
	if limit > 100 {
		limit = 100
	}

	// Get the user ID from the context (set by auth middleware)
	userID := r.Context().Value("userID").(string)

	comments, err := comment.GetComment(db.DB, postID, userID, offset, limit)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(comments) == 0 {
		utils.WriteErrorJSON(w, "No comments found for the given post ID", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

// handler for liking/unliking a comment
func LikeCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the user ID from the context (assuming you have auth middleware)
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized: User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Parse the request body to get the comment ID
	var likeRequest comment.CommentLike
	if err := json.NewDecoder(r.Body).Decode(&likeRequest); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate that comment ID is provided
	if likeRequest.CommentID == "" {
		utils.WriteErrorJSON(w, "Comment ID is required", http.StatusBadRequest)
		return
	}

	// Like/unlike the comment
	isLiked, err, likeCount := comment.LikeComment(db.DB, likeRequest.CommentID, userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to like comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"isLiked":   isLiked,
		"likeCount": likeCount,
	}

	utils.WriteSuccessJSON(w, response, http.StatusOK)
}
