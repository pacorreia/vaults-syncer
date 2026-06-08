package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
	"github.com/pacorreia/vaults-syncer/vault"
)

// ConfigHandler handles CRUD operations for vaults and sync configurations.
type ConfigHandler struct {
	store  *storage.Store
	logger *slog.Logger
	// onConfigChanged is called after any mutating operation so the caller
	// can reload the active sync configuration.
	onConfigChanged func()
}

// NewConfigHandler creates a ConfigHandler.
func NewConfigHandler(store *storage.Store, logger *slog.Logger, onConfigChanged func()) *ConfigHandler {
	return &ConfigHandler{store: store, logger: logger, onConfigChanged: onConfigChanged}
}

// ---------------------------------------------------------------------------
// Vault handlers
// ---------------------------------------------------------------------------

// ListVaultsConfig lists all stored vault configurations.
func (h *ConfigHandler) ListVaultsConfig(w http.ResponseWriter, r *http.Request) {
	vaults, err := h.store.ListVaults()
	if err != nil {
		h.logger.Error("list vaults failed", slog.String("error", err.Error()))
		jsonError(w, "failed to list vaults", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"vaults": sanitiseVaults(vaults)})
}

// CreateVault stores a new vault configuration.
func (h *ConfigHandler) CreateVault(w http.ResponseWriter, r *http.Request) {
	var v config.VaultConfig
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if v.ID == "" {
		jsonError(w, "vault id is required", http.StatusBadRequest)
		return
	}

	// Check for duplicates.
	existing, err := h.store.GetVault(v.ID)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		jsonError(w, "vault already exists", http.StatusConflict)
		return
	}

	if err := h.store.SaveVault(v); err != nil {
		h.logger.Error("create vault failed", slog.String("vault_id", v.ID), slog.String("error", err.Error()))
		jsonError(w, "failed to save vault", http.StatusInternalServerError)
		return
	}

	h.logger.Info("vault created", slog.String("vault_id", v.ID))
	h.notifyConfigChanged()

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, sanitiseVault(v))
}

// GetVaultConfig retrieves a single vault configuration.
func (h *ConfigHandler) GetVaultConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("vault_id")
	if id == "" {
		jsonError(w, "vault_id is required", http.StatusBadRequest)
		return
	}

	v, err := h.store.GetVault(id)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if v == nil {
		jsonError(w, "vault not found", http.StatusNotFound)
		return
	}
	jsonOK(w, sanitiseVault(*v))
}

// UpdateVault replaces a vault configuration.
func (h *ConfigHandler) UpdateVault(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("vault_id")
	if id == "" {
		jsonError(w, "vault_id is required", http.StatusBadRequest)
		return
	}

	var v config.VaultConfig
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	v.ID = id // ensure the URL id takes precedence

	existing, err := h.store.GetVault(id)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		jsonError(w, "vault not found", http.StatusNotFound)
		return
	}

	if err := h.store.SaveVault(v); err != nil {
		h.logger.Error("update vault failed", slog.String("vault_id", id), slog.String("error", err.Error()))
		jsonError(w, "failed to update vault", http.StatusInternalServerError)
		return
	}

	h.logger.Info("vault updated", slog.String("vault_id", id))
	h.notifyConfigChanged()
	jsonOK(w, sanitiseVault(v))
}

// DeleteVaultConfig removes a vault configuration.
func (h *ConfigHandler) DeleteVaultConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("vault_id")
	if id == "" {
		jsonError(w, "vault_id is required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteVault(id); err != nil {
		h.logger.Error("delete vault failed", slog.String("vault_id", id), slog.String("error", err.Error()))
		jsonError(w, "failed to delete vault", http.StatusInternalServerError)
		return
	}

	h.logger.Info("vault deleted", slog.String("vault_id", id))
	h.notifyConfigChanged()
	jsonOK(w, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Sync config handlers
// ---------------------------------------------------------------------------

// ListSyncsConfig lists all stored sync configurations.
func (h *ConfigHandler) ListSyncsConfig(w http.ResponseWriter, r *http.Request) {
	syncs, err := h.store.ListSyncs()
	if err != nil {
		h.logger.Error("list syncs failed", slog.String("error", err.Error()))
		jsonError(w, "failed to list syncs", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"syncs": syncs})
}

// CreateSync stores a new sync configuration.
func (h *ConfigHandler) CreateSync(w http.ResponseWriter, r *http.Request) {
	var sc config.SyncConfig
	if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if sc.ID == "" {
		jsonError(w, "sync id is required", http.StatusBadRequest)
		return
	}

	existing, err := h.store.GetSync(sc.ID)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		jsonError(w, "sync already exists", http.StatusConflict)
		return
	}

	if err := h.store.SaveSync(sc); err != nil {
		h.logger.Error("create sync failed", slog.String("sync_id", sc.ID), slog.String("error", err.Error()))
		jsonError(w, "failed to save sync", http.StatusInternalServerError)
		return
	}

	h.logger.Info("sync created", slog.String("sync_id", sc.ID))
	h.notifyConfigChanged()

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, sc)
}

