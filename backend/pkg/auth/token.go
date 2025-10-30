package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"social-network/pkg/db"
	"time"

	"github.com/google/uuid"
)

// ValidateToken validates the provided token string against the sessions table in the database.
func ValidateToken(tokenString string) (string, error) {
	// First check if the token exists in the sessions table
	var userID string
	var expiresAtstr string

	err := db.DB.QueryRow("SELECT user_id, expires_at FROM sessions WHERE token = ?", tokenString).Scan(&userID, &expiresAtstr)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("invalid session: token not found")
		}
		return "", fmt.Errorf("database error: %w", err)
	}

	// Parse the expiration time from the string to a time.Time
	expiresAt, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", expiresAtstr)
	if err != nil {
		return "", fmt.Errorf("failed to parse expiration time: %w", err)
	}

	// check if session has expired
	if time.Now().After(expiresAt) {
		// clean up expired session
		db.DB.Exec("DELETE FROM sessions WHERE token = ?", tokenString)
		return "", errors.New("session has expired")
	}

	return userID, nil
}

// InvalidateToken deletes the session token from the database (for Logout)
func InvalidateToken(tokenString string) error {
	result, err := db.DB.Exec("DELETE FROM sessions WHERE token = ?", tokenString)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// check if any rows were affectes (just in case)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("no session found for the provided token")
	}

	return nil
}

func GenerateToken(userID string) (string, error) {
	// Generate a new session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(24 * time.Hour) // Set expiration to 24 hours

	// generate a UUID for the session ID
	sessionID := uuid.New().String()

	// Strore in database
	_, err := db.DB.Exec("INSERT INTO sessions (id, user_id, token, expires_at, created_at) VALUES (?, ?, ?, ?, ?)", 
        sessionID, userID, token, expiresAt, time.Now())

	return token, err
}