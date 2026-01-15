package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength is the minimum required password length (NIST recommendation).
const MinPasswordLength = 12

var (
	ErrInvalidPassword  = errors.New("invalid password")
	ErrPasswordTooShort = errors.New("password must be at least 12 characters")
	ErrPasswordTooLong  = errors.New("password exceeds maximum length of 72 bytes")
)

// HashPassword creates a bcrypt hash of the password.
func HashPassword(password string, cost int) (string, error) {
	if len(password) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}
	// bcrypt has a 72-byte limit
	if len(password) > 72 {
		return "", ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a password with its hash.
func CheckPassword(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrInvalidPassword
		}
		return err
	}
	return nil
}

// GenerateAPIToken creates a cryptographically secure random token.
// Returns the plaintext token (to show user once) and its hash (for storage).
func GenerateAPIToken() (plaintext string, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	plaintext = hex.EncodeToString(bytes)
	hash = HashToken(plaintext)
	return plaintext, hash, nil
}

// HashToken creates a SHA-256 hash of an API token for secure storage.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// GenerateSessionSecret creates a random 32-byte secret for session signing.
func GenerateSessionSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
