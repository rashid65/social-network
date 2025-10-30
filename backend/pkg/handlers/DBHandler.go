package handlers

import (
	"encoding/json"
	"net/http"
	"social-network/pkg/db"
	"social-network/pkg/db/sqlite"
	"social-network/pkg/utils"
	"time"
)

// WALStatusHandler returns current WAL mode status and info
func WALStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check WAL mode
	isWAL, err := sqlite.CheckWALMode(db.DB)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to check WAL mode: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get WAL info
	walInfo, err := sqlite.GetWALInfo(db.DB)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to get WAL info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"wal_enabled": isWAL,
		"wal_info":    walInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// WALCheckpointHandler manually triggers a WAL checkpoint
func WALCheckpointHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteErrorJSON(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := sqlite.WALCheckpoint(db.DB)
	if err != nil {
		utils.WriteErrorJSON(w, "Failed to checkpoint WAL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.WriteSuccessJSON(w, "WAL checkpoint completed successfully", http.StatusOK)
}

// Health handler provides a simple health check endpoint
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}

	// Check database connection
	if err := db.DB.Ping(); err != nil {
		health["status"] = "unhealthy"
		health["database"] = "connection failed"
	} else {
		// Check WAL mode
		if isWAL, err := sqlite.CheckWALMode(db.DB); err != nil {
			health["database"] = "wal check failed"
		} else {
			health["database"] = map[string]interface{}{
				"connected":   true,
				"wal_enabled": isWAL,
			}
		}
	}

	if health["status"] == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
