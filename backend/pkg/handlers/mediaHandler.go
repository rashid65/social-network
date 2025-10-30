package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"social-network/pkg/utils"
	"strings"
	"time"
)

const (
	MaxMediaSize   = 10 << 20
	MediaUploadDir = "./uploads/media"
)

type MediaHandler struct{}

func NewMediaHandler() *MediaHandler {
	return &MediaHandler{}
}

// initialze the dirctory if deleted or not availabel
func init() {
	if err := os.MkdirAll(MediaUploadDir, 0755); err != nil {
		panic("Failed to create media upload directory: " + err.Error())
	}
}

func (h *MediaHandler) UploadMediaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the form data
	err := r.ParseMultipartForm(MaxMediaSize)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to parse form data: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get the file from the form data
	file, header, err := r.FormFile("media")
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get file from form data: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validation
	ext := strings.ToLower(filepath.Ext(header.Filename))
	var mediaType string
	switch ext {
	case ".jpg", ".jpeg":
		mediaType = "image/jpeg"
	case ".png":
		mediaType = "image/png"
	case ".gif":
		mediaType = "image/gif"
	default:
		utils.WriteErrorJSON(w, "Unsupported media type: "+ext, http.StatusBadRequest)
		return
	}

	// generte unique filename
	fileName := fmt.Sprintf("media_%d%s", time.Now().UnixNano(), ext)
	filePath := filepath.Join(MediaUploadDir, fileName)

	// save the file
	dst, err := os.Create(filePath)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to create file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return media info
	response := map[string]interface{}{
		"sucess": true,
		"media": map[string]string{
			"media_type": mediaType,
			"file_path": fmt.Sprintf("/uploads/media/%s", fileName),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}