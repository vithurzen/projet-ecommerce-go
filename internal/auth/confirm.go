package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Créer un code aléatoire et son hash
func generateConfirmationCode() (string, string, error) {
	bytes := make([]byte, 8)

	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	code := strings.ToUpper(hex.EncodeToString(bytes))
	return code, hashConfirmationCode(code), nil
}

// Transformer le code en empreinte SHA-256
func hashConfirmationCode(code string) string {
	hash := sha256.Sum256([]byte(strings.ToUpper(code)))
	return hex.EncodeToString(hash[:])
}

// Route de confirmation du compte
func confirmHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}

		r.Body = http.MaxBytesReader(w, r.Body, 4096)

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "JSON invalide", http.StatusBadRequest)
			return
		}

		email := strings.ToLower(strings.TrimSpace(input.Email))
		code := strings.ToUpper(strings.TrimSpace(input.Code))

		if len(code) != 16 {
			http.Error(w, "Code invalide", http.StatusBadRequest)
			return
		}

		if _, err := hex.DecodeString(code); err != nil {
			http.Error(w, "Code invalide", http.StatusBadRequest)
			return
		}

		// Démarrer une transaction SQL
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}
		defer tx.Rollback()

		var userID int64
		var storedHash, expiration string

		err = tx.QueryRowContext(r.Context(), `
            SELECT u.id, c.code_hash, c.expires_at
            FROM users u
            JOIN confirmation_codes c ON c.user_id = u.id
            WHERE u.email = ? AND u.confirmed = 0
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

		receivedHash := hashConfirmationCode(code)

		if time.Now().UTC().After(expiresAt) ||
			subtle.ConstantTimeCompare(
				[]byte(receivedHash),
				[]byte(storedHash),
			) != 1 {
			http.Error(w, "Code invalide ou expiré", 400)
			return
		}

		// Confirmer le compte utilisateur
		result, err := tx.ExecContext(r.Context(),
			"UPDATE users SET confirmed = 1 WHERE id = ? AND confirmed = 0",
			userID,
		)
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			http.Error(w, "Confirmation impossible", 400)
			return
		}

		// Supprimer le code après utilisation
		_, err = tx.ExecContext(r.Context(),
			"DELETE FROM confirmation_codes WHERE user_id = ?",
			userID,
		)
		if err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Erreur serveur", 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Compte confirmé avec succès",
		})
	}
}
