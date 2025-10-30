package user

import (
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"social-network/pkg/db"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	//Email validation errors
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrEmailTooShort      = errors.New("email must be at least 5 charcters")
	ErrEmailTooLong       = errors.New("email must be at most 254 characters")

	//Password validation errors
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 60 characters")
	ErrPasswordNoUpper  = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLower  = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoDigit  = errors.New("password must contain at least one digit")

	//Nickname validation errors
	ErrNicknameAlreadyExists = errors.New("nickname is already taken")
	ErrNicknameTooShort      = errors.New("nickname must be at least 3 characters")
	ErrNicknameTooLong       = errors.New("nickname must be at most 20 characters")
	ErrNicknameFormat        = errors.New("nickname can only contain alphanumeric characters in lowercase and numbers")

	//name validation errors
	ErrFirstNameTooShort = errors.New("first name must be at least 3 characters")
	ErrLastNameTooShort  = errors.New("last name must be at least 3 characters")
	ErrFirstNameTooLong  = errors.New("first name must be at most 15 characters")
	ErrLastNameTooLong   = errors.New("last name must be at most 15 characters")
	ErrFirstNameFormat   = errors.New("first name can only contain alphabetic characters")
	ErrLastNameFormat    = errors.New("last name can only contain alphabetic characters")

	// DOB validation errors
	ErrInvalidDOB  = errors.New("invalid date of birth format")
	ErrDOBInFuture = errors.New("date of birth cannot be in the future")
	ErrDOBTooOld   = errors.New("date of birth cannot be more than 120 years ago")
	ErrDOBTooYoung = errors.New("date of birth must be at least 13 years old")

	// AboutMe validation errors
	ErrAboutMeTooLong = errors.New("about me must be at most 500 characters")
)

