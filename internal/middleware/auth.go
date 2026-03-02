package middleware

import (
	"errors"
	"net/http"
	"context"
	"database/sql"

	"github.com/Cryptowave2-0/webhosting-goapi/api"
	"github.com/Cryptowave2-0/webhosting-goapi/internal/auth"
)

type contextKey string

const UserIDKey contextKey = "userID"

var UnAuthorizedError = errors.New("Invalid username or token.")

func AuthMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_token")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userID, err := auth.GetUserFromSession(cookie.Value)
			if err != nil {
				api.RequestErrorHandler(w, UnAuthorizedError)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
