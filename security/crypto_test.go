package security

import (
	"strings"
	"testing"
)

func TestAESEncryptorRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	enc, err := NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("NewAESEncryptor: %v", err)
	}

	plaintext := "hello, world! 🔐"
	ciphertext, err := enc.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ciphertext == plaintext {
		t.Error("ciphertext should not equal plaintext")
	}

	decrypted, err := enc.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("got %q, want %q", decrypted, plaintext)
	}
}

func TestAESEncryptorUniqueNonces(t *testing.T) {
	key := make([]byte, 32)
	enc, err := NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("NewAESEncryptor: %v", err)
	}

	c1, _ := enc.EncryptString("same")
	c2, _ := enc.EncryptString("same")
	if c1 == c2 {
		t.Error("two encryptions of the same plaintext produced the same ciphertext (nonce reuse)")
	}
}

func TestAESEncryptorBadKey(t *testing.T) {
	_, err := NewAESEncryptor([]byte("tooshort"))
	if err == nil {
		t.Error("expected error for short key, got nil")
	}
}

func TestAESEncryptorWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	enc1, _ := NewAESEncryptor(key1)

	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = 0xFF
	}
	enc2, _ := NewAESEncryptor(key2)

	ct, _ := enc1.EncryptString("secret")
	_, err := enc2.DecryptString(ct)
	if err == nil {
		t.Error("expected decryption error with wrong key")
	}
}

func TestAESEncryptorTamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewAESEncryptor(key)

	ct, _ := enc.EncryptString("secret")
	// Corrupt the last byte.
	b := []byte(ct)
	b[len(b)-1] ^= 0xFF
	_, err := enc.DecryptString(string(b))
	if err == nil {
		t.Error("expected decryption error for tampered ciphertext")
	}
}

func TestAESEncryptorEmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewAESEncryptor(key)

	ct, err := enc.EncryptString("")
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	got, err := enc.DecryptString(ct)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestGenerateMasterKey(t *testing.T) {
	key1, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}
	key2, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}
	if key1 == key2 {
		t.Error("two generated keys should not be equal")
	}
	// Keys must decode to 32 bytes.
	b, err := MasterKeyFromString(key1)
	if err != nil {
		t.Fatalf("MasterKeyFromString: %v", err)
	}
	if len(b) != 32 {
		t.Errorf("expected 32-byte key, got %d", len(b))
	}
}

func TestDeriveKey(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt: %v", err)
	}

	k1, err := DeriveKey("passphrase", salt)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	k2, err := DeriveKey("passphrase", salt)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	// Same passphrase + salt → same key.
	if string(k1) != string(k2) {
		t.Error("DeriveKey is not deterministic")
	}

	k3, err := DeriveKey("different", salt)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	if string(k1) == string(k3) {
		t.Error("different passphrases should produce different keys")
	}
}

func TestDeriveKeyBadSalt(t *testing.T) {
	_, err := DeriveKey("passphrase", "not-base64!!!")
	if err == nil {
		t.Error("expected error for invalid salt")
	}
	if !strings.Contains(err.Error(), "invalid salt") {
		t.Errorf("unexpected error: %v", err)
	}
}
