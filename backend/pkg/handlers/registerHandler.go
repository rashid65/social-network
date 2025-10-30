package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"social-network/pkg/models/user"
	"social-network/pkg/utils"

)

// RegisterHandler handles user registration
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req user.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorJSON(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	newUser, err := user.Register(req)
	if err != nil {
		// Default error status
		status := http.StatusBadRequest
		switch err {
			case user.ErrEmailAlreadyExists:
				status = http.StatusConflict
			case user.ErrNicknameAlreadyExists:
				status = http.StatusConflict

		default:
			utils.WriteErrorJSON(w, err.Error(), status)
			log.Printf("Error registering user: %v", err)
			return
		}
	}

	// Dont return the password hash (security best practice)
	newUser.PasswordHash = ""
	// Dont return the ID (security best practice)
	newUser.ID = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newUser)
}

// const (
// 	MaxFileSize = 5 << 20 // 5MB
// 	UploadDir   = "./uploads/avatars"
// )

// init creates the upload directory if it doesn't exist
// func init() {
// 	if err := os.MkdirAll(UploadDir, 0755); err != nil {
// 		log.Printf("Failed to create upload directory: %v", err)
// 	}
// }

// PublicAvatarUploadHandler handles avatar uploads (no auth required)
// func PublicAvatarUploadHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	// Parse multipart form
// 	err := r.ParseMultipartForm(MaxFileSize)
// 	if err != nil {
// 		utils.WriteErrorJSON(w, "File too large or invalid form", http.StatusBadRequest)
// 		return
// 	}

// 	// Get the file
// 	file, header, err := r.FormFile("avatar")
// 	if err != nil {
// 		utils.WriteErrorJSON(w, "No file provided", http.StatusBadRequest)
// 		return
// 	}
// 	defer file.Close()

// 	// Simple file type validation
// 	ext := strings.ToLower(filepath.Ext(header.Filename))
// 	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" {
// 		utils.WriteErrorJSON(w, "Only JPG, PNG, and GIF files allowed", http.StatusBadRequest)
// 		return
// 	}

// 	// Generate simple filename
// 	fileName := fmt.Sprintf("avatar_%d%s", time.Now().UnixNano(), ext)
// 	filePath := filepath.Join(UploadDir, fileName)

// 	// Save file
// 	dst, err := os.Create(filePath)
// 	if err != nil {
// 		utils.WriteErrorJSON(w, "Failed to save file", http.StatusInternalServerError)
// 		return
// 	}
// 	defer dst.Close()

// 	_, err = io.Copy(dst, file)
// 	if err != nil {
// 		utils.WriteErrorJSON(w, "Failed to save file", http.StatusInternalServerError)
// 		return
// 	}

// 	// Return path
// 	utils.WriteSuccessJSON(w, map[string]string{
// 		"avatar_path": fmt.Sprintf("/uploads/avatars/%s", fileName),
// 	}, http.StatusOK)
// }

// ServeAvatarHandler serves uploaded files
// func ServeAvatarHandler(w http.ResponseWriter, r *http.Request) {
// 	filename := r.URL.Path[len("/uploads/avatars/"):]
// 	if filename == "" {
// 		http.NotFound(w, r)
// 		return
// 	}

// 	filePath := filepath.Join(UploadDir, filename)
// 	if _, err := os.Stat(filePath); os.IsNotExist(err) {
// 		http.NotFound(w, r)
// 		return
// 	}

// 	http.ServeFile(w, r, filePath)
// }
