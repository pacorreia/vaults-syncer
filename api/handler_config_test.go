package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pacorreia/vaults-syncer/auth"
	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
)

func TestListVaultsConfig_Empty(t *testing.T) {
	store := openTestStore(t)
	h := NewConfigHandler(store, testLogger(t), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config/vaults", nil)
	w := httptest.NewRecorder()
	h.ListVaultsConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	vaults, ok := resp["vaults"]
	if !ok {
		t.Fatal("expected 'vaults' key in response")
	}
	if vaults == nil {
		t.Error("expected non-nil vaults")
	}
}

func TestCreateAndGetVault(t *testing.T) {
	store := openTestStore(t)
	reloadCalled := false
	h := NewConfigHandler(store, testLogger(t), func() { reloadCalled = true })

	v := config.VaultConfig{
		ID:       "vault1",
		Name:     "Test Vault",
		Type:     "generic",
		Endpoint: "http://vault.example.com",
	}
	body, _ := json.Marshal(v)

	// Create.
	req := httptest.NewRequest(http.MethodPost, "/api/config/vaults", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateVault(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Reload should be triggered.
	for i := 0; i < 10 && !reloadCalled; i++ {
	}

	// Get.
	req = httptest.NewRequest(http.MethodGet, "/api/config/vaults/vault1", nil)
	req = setPathValue(req, "vault_id", "vault1")
	w = httptest.NewRecorder()
	h.GetVaultConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateVault_Duplicate(t *testing.T) {
	store := openTestStore(t)
	h := NewConfigHandler(store, testLogger(t), nil)

	v := config.VaultConfig{ID: "vault1", Type: "generic"}
	store.SaveVault(v)

	body, _ := json.Marshal(v)
	req := httptest.NewRequest(http.MethodPost, "/api/config/vaults", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateVault(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestDeleteVaultConfig(t *testing.T) {
	store := openTestStore(t)
	h := NewConfigHandler(store, testLogger(t), nil)

	store.SaveVault(config.VaultConfig{ID: "v1", Type: "generic"})

	req := httptest.NewRequest(http.MethodDelete, "/api/config/vaults/v1", nil)
	req = setPathValue(req, "vault_id", "v1")
	w := httptest.NewRecorder()
	h.DeleteVaultConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	v, _ := store.GetVault("v1")
	if v != nil {
		t.Error("expected vault to be deleted")
	}
}

func TestGetVaultConfig_NotFound(t *testing.T) {
	store := openTestStore(t)
	h := NewConfigHandler(store, testLogger(t), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config/vaults/nope", nil)
	req = setPathValue(req, "vault_id", "nope")
	w := httptest.NewRecorder()
	h.GetVaultConfig(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateAndListSyncsConfig(t *testing.T) {
	store := openTestStore(t)
	h := NewConfigHandler(store, testLogger(t), nil)

	sc := config.SyncConfig{ID: "s1", Source: "v1", Targets: []string{"v2"}}
	body, _ := json.Marshal(sc)

	req := httptest.NewRequest(http.MethodPost, "/api/config/syncs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateSync(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/config/syncs", nil)
	w = httptest.NewRecorder()
	h.ListSyncsConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUpdateSync(t *testing.T) {
	store := openTestStore(t)
	h := NewConfigHandler(store, testLogger(t), nil)

	store.SaveSync(config.SyncConfig{ID: "s1", Source: "v1", Targets: []string{"v2"}})

	updated := config.SyncConfig{ID: "s1", Source: "v1", Targets: []string{"v2", "v3"}}
	body, _ := json.Marshal(updated)

	req := httptest.NewRequest(http.MethodPut, "/api/config/syncs/s1", bytes.NewReader(body))
	req = setPathValue(req, "sync_id", "s1")
	w := httptest.NewRecorder()
	h.UpdateSync(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDeleteSyncConfig(t *testing.T) {
	store := openTestStore(t)
	h := NewConfigHandler(store, testLogger(t), nil)

	store.SaveSync(config.SyncConfig{ID: "s1", Source: "v1", Targets: []string{"v2"}})

	req := httptest.NewRequest(http.MethodDelete, "/api/config/syncs/s1", nil)
	req = setPathValue(req, "sync_id", "s1")
	w := httptest.NewRecorder()
	h.DeleteSyncConfig(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// setPathValue sets a path value on a request using the 1.22+ PathValue mechanism.
func setPathValue(r *http.Request, key, value string) *http.Request {
	// ServeMux populates path values when routing; for tests we need to set them manually.
	// Go 1.22 pattern variables are accessible via r.PathValue - we simulate this by
	// re-routing through a minimal mux.
	mux := http.NewServeMux()
	pattern := "/api/config/vaults/{vault_id}"
	switch key {
	case "sync_id":
		pattern = "/api/config/syncs/{sync_id}"
	case "vault_id":
		pattern = "/api/config/vaults/{vault_id}"
	case "user_id":
		pattern = "/api/users/{user_id}"
	}

	var captured *http.Request
	mux.HandleFunc(pattern, func(_ http.ResponseWriter, req *http.Request) {
		captured = req
	})

	// Build a fake URL that matches the pattern.
	fakeURL := ""
	switch key {
	case "sync_id":
		fakeURL = "/api/config/syncs/" + value
	case "vault_id":
		fakeURL = "/api/config/vaults/" + value
	case "user_id":
		fakeURL = "/api/users/" + value
	}

	fakeReq := httptest.NewRequest(r.Method, fakeURL, r.Body)
	fakeW := httptest.NewRecorder()
	mux.ServeHTTP(fakeW, fakeReq)

	if captured == nil {
		// Fallback for unrecognised keys.
		return r
	}
	// Transfer original headers.
	for k, v := range r.Header {
		captured.Header[k] = v
	}
	return captured.WithContext(r.Context())
}

func TestUsersHandler_ListAndCreate(t *testing.T) {
	store := openTestStore(t)
	svc := auth.NewService(store)
	_ = svc.SetupAdmin("admin", "adminpass1")
	h := NewUsersHandler(svc, testLogger(t))

	// List.
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	h.ListUsers(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	// Create.
	body, _ := json.Marshal(map[string]string{
		"username": "newuser",
		"password": "secure123",
		"role":     storage.RoleUser,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	w = httptest.NewRecorder()
	h.CreateUser(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
}
