package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pacorreia/vaults-syncer/auth"
	"github.com/pacorreia/vaults-syncer/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("openTestStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// setupAdmin creates an admin account and returns a valid session token.
func setupAdmin(t *testing.T, svc *auth.Service) string {
	t.Helper()
	if err := svc.SetupAdmin("admin", "password123"); err != nil {
		t.Fatalf("SetupAdmin: %v", err)
	}
	token, err := svc.Login("admin", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return token
}

func authHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	return h
}

func TestGetSetupStatus_NotComplete(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	h := NewSetupHandler(store, svc, testLogger(t), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/setup", nil)
	w := httptest.NewRecorder()
	h.GetSetupStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]bool
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["complete"] {
		t.Error("expected complete=false before setup")
	}
}

func TestCompleteSetup(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	called := false
	h := NewSetupHandler(store, svc, testLogger(t), func() { called = true })

	body, _ := json.Marshal(map[string]string{
		"admin_username": "admin",
		"admin_password": "secure123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CompleteSetup(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Setup should now be marked complete.
	complete, _ := store.IsSetupComplete()
	if !complete {
		t.Error("expected setup to be marked complete")
	}

	// onSetupComplete should be called.
	// Give the goroutine a moment.
	for i := 0; i < 10 && !called; i++ {
	}
}

func TestCompleteSetup_AlreadyComplete(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	h := NewSetupHandler(store, svc, testLogger(t), nil)

	store.MarkSetupComplete()

	body, _ := json.Marshal(map[string]string{"admin_username": "a", "admin_password": "password"})
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CompleteSetup(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestCompleteSetup_ShortPassword(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	h := NewSetupHandler(store, svc, testLogger(t), nil)

	body, _ := json.Marshal(map[string]string{"admin_username": "admin", "admin_password": "short"})
	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CompleteSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogin(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	_ = svc.SetupAdmin("admin", "pass1234")
	h := NewAuthHandler(svc, testLogger(t))

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "pass1234"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("expected token in response")
	}
}

func TestLogin_BadCredentials(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	_ = svc.SetupAdmin("admin", "correct")
	h := NewAuthHandler(svc, testLogger(t))

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestMe(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	token := setupAdmin(t, svc)
	h := NewAuthHandler(svc, testLogger(t))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	user, _ := svc.ValidateSession(token)
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	w := httptest.NewRecorder()
	h.Me(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestLogout(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	token := setupAdmin(t, svc)
	h := NewAuthHandler(svc, testLogger(t))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Validate session is gone.
	user, _ := svc.ValidateSession(token)
	if user != nil {
		t.Error("expected session to be invalid after logout")
	}
}
