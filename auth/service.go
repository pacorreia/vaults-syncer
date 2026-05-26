// Package auth provides authentication and session management for vaults-syncer.
// It wraps the storage layer with bcrypt-hashed passwords and random session tokens,
// and exposes HTTP middleware that enforces authentication on protected endpoints.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/pacorreia/vaults-syncer/storage"
)

const (
	// bcryptCost is the bcrypt work factor. 12 is the recommended minimum for 2024.
	bcryptCost = 12
	// sessionTTL is how long a session token remains valid.
	sessionTTL = 24 * time.Hour
	// tokenBytes is the length of the raw random session token.
	tokenBytes = 32
)

// ErrInvalidCredentials is returned when the username or password is wrong.
var ErrInvalidCredentials = errors.New("auth: invalid username or password")

// ErrUserExists is returned when a username is already taken.
var ErrUserExists = errors.New("auth: username already exists")

// ErrLastAdmin is returned when the last admin account cannot be deleted.
var ErrLastAdmin = errors.New("auth: cannot remove the last admin account")

// Service provides user authentication and session management.
type Service struct {
	store *storage.Store
}

// NewService creates an authentication service backed by store.
func NewService(store *storage.Store) *Service {
	return &Service{store: store}
}

// SetupAdmin creates the initial admin account. It returns ErrUserExists if an
// admin account has already been created.
func (s *Service) SetupAdmin(username, password string) error {
	existing, err := s.store.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("auth: SetupAdmin lookup: %w", err)
	}
	if existing != nil {
		return ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("auth: SetupAdmin hash: %w", err)
	}

	if _, err := s.store.CreateUser(username, string(hash), storage.RoleAdmin); err != nil {
		return fmt.Errorf("auth: SetupAdmin create: %w", err)
	}
	return nil
}

// Login verifies credentials and returns a new session token on success.
func (s *Service) Login(username, password string) (string, error) {
	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("auth: Login: %w", err)
	}
	if user == nil {
		return "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("auth: Login generate token: %w", err)
	}

	expiresAt := time.Now().Add(sessionTTL).Unix()
	if _, err := s.store.CreateSession(user.ID, token, expiresAt); err != nil {
		return "", fmt.Errorf("auth: Login create session: %w", err)
	}

	return token, nil
}

// ValidateSession returns the user associated with the token, or nil if the
// session is expired or does not exist.
func (s *Service) ValidateSession(token string) (*storage.User, error) {
	sess, err := s.store.GetSessionByToken(token)
	if err != nil {
		return nil, fmt.Errorf("auth: ValidateSession: %w", err)
	}
	if sess == nil {
		return nil, nil
	}

	user, err := s.store.GetUserByID(sess.UserID)
	if err != nil {
		return nil, fmt.Errorf("auth: ValidateSession user lookup: %w", err)
	}
	return user, nil
}

// Logout invalidates the session associated with token.
func (s *Service) Logout(token string) error {
	if err := s.store.DeleteSession(token); err != nil {
		return fmt.Errorf("auth: Logout: %w", err)
	}
	return nil
}

// CreateUser creates a new user account (admin-only operation enforced at the
// handler level).
func (s *Service) CreateUser(username, password, role string) (*storage.User, error) {
	existing, err := s.store.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("auth: CreateUser: %w", err)
	}
	if existing != nil {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("auth: CreateUser hash: %w", err)
	}

	u, err := s.store.CreateUser(username, string(hash), role)
	if err != nil {
		return nil, fmt.Errorf("auth: CreateUser store: %w", err)
	}
	u.PasswordHash = "" // never expose hashes
	return u, nil
}

// ChangePassword updates the password for the given user.
func (s *Service) ChangePassword(userID int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("auth: ChangePassword hash: %w", err)
	}
	return s.store.UpdateUserPassword(userID, string(hash))
}

// ChangeRole updates the role of a user.
func (s *Service) ChangeRole(userID int64, role string) error {
	return s.store.UpdateUserRole(userID, role)
}

// DeleteUser removes a user, preventing deletion of the last admin.
func (s *Service) DeleteUser(userID int64) error {
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("auth: DeleteUser: %w", err)
	}
	if user == nil {
		return nil // already gone
	}
	if user.Role == storage.RoleAdmin {
		// Prevent removing the last admin.
		users, err := s.store.ListUsers()
		if err != nil {
			return fmt.Errorf("auth: DeleteUser list: %w", err)
		}
		adminCount := 0
		for _, u := range users {
			if u.Role == storage.RoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return ErrLastAdmin
		}
	}
	return s.store.DeleteUser(userID)
}

// ListUsers returns all users with password hashes omitted.
func (s *Service) ListUsers() ([]*storage.User, error) {
	users, err := s.store.ListUsers()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		u.PasswordHash = ""
	}
	return users, nil
}

// generateToken returns a cryptographically random hex token.
func generateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generateToken: %w", err)
	}
	return hex.EncodeToString(b), nil
}