// GetSyncConfig retrieves a single sync configuration.
func (h *ConfigHandler) GetSyncConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sync_id")
	if id == "" {
		jsonError(w, "sync_id is required", http.StatusBadRequest)
		return
	}

	sc, err := h.store.GetSync(id)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if sc == nil {
		jsonError(w, "sync not found", http.StatusNotFound)
		return
	}
	jsonOK(w, sc)
}

// UpdateSync replaces a sync configuration.
func (h *ConfigHandler) UpdateSync(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sync_id")
	if id == "" {
		jsonError(w, "sync_id is required", http.StatusBadRequest)
		return
	}

	var sc config.SyncConfig
	if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	sc.ID = id

	existing, err := h.store.GetSync(id)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		jsonError(w, "sync not found", http.StatusNotFound)
		return
	}

	if err := h.store.SaveSync(sc); err != nil {
		h.logger.Error("update sync failed", slog.String("sync_id", id), slog.String("error", err.Error()))
		jsonError(w, "failed to update sync", http.StatusInternalServerError)
		return
	}

	h.logger.Info("sync updated", slog.String("sync_id", id))
	h.notifyConfigChanged()
	jsonOK(w, sc)
}

// DeleteSyncConfig removes a sync configuration.
func (h *ConfigHandler) DeleteSyncConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sync_id")
	if id == "" {
		jsonError(w, "sync_id is required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteSync(id); err != nil {
		h.logger.Error("delete sync failed", slog.String("sync_id", id), slog.String("error", err.Error()))
		jsonError(w, "failed to delete sync", http.StatusInternalServerError)
		return
	}

	h.logger.Info("sync deleted", slog.String("sync_id", id))
	h.notifyConfigChanged()
	jsonOK(w, map[string]string{"status": "deleted"})
}

// TestVaultConnection accepts a transient VaultConfig in the request body and
// verifies connectivity without persisting the configuration.
func (h *ConfigHandler) TestVaultConnection(w http.ResponseWriter, r *http.Request) {
	var v config.VaultConfig
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if v.Endpoint == "" {
		jsonError(w, "endpoint is required", http.StatusBadRequest)
		return
	}

	backend, err := vault.NewBackend(&v)
	if err != nil {
		jsonOK(w, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}

	if err := backend.TestConnection(); err != nil {
		h.logger.Info("vault test connection failed", slog.String("endpoint", v.Endpoint), slog.String("error", err.Error()))
		jsonOK(w, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}

	h.logger.Info("vault test connection succeeded", slog.String("endpoint", v.Endpoint))
	jsonOK(w, map[string]interface{}{"ok": true, "message": "Connection successful"})
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (h *ConfigHandler) notifyConfigChanged() {
	if h.onConfigChanged == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("onConfigChanged panicked", slog.Any("panic", r))
			}
		}()
		h.onConfigChanged()
	}()
}

// sanitiseVault strips sensitive fields before sending vault config to the frontend.
func sanitiseVault(v config.VaultConfig) map[string]interface{} {
	m := map[string]interface{}{
		"id":       v.ID,
		"name":     v.Name,
		"type":     v.Type,
		"endpoint": v.Endpoint,
		"timeout":  v.Timeout,
	}
	if v.Auth != nil {
		// Return auth method without any sensitive header values.
		m["auth"] = map[string]interface{}{
			"method": v.Auth.Method,
		}
	}
	return m
}

func sanitiseVaults(vaults []config.VaultConfig) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(vaults))
	for _, v := range vaults {
		out = append(out, sanitiseVault(v))
	}
	return out
}
