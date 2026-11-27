package users

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"ecommerce-go/internal/auth"
)

// Les champs sont facultatifs.
// On peut modifier uniquement l'email ou le rôle.
type UpdateUserRequest struct {
	Email *string `json:"email"`
	Role  *string `json:"role"`
}

func updateUser(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 1. Récupérer l'ID dans l'URL
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "Identifiant invalide", 400)
			return
		}

		// 2. Lire le JSON
		var input UpdateUserRequest

		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "JSON invalide", 400)
			return
		}

		// Refuser plusieurs objets JSON
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(w, "JSON invalide", 400)
			return
		}

		if input.Email == nil && input.Role == nil {
			http.Error(w, "Aucune modification demandee", 400)
			return
		}

		// 3. Démarrer une transaction
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}
		defer tx.Rollback()

		// 4. Récupérer les informations existantes
		var email, role string

		err = tx.QueryRowContext(r.Context(), `
			SELECT email, role
			FROM users
			WHERE id = ?
		`, id).Scan(&email, &role)

		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Utilisateur introuvable", 404)
			return
		}

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// 5. Vérifier le nouvel email
		if input.Email != nil {
			newEmail := strings.ToLower(
				strings.TrimSpace(*input.Email),
			)

			address, err := mail.ParseAddress(newEmail)
			if err != nil || address.Address != newEmail {
				http.Error(w, "Email invalide", 400)
				return
			}

			email = newEmail
		}

		// 6. Vérifier le nouveau rôle
		if input.Role != nil {
			if *input.Role != "client" && *input.Role != "admin" {
				http.Error(w, "Role invalide", 400)
				return
			}
		}

		newRole := role
		if input.Role != nil {
			newRole = *input.Role
		}

		// 7. Empêcher l'admin de retirer son propre rôle
		currentUser, ok := auth.UserFromContext(r)
		if !ok {
			http.Error(w, "Authentification requise", 401)
			return
		}

		if currentUser.ID == id && newRole != "admin" {
			http.Error(w, "Impossible de retirer son propre role admin", 403)
			return
		}

		// 8. Protéger le dernier administrateur
		if role == "admin" && newRole == "client" {
			var adminCount int

			err := tx.QueryRowContext(r.Context(), `
				SELECT COUNT(*)
				FROM users
				WHERE role = 'admin'
			`).Scan(&adminCount)

			if err != nil {
				http.Error(w, "Erreur serveur", 500)
				return
			}

			if adminCount <= 1 {
				http.Error(w, "Impossible de retirer le dernier admin", 409)
				return
			}
		}

		// 9. Modifier l'utilisateur dans SQLite
		// OR IGNORE évite une erreur SQL si l'email existe déjà
		result, err := tx.ExecContext(r.Context(), `
			UPDATE OR IGNORE users
			SET email = ?, role = ?
			WHERE id = ?
		`, email, newRole, id)

		if err != nil {
			http.Error(w, "Erreur modification utilisateur", 500)
			return
		}

		affected, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		if affected == 0 {
			http.Error(w, "Modification refusee : email deja utilise ou conflit", 409)
			return
		}

		// 10. Enregistrer définitivement
		if err := tx.Commit(); err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      id,
			"email":   email,
			"role":    newRole,
			"message": "Utilisateur modifie avec succes",
		})
	})
}
