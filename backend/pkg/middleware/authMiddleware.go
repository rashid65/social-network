package middleware

import (
	"context"
	"log"
	"net/http"
	"social-network/pkg/auth"
	"social-network/pkg/utils"
)

// AuthMiddleware verifies that the user is authenticated
// before allowing access to protected routes.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		// 1. Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:] // remove "Bearer " prefix
		}

		// 2. If not found, check query parameter (for WebSocket connections)
		if tokenString == "" {
			tokenString = r.URL.Query().Get("token")
		}

		// 3. If still not found, check cookie (optional, if you use cookies for auth)
		if tokenString == "" {
			cookie, err := r.Cookie("auth_token")
			if err == nil {
				tokenString = cookie.Value
			}
		}

		if tokenString == "" {
			utils.WriteErrorJSON(w, "No token provided", http.StatusUnauthorized)
			return
		}

		// Validate token and user ID
		userID, err := auth.ValidateToken(tokenString)
		if err != nil {
			log.Printf("Error validating token: %v", err)
			utils.WriteErrorJSON(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// add userID to request context
		ctx := context.WithValue(r.Context(), "userID", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
