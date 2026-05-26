package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/pacorreia/vaults-syncer/auth"
	"github.com/pacorreia/vaults-syncer/storage"
)

// SetupHandler handles the first-run setup wizard.
type SetupHandler struct {
	store   *storage.Store
	authSvc *auth.Service
	logger  *slog.Logger
	// onSetupComplete is called after setup succeeds so the caller can reload
	// the application configuration from the database.
	onSetupComplete func()
}

// NewSetupHandler creates a SetupHandler.
func NewSetupHandler(store *storage.Store, authSvc *auth.Service, logger *slog.Logger, onSetupComplete func()) *SetupHandler {
	return &SetupHandler{
		store:           store,
		authSvc:         authSvc,
		logger:          logger,
		onSetupComplete: onSetupComplete,
	}
}

// setupStatusResponse is returned by GET /api/setup.
type setupStatusResponse struct {
	Complete bool `json:"complete"`
}

// setupRequest is the payload for POST /api/setup.
type setupRequest struct {
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
}

// GetSetupStatus reports whether first-run setup has been completed.
func (h *SetupHandler) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	complete, err := h.store.IsSetupComplete()
	if err != nil {
		h.logger.Error("setup status check failed", slog.String("error", err.Error()))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, setupStatusResponse{Complete: complete})
}

// CompleteSetup performs the first-run setup: creates the admin account and
// marks setup as complete.
func (h *SetupHandler) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	complete, err := h.store.IsSetupComplete()
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if complete {
		jsonError(w, "setup already completed", http.StatusConflict)
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AdminUsername == "" || req.AdminPassword == "" {
		jsonError(w, "admin_username and admin_password are required", http.StatusBadRequest)
		return
	}
	if len(req.AdminPassword) < 8 {
		jsonError(w, "admin_password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	if err := h.authSvc.SetupAdmin(req.AdminUsername, req.AdminPassword); err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			jsonError(w, "username already exists", http.StatusConflict)
		} else {
			h.logger.Error("setup admin creation failed", slog.String("error", err.Error()))
			jsonError(w, "failed to create admin account", http.StatusInternalServerError)
		}
		return
	}

	if err := h.store.MarkSetupComplete(); err != nil {
		h.logger.Error("failed to mark setup complete", slog.String("error", err.Error()))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("first-run setup completed", slog.String("admin", req.AdminUsername))

	if h.onSetupComplete != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("onSetupComplete panicked", slog.Any("panic", r))
				}
			}()
			h.onSetupComplete()
		}()
	}

	jsonOK(w, map[string]string{"status": "setup complete"})
}
