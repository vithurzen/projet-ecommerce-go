package auth

import (
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type ResetPasswordRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

func resetPasswordHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input ResetPasswordRequest

		r.Body = http.MaxBytesReader(w, r.Body, 4096)

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "JSON invalide", 400)
			return
		}

		email := strings.ToLower(strings.TrimSpace(input.Email))
		code := strings.ToUpper(strings.TrimSpace(input.Code))

		// Vérifier le format du code
		if len(code) != 32 {
			http.Error(w, "Code invalide", 400)
			return
		}

		if _, err := hex.DecodeString(code); err != nil {
			http.Error(w, "Code invalide", 400)
			return
		}

		// Vérifier le nouveau mot de passe
		if len(input.NewPassword) < 8 ||
			len(input.NewPassword) > 72 {
			http.Error(w, "Mot de passe invalide", 400)
			return
		}

		// Hacher le nouveau mot de passe
		passwordHash, err := bcrypt.GenerateFromPassword(
			[]byte(input.NewPassword),
			bcrypt.DefaultCost,
		)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Démarrer une transaction
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}
		defer tx.Rollback()

		var userID int64
		var storedHash, expiration string

		// Récupérer le code stocké
		err = tx.QueryRowContext(r.Context(), `
            SELECT u.id, p.code_hash, p.expires_at
            FROM users u
            JOIN password_reset_codes p
                ON p.user_id = u.id
            WHERE u.email = ?
        `, email).Scan(&userID, &storedHash, &expiration)

		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Code invalide ou expiré", 400)
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

		// Comparer les empreintes et l'expiration
		receivedHash := hashResetCode(code)

		if !time.Now().Before(expiresAt) ||
			subtle.ConstantTimeCompare(
				[]byte(receivedHash),
				[]byte(storedHash),
			) != 1 {
			http.Error(w, "Code invalide ou expiré", 400)
			return
		}

		// Enregistrer le nouveau mot de passe
		_, err = tx.ExecContext(r.Context(), `
            UPDATE users
            SET password_hash = ?
            WHERE id = ?
        `, string(passwordHash), userID)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Supprimer le code utilisé
		_, err = tx.ExecContext(r.Context(), `
            DELETE FROM password_reset_codes
            WHERE user_id = ?
        `, userID)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Déconnecter les anciennes sessions
		_, err = tx.ExecContext(r.Context(), `
            DELETE FROM sessions
            WHERE user_id = ?
        `, userID)

		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		// Valider toutes les opérations
		if err := tx.Commit(); err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Mot de passe réinitialisé avec succès",
		})
	}
}
