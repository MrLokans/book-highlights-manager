// Package crypto provides AES-256-GCM encryption for sensitive data storage.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const (
	// KeySize is the required size for AES-256 keys (32 bytes)
	KeySize = 32
	// NonceSize is the standard size for GCM nonces (12 bytes)
	NonceSize = 12
)

var (
	ErrInvalidKeySize     = errors.New("encryption key must be 32 bytes for AES-256")
	ErrCiphertextTooShort = errors.New("ciphertext too short")
	ErrDecryptionFailed   = errors.New("decryption failed: authentication error")
)

// Encryptor handles AES-256-GCM encryption and decryption
type Encryptor struct {
	key []byte
}

// NewEncryptor creates a new Encryptor with the given key.
// Key must be exactly 32 bytes for AES-256.
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}

	// Copy key to avoid external mutation
	keyCopy := make([]byte, KeySize)
	copy(keyCopy, key)

	return &Encryptor{key: keyCopy}, nil
}

// NewEncryptorFromBase64 creates a new Encryptor from a base64-encoded key.
func NewEncryptorFromBase64(encodedKey string) (*Encryptor, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 key: %w", err)
	}
	return NewEncryptor(key)
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns base64-encoded ciphertext (nonce prepended).
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64-encoded result
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM.
func (e *Encryptor) Decrypt(encodedCiphertext string) (string, error) {
	if encodedCiphertext == "" {
		return "", nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", ErrCiphertextTooShort
	}

	// Extract nonce from beginning of ciphertext
	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// GenerateKey generates a new random 32-byte key for AES-256.
// Returns the key as a base64-encoded string.
func GenerateKey() (string, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// GenerateKeyBytes generates a new random 32-byte key for AES-256.
func GenerateKeyBytes() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}
