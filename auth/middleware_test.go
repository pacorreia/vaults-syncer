package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuth_NoBearerToken(t *testing.T) {
	svc := NewService(openTestStore(t))
	handler := RequireAuth(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "pass")
	token, _ := svc.Login("admin", "pass")

	reached := false
	handler := RequireAuth(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		user := UserFromContext(r.Context())
		if user == nil {
			t.Error("expected user in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !reached {
		t.Error("handler was not called")
	}
}

func TestRequireAuth_Cookie(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "pass")
	token, _ := svc.Login("admin", "pass")

	handler := RequireAuth(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireAdmin_NonAdmin(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "pass")
	_, _ = svc.CreateUser("user", "pass", "user")
	token, _ := svc.Login("user", "pass")

	handler := RequireAuth(svc)(RequireAdmin(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireAdmin_Admin(t *testing.T) {
	svc := NewService(openTestStore(t))
	_ = svc.SetupAdmin("admin", "pass")
	token, _ := svc.Login("admin", "pass")

	handler := RequireAuth(svc)(RequireAdmin(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
