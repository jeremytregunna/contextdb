package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const AuthContextKey contextKey = "auth_context"

// AuthMiddleware provides authentication for HTTP requests
func AuthMiddleware(authManager *AuthManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var authContext *AuthContext

			// Check if authentication is required
			if !authManager.IsAuthRequired() {
				// Auth disabled, use anonymous context
				authContext = authManager.GetAnonymousContext()
			} else {
				// Try to authenticate
				apiKey := extractAPIKey(r)
				if apiKey != "" {
					ctx, err := authManager.ValidateAPIKey(apiKey)
					if err != nil {
						writeAuthError(w, "Invalid API key", http.StatusUnauthorized)
						return
					}
					authContext = ctx
				} else {
					writeAuthError(w, "API key required", http.StatusUnauthorized)
					return
				}
			}

			// Add auth context to request
			ctx := context.WithValue(r.Context(), AuthContextKey, authContext)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission creates middleware that checks for specific permissions
func RequirePermission(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authContext := GetAuthContext(r.Context())
			if authContext == nil {
				writeAuthError(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			if !authContext.HasPermission(perm) {
				writeAuthError(w, "Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetAuthContext retrieves the auth context from request context
func GetAuthContext(ctx context.Context) *AuthContext {
	if authContext, ok := ctx.Value(AuthContextKey).(*AuthContext); ok {
		return authContext
	}
	return nil
}

// extractAPIKey gets API key from Authorization header or query parameter
func extractAPIKey(r *http.Request) string {
	// Try Authorization header first (Bearer token)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
		if strings.HasPrefix(authHeader, "ApiKey ") {
			return strings.TrimPrefix(authHeader, "ApiKey ")
		}
	}

	// Try query parameter as fallback
	return r.URL.Query().Get("api_key")
}

func writeAuthError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write([]byte(`{"error":"` + message + `"}`))
}
