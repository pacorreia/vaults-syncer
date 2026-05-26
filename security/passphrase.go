package security

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const (
	// pbkdf2Iterations is the number of PBKDF2 iterations for key derivation.
	// 600 000 is the OWASP-recommended minimum for SHA-256 as of 2023.
	pbkdf2Iterations = 600_000
	// DerivedKeyLen is the length of the AES-256 key derived by DeriveKey.
	DerivedKeyLen = 32
	// SaltLen is the byte length of a newly generated salt.
	SaltLen = 32
)

// GenerateRandomBytes returns n cryptographically random bytes.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("security: failed to generate random bytes: %w", err)
	}
	return b, nil
}

// GenerateMasterKey produces a new random 32-byte master key and returns it
// as a URL-safe base64 string suitable for storing in an environment variable.
func GenerateMasterKey() (string, error) {
	b, err := GenerateRandomBytes(DerivedKeyLen)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// MasterKeyFromString decodes a base64-encoded master key produced by GenerateMasterKey.
func MasterKeyFromString(encoded string) ([]byte, error) {
	b, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("security: invalid master key encoding: %w", err)
	}
	if len(b) != DerivedKeyLen {
		return nil, fmt.Errorf("security: master key must be %d bytes, got %d", DerivedKeyLen, len(b))
	}
	return b, nil
}

// GenerateSalt returns a new random salt of SaltLen bytes encoded as base64.
func GenerateSalt() (string, error) {
	b, err := GenerateRandomBytes(SaltLen)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DeriveKey derives a 32-byte AES key from the given passphrase and base64-encoded salt
// using PBKDF2-SHA256.
func DeriveKey(passphrase, saltBase64 string) ([]byte, error) {
	salt, err := base64.StdEncoding.DecodeString(saltBase64)
	if err != nil {
		return nil, fmt.Errorf("security: invalid salt encoding: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, salt, pbkdf2Iterations, DerivedKeyLen)
	if err != nil {
		return nil, fmt.Errorf("security: PBKDF2 key derivation failed: %w", err)
	}
	return key, nil
}
