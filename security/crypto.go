// Package security provides encryption and key-management utilities for vaults-syncer.
// All sensitive configuration data (vault credentials, auth tokens, etc.) stored in the
// database is encrypted with AES-256-GCM using a key derived from the master passphrase.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Encryptor encrypts and decrypts arbitrary byte slices and strings.
type Encryptor interface {
	// Encrypt returns a base64-encoded ciphertext that can be stored safely.
	Encrypt(plaintext []byte) (string, error)
	// Decrypt decodes and decrypts a ciphertext produced by Encrypt.
	Decrypt(ciphertext string) ([]byte, error)
	// EncryptString is a convenience wrapper for encrypting a UTF-8 string.
	EncryptString(plaintext string) (string, error)
	// DecryptString is a convenience wrapper for decrypting to a UTF-8 string.
	DecryptString(ciphertext string) (string, error)
}

// AESEncryptor implements Encryptor using AES-256-GCM.
// The key must be exactly 32 bytes.
type AESEncryptor struct {
	key []byte
}

// NewAESEncryptor creates a new AESEncryptor with the given 32-byte key.
// It returns an error if the key length is not 32 bytes.
func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("security: AES-256 key must be 32 bytes, got %d", len(key))
	}
	// Copy to avoid external mutation.
	k := make([]byte, 32)
	copy(k, key)
	return &AESEncryptor{key: k}, nil
}

// Encrypt encrypts plaintext with AES-256-GCM and returns a base64-encoded string
// that contains the nonce prepended to the ciphertext.
func (e *AESEncryptor) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("security: failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("security: failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("security: failed to generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce.
	sealed := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decodes and decrypts a base64-encoded ciphertext produced by Encrypt.
func (e *AESEncryptor) Decrypt(ciphertext string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("security: failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("security: failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("security: failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("security: ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("security: decryption failed (wrong key or corrupted data): %w", err)
	}

	return plaintext, nil
}

// EncryptString is a convenience wrapper for encrypting a UTF-8 string.
func (e *AESEncryptor) EncryptString(plaintext string) (string, error) {
	return e.Encrypt([]byte(plaintext))
}

// DecryptString is a convenience wrapper for decrypting to a UTF-8 string.
func (e *AESEncryptor) DecryptString(ciphertext string) (string, error) {
	b, err := e.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
