// Package users provides database operations for user management.
//
// # Usage
//
//	repo := users.NewRepository(db)
//	user, err := repo.GetUserByToken(token)
package users

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles all user database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new users repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateUser creates a new user with a generated token.
func (r *Repository) CreateUser(username, email string) (*entities.User, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	user := &entities.User{
		Username: username,
		Email:    email,
		Token:    token,
	}

	if err := r.db.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByToken retrieves a user by their token.
func (r *Repository) GetUserByToken(token string) (*entities.User, error) {
	var user entities.User
	err := r.db.Where("token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID retrieves a user by ID.
func (r *Repository) GetUserByID(id uint) (*entities.User, error) {
	var user entities.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username.
func (r *Repository) GetUserByUsername(username string) (*entities.User, error) {
	var user entities.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
