package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/pacorreia/vaults-syncer/auth"
	"github.com/pacorreia/vaults-syncer/storage"
)

// UsersHandler handles user management (admin-only).
type UsersHandler struct {
	authSvc *auth.Service
	logger  *slog.Logger
}

// NewUsersHandler creates a UsersHandler.
func NewUsersHandler(authSvc *auth.Service, logger *slog.Logger) *UsersHandler {
	return &UsersHandler{authSvc: authSvc, logger: logger}
}

// createUserRequest is the payload for POST /api/users.
type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// updateUserRequest is the payload for PUT /api/users/{id}.
type updateUserRequest struct {
	Password string `json:"password,omitempty"`
	Role     string `json:"role,omitempty"`
}

// ListUsers returns all users.
// GET /api/users
func (h *UsersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.authSvc.ListUsers()
	if err != nil {
		h.logger.Error("list users failed", slog.String("error", err.Error()))
		jsonError(w, "failed to list users", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"users": users})
}

// CreateUser adds a new user account.
// POST /api/users
func (h *UsersHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		jsonError(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	role := req.Role
	if role == "" {
		role = storage.RoleUser
	}
	if role != storage.RoleAdmin && role != storage.RoleUser {
		jsonError(w, "role must be 'admin' or 'user'", http.StatusBadRequest)
		return
	}

	user, err := h.authSvc.CreateUser(req.Username, req.Password, role)
	if err != nil {
		h.logger.Error("create user failed", slog.String("error", err.Error()))
		jsonError(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	h.logger.Info("user created", slog.String("username", user.Username))
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, user)
}

// UpdateUser modifies an existing user.
// PUT /api/users/{user_id}
func (h *UsersHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r)
	if err != nil {
		jsonError(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Password != "" {
		if len(req.Password) < 8 {
			jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		if err := h.authSvc.ChangePassword(userID, req.Password); err != nil {
			h.logger.Error("change password failed", slog.String("error", err.Error()))
			jsonError(w, "failed to update password", http.StatusInternalServerError)
			return
		}
	}

	if req.Role != "" {
		if req.Role != storage.RoleAdmin && req.Role != storage.RoleUser {
			jsonError(w, "role must be 'admin' or 'user'", http.StatusBadRequest)
			return
		}
		if err := h.authSvc.ChangeRole(userID, req.Role); err != nil {
			h.logger.Error("change role failed", slog.String("error", err.Error()))
			jsonError(w, "failed to update role", http.StatusInternalServerError)
			return
		}
	}

	h.logger.Info("user updated", slog.Int64("user_id", userID))
	jsonOK(w, map[string]string{"status": "updated"})
}

// DeleteUserAccount removes a user account.
// DELETE /api/users/{user_id}
func (h *UsersHandler) DeleteUserAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r)
	if err != nil {
		jsonError(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	// Prevent self-deletion.
	caller := auth.UserFromContext(r.Context())
	if caller != nil && caller.ID == userID {
		jsonError(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}

	if err := h.authSvc.DeleteUser(userID); err != nil {
		h.logger.Error("delete user failed", slog.String("error", err.Error()))
		jsonError(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	h.logger.Info("user deleted", slog.Int64("user_id", userID))
	jsonOK(w, map[string]string{"status": "deleted"})
}

func parseUserID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("user_id"), 10, 64)
}
