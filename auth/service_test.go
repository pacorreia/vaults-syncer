package auth

import (
	"errors"
	"testing"

	"github.com/pacorreia/vaults-syncer/storage"
)

// openTestStore opens an in-memory SQLite store for unit testing.
func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("openTestStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSetupAdmin(t *testing.T) {
	svc := NewService(openTestStore(t))

	if err := svc.SetupAdmin("admin", "password123"); err != nil {
		t.Fatalf("SetupAdmin: %v", err)
	}

	// Duplicate should fail.
	err := svc.SetupAdmin("admin", "other")
	if !errors.Is(err, ErrUserExists) {
		t.Errorf("expected ErrUserExists, got %v", err)
	}
}

func TestLoginLogout(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "secret")

	token, err := svc.Login("admin", "secret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	user, err := svc.ValidateSession(token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if user == nil || user.Username != "admin" {
		t.Errorf("expected user 'admin', got %v", user)
	}

	if err := svc.Logout(token); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	user, err = svc.ValidateSession(token)
	if err != nil {
		t.Fatalf("ValidateSession after logout: %v", err)
	}
	if user != nil {
		t.Error("expected nil user after logout")
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "secret")

	_, err := svc.Login("admin", "wrongpassword")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	_, err = svc.Login("nobody", "secret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestDeleteLastAdminBlocked(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "secret")

	u, _ := svc.store.GetUserByUsername("admin")
	err := svc.DeleteUser(u.ID)
	if !errors.Is(err, ErrLastAdmin) {
		t.Errorf("expected ErrLastAdmin, got %v", err)
	}
}

func TestDeleteUserWithSecondAdmin(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "secret")
	_, _ = svc.CreateUser("admin2", "pass", storage.RoleAdmin)

	u, _ := svc.store.GetUserByUsername("admin")
	if err := svc.DeleteUser(u.ID); err != nil {
		t.Errorf("DeleteUser: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "old")

	u, _ := svc.store.GetUserByUsername("admin")
	if err := svc.ChangePassword(u.ID, "new"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	// Old password no longer works.
	_, err := svc.Login("admin", "old")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials with old password, got %v", err)
	}

	// New password works.
	if _, err := svc.Login("admin", "new"); err != nil {
		t.Errorf("Login with new password: %v", err)
	}
}

func TestListUsers(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "pass")
	_, _ = svc.CreateUser("user1", "pass", storage.RoleUser)

	users, err := svc.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
	for _, u := range users {
		if u.PasswordHash != "" {
			t.Errorf("password hash should not be exposed, user %q", u.Username)
		}
	}
}
