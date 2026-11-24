package auth

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Informations de l'utilisateur connecté
type CurrentUser struct {
	ID    int64
	Email string
	Role  string
}

type userContextKey struct{}

// Récupérer l'utilisateur depuis une requête
func UserFromContext(r *http.Request) (CurrentUser, bool) {
	user, ok := r.Context().Value(userContextKey{}).(CurrentUser)
	return user, ok
}

// Vérifier qu'un utilisateur est connecté
func RequireAuth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		parts := strings.Fields(r.Header.Get("Authorization"))

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "Authentification requise", 401)
			return
		}

		token := parts[1]

		// Vérifier le format du token
		if len(token) != 64 {
			http.Error(w, "Token invalide", 401)
			return
		}

		if _, err := hex.DecodeString(token); err != nil {
			http.Error(w, "Token invalide", 401)
			return
		}

		var user CurrentUser
		var expiration string

		// Chercher la session dans SQLite
		err := db.QueryRowContext(r.Context(), `
            SELECT u.id, u.email, u.role, s.expires_at
            FROM sessions s
            JOIN users u ON u.id = s.user_id
            WHERE s.token_hash = ?
              AND u.confirmed = 1
        `, sessionHash(token)).Scan(
			&user.ID,
			&user.Email,
			&user.Role,
			&expiration,
		)

		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Session invalide", 401)
			return
		}

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		expiresAt, err := time.Parse(time.RFC3339, expiration)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		if !time.Now().Before(expiresAt) {
			http.Error(w, "Session expirée", 401)
			return
		}

		// Ajouter l'utilisateur à la requête
		ctx := context.WithValue(
			r.Context(),
			userContextKey{},
			user,
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Vérifier que l'utilisateur est administrateur
func RequireAdmin(db *sql.DB, next http.Handler) http.Handler {
	return RequireAuth(db, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {

			user, ok := UserFromContext(r)

			if !ok {
				http.Error(w, "Authentification requise", 401)
				return
			}

			if user.Role != "admin" {
				http.Error(w, "Accès interdit", 403)
				return
			}

			next.ServeHTTP(w, r)
		},
	))
}
