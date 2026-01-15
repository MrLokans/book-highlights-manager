package auth

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
)

// Validation patterns
var (
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)
	emailPattern    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrAuthRequired       = errors.New("authentication required")
	ErrInvalidRole        = errors.New("invalid role")
	ErrNoUsers            = errors.New("no users exist")
	ErrUsernameRequired   = errors.New("username is required")
	ErrEmailRequired      = errors.New("email is required")
	ErrPasswordRequired   = errors.New("password is required")
	ErrAccountLocked      = errors.New("account is locked due to too many failed login attempts")
	ErrUsernameInvalid    = errors.New("username must be 3-64 characters, alphanumeric and underscore/hyphen only")
	ErrEmailInvalid       = errors.New("invalid email format")
)

// UserRepository defines the interface for user data access.
type UserRepository interface {
	CreateUser(username, email string) (*entities.User, error)
	GetUserByID(id uint) (*entities.User, error)
	GetUserByUsername(username string) (*entities.User, error)
	GetUserByToken(token string) (*entities.User, error)
}

// Service handles authentication and user management.
type Service struct {
	db     *gorm.DB
	config config.Auth
}

// NewService creates a new authentication service.
func NewService(db *gorm.DB, cfg config.Auth) *Service {
	return &Service{
		db:     db,
		config: cfg,
	}
}

// CreateUser creates a new user with password authentication.
func (s *Service) CreateUser(username, email, password string, role entities.UserRole) (*entities.User, error) {
	if username == "" {
		return nil, ErrUsernameRequired
	}
	if email == "" {
		return nil, ErrEmailRequired
	}
	if password == "" {
		return nil, ErrPasswordRequired
	}

	// Validate username format: 3-64 chars, alphanumeric + underscore/hyphen
	if !usernamePattern.MatchString(username) {
		return nil, ErrUsernameInvalid
	}

	// Validate email format and length (RFC 5321 limit is 254)
	if len(email) > 254 || !emailPattern.MatchString(email) {
		return nil, ErrEmailInvalid
	}

	// Validate role
	switch role {
	case entities.UserRoleAdmin, entities.UserRoleEditor, entities.UserRoleViewer:
		// Valid
	default:
		return nil, ErrInvalidRole
	}

	// Check if user already exists
	var existing entities.User
	err := s.db.Where("username = ? OR email = ?", username, email).First(&existing).Error
	if err == nil {
		return nil, ErrUserExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	// Hash password
	passwordHash, err := HashPassword(password, s.config.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &entities.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// Authenticate validates credentials and returns the user.
// Implements account lockout after too many failed attempts.
func (s *Service) Authenticate(username, password string) (*entities.User, error) {
	var user entities.User
	err := s.db.Where("username = ? OR email = ?", username, username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Check if account is locked
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, ErrAccountLocked
	}

	if err := CheckPassword(password, user.PasswordHash); err != nil {
		// Record failed login attempt
		s.recordFailedLogin(&user)
		return nil, err
	}

	// Successful login - reset failed attempts and update last login
	now := time.Now()
	s.db.Model(&user).Updates(map[string]any{
		"last_login_at":      now,
		"failed_login_count": 0,
		"locked_until":       nil,
	})

	return &user, nil
}

// recordFailedLogin increments the failed login counter and locks the account if threshold reached.
func (s *Service) recordFailedLogin(user *entities.User) {
	user.FailedLoginCount++

	updates := map[string]any{
		"failed_login_count": user.FailedLoginCount,
	}

	// Lock account after 5 failed attempts (default lockout duration from config)
	if user.FailedLoginCount >= 5 {
		lockoutDuration := s.config.LockoutDuration
		if lockoutDuration == 0 {
			lockoutDuration = 30 * time.Minute
		}
		lockedUntil := time.Now().Add(lockoutDuration)
		updates["locked_until"] = lockedUntil
	}

	s.db.Model(user).Updates(updates)
}

// GetUserByID retrieves a user by their ID.
func (s *Service) GetUserByID(id uint) (*entities.User, error) {
	var user entities.User
	err := s.db.First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByTokenHash retrieves a user by their hashed API token.
func (s *Service) GetUserByTokenHash(tokenHash string) (*entities.User, error) {
	var user entities.User
	err := s.db.Where("token_hash = ?", tokenHash).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	return &user, nil
}

// ValidateToken checks a plaintext token and returns the associated user.
// Returns ErrTokenExpired if the token is past its expiry time.
func (s *Service) ValidateToken(token string) (*entities.User, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	tokenHash := HashToken(token)
	user, err := s.GetUserByTokenHash(tokenHash)
	if err != nil {
		return nil, err
	}

	// Check token expiry if configured
	if s.config.TokenExpiry > 0 && user.TokenCreatedAt != nil {
		if time.Since(*user.TokenCreatedAt) > s.config.TokenExpiry {
			return nil, ErrTokenExpired
		}
	}

	return user, nil
}

// GenerateToken creates a new API token for a user.
// Returns the plaintext token (show to user once) - only the hash is stored in DB.
func (s *Service) GenerateToken(userID uint) (string, error) {
	plaintext, hash, err := GenerateAPIToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now()
	result := s.db.Model(&entities.User{}).Where("id = ?", userID).Updates(map[string]any{
		"token":            "", // Clear any legacy plaintext token
		"token_hash":       hash,
		"token_created_at": now,
	})
	if result.Error != nil {
		return "", fmt.Errorf("failed to save token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return "", ErrUserNotFound
	}

	return plaintext, nil
}

// RevokeToken removes a user's API token.
func (s *Service) RevokeToken(userID uint) error {
	result := s.db.Model(&entities.User{}).Where("id = ?", userID).Updates(map[string]any{
		"token":            "",
		"token_hash":       "",
		"token_created_at": nil,
	})
	if result.Error != nil {
		return fmt.Errorf("failed to revoke token: %w", result.Error)
	}
	return nil
}

// ChangePassword updates a user's password.
func (s *Service) ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}

	// Verify old password
	if err := CheckPassword(oldPassword, user.PasswordHash); err != nil {
		return err
	}

	// Hash new password
	newHash, err := HashPassword(newPassword, s.config.BcryptCost)
	if err != nil {
		return err
	}

	return s.db.Model(user).Update("password_hash", newHash).Error
}

// HasUsers returns true if any users exist in the database.
func (s *Service) HasUsers() (bool, error) {
	var count int64
	err := s.db.Model(&entities.User{}).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserCount returns the number of users in the database.
func (s *Service) GetUserCount() (int64, error) {
	var count int64
	err := s.db.Model(&entities.User{}).Count(&count).Error
	return count, err
}

// IsAuthEnabled returns true if authentication is required.
func (s *Service) IsAuthEnabled() bool {
	return s.config.Mode == config.AuthModeLocal
}

// GetAuthMode returns the current authentication mode.
func (s *Service) GetAuthMode() config.AuthMode {
	return s.config.Mode
}
