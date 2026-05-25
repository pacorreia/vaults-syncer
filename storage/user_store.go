package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// User roles.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User represents an application user.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    int64
	UpdatedAt    int64
}

// Session represents an active user session.
type Session struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt int64
	CreatedAt int64
}

// UserStore handles user and session persistence.
type UserStore struct {
	db     *sql.DB
	dbType DBType
}

// CreateUser inserts a new user. Returns an error if the username already exists.
func (s *UserStore) CreateUser(username, passwordHash, role string) (*User, error) {
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, role, created_at, updated_at) VALUES (?,?,?,?,?)`,
		username, passwordHash, role, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("storage: CreateUser: %w", err)
	}
	return s.GetUserByUsername(username)
}

// GetUserByUsername looks up a user by their username.
func (s *UserStore) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at, updated_at FROM users WHERE username=?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: GetUserByUsername: %w", err)
	}
	return u, nil
}

// GetUserByID looks up a user by ID.
func (s *UserStore) GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, created_at, updated_at FROM users WHERE id=?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: GetUserByID: %w", err)
	}
	return u, nil
}

// ListUsers returns all users (without password hashes).
func (s *UserStore) ListUsers() ([]*User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, role, created_at, updated_at FROM users ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("storage: ListUsers: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateUserPassword updates a user's password hash.
func (s *UserStore) UpdateUserPassword(userID int64, passwordHash string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`UPDATE users SET password_hash=?, updated_at=? WHERE id=?`,
		passwordHash, now, userID,
	)
	if err != nil {
		return fmt.Errorf("storage: UpdateUserPassword: %w", err)
	}
	return nil
}

// UpdateUserRole updates a user's role.
func (s *UserStore) UpdateUserRole(userID int64, role string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`UPDATE users SET role=?, updated_at=? WHERE id=?`,
		role, now, userID,
	)
	if err != nil {
		return fmt.Errorf("storage: UpdateUserRole: %w", err)
	}
	return nil
}

// DeleteUser removes a user and their sessions.
func (s *UserStore) DeleteUser(userID int64) error {
	// Sessions reference users; delete sessions first for DBs without CASCADE.
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE user_id=?`, userID); err != nil {
		return fmt.Errorf("storage: DeleteUser sessions: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM users WHERE id=?`, userID); err != nil {
		return fmt.Errorf("storage: DeleteUser: %w", err)
	}
	return nil
}

// HasUsers reports whether any users exist in the database.
func (s *UserStore) HasUsers() (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, fmt.Errorf("storage: HasUsers: %w", err)
	}
	return count > 0, nil
}

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

// CreateSession persists a new session token for the given user.
func (s *UserStore) CreateSession(userID int64, token string, expiresAt int64) (*Session, error) {
	now := time.Now().Unix()
	_, err := s.db.Exec(
		`INSERT INTO sessions (user_id, token, expires_at, created_at) VALUES (?,?,?,?)`,
		userID, token, expiresAt, now,
	)
	if err != nil {
		return nil, fmt.Errorf("storage: CreateSession: %w", err)
	}
	return &Session{UserID: userID, Token: token, ExpiresAt: expiresAt, CreatedAt: now}, nil
}

// GetSessionByToken retrieves a non-expired session by token.
func (s *UserStore) GetSessionByToken(token string) (*Session, error) {
	now := time.Now().Unix()
	sess := &Session{}
	err := s.db.QueryRow(
		`SELECT id, user_id, token, expires_at, created_at FROM sessions WHERE token=? AND expires_at>?`,
		token, now,
	).Scan(&sess.ID, &sess.UserID, &sess.Token, &sess.ExpiresAt, &sess.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: GetSessionByToken: %w", err)
	}
	return sess, nil
}

// DeleteSession removes a session by token (logout).
func (s *UserStore) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token=?`, token)
	if err != nil {
		return fmt.Errorf("storage: DeleteSession: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes all sessions that have passed their expiry time.
func (s *UserStore) DeleteExpiredSessions() error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at<=?`, now)
	return err
}
