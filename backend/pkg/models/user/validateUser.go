package user

import (
	"errors"
	"log"
	"regexp"
	"social-network/pkg/auth"
	"social-network/pkg/db"
	"golang.org/x/crypto/bcrypt"
)

var (
	//Authentication errors (give minimal information to the user)
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// LoginRequest represents the data needed to log in a user
type LoginRequest struct {
	Identifier string `json:"identifier"` // can be email or nickname
	Password   string `json:"password"`
}

// Login function validates the login request
func Login(request LoginRequest) (*User, string, error) {
	// validation for identifer (email or nicnkname)
	if len(request.Identifier) < 3 {
		return nil, "", errors.New("identifier must be at least 3 characters")
	}

	// Password validation (basic check only since
	// we are only checking the password and not storing it)
	if len(request.Password) < 8 {
		return nil, "", ErrPasswordTooShort
	}
	if len(request.Password) > 60 {
		return nil, "", ErrPasswordTooLong
	}

	identifier := request.Identifier

	// check if its email or nickname
	var user User
	var err error

	// to know if the identifier is an email check if it contains '@'
	emailRegex := regexp.MustCompile(`@`)
	if emailRegex.MatchString(identifier) {
		// If it contains '@', treat it as an email
		if valid, err := ValidateEmail(identifier); !valid {
			return nil, "", err
		}
		user, err = GetUserByEmail(identifier)

	} else {
		// Validate as nickname
		if len(identifier) < 3 {
			return nil, "", ErrNicknameTooShort
		}

		// Format validation if needed (alphanumeric in lowercase)
		nicknameRegex := regexp.MustCompile(`^[a-z0-9]+$`)
		if !nicknameRegex.MatchString(identifier) {
			return nil, "", ErrNicknameFormat
		}
		user, err = GetUserByNickname(identifier)
		if err != nil {
			log.Printf("Error retrieving user by nickname: %v", err)
		}
	}

	// return the same error for both nickname and email
	if err != nil {
		if err == ErrUserNotFound {
			log.Printf("User not found: %v", err)
			return nil, "", ErrInvalidCredentials
		}
		log.Printf("Error retrieving user: %v", err)
		return nil, "", err
	}

	// check the password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password))
	if err != nil {
		// again dont show that the password matched or not
		return nil, "", ErrInvalidCredentials
	}

	// Cleanup old sessions for the user
	if err := cleanupOldSessions(user.ID); err != nil {
		log.Printf("Error cleaning up old sessions: %v", err)
		return nil, "", err
	}

	// Generate a session token
	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		return nil, "", err
	}

	return &user, token, nil
}


func cleanupOldSessions(userID string) error {
	query := `DELETE FROM sessions WHERE user_id = ?`
	_, err := db.DB.Exec(query, userID)
	return err
}