// RegisterRequest represents the data needed to sign up a new user
type RegisterRequest struct {
	Nickname        string `json:"nickname"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	DOB             string `json:"dob"`
	AboutMe         string `json:"aboutMe"`
	Avatar          string `json:"avatar_path"`
}

// ValidateEmail checks if the register request is valid
func ValidateEmail(email string) (bool, error) {
	if len(email) < 5 {
		return false, ErrEmailTooShort
	}
	if len(email) > 254 {
		return false, ErrEmailTooLong
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return false, ErrInvalidEmail
	}

	return true, nil
}

// ValidatePassword checks if the password is strong enough
func ValidatePassword(password string) (bool, error) {
	// checking password lengeth
	if len(password) < 8 {
		return false, ErrPasswordTooShort
	}
	if len(password) > 60 {
		return false, ErrPasswordTooLong
	}

	// check for at least one uppercase letter
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	if !hasUpper {
		return false, ErrPasswordNoUpper
	}

	//check for at lease one lowercase letter
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	if !hasLower {
		return false, ErrPasswordNoLower
	}

	// check for at least one digit
	hasDigit := regexp.MustCompile(`\d`).MatchString(password)
	if !hasDigit {
		return false, ErrPasswordNoDigit
	}

	return true, nil
}

// function to generate a random nickname if the user does not provide one
func generateNickname() (string, error) {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// try up to 10 attempts to generate a unique name
	for i := 0; i < 10; i++ {
		//Get 4 digit numbers
		randNum := rand.Intn(9000) + 1000
		nicnkname := fmt.Sprintf("user%d", randNum)

		// check if it exists or not
		query := `SELECT COUNT(*) FROM users WHERE nickname = ?`
		var count int
		err := db.DB.QueryRow(query, nicnkname).Scan(&count)
		if err != nil {
			return "", err
		}

		if count == 0 {
			return nicnkname, nil
		}
	}
	// if the function failed
	return "", errors.New("failed to generate a unique nickname after 10 attempts")

}

// ValidateNickname checks if the Nickname is valid
func ValidateNickname(nickname string) (bool, error) {
	// If the nickname is empty, generate a random one
	if nickname == "" {
		return true, nil
	}

	if len(nickname) < 3 {
		return false, ErrNicknameTooShort
	}
	if len(nickname) > 20 {
		return false, ErrNicknameTooLong
	}

	nicknameRegex := regexp.MustCompile(`^[a-z0-9]+$`)
	if !nicknameRegex.MatchString(nickname) {
		return false, ErrNicknameFormat
	}

	// Check if the nickname already exists in the database
	query := `SELECT COUNT(*) FROM users WHERE nickname = ?`

	var count int
	err := db.DB.QueryRow(query, nickname).Scan(&count)
	if err != nil {
		return false, err
	}

	if count > 0 {
		return false, ErrNicknameAlreadyExists
	}

	return true, nil
}

// ValidateName checks if the first and last names are valid
func ValidateName(firstName, LastName string) (bool, error) {
	if len(firstName) < 3 {
		return false, ErrFirstNameTooShort
	}
	if len(LastName) < 3 {
		return false, ErrLastNameTooShort
	}

	if len(firstName) > 15 {
		return false, ErrFirstNameTooLong
	}
	if len(LastName) > 15 {
		return false, ErrLastNameTooLong
	}

	firstNameRegex := regexp.MustCompile(`^[a-zA-Z]+$`)
	if !firstNameRegex.MatchString(firstName) {
		return false, ErrFirstNameFormat
	}
	lastNameRegex := regexp.MustCompile(`^[a-zA-Z]+$`)
	if !lastNameRegex.MatchString(LastName) {
		return false, ErrLastNameFormat
	}

	return true, nil
}

func ValidateDOB(dobstr string) (bool, error) {
	// DOB can be empty since its optional
	if dobstr == "" {
		return true, nil
	}

	// Parse the date to validate
	dob, err := time.Parse("2006-01-02", dobstr)
	if err != nil {
		return false, ErrInvalidDOB
	}

	now := time.Now()

	// check if the date is in the future
	if dob.After(now) {
		return false, ErrDOBInFuture
	}

	// Age restrictions =================================
	minAge := now.AddDate(-13, 0, 0) // 13 years ago (13 years ago)
	if dob.After(minAge) {
		return false, ErrDOBTooYoung
	}

	maxAge := now.AddDate(-120, 0, 0) // 120 years ago
	if dob.Before(maxAge) {
		return false, ErrDOBTooOld
	}

	return true, nil
}

func ValidateAboutMe(aboutME string) (bool, error) {
	// AboutMe can be empty since its optional
	if aboutME == "" {
		return true, nil
	}

	// check the length
	if len(aboutME) > 500 {
		return false, ErrAboutMeTooLong
	}

	return true, nil
}

// Register creates a new user account
func Register(req RegisterRequest) (*User, error) {
	// Validate the email
	if valid, err := ValidateEmail(req.Email); !valid {
		return nil, err
	}

	// Validate password
	if valid, err := ValidatePassword(req.Password); !valid {
		return nil, err
	}

	// Validate first and last names
	if valid, err := ValidateName(req.FirstName, req.LastName); !valid {
		return nil, err
	}

	// validate date of birth
	if valid, err := ValidateDOB(req.DOB); !valid {
		return nil, err
	}

	// Validate about me
	if valid, err := ValidateAboutMe(req.AboutMe); !valid {
		return nil, err
	}

	// check if name is empty and generate one
	if req.Nickname == "" {
		generateNickname, err := generateNickname()
		if err != nil {
			return nil, err
		}
		req.Nickname = generateNickname
	} else {
		// Validate nickname
		if valid, err := ValidateNickname(req.Nickname); !valid {
			return nil, err
		}
	}

	// Check if the email already exists
	_, err := GetUserByEmail(req.Email)
	if err == nil {
		return nil, ErrEmailAlreadyExists
	} else if err != ErrUserNotFound {
		return nil, err
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create the user
	user := User{
		Nickname:     req.Nickname,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		DOB:          req.DOB,
		AboutMe:      req.AboutMe,
		Avatar:       req.Avatar,
		IsPublic:     true, // default to public profile
	}

	userID, err := CreateUser(user)
	if err != nil {
		// Check for specific database constraint errors
		// also handle the case of race conditions
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			return nil, ErrEmailAlreadyExists
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.nickname") {
			return nil, ErrNicknameAlreadyExists
		}
		// Other database errors
		return nil, err
	}

	user.ID = userID

	return &user, nil
}
