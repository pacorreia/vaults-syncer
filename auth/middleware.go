package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/pacorreia/vaults-syncer/storage"
)

// contextKey is the unexported type for context values stored by this package.
type contextKey int

const (
	userContextKey contextKey = iota
)

// UserFromContext retrieves the authenticated user from the request context.
// Returns nil if the request is not authenticated.
func UserFromContext(ctx context.Context) *storage.User {
	u, _ := ctx.Value(userContextKey).(*storage.User)
	return u
}

// RequireAuth is an HTTP middleware that enforces a valid session token.
// The token is read from the Authorization header ("Bearer <token>") or
// the "session_token" cookie. If the session is invalid the request is
// rejected with 401 Unauthorized.
func RequireAuth(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			user, err := svc.ValidateSession(token)
			if err != nil || user == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin is an HTTP middleware that enforces admin role.
// Must be used after RequireAuth.
func RequireAdmin(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || user.Role != storage.RoleAdmin {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ContextWithUser returns a new context with user stored as a value.
// This is exported for use in tests.
func ContextWithUser(ctx context.Context, user *storage.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// ExtractBearerToken reads the bearer token from the Authorization header or a cookie.
// It is exported so that API handlers outside the auth package can access the token
// for explicit logout operations.
func ExtractBearerToken(r *http.Request) string {
	return extractToken(r)
}

// extractToken reads the bearer token from the Authorization header or a cookie.
func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if cookie, err := r.Cookie("session_token"); err == nil {
		return cookie.Value
	}
	return ""
}
