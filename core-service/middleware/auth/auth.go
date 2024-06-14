package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"
	"github.com/lu1a/lcaas/core-service/db"
	"github.com/lu1a/lcaas/core-service/types"
)

func AuthMiddleware(next http.Handler, adminDB *sqlx.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// the only routes that don't need to be authed
		if strings.HasPrefix(r.URL.Path, "/static") || r.URL.Path == "/login" || r.URL.Path == "/" || r.URL.Path == "/api/auth/oauth/redirect" {
			next.ServeHTTP(w, r)
			return
		}

		apiToken, err := GetAPIToken(r)
		if err != nil || apiToken == "" {
			sessionToken, sessionTokenErr := GetSessionToken(r)
			if sessionTokenErr != nil {
				switch {
				case errors.Is(sessionTokenErr, http.ErrNoCookie):
					w.Header().Set("Content-Type", "text/html")
					http.Redirect(w, r, "/login", http.StatusSeeOther)
				default:
					log.Error("Error getting session token", "error", sessionTokenErr)
					http.Error(w, "server error", http.StatusInternalServerError)
				}
				return
			}
			account, dbErr := db.GetAccountBySession(adminDB, sessionToken)
			if dbErr != nil {
				log.Error("Error getting session from session token", "error", dbErr)
				http.Error(w, "Not authorised, go log in", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), types.Account{}, account)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		account, dbErr := db.GetAccountByAPIToken(adminDB, apiToken)
		if dbErr != nil || account.AccountID == 0 {
			log.Error("Error getting API token from DB", "error", dbErr)
			http.Error(w, "Not authorised, wrong token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), types.Account{}, account)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetAPIToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("Authorization header is missing")
	}

	// Check if the Authorization header has the Bearer scheme
	authParts := strings.Split(authHeader, " ")
	if len(authParts) != 2 || authParts[0] != "Bearer" {
		return "", fmt.Errorf("Invalid authorization header format")
	}

	token := authParts[1]
	return token, nil
}

func GetSessionToken(r *http.Request) (string, error) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}
