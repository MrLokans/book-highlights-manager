package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		cost     int
		wantErr  error
	}{
		{
			name:     "valid password",
			password: "validpassword123",
			cost:     10,
			wantErr:  nil,
		},
		{
			name:     "password too short",
			password: "short",
			cost:     10,
			wantErr:  ErrPasswordTooShort,
		},
		{
			name:     "password at minimum length",
			password: "123456789012", // 12 characters (new minimum)
			cost:     10,
			wantErr:  nil,
		},
		{
			name:     "password too long",
			password: strings.Repeat("a", 73),
			cost:     10,
			wantErr:  ErrPasswordTooLong,
		},
		{
			name:     "password at maximum length",
			password: strings.Repeat("a", 72),
			cost:     10,
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password, tt.cost)
			if err != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil && hash == "" {
				t.Error("HashPassword() returned empty hash for valid password")
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password, 10)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "correct password",
			password: password,
			wantErr:  nil,
		},
		{
			name:     "incorrect password",
			password: "wrongpassword",
			wantErr:  ErrInvalidPassword,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  ErrInvalidPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPassword(tt.password, hash)
			if err != tt.wantErr {
				t.Errorf("CheckPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateAPIToken(t *testing.T) {
	plaintext, hash, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken() error = %v", err)
	}

	// Token should be 64 hex characters (32 bytes)
	if len(plaintext) != 64 {
		t.Errorf("Token length = %d, want 64", len(plaintext))
	}

	// Hash should also be 64 hex characters (SHA-256)
	if len(hash) != 64 {
		t.Errorf("Hash length = %d, want 64", len(hash))
	}

	// HashToken should produce the same hash
	if HashToken(plaintext) != hash {
		t.Error("HashToken(plaintext) does not match returned hash")
	}

	// Generate another token, should be different
	plaintext2, _, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("Second GenerateAPIToken() error = %v", err)
	}
	if plaintext == plaintext2 {
		t.Error("Generated tokens should be unique")
	}
}

func TestGenerateSessionSecret(t *testing.T) {
	secret, err := GenerateSessionSecret()
	if err != nil {
		t.Fatalf("GenerateSessionSecret() error = %v", err)
	}

	// Secret should be 64 hex characters (32 bytes)
	if len(secret) != 64 {
		t.Errorf("Secret length = %d, want 64", len(secret))
	}

	// Generate another, should be different
	secret2, err := GenerateSessionSecret()
	if err != nil {
		t.Fatalf("Second GenerateSessionSecret() error = %v", err)
	}
	if secret == secret2 {
		t.Error("Generated secrets should be unique")
	}
}
