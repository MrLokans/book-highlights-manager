package auth

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	if err := db.AutoMigrate(&entities.User{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestService_CreateUser(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, config.Auth{BcryptCost: 10})

	tests := []struct {
		name     string
		username string
		email    string
		password string
		role     entities.UserRole
		wantErr  error
	}{
		{
			name:     "valid admin user",
			username: "admin",
			email:    "admin@example.com",
			password: "password12345",
			role:     entities.UserRoleAdmin,
			wantErr:  nil,
		},
		{
			name:     "missing username",
			username: "",
			email:    "test@example.com",
			password: "password12345",
			role:     entities.UserRoleViewer,
			wantErr:  ErrUsernameRequired,
		},
		{
			name:     "missing email",
			username: "testuser",
			email:    "",
			password: "password12345",
			role:     entities.UserRoleViewer,
			wantErr:  ErrEmailRequired,
		},
		{
			name:     "missing password",
			username: "testuser",
			email:    "test@example.com",
			password: "",
			role:     entities.UserRoleViewer,
			wantErr:  ErrPasswordRequired,
		},
		{
			name:     "password too short",
			username: "testuser",
			email:    "test@example.com",
			password: "short",
			role:     entities.UserRoleViewer,
			wantErr:  ErrPasswordTooShort,
		},
		{
			name:     "invalid role",
			username: "testuser",
			email:    "test@example.com",
			password: "password12345",
			role:     entities.UserRole("invalid"),
			wantErr:  ErrInvalidRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := svc.CreateUser(tt.username, tt.email, tt.password, tt.role)

			// Check for expected error (including wrapped errors)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("CreateUser() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateUser() unexpected error = %v", err)
				return
			}
			if user == nil {
				t.Error("CreateUser() returned nil user")
				return
			}
			if user.Username != tt.username {
				t.Errorf("user.Username = %v, want %v", user.Username, tt.username)
			}
			if user.Role != tt.role {
				t.Errorf("user.Role = %v, want %v", user.Role, tt.role)
			}
			if user.PasswordHash == "" {
				t.Error("user.PasswordHash is empty")
			}
		})
	}
}

func TestService_CreateUser_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, config.Auth{BcryptCost: 10})

	// Create first user
	_, err := svc.CreateUser("admin", "admin@example.com", "password12345", entities.UserRoleAdmin)
	if err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	// Try to create duplicate username
	_, err = svc.CreateUser("admin", "other@example.com", "password12345", entities.UserRoleViewer)
	if err != ErrUserExists {
		t.Errorf("Expected ErrUserExists for duplicate username, got %v", err)
	}

	// Try to create duplicate email
	_, err = svc.CreateUser("other", "admin@example.com", "password12345", entities.UserRoleViewer)
	if err != ErrUserExists {
		t.Errorf("Expected ErrUserExists for duplicate email, got %v", err)
	}
}

func TestService_Authenticate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, config.Auth{BcryptCost: 10})

	// Create a user
	_, err := svc.CreateUser("testuser", "test@example.com", "password12345", entities.UserRoleViewer)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{
			name:     "valid credentials with username",
			username: "testuser",
			password: "password12345",
			wantErr:  nil,
		},
		{
			name:     "valid credentials with email",
			username: "test@example.com",
			password: "password12345",
			wantErr:  nil,
		},
		{
			name:     "wrong password",
			username: "testuser",
			password: "wrongpassword",
			wantErr:  ErrInvalidPassword,
		},
		{
			name:     "non-existent user",
			username: "nobody",
			password: "password12345",
			wantErr:  ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := svc.Authenticate(tt.username, tt.password)
			if err != tt.wantErr {
				t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil && user == nil {
				t.Error("Authenticate() returned nil user for valid credentials")
			}
		})
	}
}

func TestService_TokenOperations(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, config.Auth{BcryptCost: 10})

	// Create a user
	user, err := svc.CreateUser("testuser", "test@example.com", "password12345", entities.UserRoleViewer)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Generate token
	token, err := svc.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if len(token) != 64 {
		t.Errorf("Token length = %d, want 64", len(token))
	}

	// Validate token
	validatedUser, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if validatedUser.ID != user.ID {
		t.Errorf("ValidateToken() user.ID = %d, want %d", validatedUser.ID, user.ID)
	}

	// Validate invalid token
	_, err = svc.ValidateToken("invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken(invalid) error = %v, want ErrInvalidToken", err)
	}

	// Revoke token
	if err := svc.RevokeToken(user.ID); err != nil {
		t.Fatalf("RevokeToken() error = %v", err)
	}

	// Validate revoked token should fail
	_, err = svc.ValidateToken(token)
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken(revoked) error = %v, want ErrInvalidToken", err)
	}
}

func TestService_ChangePassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, config.Auth{BcryptCost: 10})

	// Create a user
	user, err := svc.CreateUser("testuser", "test@example.com", "oldpassword1", entities.UserRoleViewer)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Change password with wrong old password
	err = svc.ChangePassword(user.ID, "wrongpassword", "newpassword1")
	if err != ErrInvalidPassword {
		t.Errorf("ChangePassword(wrong old) error = %v, want ErrInvalidPassword", err)
	}

	// Change password with correct old password
	err = svc.ChangePassword(user.ID, "oldpassword1", "newpassword1")
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	// Authenticate with new password
	_, err = svc.Authenticate("testuser", "newpassword1")
	if err != nil {
		t.Errorf("Authenticate(new password) error = %v", err)
	}

	// Old password should no longer work
	_, err = svc.Authenticate("testuser", "oldpassword1")
	if err != ErrInvalidPassword {
		t.Errorf("Authenticate(old password) error = %v, want ErrInvalidPassword", err)
	}
}

func TestService_HasUsers(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, config.Auth{BcryptCost: 10})

	// No users initially
	hasUsers, err := svc.HasUsers()
	if err != nil {
		t.Fatalf("HasUsers() error = %v", err)
	}
	if hasUsers {
		t.Error("HasUsers() = true, want false for empty DB")
	}

	// Create a user
	_, err = svc.CreateUser("testuser", "test@example.com", "password12345", entities.UserRoleViewer)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Now has users
	hasUsers, err = svc.HasUsers()
	if err != nil {
		t.Fatalf("HasUsers() after create error = %v", err)
	}
	if !hasUsers {
		t.Error("HasUsers() = false, want true after creating user")
	}
}

func TestService_IsAuthEnabled(t *testing.T) {
	db := setupTestDB(t)

	// None mode
	svc := NewService(db, config.Auth{Mode: config.AuthModeNone})
	if svc.IsAuthEnabled() {
		t.Error("IsAuthEnabled() = true for AuthModeNone")
	}

	// Local mode
	svc = NewService(db, config.Auth{Mode: config.AuthModeLocal})
	if !svc.IsAuthEnabled() {
		t.Error("IsAuthEnabled() = false for AuthModeLocal")
	}
}
