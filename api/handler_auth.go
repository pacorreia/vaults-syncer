package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/pacorreia/vaults-syncer/auth"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authSvc *auth.Service
	logger  *slog.Logger
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(authSvc *auth.Service, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, logger: logger}
}

// loginRequest is the payload for POST /api/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login authenticates a user and returns a session token.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		jsonError(w, "username and password are required", http.StatusBadRequest)
		return
	}

	token, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			jsonError(w, "invalid credentials", http.StatusUnauthorized)
		} else {
			h.logger.Error("login error", slog.String("error", err.Error()))
			jsonError(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	h.logger.Info("user logged in", slog.String("username", req.Username))
	jsonOK(w, map[string]string{"token": token})
}

// Logout invalidates the current session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := auth.ExtractBearerToken(r)
	if token != "" {
		if err := h.authSvc.Logout(token); err != nil {
			h.logger.Error("logout error", slog.String("error", err.Error()))
		}
	}
	jsonOK(w, map[string]string{"status": "logged out"})
}

// Me returns the current authenticated user's info.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	jsonOK(w, map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"role":       user.Role,
		"created_at": user.CreatedAt,
	})
}
