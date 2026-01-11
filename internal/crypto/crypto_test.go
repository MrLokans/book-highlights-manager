package crypto

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEncryptor(t *testing.T) {
	t.Run("valid key size", func(t *testing.T) {
		key := make([]byte, 32)
		enc, err := NewEncryptor(key)
		require.NoError(t, err)
		assert.NotNil(t, enc)
	})

	t.Run("invalid key size - too short", func(t *testing.T) {
		key := make([]byte, 16)
		enc, err := NewEncryptor(key)
		assert.ErrorIs(t, err, ErrInvalidKeySize)
		assert.Nil(t, enc)
	})

	t.Run("invalid key size - too long", func(t *testing.T) {
		key := make([]byte, 64)
		enc, err := NewEncryptor(key)
		assert.ErrorIs(t, err, ErrInvalidKeySize)
		assert.Nil(t, enc)
	})
}

func TestNewEncryptorFromBase64(t *testing.T) {
	t.Run("valid base64 key", func(t *testing.T) {
		key := make([]byte, 32)
		encoded := base64.StdEncoding.EncodeToString(key)
		enc, err := NewEncryptorFromBase64(encoded)
		require.NoError(t, err)
		assert.NotNil(t, enc)
	})

	t.Run("invalid base64", func(t *testing.T) {
		enc, err := NewEncryptorFromBase64("not-valid-base64!!!")
		assert.Error(t, err)
		assert.Nil(t, enc)
	})

	t.Run("valid base64 but wrong size", func(t *testing.T) {
		key := make([]byte, 16)
		encoded := base64.StdEncoding.EncodeToString(key)
		enc, err := NewEncryptorFromBase64(encoded)
		assert.ErrorIs(t, err, ErrInvalidKeySize)
		assert.Nil(t, enc)
	})
}

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateKeyBytes()
	require.NoError(t, err)

	enc, err := NewEncryptor(key)
	require.NoError(t, err)

	t.Run("encrypt and decrypt plaintext", func(t *testing.T) {
		plaintext := "my-secret-token-12345"
		ciphertext, err := enc.Encrypt(plaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, ciphertext)
		assert.NotEqual(t, plaintext, ciphertext)

		decrypted, err := enc.Decrypt(ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("empty string", func(t *testing.T) {
		ciphertext, err := enc.Encrypt("")
		require.NoError(t, err)
		assert.Empty(t, ciphertext)

		decrypted, err := enc.Decrypt("")
		require.NoError(t, err)
		assert.Empty(t, decrypted)
	})

	t.Run("long plaintext", func(t *testing.T) {
		plaintext := "sl.A0b1C2d3E4f5G6h7I8j9K0l1M2n3O4p5Q6r7S8t9U0v1W2x3Y4z5A6b7C8d9E0f1G2h3I4j5K6l7M8n9O0p1Q2r3S4t5U6v7W8x9Y0z"
		ciphertext, err := enc.Encrypt(plaintext)
		require.NoError(t, err)

		decrypted, err := enc.Decrypt(ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("unicode text", func(t *testing.T) {
		plaintext := "🔐 Тест Unicode текст 日本語"
		ciphertext, err := enc.Encrypt(plaintext)
		require.NoError(t, err)

		decrypted, err := enc.Decrypt(ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("unique ciphertexts for same plaintext", func(t *testing.T) {
		plaintext := "same-text"
		ciphertext1, err := enc.Encrypt(plaintext)
		require.NoError(t, err)

		ciphertext2, err := enc.Encrypt(plaintext)
		require.NoError(t, err)

		// Due to random nonce, ciphertexts should be different
		assert.NotEqual(t, ciphertext1, ciphertext2)

		// But both should decrypt to same plaintext
		decrypted1, err := enc.Decrypt(ciphertext1)
		require.NoError(t, err)
		decrypted2, err := enc.Decrypt(ciphertext2)
		require.NoError(t, err)
		assert.Equal(t, decrypted1, decrypted2)
	})
}

func TestDecryptErrors(t *testing.T) {
	key, err := GenerateKeyBytes()
	require.NoError(t, err)
	enc, err := NewEncryptor(key)
	require.NoError(t, err)

	t.Run("invalid base64", func(t *testing.T) {
		_, err := enc.Decrypt("not-valid-base64!!!")
		assert.Error(t, err)
	})

	t.Run("ciphertext too short", func(t *testing.T) {
		// Less than nonce size (12 bytes)
		shortData := base64.StdEncoding.EncodeToString([]byte("short"))
		_, err := enc.Decrypt(shortData)
		assert.ErrorIs(t, err, ErrCiphertextTooShort)
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		plaintext := "secret"
		ciphertext, err := enc.Encrypt(plaintext)
		require.NoError(t, err)

		// Decode, tamper, re-encode
		data, _ := base64.StdEncoding.DecodeString(ciphertext)
		data[len(data)-1] ^= 0xFF // Flip bits in last byte
		tampered := base64.StdEncoding.EncodeToString(data)

		_, err = enc.Decrypt(tampered)
		assert.ErrorIs(t, err, ErrDecryptionFailed)
	})

	t.Run("wrong key", func(t *testing.T) {
		plaintext := "secret"
		ciphertext, err := enc.Encrypt(plaintext)
		require.NoError(t, err)

		// Create encryptor with different key
		otherKey, _ := GenerateKeyBytes()
		otherEnc, _ := NewEncryptor(otherKey)

		_, err = otherEnc.Decrypt(ciphertext)
		assert.ErrorIs(t, err, ErrDecryptionFailed)
	})
}

func TestGenerateKey(t *testing.T) {
	t.Run("generates valid key", func(t *testing.T) {
		encodedKey, err := GenerateKey()
		require.NoError(t, err)
		assert.NotEmpty(t, encodedKey)

		// Should be usable
		enc, err := NewEncryptorFromBase64(encodedKey)
		require.NoError(t, err)
		assert.NotNil(t, enc)
	})

	t.Run("generates unique keys", func(t *testing.T) {
		key1, _ := GenerateKey()
		key2, _ := GenerateKey()
		assert.NotEqual(t, key1, key2)
	})
}

func TestGenerateKeyBytes(t *testing.T) {
	t.Run("generates valid key bytes", func(t *testing.T) {
		key, err := GenerateKeyBytes()
		require.NoError(t, err)
		assert.Len(t, key, KeySize)
	})

	t.Run("generates unique keys", func(t *testing.T) {
		key1, _ := GenerateKeyBytes()
		key2, _ := GenerateKeyBytes()
		assert.NotEqual(t, key1, key2)
	})
}
