package handlers

import (
	"encoding/json"
	"net/http"
	"social-network/pkg/models/follow"
	"social-network/pkg/utils"
)

type FollowHandler struct {
	FollowService *follow.FollowService
}

func NewFollowHandler(followService *follow.FollowService) *FollowHandler {
	return &FollowHandler{
		FollowService: followService,
	}
}

func (h *FollowHandler) SendFollowRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
		return
	}

	var req struct {
		FolloweeID string `json:"followee_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if userID == req.FolloweeID {
		utils.WriteErrorJSON(w, "You cannot follow yourself", http.StatusBadRequest)
		return
	}

	if err := h.FollowService.SendFollowRequest(userID, req.FolloweeID); err != nil {
		if err.Error() == "follow request already exists" {
			utils.WriteErrorJSON(w, "Follow request already exists", http.StatusBadRequest)
			return
		}
		utils.WriteErrorJSON(w, "Failed to send follow request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Follow request sent"})
}

func (h *FollowHandler) AcceptFollowRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
		return
	}

	var req struct {
		FollowerID string `json:"follower_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.FollowService.AcceptFollowRequest(req.FollowerID, userID); err != nil {
		utils.WriteErrorJSON(w, "Failed to accept follow request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Follow request accepted"})
}

func (h *FollowHandler) RejectFollowRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
		return
	}

	var req struct {
		FollowerID string `json:"follower_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.FollowService.RejectFollowRequest(req.FollowerID, userID); err != nil {
		utils.WriteErrorJSON(w, "Failed to reject follow request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Follow request rejected"})
}

func (h *FollowHandler) GetPendingRequestsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
		return
	}

	requests, err := h.FollowService.GetPendingRequests(userID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get pending requests: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

func (h *FollowHandler) GetUserFollowersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the userID from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
		return
	}

	// Parse target userID from request body
	var reqBody struct {
		UserID string `json:"user_id"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate target userID
	if reqBody.UserID == "" {
		utils.WriteErrorJSON(w, "Invalid request: UserID is required", http.StatusBadRequest)
		return
	}

	// Set default values for offset and limit
	offset := reqBody.Offset
	if offset < 0 {
		offset = 0
	}

	limit := reqBody.Limit
	if limit <= 0 {
		limit = 20 // Default limit
	}
	if limit > 100 {
		limit = 100 // Maximum limit
	}

	// check privacy related settings between the two users
	canView, err := h.FollowService.CanViewUserData(userID, reqBody.UserID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to check privacy settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !canView {
		utils.WriteErrorJSON(w, "Unauthorized access: You cannot view this user's followers", http.StatusForbidden)
		return
	}

	// Get followers from DB
	followers, err := h.FollowService.GetUserFollowers(userID, reqBody.UserID, offset, limit)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get followers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return followers as JSON
	response := map[string]interface{}{
		"success":   true,
		"followers": followers,
		"hasMore":   len(followers) >= limit,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *FollowHandler) GetUserFollowingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the userID from the context
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
		return
	}

	// PArse target user ID from request body
	var reqBody struct {
		UserID string `json:"user_id"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate target UserID
	if reqBody.UserID == "" {
		utils.WriteErrorJSON(w, "Invalid request: UserID is required", http.StatusBadRequest)
		return
	}

	// set deafult offset + limit
	offset := reqBody.Offset
	if offset < 0 {
		offset = 0
	}

	limit := reqBody.Limit
	if limit <= 0 {
		limit = 20 // Default limit
	}
	if limit > 100 {
		limit = 100 // Maximum limit
	}

	// check privacy related settings between the two users
	canView, err := h.FollowService.CanViewUserData(userID, reqBody.UserID)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to check privacy settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !canView {
		utils.WriteErrorJSON(w, "Unauthorized access: You cannot view this user's following", http.StatusForbidden)
		return
	}

	// Get following from DB
	following, err := h.FollowService.GetUserFollowing(userID, reqBody.UserID, offset, limit)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get following: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return following as JSON
	response := map[string]interface{}{
		"success":   true,
		"following": following,
		"hasMore":   len(following) >= limit,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *FollowHandler) UnfollowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		utils.WriteErrorJSON(w, "Method not allowed: should be delete", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("userID").(string)
	if userID == "" {
		utils.WriteErrorJSON(w, "Unauthorized access: UserID not found in context", http.StatusUnauthorized)
		return
	}

	var req struct {
		FolloweeID string `json:"followee_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.FolloweeID == "" {
		utils.WriteErrorJSON(w, "Invalid request: FolloweeID is required", http.StatusBadRequest)
		return
	}

	if userID == req.FolloweeID {
		utils.WriteErrorJSON(w, "Invalid request: Cannot unfollow yourself", http.StatusBadRequest)
		return
	}

	if err := h.FollowService.Unfollow(userID, req.FolloweeID); err != nil {
		if err.Error() == "you are not following this user" {
			utils.WriteErrorJSON(w, "You are not following this user", http.StatusBadRequest)
			return
		}
		utils.WriteErrorJSON(w, "Failed to unfollow user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteSuccessJSON(w, "Successfully unfollowed user", http.StatusOK)
}
