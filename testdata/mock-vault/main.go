package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// Cipher represents a secret in the mock vault
type Cipher struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Name  string `json:"name"`
	Login Login  `json:"login"`
}

// Login represents login details
type Login struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CiphersResponse wraps the cipher list
type CiphersResponse struct {
	Data []Cipher `json:"data"`
}

// MockVault stores secrets in memory
type MockVault struct {
	mu      sync.RWMutex
	ciphers map[string]Cipher
	token   string
}

// NewMockVault creates a new in-memory vault
func NewMockVault(token string) *MockVault {
	return &MockVault{
		ciphers: make(map[string]Cipher),
		token:   token,
	}
}

// ValidateAuth checks if the request has valid authentication
func (v *MockVault) ValidateAuth(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	return auth == "Bearer "+v.token
}

// ListCiphers returns all ciphers
func (v *MockVault) ListCiphers(w http.ResponseWriter, r *http.Request) {
	if !v.ValidateAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	var ciphers []Cipher
	for _, cipher := range v.ciphers {
		ciphers = append(ciphers, cipher)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":            ciphers,
		"continuationToken": nil,
	})
}

// CreateCipher creates a new cipher
func (v *MockVault) CreateCipher(w http.ResponseWriter, r *http.Request) {
	if !v.ValidateAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var cipher Cipher
	if err := json.Unmarshal(body, &cipher); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Set type to 1 (login cipher) if not provided
	if cipher.Type == 0 {
		cipher.Type = 1
	}

	// Generate simple ID
	cipher.ID = fmt.Sprintf("cipher_%d", time.Now().UnixNano())

	v.mu.Lock()
	v.ciphers[cipher.ID] = cipher
	v.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cipher)
}

// GetCipher retrieves a specific cipher
func (v *MockVault) GetCipher(w http.ResponseWriter, r *http.Request, id string) {
	if !v.ValidateAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	v.mu.RLock()
	cipher, exists := v.ciphers[id]
	v.mu.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cipher)
}

// Health check endpoint
func (v *MockVault) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// ListCiphersHandler handles GET /api/ciphers
func (v *MockVault) ListCiphersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		v.ListCiphers(w, r)
	} else if r.Method == http.MethodPost {
		v.CreateCipher(w, r)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func main() {
	sourceToken := flag.String("source-token", "source_admin_token_12345", "Source vault token")
	targetToken := flag.String("target-token", "target_admin_token_12345", "Target vault token")
	sourcePort := flag.String("source-port", "8000", "Source vault port")
	targetPort := flag.String("target-port", "8001", "Target vault port")
	flag.Parse()

	sourceVault := NewMockVault(*sourceToken)
	targetVault := NewMockVault(*targetToken)

	// Setup source vault routes
	sourceMux := http.NewServeMux()
	sourceMux.HandleFunc("/alive", sourceVault.Health)
	sourceMux.HandleFunc("/api/ciphers", sourceVault.ListCiphersHandler)

	// Setup target vault routes
	targetMux := http.NewServeMux()
	targetMux.HandleFunc("/alive", targetVault.Health)
	targetMux.HandleFunc("/api/ciphers", targetVault.ListCiphersHandler)

	// Start servers
	go func() {
		log.Printf("Source vault listening on :%s", *sourcePort)
		if err := http.ListenAndServe(":"+*sourcePort, sourceMux); err != nil {
			log.Fatalf("Source vault error: %v", err)
		}
	}()

	go func() {
		log.Printf("Target vault listening on :%s", *targetPort)
		if err := http.ListenAndServe(":"+*targetPort, targetMux); err != nil {
			log.Fatalf("Target vault error: %v", err)
		}
	}()

	// Keep running
	select {}
}
